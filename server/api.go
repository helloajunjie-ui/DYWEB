package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *App) registerAPI(mux *http.ServeMux) {
	// 一次性把所有初始数据给控制台，避免首屏多次请求
	mux.HandleFunc("/api/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{
			"config":     a.cfg.Get(),
			"rules":      a.rules.List(),
			"modules":    a.mods.List(),
			"categories": a.mods.Categories(),
			"stats":      a.store.Snapshot(),
			"events":     a.store.Recent(60),
			"traces":     a.TraceList(60),
			"fans":       a.store.Fans(),
			"gifts":      a.gifts.List(),
			"scripts":    a.scripts.List(),
			"scriptCategories": scriptCategories,
			"products":   a.products.List(),
			"modes":      liveModes,
			"mode":       a.cfg.Get().Mode,
			"sfx":        a.sfxList(),
			"voices":     listVoices(),
			"info": map[string]any{
				"root":      a.root,
				"dataDir":   a.dataDir,
				"sfxDir":    a.sfxDir,
				"startedAt": a.started,
				"clients":   a.hub.Count(),
				"merge":     a.merger.MergeStats(),
				"tts":       a.tts.Stats(),
				"version":   "0.1.0",
			},
		})
	})

	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		n := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				n = parsed
			}
		}
		writeJSON(w, 200, map[string]any{"events": a.store.Recent(n)})
	})

	// ---- 主播端中控面板的玩法接口（/panel 页用）----
	mux.HandleFunc("/api/game", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "game": a.game.View()})
	})

	mux.HandleFunc("/api/game/wheel", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Theme string `json:"theme"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		v := a.StartWheel(body.Theme)
		writeJSON(w, 200, map[string]any{"ok": true, "game": v})
	})

	mux.HandleFunc("/api/game/guess", func(w http.ResponseWriter, r *http.Request) {
		v := a.StartGuess()
		writeJSON(w, 200, map[string]any{"ok": true, "game": v})
	})

	mux.HandleFunc("/api/game/result", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		a.WheelResult(body.Text)
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/game/traitor", func(w http.ResponseWriter, r *http.Request) {
		v := a.CatchTraitor()
		writeJSON(w, 200, map[string]any{"ok": true, "game": v})
	})

	mux.HandleFunc("/api/game/clear", func(w http.ResponseWriter, r *http.Request) {
		a.ClearGame()
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// 切换直播定位：只改一个字段，不动其它配置
	mux.HandleFunc("/api/mode", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Mode string `json:"mode"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		ok := false
		for _, m := range liveModes {
			if m.ID == body.Mode {
				ok = true
			}
		}
		if !ok {
			writeJSON(w, 400, map[string]any{"ok": false, "error": "没有这个直播定位"})
			return
		}
		cur := a.cfg.Get()
		cur.Mode = body.Mode
		if err := a.cfg.Set(cur); err != nil {
			writeJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "mode": body.Mode})
	})

	mux.HandleFunc("/api/scripts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body struct {
				Scripts []Script `json:"scripts"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "JSON 解析失败：" + err.Error()})
				return
			}
			if err := a.scripts.SetCustom(body.Scripts); err != nil {
				writeJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			writeJSON(w, 200, map[string]any{"ok": true, "count": len(a.scripts.List())})
		default:
			q := strings.TrimSpace(r.URL.Query().Get("q"))
			list := a.scripts.List()
			if q != "" {
				list = a.scripts.Relevant(q, 20)
			}
			writeJSON(w, 200, map[string]any{"ok": true, "categories": scriptCategories, "scripts": list})
		}
	})

	// 直接念一条话术（变量用示例值替换，主播自己也能改）
	mux.HandleFunc("/api/scripts/say", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
			User string `json:"user"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.TrimSpace(body.Text) == "" {
			writeJSON(w, 400, map[string]any{"ok": false, "error": "话术为空"})
			return
		}
		user := body.User
		if user == "" {
			user = "新来的朋友"
		}
		text := strings.ReplaceAll(body.Text, "{user}", user)
		text = strings.ReplaceAll(text, "{gift}", "礼物")
		text = strings.ReplaceAll(text, "{count}", "1")
		a.tts.Speak(text)
		a.addTrace(Trace{TS: time.Now().UnixMilli(), Rule: "话术库", Action: "tts", OK: true, Msg: "念：" + text, User: user})
		writeJSON(w, 200, map[string]any{"ok": true, "text": text})
	})

	// 让 AI 从话术库里挑最贴的一句，并结合当下场景改写
	mux.HandleFunc("/api/scripts/ask", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query string `json:"query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		q := strings.TrimSpace(body.Query)
		if q == "" {
			q = "现在直播有点冷场，帮我说一句留人的话"
		}
		refs := a.scripts.Relevant(q, 4)
		var sb strings.Builder
		sb.WriteString("以下是主播话术库里的参考话术，风格自然、口语、不要喊口号。\n")
		for i, s := range refs {
			sb.WriteString(itoa(i+1) + ". " + s.Title + "：" + s.Text + "\n")
		}
		sb.WriteString("请基于这些参考，结合我描述的场景，输出一句（最多 2 句）可以直接念出来的中文口播，不要解释、不要编号。")
		reply, err := a.llm.ChatWith(q, sb.String())
		if err != nil {
			writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error(), "refs": refs})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "text": reply, "refs": refs})
	})

	mux.HandleFunc("/api/products", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body struct {
				Products []Product `json:"products"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "JSON 解析失败：" + err.Error()})
				return
			}
			if err := a.products.Replace(body.Products); err != nil {
				writeJSON(w, 500, map[string]any{"ok": false, "error": err.Error()})
				return
			}
			writeJSON(w, 200, map[string]any{"ok": true, "count": len(a.products.List())})
		default:
			writeJSON(w, 200, map[string]any{"ok": true, "products": a.products.List()})
		}
	})

	// 按商品资料生成一段直播推荐话术（两步：算账 + 帮避坑）
	mux.HandleFunc("/api/products/pitch", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID    string `json:"id"`
			Style string `json:"style"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prod, ok := a.products.Get(body.ID)
		if !ok {
			writeJSON(w, 200, map[string]any{"ok": false, "error": "没找到这个商品"})
			return
		}
		style := body.Style
		if style == "" {
			style = "算账式：把总价拆到每天多少钱；再帮不适合的人避坑，降低退货"
		}
		prompt := "请为下面这个商品写一段直播间口播推荐，要求：\n" +
			"1) " + style + "\n" +
			"2) 口语、像朋友推荐，2~4 句，不要夸张承诺、不要绝对化用语（最、第一、绝对）\n" +
			"3) 一定要提一句谁不适合买，帮他省钱\n" +
			"4) 直接给口播正文，不要标题、不要解释\n\n" + prod.Brief()
		reply, err := a.llm.ChatWith(prompt, "")
		if err != nil {
			writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "text": reply, "product": prod.Name})
	})

	mux.HandleFunc("/api/gifts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body struct {
				Gifts []Gift `json:"gifts"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "JSON 解析失败：" + err.Error()})
				return
			}
			a.gifts.Replace(body.Gifts)
			writeJSON(w, 200, map[string]any{"ok": true, "count": len(body.Gifts)})
		default:
			writeJSON(w, 200, map[string]any{"gifts": a.gifts.List()})
		}
	})

	mux.HandleFunc("/api/traces", func(w http.ResponseWriter, r *http.Request) {
		n := 100
		if v := r.URL.Query().Get("limit"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil {
				n = parsed
			}
		}
		writeJSON(w, 200, map[string]any{"traces": a.TraceList(n)})
	})

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, a.store.Snapshot())
	})

	// 规则：整表读写
	mux.HandleFunc("/api/rules", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeJSON(w, 200, map[string]any{"rules": a.rules.List()})
			return
		}
		var body struct {
			Rules []Rule `json:"rules"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		if err := a.rules.Replace(body.Rules); err != nil {
			writeJSON(w, 500, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "count": len(body.Rules)})
	})

	// 观众画像
	mux.HandleFunc("/api/fans", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"fans": a.store.Fans()})
	})
	mux.HandleFunc("/api/fans/note", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Key  string   `json:"key"`
			Note string   `json:"note"`
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		if !a.store.SetFanNote(body.Key, body.Note, body.Tags) {
			writeJSON(w, 404, map[string]any{"ok": false, "msg": "找不到该观众"})
			return
		}
		a.store.SaveFansIfDirty()
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// 功能中心
	mux.HandleFunc("/api/modules", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"modules": a.mods.List(), "categories": a.mods.Categories()})
	})
	mux.HandleFunc("/api/modules/toggle", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		if !a.mods.SetEnabled(body.ID, body.Enabled) {
			writeJSON(w, 404, map[string]any{"ok": false, "msg": "模块不存在"})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/modules/params", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID     string         `json:"id"`
			Params map[string]any `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		if !a.mods.SetParams(body.ID, body.Params) {
			writeJSON(w, 404, map[string]any{"ok": false, "msg": "模块不存在"})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// 配置
	// 拉模型列表：小白不用记模型名，选就行
	mux.HandleFunc("/api/llm/models", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			BaseURL string `json:"baseUrl"`
			APIKey  string `json:"apiKey"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		cur := a.cfg.Get()
		if body.BaseURL == "" {
			body.BaseURL = cur.LLM.BaseURL
		}
		if body.APIKey == "" {
			body.APIKey = cur.LLM.APIKey
		}
		models, err := ListModels(body.BaseURL, body.APIKey)
		if err != nil {
			writeJSON(w, 200, map[string]any{
				"ok":          false,
				"error":       err.Error(),
				"suggestions": modelSuggestions(body.BaseURL),
			})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "models": models})
	})

	// 真发一句验证：Key/地址/模型三样都通才算配置成功
	mux.HandleFunc("/api/llm/probe", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			BaseURL string `json:"baseUrl"`
			APIKey  string `json:"apiKey"`
			Model   string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		cur := a.cfg.Get()
		cfg := cur.LLM
		if body.BaseURL != "" {
			cfg.BaseURL = body.BaseURL
		}
		if body.APIKey != "" {
			cfg.APIKey = body.APIKey
		}
		if body.Model != "" {
			cfg.Model = body.Model
		}
		cfg.Enabled = true
		reply, err := ProbeLLM(cfg)
		if err != nil {
			writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "reply": reply})
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeJSON(w, 200, a.cfg.Get())
			return
		}
		var c Config
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			writeJSON(w, 400, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		if c.Port <= 0 {
			c.Port = a.cfg.Get().Port
		}
		if err := a.cfg.Set(c); err != nil {
			writeJSON(w, 500, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		// 开了 AI 就顺手把「AI 话术」模块也打开，否则规则里的 llm 动作会被跳过
		autoOn := ""
		if c.LLM.Enabled && !a.mods.IsEnabled("llm_chat") {
			if a.mods.SetEnabled("llm_chat", true) {
				autoOn = "llm_chat"
			}
		}
		msg := "已保存，端口/映射改动需重启生效"
		if autoOn != "" {
			msg += "；已自动开启模块 " + autoOn
		}
		writeJSON(w, 200, map[string]any{"ok": true, "msg": msg, "autoEnabled": autoOn})
	})

	// 模拟推送（调试台核心）
	mux.HandleFunc("/api/simulate", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Raw  map[string]any `json:"raw"`
			Type string         `json:"type"`
			User string         `json:"user"`
			Text string         `json:"text"`
			Gift string         `json:"giftName"`
			Count int           `json:"giftCount"`
			Value float64       `json:"giftValue"`
			Level int           `json:"level"`
			GuardLevel int      `json:"guardLevel"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, 400, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		raw := body.Raw
		if raw == nil {
			raw = map[string]any{}
			if body.Type != "" {
				raw["type"] = body.Type
			}
			if body.User != "" {
				raw["user"] = body.User
			}
			if body.Text != "" {
				raw["text"] = body.Text
			}
			if body.Gift != "" {
				raw["giftName"] = body.Gift
			}
			if body.Count > 0 {
				raw["giftCount"] = body.Count
			}
			if body.Value > 0 {
				raw["giftValue"] = body.Value
			}
			if body.Level > 0 {
				raw["level"] = body.Level
			}
			if body.GuardLevel > 0 {
				raw["guardLevel"] = body.GuardLevel
			}
		}
		ev := a.HandleRaw(raw)
		writeJSON(w, 200, map[string]any{"ok": true, "event": ev})
	})

	// 单项测试
	mux.HandleFunc("/api/test/effect", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Preset     string `json:"preset"`
			Text       string `json:"text"`
			User       string `json:"user"`
			Color      string `json:"color"`
			DurationMS int    `json:"durationMs"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.DurationMS <= 0 {
			body.DurationMS = 4000
		}
		if body.Preset == "banner" {
			a.hub.Broadcast("effect", EffectCmd{Preset: "banner", Text: body.Text, User: body.User, Color: body.Color, DurationMS: body.DurationMS})
		} else {
			a.hub.Broadcast("effect", EffectCmd{Preset: body.Preset, Text: body.Text, User: body.User, Color: body.Color, DurationMS: body.DurationMS, Strength: 1})
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/test/tts", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.TrimSpace(body.Text) == "" {
			body.Text = "测试一下，声音正常吗"
		}
		a.tts.Speak(body.Text)
		writeJSON(w, 200, map[string]any{"ok": true, "mode": a.cfg.Get().TTS.Mode})
	})

	mux.HandleFunc("/api/test/sound", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			File   string  `json:"file"`
			Volume float64 `json:"volume"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		name := a.resolveSFX(body.File)
		if name == "" {
			writeJSON(w, 404, map[string]any{"ok": false, "msg": "音效不存在"})
			return
		}
		a.hub.Broadcast("sound", map[string]any{"file": "/sfx/" + name, "volume": body.Volume})
		if strings.HasSuffix(strings.ToLower(name), ".wav") {
			go playWavLocal(filepath.Join(a.sfxDir, name))
		}
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	mux.HandleFunc("/api/test/hotkey", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Keys string `json:"keys"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Keys == "" {
			body.Keys = "ctrl+space"
		}
		if err := pressHotkey(body.Keys); err != nil {
			writeJSON(w, 400, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "msg": "已按下 " + body.Keys})
	})

	mux.HandleFunc("/api/test/llm", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Text == "" {
			body.Text = "观众问：今天主播状态怎么样？"
		}
		go func(text string) {
			out, err := a.llm.Chat(text)
			if err != nil {
				a.hub.Broadcast("llm", map[string]any{"text": "调用失败：" + err.Error(), "ok": false})
				return
			}
			a.hub.Broadcast("llm", map[string]any{"text": out, "ok": true})
			a.tts.Speak(out)
		}(body.Text)
		writeJSON(w, 200, map[string]any{"ok": true, "msg": "已请求，结果会出现在事件流"})
	})

	// 模块一键测试
	mux.HandleFunc("/api/modules/test", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mod, ok := a.mods.Get(body.ID)
		if !ok {
			writeJSON(w, 404, map[string]any{"ok": false, "msg": "模块不存在"})
			return
		}
		if !mod.Enabled {
			writeJSON(w, 409, map[string]any{"ok": false, "msg": "模块未启用"})
			return
		}
		msg, err := a.testModule(mod)
		if err != nil {
			writeJSON(w, 400, map[string]any{"ok": false, "msg": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "msg": msg})
	})

	mux.HandleFunc("/api/voices", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"voices": listVoices()})
	})
	mux.HandleFunc("/api/sfx", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"files": a.sfxList()})
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "clients": a.hub.Count(), "uptimeMs": time.Now().UnixMilli() - a.started})
	})
}

func (a *App) testModule(m Module) (string, error) {
	switch m.ID {
	case "tts_local":
		a.tts.Speak("模块测试，声音正常")
		return "已试念", nil
	case "fx_fireworks", "fx_hearts", "fx_meteor", "fx_confetti", "fx_shake", "fx_spotlight":
		preset := strings.TrimPrefix(m.ID, "fx_")
		a.hub.Broadcast("effect", EffectCmd{Preset: preset, DurationMS: 4000, Strength: 1})
		return "已触发 " + preset, nil
	case "fx_banner":
		a.hub.Broadcast("effect", EffectCmd{Preset: "banner", Text: "横幅测试：欢迎来到直播间", DurationMS: 4000})
		return "已触发横幅", nil
	case "http_push":
		return fmt.Sprintf("推送地址：http://127.0.0.1:%d/push", a.cfg.Get().Port), nil
	case "field_mapping":
		ev := a.HandleRaw(map[string]any{"type": "danmaku", "user": "测试观众", "text": "这条是映射测试"})
		return fmt.Sprintf("映射结果：type=%s user=%s text=%s", ev.Type, ev.User, ev.Text), nil
	case "llm_chat":
		if !a.cfg.Get().LLM.Enabled {
			return "", fmt.Errorf("先去设置里打开 LLM 并填 model")
		}
		out, err := a.llm.Chat("用一句话证明你在工作")
		if err != nil {
			return "", err
		}
		return "AI：" + out, nil
	case "sfx_play":
		files := a.sfxList()
		if len(files) == 0 {
			return "", fmt.Errorf("sfx 目录没有音频文件")
		}
		a.hub.Broadcast("sound", map[string]any{"file": "/sfx/" + files[0], "volume": 1})
		return "已播放 " + files[0], nil
	case "hook_webhook":
		return "", fmt.Errorf("请在规则里配置具体 URL 后测试")
	case "key_hotkey":
		if err := pressHotkey("ctrl+space"); err != nil {
			return "", err
		}
		return "已按下 ctrl+space", nil
	case "fan_memory":
		return fmt.Sprintf("已记录 %d 位观众", len(a.store.Fans())), nil
	case "merge_antispam":
		st := a.merger.MergeStats()
		return fmt.Sprintf("累计合并 %d 次，节省 %d 次派发", st["merged"], st["saved"]), nil
	default:
		if !m.Ready {
		return "", fmt.Errorf("该模块尚未实现（路线图上）")
		}
		return "无需测试", nil
	}
}
