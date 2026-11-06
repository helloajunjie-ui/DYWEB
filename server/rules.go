package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Hit 一次规则命中
type Hit struct {
	Rule  Rule
	Combo int
}

type comboState struct {
	count  int
	lastTS int64
}

// RulesStore 规则集 + 匹配引擎（含冷却与连击累计）
type RulesStore struct {
	mu        sync.Mutex
	rules     []Rule
	path      string
	cooldowns map[string]int64
	combos    map[string]*comboState
	regex     map[string]*regexp.Regexp
}

func newRulesStore(dataDir string) *RulesStore {
	rs := &RulesStore{
		path:      filepath.Join(dataDir, "rules.json"),
		cooldowns: map[string]int64{},
		combos:    map[string]*comboState{},
		regex:     map[string]*regexp.Regexp{},
	}
	if b, err := os.ReadFile(rs.path); err == nil {
		var list []Rule
		if json.Unmarshal(b, &list) == nil && len(list) > 0 {
			rs.rules = list
			return rs
		}
	}
	rs.rules = defaultRules()
	_ = rs.save()
	return rs
}

func (rs *RulesStore) List() []Rule {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	out := make([]Rule, len(rs.rules))
	copy(out, rs.rules)
	return out
}

func (rs *RulesStore) Replace(list []Rule) error {
	rs.mu.Lock()
	for i := range list {
		if list[i].ID == "" {
			list[i].ID = "r_" + time.Now().Format("150405") + "_" + itoa(i)
		}
		if len(list[i].Match.Types) == 0 && list[i].Match.Keyword == "" && len(list[i].Match.GiftNames) == 0 {
			// 允许空条件（等于全量命中），不做拦截，主播可能就是要「所有事件都响应」
			list[i].Match.Types = list[i].Match.Types
		}
	}
	rs.rules = list
	rs.cooldowns = map[string]int64{}
	rs.mu.Unlock()
	return rs.save()
}

func (rs *RulesStore) save() error {
	rs.mu.Lock()
	b, err := json.MarshalIndent(rs.rules, "", "  ")
	rs.mu.Unlock()
	if err != nil {
		return err
	}
	return os.WriteFile(rs.path, b, 0o644)
}

// Eval 匹配事件，返回命中的规则（已处理冷却与连击）
func (rs *RulesStore) Eval(ev *Event) []Hit {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	now := time.Now().UnixMilli()

	// 按优先级排序后的顺序执行（同优先级保持配置顺序）
	sorted := make([]Rule, len(rs.rules))
	copy(sorted, rs.rules)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Priority > sorted[j].Priority })

	hits := []Hit{}
	for i := range sorted {
		r := sorted[i]
		if !r.Enabled {
			continue
		}
		if !rs.match(&r, ev) {
			continue
		}
		if cd, ok := rs.cooldowns[r.ID]; ok && r.CooldownMS > 0 && now-cd < int64(r.CooldownMS) {
			continue
		}
		combo := 0
		if r.Match.ComboWithinMS > 0 && ev.Type == "gift" {
			key := r.ID + "|" + ev.User + "|" + ev.GiftName
			st := rs.combos[key]
			if st == nil || now-st.lastTS > int64(r.Match.ComboWithinMS) {
				st = &comboState{count: ev.GiftCount, lastTS: now}
				rs.combos[key] = st
			} else {
				st.count += ev.GiftCount
				st.lastTS = now
			}
			combo = st.count
		}
		rs.cooldowns[r.ID] = now
		// 回写命中计数
		for j := range rs.rules {
			if rs.rules[j].ID == r.ID {
				rs.rules[j].HitCount++
				rs.rules[j].LastHit = now
				break
			}
		}
		hits = append(hits, Hit{Rule: r, Combo: combo})
	}
	if len(hits) > 0 && len(rs.combos) > 2000 {
		rs.combos = map[string]*comboState{} // 防泄漏
	}
	return hits
}

