package main

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
)

// normalize 把第三方工具的原始 JSON 映射为统一 Event
func normalize(raw map[string]any, cfg Config) Event {
	ev := Event{TS: time.Now().UnixMilli()}

	ev.User = lookupAny(raw, cfg.Mapping["user"])
	ev.UserID = lookupAny(raw, cfg.Mapping["userId"])
	ev.Avatar = lookupAny(raw, cfg.Mapping["avatar"])
	ev.Text = lookupAny(raw, cfg.Mapping["text"])
	ev.GiftName = lookupAny(raw, cfg.Mapping["giftName"])
	ev.GiftCount = int(parseNum(lookupAny(raw, cfg.Mapping["giftCount"])))
	if ev.GiftCount <= 0 {
		ev.GiftCount = 1
	}
	ev.GiftValue = parseNum(lookupAny(raw, cfg.Mapping["giftValue"]))
	ev.Level = int(parseNum(lookupAny(raw, cfg.Mapping["level"])))
	ev.GuardLevel = int(parseNum(lookupAny(raw, cfg.Mapping["guardLevel"])))
	ev.FansClubLevel = int(parseNum(lookupAny(raw, cfg.Mapping["fansClubLevel"])))
	if ts := parseNum(lookupAny(raw, cfg.Mapping["ts"])); ts > 0 {
		if ts < 1e11 { // 秒级时间戳
			ts *= 1000
		}
		ev.TS = int64(ts)
	}

	// 类型
	rawType := lookupAny(raw, cfg.Mapping["type"])
	ev.Type = resolveType(rawType, cfg.TypeAliases)
	if ev.Type == "" {
		switch {
		case ev.GiftName != "":
			ev.Type = "gift"
		case ev.Text != "":
			ev.Type = "danmaku"
		case ev.GuardLevel > 0:
			ev.Type = "guard"
		default:
			ev.Type = "unknown"
		}
	}

	ev.Total = ev.GiftValue * float64(ev.GiftCount)

	if b, err := json.Marshal(raw); err == nil {
		ev.Raw = string(b)
	}
	return ev
}

func resolveType(rawType string, aliases map[string][]string) string {
	t := strings.ToLower(strings.TrimSpace(rawType))
	if t == "" {
		return ""
	}
	for canonical, list := range aliases {
		for _, a := range list {
			if strings.ToLower(a) == t {
				return canonical
			}
		}
	}
	// 包含匹配（如 "WebcastGiftMessage" 大小写差异已在上面处理）
	for canonical, list := range aliases {
		for _, a := range list {
			al := strings.ToLower(a)
			if len(al) > 3 && strings.Contains(t, al) {
				return canonical
			}
		}
	}
	return ""
}

// lookupAny 按候选路径列表取值，返回字符串形式
func lookupAny(m map[string]any, paths []string) string {
	for _, p := range paths {
		if v, ok := lookupPath(m, p); ok {
			return stringify(v)
		}
	}
	return ""
}

func lookupPath(m map[string]any, path string) (any, bool) {
	segs := strings.Split(path, ".")
	var cur any = m
	for _, seg := range segs {
		mm, ok := asMap(cur)
		if !ok {
			return nil, false
		}
		v, found := getKey(mm, seg)
		if !found {
			// 首段找不到时，尝试下沉到 data 容器
			if d, ok := getKey(mm, "data"); ok {
				if dm, ok2 := asMap(d); ok2 {
					if v2, ok3 := getKey(dm, seg); ok3 {
						cur = v2
						continue
					}
				}
			}
			return nil, false
		}
		cur = v
	}
	// 容器类型不算命中（否则 "user" 会误取到整个 user 对象）
	switch cur.(type) {
	case map[string]any, []any:
		return nil, false
	}
	return cur, true
}

func getKey(m map[string]any, key string) (any, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	lower := strings.ToLower(key)
	for k, v := range m {
		if strings.ToLower(k) == lower {
			return v, true
		}
	}
	return nil, false
}

func asMap(v any) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		return t, true
	}
	return nil, false
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case int:
		return strconv.Itoa(t)
	case int8:
		return strconv.FormatInt(int64(t), 10)
	case int16:
		return strconv.FormatInt(int64(t), 10)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	default:
		return ""
	}
}

// parseNum 解析数字，兼容 "1.2万"、"+3"、带逗号等写法
func parseNum(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "+", "")
	s = strings.ReplaceAll(s, "抖币", "")
	mul := 1.0
	switch {
	case strings.HasSuffix(s, "万") || strings.HasSuffix(s, "w") || strings.HasSuffix(s, "W"):
		mul = 10000
		s = strings.TrimRight(s, "万wW")
	case strings.HasSuffix(s, "k") || strings.HasSuffix(s, "K"):
		mul = 1000
		s = strings.TrimRight(s, "kK")
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f * mul
}
