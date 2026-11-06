//go:build windows

package main

import (
	"encoding/base64"
	"errors"
	"syscall"
	"unsafe"
)

// 用 Windows DPAPI 加密本机敏感字段（API Key）。
// 密文只能在同一台机器、同一 Windows 账户下解开，拷走 secret.dat 也读不出 Key。
var (
	crypt32          = syscall.NewLazyDLL("crypt32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procProtect      = crypt32.NewProc("CryptProtectData")
	procUnprotect    = crypt32.NewProc("CryptUnprotectData")
	procLocalFree    = kernel32.NewProc("LocalFree")
	errProtectFailed = errors.New("DPAPI 加密失败")
	errUnprotect     = errors.New("DPAPI 解密失败")
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

const cryptProtectUIForbidden = 0x1

func protectSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	inBytes := []byte(plain)
	in := dataBlob{cbData: uint32(len(inBytes)), pbData: &inBytes[0]}
	var out dataBlob
	r, _, _ := procProtect.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return "", errProtectFailed
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	raw := unsafe.Slice(out.pbData, out.cbData)
	return base64.StdEncoding.EncodeToString(raw), nil
}

func unprotectSecret(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil || len(raw) == 0 {
		return "", errUnprotect
	}
	in := dataBlob{cbData: uint32(len(raw)), pbData: &raw[0]}
	var out dataBlob
	r, _, _ := procUnprotect.Call(
		uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, cryptProtectUIForbidden,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return "", errUnprotect
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	plain := unsafe.Slice(out.pbData, out.cbData)
	return string(plain), nil
}
