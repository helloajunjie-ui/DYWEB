package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// registerIngest 数据入口：HTTP POST /push、HTTP GET /push?data=、WS /ws/ingest
func (a *App) registerIngest(mux *http.ServeMux) {
	mux.HandleFunc("/push", func(w http.ResponseWriter, r *http.Request) {
		if !a.mods.IsEnabled("http_push") {
			writeJSON(w, 200, map[string]any{"ok": false, "msg": "HTTP 推送接入已关闭"})
			return
		}

		var payload []byte
		if r.Method == http.MethodGet {
			d := r.URL.Query().Get("data")
			if d == "" {
				writeJSON(w, 400, map[string]any{"ok": false, "msg": "缺少 data 参数"})
				return
			}
			if unescaped, err := url.QueryUnescape(d); err == nil {
				d = unescaped
			}
			payload = []byte(d)
		} else {
			body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
			if err != nil {
				writeJSON(w, 400, map[string]any{"ok": false, "msg": "读取 body 失败"})
				return
			}
			payload = body
		}

		n, err := a.ingestPayload(payload)
		if err != nil {
			writeJSON(w, 400, map[string]any{"ok": false, "msg": err.Error(), "count": n})
			return
		}
		if n > 0 {
			atomic.StoreInt64(&a.lastIngest, time.Now().Unix())
		}
		writeJSON(w, 200, map[string]any{"ok": true, "count": n})
	})

	mux.HandleFunc("/ws/ingest", func(w http.ResponseWriter, r *http.Request) {
		if !a.mods.IsEnabled("ws_ingest") {
			http.Error(w, "ws_ingest disabled", http.StatusServiceUnavailable)
			return
		}
		conn, err := a.hub.upgr.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetReadLimit(1 << 20)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if n, err := a.ingestPayload(msg); err == nil && n > 0 {
				atomic.StoreInt64(&a.lastIngest, time.Now().Unix())
			}
		}
	})
}

// ingestPayload 支持单对象、数组、{events:[...]}
func (a *App) ingestPayload(payload []byte) (int, error) {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return 0, errString("空 payload")
	}

	if strings.HasPrefix(trimmed, "[") {
		var arr []map[string]any
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return 0, err
		}
		for _, item := range arr {
			a.HandleRaw(item)
		}
		return len(arr), nil
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return 0, err
	}
	// 兼容 {"events":[...]} / {"data":[...]} 批量包装
	for _, key := range []string{"events", "list", "messages"} {
		if raw, ok := obj[key]; ok {
			if arr, ok := raw.([]any); ok {
				count := 0
				for _, it := range arr {
					if m, ok := it.(map[string]any); ok {
						a.HandleRaw(m)
						count++
					}
				}
				return count, nil
			}
		}
	}
	a.HandleRaw(obj)
	return 1, nil
}

type errString string

func (e errString) Error() string { return string(e) }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
