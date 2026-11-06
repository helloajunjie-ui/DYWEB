package main

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"
)

// GameView 一次互动游戏的完整状态：主播端面板和舞台层看同一份
type GameView struct {
	Type    string   `json:"type"` // wheel / guess / traitor / 空=没有正在玩的
	Title   string   `json:"title"`
	Items   []string `json:"items,omitempty"`
	Winner  int      `json:"winner"`
	SpinMs  int      `json:"spinMs,omitempty"`
	Lo      int      `json:"lo,omitempty"`
	Hi      int      `json:"hi,omitempty"`
	User    string   `json:"user,omitempty"`
	Reason  string   `json:"reason,omitempty"`
	Tip     string   `json:"tip,omitempty"`
	Won     bool     `json:"won,omitempty"`
	Started int64    `json:"started,omitempty"`
}

// GameEngine 手动卡点玩法：一键转盘 / 猜数字 / 抓叛徒
type GameEngine struct {
	mu       sync.Mutex
	cur      GameView
	guessOn  bool
	guessNum int
	lastSnark int64
}

func newGameEngine() *GameEngine { return &GameEngine{} }

// View 返回当前状态
func (g *GameEngine) View() GameView {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.cur
}

func (g *GameEngine) set(v GameView) GameView {
	g.mu.Lock()
	g.cur = v
	g.mu.Unlock()
	return v
}

func (a *App) broadcastGame(v GameView) {
	if v.Type == "" {
		a.hub.Broadcast("game", map[string]any{"type": "clear"})
		return
	}
	a.hub.Broadcast("game", v)
}

// StartWheel 主播自定义盘面优先；没填就让 AI 按主题现编。转动手感交给舞台层。
func (a *App) StartWheel(theme string, items []string, speed int) GameView {
	theme = strings.TrimSpace(theme)
	items = cleanWheelItems(items)
	if len(items) == 0 {
		items = a.wheelItems(theme)
	}

	g := a.game
	g.mu.Lock()
	g.guessOn = false
	g.lastSnark = 0
	g.mu.Unlock()

	v := GameView{
		Type:    "wheel",
		Title:   firstNonEmpty(theme, "命运大转盘"),
		Items:   items,
		SpinMs:  speed,
		Started: time.Now().UnixMilli(),
	}
	v.Tip = "转盘开转了，落点在大屏上"
	a.broadcastGame(v)
	a.tts.Speak("大转盘转起来了，看看今天的运气落在谁头上")
	a.addTrace(Trace{TS: time.Now().UnixMilli(), Rule: "面板·大转盘", Action: "game", OK: true, Msg: "盘面：" + strings.Join(items, "、")})
	g.set(v)
	return v
}

func (a *App) wheelItems(theme string) []string {
	fallback := []string{"唱首歌", "做10个深蹲", "模仿老板说话", "讲个土味情话", "送一个飞吻", "现场喝口水"}
	if a.cfg.Get().LLM.Enabled && a.mods.IsEnabled("llm_chat") {
		prompt := "帮我给直播大转盘想 6 个惩罚/整活词条。"
		if theme != "" {
			prompt += "主题是：" + theme + "。"
		}
		prompt += "要求：每行一个，只输出词条本身，不要序号、不要解释，每条不超过 8 个字，要具体、能现场做、有点好笑。"
		extra := "你在给主播做互动转盘，只输出词条，绝对不要写解释或编号。"
		if out, err := a.llm.ChatWithTimeout(prompt, extra, 3500*time.Millisecond); err == nil {
			if list := splitItems(out, 6); len(list) >= 4 {
				return list
			}
		}
	}
	if theme != "" {
		// 没有 AI 时按主题做个粗糙拼接，至少盘面不是死的
		r := []rune(theme)
		head := string(r[:minInt(len(r), 4)])
		out := make([]string, 0, 6)
		for i, f := range fallback {
			if i < 3 {
				out = append(out, head+f)
			} else {
				out = append(out, f)
			}
		}
		return out
	}
	return fallback
}

var itemNoise = regexp.MustCompile(`^[\s\-*•·0-9.、)）]+`)

func splitItems(text string, max int) []string {
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	out := []string{}
	for _, line := range lines {
		line = itemNoise.ReplaceAllString(strings.TrimSpace(line), "")
		line = strings.Trim(line, "“”\"'`：:，,。.")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		r := []rune(line)
		if len(r) > 12 {
			line = string(r[:12])
		}
		out = append(out, line)
		if len(out) >= max {
			break
		}
	}
	return out
}

