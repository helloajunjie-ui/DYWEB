package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TTS 本机语音播报（Windows SAPI），串行队列避免叠音
type TTS struct {
	mu       sync.Mutex
	cfg      func() TTSConfig
	ch       chan string
	lastText string
	lastTS   int64
	tmpDir   string
	spoken   int64
	dropped  int64
	browser  bool // 是否同时让 overlay 朗读
	onBrowser func(string)
}

func newTTS(cfg func() TTSConfig, tmpDir string) *TTS {
	t := &TTS{cfg: cfg, ch: make(chan string, 200), tmpDir: tmpDir}
	go t.worker()
	return t
}

// Speak 入队播报；同文案 2 秒内不重复
func (t *TTS) Speak(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	c := t.cfg()
	if !c.Enabled {
		return
	}

	t.mu.Lock()
	if text == t.lastText && time.Now().UnixMilli()-t.lastTS < 2000 {
		t.mu.Unlock()
		return
	}
	t.lastText = text
	t.lastTS = time.Now().UnixMilli()
	t.mu.Unlock()

	mode := c.Mode
	if mode == "browser" || mode == "both" {
		if t.onBrowser != nil {
			t.onBrowser(text)
		}
	}
	if mode == "browser" {
		return
	}

	select {
	case t.ch <- text:
	default:
		t.mu.Lock()
		t.dropped++
		t.mu.Unlock()
	}
}

func (t *TTS) worker() {
	for text := range t.ch {
		t.speakLocal(text)
	}
}

func (t *TTS) speakLocal(text string) {
	c := t.cfg()
	tmp := filepath.Join(t.tmpDir, "tts_"+strconv.FormatInt(time.Now().UnixNano(), 10)+".txt")
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return
	}
	defer os.Remove(tmp)

	voice := strings.ReplaceAll(c.Voice, "'", "''")
	voiceLine := ""
	if c.Voice != "" {
		voiceLine = "try { $s.SelectVoice('" + voice + "') } catch {}"
	}

	script := "$ErrorActionPreference='Stop'; " +
		"Add-Type -AssemblyName System.Speech; " +
		"$s = New-Object System.Speech.Synthesis.SpeechSynthesizer; " +
		"$s.Rate = " + strconv.Itoa(c.Rate) + "; " +
		"$s.Volume = " + strconv.Itoa(defaultInt(c.Volume, 90)) + "; " +
		voiceLine + "; " +
		"$s.Speak([IO.File]::ReadAllText('" + strings.ReplaceAll(tmp, "'", "''") + "', [Text.Encoding]::UTF8)); " +
		"$s.Dispose()"

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden",
		"-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logErr("tts", err)
		return
	}
	_ = out

	t.mu.Lock()
	t.spoken++
	t.mu.Unlock()
}

func (t *TTS) Stats() map[string]int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return map[string]int64{"spoken": t.spoken, "dropped": t.dropped}
}

// listVoices 列出本机可用音色
func listVoices() []string {
	script := "Add-Type -AssemblyName System.Speech; " +
		"$s = New-Object System.Speech.Synthesis.SpeechSynthesizer; " +
		"$s.GetInstalledVoices() | ForEach-Object { $_.VoiceInfo.Name }"
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden",
		"-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r", ""), "\n")
	res := []string{}
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			res = append(res, l)
		}
	}
	return res
}

func defaultInt(v, d int) int {
	if v == 0 {
		return d
	}
	return v
}
