package main

import (
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 直播状态探测：只通过「伴侣进程是否存在 + 它当前的上传速度 + 最近有没有收到事件」推断，
// 不读伴侣内部数据、不逆向它的协议。
type liveProbe struct {
	mu      sync.Mutex
	checked time.Time
	running bool
	title   string
	rate    int64 // 每秒写+其它字节，近似上传速度
}

var liveCache liveProbe

const liveProbeScript = `$OutputEncoding=[Console]::OutputEncoding=[Text.Encoding]::UTF8; ` +
	`$p=Get-Process -Name 直播伴侣 -ErrorAction SilentlyContinue | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1; ` +
	`if(-not $p){ 'NONE'; exit }; ` +
	`function Bytes { $rows=Get-CimInstance Win32_PerfRawData_PerfProc_Process -ErrorAction SilentlyContinue | Where-Object { $_.Name -like '直播伴侣*' }; ` +
	`$sum=0; foreach($r in $rows){ $sum += [int64]$r.IOWriteBytesPersec + [int64]$r.IOOtherBytesPersec }; return $sum }; ` +
	`$a=Bytes; Start-Sleep -Milliseconds 1500; $b=Bytes; $rate=[int64](($b-$a)/1.5); ` +
	`$p.Id.ToString() + '|' + $p.MainWindowTitle + '|' + $rate`

// probeLive 5 秒内复用，避免每次刷新都起 PowerShell
func probeLive() (bool, string, int64) {
	liveCache.mu.Lock()
	defer liveCache.mu.Unlock()
	if time.Since(liveCache.checked) < 5*time.Second {
		return liveCache.running, liveCache.title, liveCache.rate
	}
	liveCache.checked = time.Now()
	liveCache.running = false
	liveCache.title = ""
	liveCache.rate = 0
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden",
		"-Command", liveProbeScript).Output()
	if err != nil {
		return false, "", 0
	}
	line := strings.TrimSpace(string(out))
	if line == "" || line == "NONE" {
		return false, "", 0
	}
	parts := strings.SplitN(line, "|", 3)
	if len(parts) < 3 {
		return true, "", 0
	}
	if _, err := strconv.Atoi(strings.TrimSpace(parts[0])); err != nil {
		return true, "", 0
	}
	rate, _ := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
	return true, strings.TrimSpace(parts[1]), rate
}