func (rs *RulesStore) match(r *Rule, ev *Event) bool {
	m := r.Match

	if len(m.Types) > 0 {
		ok := false
		for _, t := range m.Types {
			if t == ev.Type {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}

	if m.Keyword != "" && ev.Type == "danmaku" {
		if !matchKeyword(m, ev.Text, rs) {
			return false
		}
	}

	if len(m.GiftNames) > 0 {
		if ev.Type != "gift" {
			return false
		}
		ok := false
		for _, g := range m.GiftNames {
			if g != "" && (strings.Contains(ev.GiftName, g) || strings.Contains(g, ev.GiftName)) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}

	if m.MinValue > 0 && ev.GiftValue < m.MinValue {
		return false
	}
	if m.MinCount > 0 && ev.GiftCount < m.MinCount {
		return false
	}
	if m.MinLevel > 0 && ev.Level < m.MinLevel {
		return false
	}
	if m.UserName != "" && !strings.Contains(ev.User, m.UserName) {
		return false
	}
	if m.OnlyRegular {
		threshold := m.RegularMin
		if threshold <= 0 {
			threshold = 5
		}
		if !ev.IsRegular {
			return false
		}
	}
	return true
}

func matchKeyword(m MatchSpec, text string, rs *RulesStore) bool {
	mode := m.KeywordMode
	if mode == "" {
		mode = "contains"
	}
	switch mode {
	case "equals":
		return strings.TrimSpace(text) == strings.TrimSpace(m.Keyword)
	case "prefix":
		return strings.HasPrefix(strings.ToLower(text), strings.ToLower(m.Keyword))
	case "regex":
		re := rs.regex[m.Keyword]
		if re == nil {
			compiled, err := regexp.Compile(m.Keyword)
			if err != nil {
				return false
			}
			rs.regex[m.Keyword] = compiled
			re = compiled
		}
		return re.MatchString(text)
	default:
		return strings.Contains(strings.ToLower(text), strings.ToLower(m.Keyword))
	}
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// defaultRules 预置规则：开箱就能听见动静
func defaultRules() []Rule {
	return []Rule{
		{
			ID: "r_thanks_gift", Name: "礼物感谢", Enabled: true, CooldownMS: 1500, Priority: 10,
			Match: MatchSpec{Types: []string{"gift"}, MinValue: 1, ComboWithinMS: 3000},
			Actions: []Action{{
				Type:           "tts",
				Template:       "感谢{user}送的{gift}×{count}，谢谢老板！{fanNote}",
				DurationMS:     3000,
			}},
		},
		{
			ID: "r_big_gift", Name: "大礼物：烟花+横幅", Enabled: true, CooldownMS: 5000, Priority: 30,
			Match: MatchSpec{Types: []string{"gift"}, MinValue: 300},
			Actions: []Action{
				{Type: "effect", Preset: "fireworks", DurationMS: 6000, Merge: true},
				{Type: "banner", Template: "感谢 {user} 的 {gift} × {count}", DurationMS: 6000, Color: "#FFD24A"},
				{Type: "sound", File: "boom.wav", Volume: 0.85},
				{Type: "tts", Template: "{user}送出{gift}，全场给大哥让让道！"},
			},
		},
		{
			ID: "r_keyword_llm", Name: "关键词：AI 回一句", Enabled: true, CooldownMS: 4000, Priority: 20,
			Match: MatchSpec{Types: []string{"danmaku"}, Keyword: "点什么", KeywordMode: "contains", ComboWithinMS: 0},
			Actions: []Action{
				{Type: "llm", LLMPrompt: "观众说：{text}。用一句有梗的话回应他，可以推荐一首歌。"},
				{Type: "effect", Preset: "hearts", DurationMS: 2500, Merge: true},
			},
		},
		{
			ID: "r_guard", Name: "星守护开通：全场撒花", Enabled: true, CooldownMS: 3000, Priority: 60,
			Match: MatchSpec{Types: []string{"guard"}},
			Actions: []Action{
				{Type: "effect", Preset: "fireworks", DurationMS: 7000, Merge: true},
				{Type: "effect", Preset: "spotlight", DurationMS: 5000},
				{Type: "banner", Template: "守护降临 · {user}", DurationMS: 6000, Color: "#B98BFF"},
				{Type: "sound", File: "ding.wav", Volume: 0.7},
				{Type: "tts", Template: "恭喜{user}开通星守护，直播间有守护神了！"},
			},
		},
		{
			ID: "r_fansclub", Name: "加入粉丝团", Enabled: true, CooldownMS: 2000, Priority: 35,
			Match: MatchSpec{Types: []string{"fansclub"}},
			Actions: []Action{
				{Type: "effect", Preset: "hearts", DurationMS: 3000, Merge: true},
				{Type: "banner", Template: "{user} 加入粉丝团啦", DurationMS: 4000, Color: "#FF7EA8"},
				{Type: "tts", Template: "欢迎{user}加入粉丝团，牌牌亮起来！"},
			},
		},
		{
			ID: "r_follow", Name: "关注感谢", Enabled: true, CooldownMS: 2000, Priority: 15,
			Match: MatchSpec{Types: []string{"follow"}},
			Actions: []Action{{Type: "tts", Template: "感谢{user}的关注，常来玩啊！"}},
		},
		{
			ID: "r_fan_enter", Name: "老粉进场提醒", Enabled: true, CooldownMS: 0, Priority: 40,
			Match: MatchSpec{Types: []string{"enter"}, OnlyRegular: true, RegularMin: 5},
			Actions: []Action{
				{Type: "banner", Template: "{user} 来啦", DurationMS: 4000, Color: "#7ED0FF"},
				{Type: "tts", Template: "真爱粉{user}进场了。{fanNote}"},
			},
		},
	}
}
