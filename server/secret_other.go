//go:build !windows

package main

import "errors"

// 非 Windows 环境没有 DPAPI：明确报错，不退回明文落盘。
func protectSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	return "", errors.New("当前系统不支持 DPAPI 加密，拒绝把 Key 明文写入磁盘")
}

func unprotectSecret(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	return "", errors.New("当前系统不支持 DPAPI 解密")
}
