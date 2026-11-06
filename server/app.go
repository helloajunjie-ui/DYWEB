package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// App 装配体
type App struct {
	cfg     *ConfigStore
	rules   *RulesStore
	store   *Store
	hub     *Hub
	tts     *TTS
	mods    *ModuleStore
	merger  *Merger
	llm     *LLMClient
	gifts   *GiftStore
	scripts *ScriptStore
	products *ProductStore
	game    *GameEngine
	root    string
	dataDir string
	sfxDir  string
	started int64

	traceMu sync.Mutex
	traces  []Trace
}

// Trace 一条动作执行记录
type Trace struct {
	TS      int64  `json:"ts"`
	Rule    string `json:"rule"`
	Action  string `json:"action"`
	OK      bool   `json:"ok"`
	Msg     string `json:"msg"`
	User    string `json:"user"`
	EventID int64  `json:"eventId"`
}

func (a *App) addTrace(t Trace) {
	a.traceMu.Lock()
	a.traces = append(a.traces, t)
	if len(a.traces) > 200 {
		a.traces = a.traces[len(a.traces)-200:]
	}
	a.traceMu.Unlock()
	a.hub.Broadcast("trace", t)
}

func (a *App) TraceList(n int) []Trace {
	a.traceMu.Lock()
	defer a.traceMu.Unlock()
	if n <= 0 || n > len(a.traces) {
		n = len(a.traces)
	}
	out := make([]Trace, n)
	copy(out, a.traces[len(a.traces)-n:])
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func newApp(root string) *App {
	dataDir := filepath.Join(root, "data")
	_ = os.MkdirAll(dataDir, 0o755)
	sfxDir := filepath.Join(root, "sfx")
	_ = os.MkdirAll(sfxDir, 0o755)

	a := &App{
		cfg:     loadConfig(dataDir),
		rules:   newRulesStore(dataDir),
		store:   newStore(dataDir),
		hub:     newHub(),
		mods:    newModuleStore(dataDir),
		gifts:   newGiftStore(dataDir),
		scripts: newScriptStore(dataDir),
		products: newProductStore(dataDir),
		game:    newGameEngine(),
		llm:     nil,
		root:    root,
		dataDir: dataDir,
		sfxDir:  sfxDir,
		started: time.Now().UnixMilli(),
	}
	a.llm = newLLMClient(func() LLMConfig { return a.cfg.Get().LLM })
	a.tts = newTTS(func() TTSConfig { return a.cfg.Get().TTS }, a.dataDir)
	a.tts.onBrowser = func(text string) {
		a.hub.Broadcast("tts", map[string]any{"text": text})
	}
	a.merger = newMerger(time.Second, func(b *mergeBucket) {
		cmd := b.Sample
		if cmd.Strength >= 4 {
			cmd.Text = "全屏特效：" + firstNonEmpty(cmd.Text, "一波礼物刷屏")
		}
		a.hub.Broadcast("effect", cmd)
	})
	a.merger.Start()
	return a
}

// HandleRaw 原始 JSON → 事件 → 规则 → 动作
func (a *App) HandleRaw(raw map[string]any) Event {
	cfg := a.cfg.Get()
	ev := normalize(raw, cfg)
	return a.HandleEvent(ev)
}

func (a *App) HandleEvent(ev Event) Event {
	cfg := a.cfg.Get()
	// 礼物价目补全：工具只推礼物名时，用礼物库算单价
	if ev.Type == "gift" && ev.GiftName != "" {
		if ev.GiftValue <= 0 {
			if v, ok := a.gifts.Value(ev.GiftName); ok {
				ev.GiftValue = v
			}
		}
		ev.Total = ev.GiftValue * float64(ev.GiftCount)
	}
	ev = a.store.Add(ev)
	a.hub.Broadcast("event", ev)
	// 猜数字玩法要在每条弹幕上判断，跟规则引擎无关
	go a.guessFeed(ev)

	if !a.mods.IsEnabled("field_mapping") {
		return ev
	}
	if cfg.Welcome.Enabled && ev.Type == "enter" {
		// 只欢迎「高等级 / 高粉丝团 / 高消费」，门槛由主播设；关掉门槛就是谁都欢迎
		f, ok := a.store.FanOf(ev.User, ev.UserID)
		hit := ok && ((cfg.Welcome.MinLevel > 0 && f.Level >= cfg.Welcome.MinLevel) ||
			(cfg.Welcome.MinFans > 0 && f.FansClubLevel >= cfg.Welcome.MinFans) ||
			(cfg.Welcome.MinValue > 0 && f.GiftValue >= cfg.Welcome.MinValue))
		if !hit {
			a.addTrace(Trace{TS: time.Now().UnixMilli(), Rule: "欢迎门槛", Action: "skip", OK: true, User: ev.User, EventID: ev.ID, Msg: "未达欢迎门槛，跳过欢迎"})
			return ev
		}
	}
	for _, h := range a.rules.Eval(&ev) {
		a.store.IncRuleHits()
		a.hub.Broadcast("rulehit", map[string]any{
			"rule": h.Rule.Name, "id": h.Rule.ID, "user": ev.User, "eventId": ev.ID,
		})
		go a.runRule(h, ev)
	}
	return ev
}

// runRule 一条规则内的动作串行执行，规则之间并行
func (a *App) runRule(h Hit, ev Event) {
	for _, act := range h.Rule.Actions {
		act := act
		if modID := moduleForAction(act); modID != "" && !a.mods.IsEnabled(modID) {
			a.trace(h.Rule, act, ev, false, "模块已关闭："+modID)
			continue
		}
		msg, err := a.performAction(h.Rule, act, ev, h.Combo)
		if err != nil {
			a.trace(h.Rule, act, ev, false, err.Error())
		} else {
			a.trace(h.Rule, act, ev, true, msg)
		}
	}
}

func (a *App) performAction(rule Rule, act Action, ev Event, combo int) (string, error) {
	cfg := a.cfg.Get()
	dur := act.DurationMS
	if dur <= 0 {
		dur = cfg.PadDuration
	}

	switch act.Type {
	case "tts":
		text := a.render(act.Template, ev, combo)
		if text == "" {
			return "", fmt.Errorf("TTS 文案为空")
		}
		a.tts.Speak(text)
		return "念：" + text, nil

	case "sound":
		file := a.resolveSFX(act.File)
		if file == "" {
			return "", fmt.Errorf("找不到音效文件：%s", act.File)
		}
		a.hub.Broadcast("sound", map[string]any{"file": "/sfx/" + file, "volume": act.Volume})
		if strings.HasSuffix(strings.ToLower(file), ".wav") {
			go playWavLocal(filepath.Join(a.sfxDir, file))
		}
		return "播放音效：" + file, nil

	case "effect":
		cmd := EffectCmd{
			Preset:     act.Preset,
			DurationMS: dur,
			User:       ev.User,
			Avatar:     ev.Avatar,
			Gift:       ev.GiftName,
			Count:      ev.GiftCount,
			Color:      act.Color,
			Sound:      act.File,
		}
		if act.Merge {
			weight := 1.0
			if ev.Type == "gift" {
				if ev.Total > 0 {
					weight = ev.Total
				} else {
					weight = float64(ev.GiftCount)
				}
			}
			a.merger.Add(firstNonEmpty(act.Preset, "hearts"), cmd, weight, ev.User)
			return "已入合并队列：" + act.Preset, nil
		}
		a.hub.Broadcast("effect", cmd)
		return "触发特效：" + act.Preset, nil

	case "banner":
		text := a.render(act.Template, ev, combo)
		a.hub.Broadcast("effect", EffectCmd{
			Preset:     "banner",
			DurationMS: dur,
			Text:       text,
			User:       ev.User,
			Avatar:     ev.Avatar,
			Color:      act.Color,
		})
		return "横幅：" + text, nil

	case "webhook":
		body := a.render(act.Template, ev, combo)
		if body == "" {
			b, _ := json.Marshal(ev)
			body = string(b)
		}
		status, err := postJSON(act.URL, body, act.Method, act.Headers)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("回调 %s → %d", act.URL, status), nil

	case "hotkey":
		if err := pressHotkey(act.Keys); err != nil {
			return "", err
		}
		return "按键：" + act.Keys, nil

	case "llm":
		prompt := a.render(act.LLMPrompt, ev, combo)
		if prompt == "" {
			prompt = a.render(act.Template, ev, combo)
		}
		go func() {
			// 让 AI 心里有数：喂话术库 + 当前主推商品的资料
			var sb strings.Builder
			if refs := a.scripts.Relevant(prompt, 3); len(refs) > 0 {
				sb.WriteString("主播话术库里的参考说法（学语气，不要照抄）：\n")
				for _, s := range refs {
					sb.WriteString("- " + s.Text + "\n")
				}
			}
			if p, ok := a.products.ActiveOne(); ok {
				sb.WriteString("\n当前主推商品资料（讲的时候以这个为准，不要编参数）：\n")
				sb.WriteString(p.Brief())
				sb.WriteString("\n")
			}
			text, err := a.llm.ChatWith(prompt, sb.String())
			if err != nil {
				a.trace(rule, act, ev, false, err.Error())
				return
			}
			a.hub.Broadcast("llm", map[string]any{"text": text, "user": ev.User, "rule": rule.Name})
			a.tts.Speak(text)
			a.trace(rule, act, ev, true, "AI："+text)
		}()
		return "已请求 AI 生成", nil

	default:
		return "", fmt.Errorf("未知动作类型：%s", act.Type)
	}
}

func (a *App) trace(rule Rule, act Action, ev Event, ok bool, msg string) {
	a.addTrace(Trace{
		TS:      time.Now().UnixMilli(),
		Rule:    rule.Name,
		Action:  act.Type,
		OK:      ok,
		Msg:     msg,
		User:    ev.User,
		EventID: ev.ID,
	})
}

// render 模板变量替换
func (a *App) render(tpl string, ev Event, combo int) string {
	if tpl == "" {
		return ""
	}
	count := ev.GiftCount
	if combo > 0 {
		count = combo
	}
	pairs := []string{
		"{user}", ev.User,
		"{text}", ev.Text,
		"{gift}", ev.GiftName,
		"{count}", strconv.Itoa(count),
		"{value}", trimFloat(ev.GiftValue),
		"{total}", trimFloat(ev.Total),
		"{level}", strconv.Itoa(ev.Level),
		"{guard}", strconv.Itoa(ev.GuardLevel),
		"{combo}", strconv.Itoa(combo),
		"{fanNote}", ev.FanNote,
		"{time}", time.Now().Format("15:04"),
	}
	out := tpl
	for i := 0; i < len(pairs); i += 2 {
		out = strings.ReplaceAll(out, pairs[i], pairs[i+1])
	}
	// 清理未替换的变量和多余空格
	out = strings.TrimSpace(out)
	return out
}

func (a *App) resolveSFX(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	if _, err := os.Stat(filepath.Join(a.sfxDir, name)); err != nil {
		return ""
	}
	return name
}

func (a *App) sfxList() []string {
	entries, err := os.ReadDir(a.sfxDir)
	if err != nil {
		return []string{}
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".wav" || ext == ".mp3" || ext == ".ogg" || ext == ".m4a" {
			out = append(out, e.Name())
		}
	}
	return out
}

func postJSON(url, body, method string, headers map[string]string) (int, error) {
	if url == "" {
		return 0, fmt.Errorf("webhook URL 为空")
	}
	if method == "" {
		method = "POST"
	}
	req, err := http.NewRequest(method, url, bytes.NewReader([]byte(body)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	deferred := resp.Body
	defer deferred.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("webhook 返回 %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func playWavLocal(path string) {
	script := "$p = New-Object Media.SoundPlayer '" + strings.ReplaceAll(path, "'", "''") + "'; $p.PlaySync()"
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden",
		"-ExecutionPolicy", "Bypass", "-Command", script)
	_ = cmd.Run()
}

func trimFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', 2, 64)
}