// cleanWheelItems 主播自己填的词条：去空行、去重、限长、限条数（2~12 条）
func cleanWheelItems(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, raw := range in {
		t := strings.Trim(strings.TrimSpace(raw), "，,、")
		if t == "" || seen[t] {
			continue
		}
		if r := []rune(t); len(r) > 12 {
			t = string(r[:12])
		}
		seen[t] = true
		out = append(out, t)
		if len(out) >= 12 {
			break
		}
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

// StartGuess 开一局猜数字
func (a *App) StartGuess() GameView {
	n := rand.Intn(1000) + 1
	g := a.game
	g.mu.Lock()
	g.guessOn = true
	g.guessNum = n
	g.lastSnark = 0
	g.mu.Unlock()

	v := GameView{Type: "guess", Title: "弹幕猜数字", Lo: 1, Hi: 1000, Started: time.Now().UnixMilli()}
	v.Tip = fmt.Sprintf("数字想好了（1~1000），让观众扣数字，猜中就开奖")
	a.broadcastGame(v)
	a.tts.Speak("猜数字开始，一到一千，公屏扣数字，猜中直接开奖")
	a.addTrace(Trace{TS: time.Now().UnixMilli(), Rule: "面板·猜数字", Action: "game", OK: true, Msg: "开局"})
	g.set(v)
	return v
}

var numRe = regexp.MustCompile(`\d{1,4}`)

// guessFeed 弹幕里抓数字：在区间内就缩区间，猜中开奖
func (a *App) guessFeed(ev Event) {
	g := a.game
	g.mu.Lock()
	if !g.guessOn || ev.Type != "danmaku" {
		g.mu.Unlock()
		return
	}
	target := g.guessNum
	matched := numRe.FindString(ev.Text)
	g.mu.Unlock()
	if matched == "" {
		return
	}
	n := atoiSafe(matched)
	if n <= 0 || n > 1000 {
		return
	}

	g.mu.Lock()
	lo, hi := g.cur.Lo, g.cur.Hi
	g.mu.Unlock()
	if n < lo || n > hi {
		a.snarkOutOfRange(ev.User, n, lo, hi)
		return
	}
	if n == target {
		v := GameView{Type: "guess", Title: "弹幕猜数字", Lo: lo, Hi: hi, User: ev.User, Won: true, Started: time.Now().UnixMilli()}
		v.Tip = "猜中了：" + ev.User
		g.mu.Lock()
		g.guessOn = false
		g.cur = v
		g.mu.Unlock()
		a.broadcastGame(v)
		a.hub.Broadcast("effect", EffectCmd{Preset: "confetti", DurationMS: 4000, User: ev.User, Strength: 3})
		a.tts.Speak("恭喜" + ev.User + "猜中了，数字就是" + matched)
		a.addTrace(Trace{TS: time.Now().UnixMilli(), Rule: "面板·猜数字", Action: "game", OK: true, User: ev.User, Msg: "猜中 " + matched})
		return
	}
	// 缩区间
	v := GameView{Type: "guess", Title: "弹幕猜数字", Lo: lo, Hi: hi, Started: time.Now().UnixMilli()}
	if n < target {
		v.Lo = n + 1
	} else {
		v.Hi = n - 1
	}
	v.Tip = fmt.Sprintf("%s 猜 %d，%s，区间缩到 %d ~ %d", ev.User, n, map[bool]string{true: "小了", false: "大了"}[n < target], v.Lo, v.Hi)
	g.mu.Lock()
	g.cur = v
	g.mu.Unlock()
	a.broadcastGame(v)
	a.hub.Broadcast("effect", EffectCmd{Preset: "shake", DurationMS: 300})
}

// snarkOutOfRange 有人猜了区间外的数字，让 AI 毒舌一句（5 秒一次，防刷）
func (a *App) snarkOutOfRange(user string, n, lo, hi int) {
	g := a.game
	g.mu.Lock()
	if time.Now().UnixMilli()-g.lastSnark < 5000 {
		g.mu.Unlock()
		return
	}
	g.lastSnark = time.Now().UnixMilli()
	g.mu.Unlock()

	if !a.cfg.Get().LLM.Enabled || !a.mods.IsEnabled("llm_chat") {
		return
	}
	prompt := fmt.Sprintf("观众%s在猜数字游戏里发了 %d，但现在区间已经缩到 %d 到 %d，他明显没看屏。用一句不超过25字的中文毒舌他，要好笑不要骂人。", user, n, lo, hi)
	extra := "你只说一句吐槽，不要解释、不要引号、不要emoji。"
	go func() {
		out, err := a.llm.ChatWith(prompt, extra)
		if err != nil || out == "" {
			return
		}
		a.hub.Broadcast("llm", map[string]any{"text": out, "user": user, "rule": "猜数字"})
		a.tts.Speak(out)
	}()
}

// CatchTraitor 抓最近 10 秒最带节奏的那位
func (a *App) CatchTraitor() GameView {
	cut := time.Now().UnixMilli() - 10000
	events := a.store.Recent(300)
	type cand struct {
		user string
		text []string
	}
	counts := map[string]int{}
	texts := map[string][]string{}
	latest := map[string]int64{}
	for _, e := range events {
		if e.Type != "danmaku" || e.TS < cut || e.User == "" || strings.TrimSpace(e.Text) == "" {
			continue
		}
		counts[e.User]++
		if len(texts[e.User]) < 3 {
			texts[e.User] = append(texts[e.User], e.Text)
		}
		if e.TS > latest[e.User] {
			latest[e.User] = e.TS
		}
	}
	if len(counts) == 0 {
		v := GameView{Type: "traitor", Title: "AI 抓叛徒", Reason: "最近 10 秒公屏没人说话，抓不到人"}
		v.Tip = "公屏太安静了，先聊两句再抓"
		a.broadcastGame(v)
		a.game.set(v)
		return v
	}
	// 没有 AI 就抓刷屏最多的
	best, bestN := "", 0
	for u, n := range counts {
		if n > bestN || (n == bestN && latest[u] > latest[best]) {
			best, bestN = u, n
		}
	}

	reason := fmt.Sprintf("10 秒发了 %d 条，公屏被他带飞了", bestN)
	if a.cfg.Get().LLM.Enabled && a.mods.IsEnabled("llm_chat") {
		var sb strings.Builder
		sb.WriteString("最近 10 秒的弹幕：\n")
		for u, n := range counts {
			sb.WriteString(fmt.Sprintf("- %s（%d 条）：%s\n", u, n, strings.Join(texts[u], " / ")))
		}
		extra := "你只在这些人里挑一个最带节奏/最好笑的，回复格式严格为：名字|一句不超过20字的宣判"
		out, err := a.llm.ChatWithTimeout("抓一个人出来，按格式回复。", extra+"\n\n"+sb.String(), 4*time.Second)
		if err == nil && strings.Contains(out, "|") {
			parts := strings.SplitN(out, "|", 2)
			name := strings.TrimSpace(parts[0])
			if _, ok := counts[name]; ok {
				best = name
				reason = strings.TrimSpace(parts[1])
			}
		}
	}

	v := GameView{Type: "traitor", Title: "AI 抓叛徒", User: best, Reason: reason, Started: time.Now().UnixMilli()}
	v.Tip = "通缉：" + best
	a.broadcastGame(v)
	a.hub.Broadcast("effect", EffectCmd{Preset: "spotlight", DurationMS: 4000, User: best})
	a.tts.Speak("全场通缉，" + best + "，" + reason)
	a.addTrace(Trace{TS: time.Now().UnixMilli(), Rule: "面板·抓叛徒", Action: "game", OK: true, User: best, Msg: reason})
	a.game.set(v)
	return v
}

// WheelResult 舞台层转完把落点回传，服务端宣布结果
func (a *App) WheelResult(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	a.tts.Speak("大转盘停在：" + text)
	a.hub.Broadcast("gameResult", map[string]any{"text": text})
	a.addTrace(Trace{TS: time.Now().UnixMilli(), Rule: "面板·大转盘", Action: "game", OK: true, Msg: "落在：" + text})
}

// ClearGame 收掉面板上的游戏
func (a *App) ClearGame() {
	g := a.game
	g.mu.Lock()
	g.guessOn = false
	g.cur = GameView{}
	g.mu.Unlock()
	a.broadcastGame(GameView{})
	a.addTrace(Trace{TS: time.Now().UnixMilli(), Rule: "面板", Action: "game", OK: true, Msg: "清屏"})
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
		if n > 99999 {
			return n
		}
	}
	return n
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
