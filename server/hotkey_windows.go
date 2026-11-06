//go:build windows

package main

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32       = syscall.NewLazyDLL("user32.dll")
	procSendInput = user32.NewProc("SendInput")
)

const (
	inputKeyboard = 1
	keyeventfKeyUp = 0x0002
)

type keybdInput struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type input struct {
	Type uint32
	_    uint32
	Ki   keybdInput
	_    [8]byte
}

var vkMap = map[string]uint16{
	"ctrl": 0x11, "control": 0x11, "alt": 0x12, "shift": 0x10, "win": 0x5B,
	"space": 0x20, "enter": 0x0D, "return": 0x0D, "esc": 0x1B, "escape": 0x1B,
	"tab": 0x09, "backspace": 0x08, "delete": 0x2E, "del": 0x2E,
	"up": 0x26, "down": 0x28, "left": 0x25, "right": 0x27,
	"home": 0x24, "end": 0x23, "pageup": 0x21, "pagedown": 0x22,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74, "f6": 0x75,
	"f7": 0x76, "f8": 0x77, "f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
	"-": 0xBD, "=": 0xBB, ",": 0xBC, ".": 0xBE, "/": 0xBF, ";": 0xBA, ":": 0xBA,
	"'": 0xDE, "[": 0xDB, "]": 0xDD, "\\": 0xDC,
}

func init() {
	for c := 'a'; c <= 'z'; c++ {
		vkMap[string(c)] = uint16(c - 32)
	}
	for c := '0'; c <= '9'; c++ {
		vkMap[string(c)] = uint16(c)
	}
}

func vkOf(name string) (uint16, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if v, ok := vkMap[n]; ok {
		return v, true
	}
	return 0, false
}

// pressHotkey 模拟按键，如 "ctrl+space"、"f5"、"ctrl+shift+a"
func pressHotkey(spec string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return fmt.Errorf("按键为空")
	}
	parts := strings.Split(spec, "+")
	keys := make([]uint16, 0, len(parts))
	for _, p := range parts {
		vk, ok := vkOf(p)
		if !ok {
			return fmt.Errorf("无法识别的按键: %s", strings.TrimSpace(p))
		}
		keys = append(keys, vk)
	}

	var seq []input
	// 按下
	for _, vk := range keys {
		seq = append(seq, input{Type: inputKeyboard, Ki: keybdInput{Vk: vk}})
	}
	// 松开（倒序）
	for i := len(keys) - 1; i >= 0; i-- {
		seq = append(seq, input{Type: inputKeyboard, Ki: keybdInput{Vk: keys[i], Flags: keyeventfKeyUp}})
	}

	for _, in := range seq {
		ret, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
		if ret == 0 {
			return fmt.Errorf("SendInput 失败: %v", err)
		}
		time.Sleep(8 * time.Millisecond)
	}
	return nil
}
