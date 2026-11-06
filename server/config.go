package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// TTSConfig TTS 相关配置
type TTSConfig struct {
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"` // local(本机SAPI) | browser(overlay页朗读) | both
	Voice   string `json:"voice"`
	Rate    int    `json:"rate"`   // -10 ~ 10
	Volume  int    `json:"volume"` // 0 ~ 100
}

// LLMConfig OpenAI 兼容接口配置（火山方舟/DeepSeek/Ollama 均可）
type LLMConfig struct {
	Enabled     bool    `json:"enabled"`
	BaseURL     string  `json:"baseUrl"`
	APIKey      string  `json:"apiKey"`
	Model       string  `json:"model"`
	System      string  `json:"system"`
	TimeoutMS   int     `json:"timeoutMs"`
	Temperature float64 `json:"temperature"`
}

// LiveMode 直播定位：三种常见直播间形态，决定「推荐开什么、重点看什么」
type LiveMode struct {
	ID     string   `json:"id"`
	Short  string   `json:"short"`
	Name   string   `json:"name"`
	Desc   string   `json:"desc"`
	Focus  string   `json:"focus"`
	Script string   `json:"script"` // 话术库默认分类
	Rec    []string `json:"rec"`    // 推荐模块 id
	Tips   []string `json:"tips"`
}

var liveModes = []LiveMode{
	{
		ID:     "fun",
		Short:  "娱乐",
		Name:   "娱乐直播（单人播）",
		Desc:   "你一个人在播，场控替你看弹幕、接礼物、救冷场",
		Focus:  "礼物感谢要快、冷场要有人接话、整活要跟得上",
		Script: "keep",
		Rec:    []string{"tts_local", "llm_chat", "fan_memory", "fx_particles", "fx_banner", "sfx_play", "key_hotkey"},
		Tips: []string{
			"礼物规则门槛别设太高，1 币也谢，气势要有",
			"冷场时点「话术库 → 冷场救场」念一句，比干坐着强",
			"整蛊按键会把键按在当前前台窗口，直播时把游戏窗口切到前台",
		},
	},
	{
		ID:     "auto",
		Short:  "无人",
		Name:   "无人直播（AI 主导）",
		Desc:   "人不在也撑住场面：AI 循环讲、自动应答、特效防刷",
		Focus:  "稳定性第一，别让画面炸、别让 AI 乱说",
		Script: "cold",
		Rec:    []string{"llm_chat", "tts_local", "merge_antispam", "overlay_watchdog", "fps_guard", "fx_particles", "fan_memory"},
		Tips: []string{
			"先把「特效合并防刷」和「帧率护栏」开着，无人时最容易堆满粒子",
			"AI 人设里加一句「不许承诺价格、不许说绝对」，能挡掉大部分口播事故",
			"规则冷却时间加长（≥5 秒），避免同一个人连刷触发几十次",
		},
	},
	{
		ID:     "ecom",
		Short:  "电商",
		Name:   "电商直播（主播助理）",
		Desc:   "讲品、算账、避坑、催单，AI 当你的助播",
		Focus:  "产品资料要录全、避坑话要说、退单率要压",
		Script: "push",
		Rec:    []string{"llm_chat", "tts_local", "tts_browser", "fx_banner", "hook_webhook", "fan_memory", "obs_ws"},
		Tips: []string{
			"先去「产品库」录：卖点、适合谁、不适合谁、常见问题，AI 才有料讲",
			"把主推商品打开开关，AI 生成的话会以它为准，不会编参数",
			"话术库里「促单 · 算账」「退货率友好版」是压退货率的主力",
		},
	},
}

// WelcomeConfig 进场欢迎门槛：三项可分别开关（只欢迎高等级 / 高粉丝团 / 高消费，任意一项达标就欢迎）
type WelcomeConfig struct {
	Enabled   bool    `json:"enabled"`   // 总开关：关=所有人都欢迎
	LevelOn   bool    `json:"levelOn"`   // 看账号等级
	FansOn    bool    `json:"fansOn"`    // 看粉丝团等级
	ValueOn   bool    `json:"valueOn"`   // 看累计消费
	MinLevel  int     `json:"minLevel"`  // 账号等级 ≥
	MinFans   int     `json:"minFans"`   // 粉丝团等级 ≥
	MinValue  float64 `json:"minValue"`  // 累计消费抖币 ≥
}

// Config 全局配置
type Config struct {
	Port        int                 `json:"port"`
	Mode        string              `json:"mode"` // 直播定位：fun 娱乐单人 / auto 无人直播 / ecom 电商助理
	SFXDir      string              `json:"sfxDir"`
	PadDuration int                 `json:"padDurationMs"` // 礼物Banner默认时长
	TTS         TTSConfig           `json:"tts"`
	LLM         LLMConfig           `json:"llm"`
	Mapping     map[string][]string `json:"mapping"`     // 字段映射：逻辑字段 -> 候选路径
	TypeAliases map[string][]string `json:"typeAliases"` // 事件类型别名
	Welcome     WelcomeConfig       `json:"welcome"`      // 进场欢迎门槛
}

func defaultConfig() Config {
	return Config{
		Port:        9101,
		Mode:        "fun",
		SFXDir:      "sfx",
		PadDuration: 5000,
		TTS: TTSConfig{
			Enabled: true,
			Mode:    "local",
			Rate:    0,
			Volume:  90,
		},
		LLM: LLMConfig{
			Enabled:     false,
			BaseURL:     "https://ark.cn-beijing.volces.com/api/v3",
			Model:       "",
			System:      "你是直播间主播的场控助理。用一句不超过30字的中文口语回复观众，语气亲切有梗，不要解释、不要加引号、不要emoji。",
			TimeoutMS:   2500,
			Temperature: 0.7,
		},
		Mapping: map[string][]string{
			"type":      {"type", "Type", "msg_type", "msgType", "cmd", "event", "Category"},
			"user":      {"user", "User", "nickname", "nickName", "nick_name", "uname", "userName", "user_name", "data.user.nickname", "Data.User.NickName"},
			"userId":    {"userId", "user_id", "uid", "sec_uid", "openid", "data.user.id", "Data.User.Id"},
			"avatar":    {"avatar", "Avatar", "head_img", "headImg", "avatar_url", "data.user.avatar", "Data.User.Avatar"},
			"level":     {"level", "Level", "user_level", "userLevel", "data.user.level"},
			"text":      {"text", "Text", "content", "Content", "msg", "danmaku", "message", "data.content", "Data.Content"},
			"giftName":  {"giftName", "gift_name", "gift", "GiftName", "giftNameStr", "data.gift.name", "Data.Gift.Name"},
			"giftCount": {"giftCount", "gift_count", "count", "num", "repeat", "repeatCount", "data.gift.count", "Data.Gift.RepeatCount"},
			"giftValue": {"giftValue", "gift_value", "price", "diamond", "diamondCount", "data.gift.diamond", "Data.Gift.DiamondCount"},
			"guardLevel": {"guardLevel", "guard_level", "guardType", "data.guard.level", "Data.User.FansClubInfo.Data.GuardLevel"},
			"fansClubLevel": {"fansClubLevel", "fansclub_level", "fansClubLevelInt", "fanClubLevel", "fansLevel", "data.user.fansClubInfo.level", "Data.User.FansClubInfo.Level"},
			"ts":        {"ts", "TS", "timestamp", "time", "createTime", "data.ts"},
		},
		TypeAliases: map[string][]string{
			"danmaku":  {"danmaku", "chat", "comment", "message", "弹幕", "1", "WebcastChatMessage"},
			"gift":     {"gift", "礼物", "2", "WebcastGiftMessage"},
			"like":     {"like", "点赞", "3", "WebcastLikeMessage"},
			"enter":    {"enter", "member", "进场", "进入", "5", "WebcastMemberMessage"},
			"follow":   {"follow", "social", "关注", "6", "WebcastSocialMessage"},
			"guard":    {"guard", "星守护", "守护", "开通守护", "guardbuy", "WeekGuardMessage"},
			"fansclub": {"fansclub", "fans_club", "粉丝团", "加入粉丝团", "fansclubjoin", "FansClubMessage"},
		},
	}
}

// ConfigStore 配置读写
type ConfigStore struct {
	mu   sync.RWMutex
	cfg  Config
	path string
}

func loadConfig(dir string) *ConfigStore {
	cs := &ConfigStore{cfg: defaultConfig(), path: filepath.Join(dir, "config.json")}
	b, err := os.ReadFile(cs.path)
	if err != nil {
		_ = cs.write()
		return cs
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return cs
	}
	// 补齐缺省项
	def := defaultConfig()
	if c.Port <= 0 {
		c.Port = def.Port
	}
	if c.SFXDir == "" {
		c.SFXDir = def.SFXDir
	}
	if c.Mapping == nil {
		c.Mapping = def.Mapping
	} else {
		// 补齐新增字段的默认候选路径，不覆盖用户已改过的
		for k, v := range def.Mapping {
			if _, ok := c.Mapping[k]; !ok {
				c.Mapping[k] = v
			}
		}
	}
	if c.TypeAliases == nil {
		c.TypeAliases = def.TypeAliases
	} else {
		for k, v := range def.TypeAliases {
			if _, ok := c.TypeAliases[k]; !ok {
				c.TypeAliases[k] = v
			}
		}
	}
	if c.TTS.Mode == "" {
		c.TTS.Mode = "local"
	}
	if c.TTS.Volume == 0 {
		c.TTS.Volume = 90
	}
	if c.PadDuration == 0 {
		c.PadDuration = 5000
	}
	// 老配置只有 minLevel/minFans/minValue，没有分项开关：按阈值 >0 自动打开对应项
	if c.Welcome.Enabled && !c.Welcome.LevelOn && !c.Welcome.FansOn && !c.Welcome.ValueOn {
		c.Welcome.LevelOn = c.Welcome.MinLevel > 0
		c.Welcome.FansOn = c.Welcome.MinFans > 0
		c.Welcome.ValueOn = c.Welcome.MinValue > 0
	}
	if c.Mode == "" {
		c.Mode = def.Mode
	}
	if c.LLM.TimeoutMS == 0 {
		c.LLM.TimeoutMS = def.LLM.TimeoutMS
	}
	if c.LLM.System == "" {
		c.LLM.System = def.LLM.System
	}
	cs.cfg = c
	return cs
}

func (cs *ConfigStore) Get() Config {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.cfg
}

func (cs *ConfigStore) Set(c Config) error {
	cs.mu.Lock()
	cs.cfg = c
	cs.mu.Unlock()
	return cs.write()
}

func (cs *ConfigStore) write() error {
	cs.mu.RLock()
	b, err := json.MarshalIndent(cs.cfg, "", "  ")
	cs.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(cs.path, b, 0o644)
}
