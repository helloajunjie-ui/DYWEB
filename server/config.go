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

// LiveMode 直播定位：两个轴——讲什么（娱乐/电商）× 有没有人（真人在/无人值守），组合成四种
type LiveMode struct {
	ID     string   `json:"id"`
	Short  string   `json:"short"`
	Name   string   `json:"name"`
	Desc   string   `json:"desc"`
	Focus  string   `json:"focus"`
	Script string   `json:"script"` // 话术库默认分类
	Scene  string   `json:"scene"`  // 话术库默认场景：fun / ecom
	Rec    []string `json:"rec"`    // 推荐模块 id
	Tips   []string `json:"tips"`
}

var liveModes = []LiveMode{
	{
		ID:     "fun",
		Short:  "娱乐",
		Name:   "娱乐直播（真人在）",
	Desc:   "你本人在镜头前，场控替你看弹幕、接礼物、救冷场",
		Focus:  "礼物感谢要快、冷场要有人接话、整活要跟得上",
		Script: "keep",
		Scene:  "fun",
		Rec:    []string{"tts_local", "llm_chat", "fan_memory", "fx_particles", "fx_banner", "sfx_play", "key_hotkey"},
		Tips: []string{
			"礼物规则门槛别设太高，1 币也谢，气势要有",
			"冷场时点「话术库 → 冷场救场」念一句，比干坐着强",
			"整蛊按键会把键按在当前前台窗口，直播时把游戏窗口切到前台",
		},
	},
	{
		ID:     "fun_auto",
		Short:  "无人娱乐",
		Name:   "无人娱乐直播（做游戏和聊天）",
		Desc:   "人不在镜头前，靠游戏轮换和 AI 聊天撑场，弹幕来了自动接",
		Focus:  "别冷场：游戏一轮接一轮，聊天有人接",
		Script: "interact",
		Scene:  "fun",
		Rec:    []string{"llm_chat", "tts_local", "merge_antispam", "overlay_watchdog", "fps_guard", "game_debate", "fx_particles"},
		Tips: []string{
			"游戏别手动一个个点：用「手机中控台」循环开转盘 / 猜数字 / 名字抽奖，一轮接一轮",
			"聊天交给 AI：先设好 AI 人设（比如陪聊室友），弹幕问什么它答什么",
			"无人时先开「特效合并防刷」和「帧率护栏」，画面别堆粒子堆到卡死",
		},
	},
	{
		ID:     "ecom",
		Short:  "电商",
		Name:   "电商直播（真人在）",
		Desc:   "你本人在镜头前讲品，AI 当你的助播",
		Focus:  "产品资料要录全、避坑话要说、退单率要压",
		Script: "push",
		Scene:  "ecom",
		Rec:    []string{"llm_chat", "tts_local", "tts_browser", "fx_banner", "hook_webhook", "fan_memory", "obs_ws"},
		Tips: []string{
			"先去「产品库」录：卖点、适合谁、不适合谁、常见问题，AI 才有料讲",
			"把主推商品打开开关，AI 生成的话会以它为准，不会编参数",
			"话术库里「促单 · 算账」「退货率友好版」是压退货率的主力",
		},
	},
	{
		ID:     "ecom_auto",
		Short:  "无人电商",
		Name:   "无人电商直播（自动推销和安利）",
		Desc:   "人不在镜头前，自动循环讲品、安利卖点、答疑，接住进来的流量",
		Focus:  "产品资料要全、口播要循环但不烦人、别让 AI 乱承诺",
		Script: "push",
		Scene:  "ecom",
		Rec:    []string{"llm_chat", "tts_local", "merge_antispam", "overlay_watchdog", "fps_guard", "hook_webhook", "obs_ws"},
		Tips: []string{
			"先把「产品库」和主推商品录好，无人循环只讲这一条，别让它自己编",
			"循环口播用「话术库 → 卖货」里的安利版，一段接一段，中间插感谢和避坑",
			"AI 人设里加一句「不许承诺价格、不许说绝对」，无人时最怕口播事故",
		},
	},
}

func modeByID(id string) (LiveMode, bool) {
	for _, m := range liveModes {
		if m.ID == id {
			return m, true
		}
	}
	return LiveMode{}, false
}

// ModeScene 把直播定位换算成话术场景（无人不影响「讲什么」）
func ModeScene(mode string) string {
	if m, ok := modeByID(mode); ok {
		return m.Scene
	}
	return ""
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
	mu         sync.RWMutex
	cfg        Config
	path       string
	secretPath string
}

func loadConfig(dir string) *ConfigStore {
	cs := &ConfigStore{cfg: defaultConfig(), path: filepath.Join(dir, "config.json"), secretPath: filepath.Join(dir, "secret.dat")}
	b, err := os.ReadFile(cs.path)
	if err != nil {
		_ = cs.write()
		return cs
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return cs
	}
	legacyPlain := c.LLM.APIKey != ""
	if legacyPlain {
		// 老版本把 Key 明文写在 config.json：先读出来，稍后加密迁移
	} else if raw, err := os.ReadFile(cs.secretPath); err == nil {
		if key, derr := unprotectSecret(string(raw)); derr == nil {
			c.LLM.APIKey = key
		}
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
	// 老配置把「无人」当成第三种直播类型（auto），现在拆成「有没有人 × 讲什么」两个轴：auto 按无人娱乐迁移
	if c.Mode == "auto" {
		c.Mode = "fun_auto"
	}
	if _, ok := modeByID(c.Mode); !ok {
		c.Mode = "fun"
	}
	if c.LLM.TimeoutMS == 0 {
		c.LLM.TimeoutMS = def.LLM.TimeoutMS
	}
	if c.LLM.System == "" {
		c.LLM.System = def.LLM.System
	}
	cs.cfg = c
	if legacyPlain {
		// 明文迁移：加密写入 secret.dat，同时把 config.json 里的 apiKey 清空
		_ = cs.write()
	}
	return cs
}

// masked 返回可安全落盘/下发的配置副本：不含明文 Key
func (cs *ConfigStore) masked() Config {
	c := cs.cfg
	c.LLM.APIKey = ""
	return c
}

func (cs *ConfigStore) Get() Config {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.cfg
}

func (cs *ConfigStore) Set(c Config) error {
	cs.mu.Lock()
	// 前端拿不到明文 Key：空值表示「沿用已保存的那把」，不能把它当成清空
	if c.LLM.APIKey == "" {
		c.LLM.APIKey = cs.cfg.LLM.APIKey
	}
	cs.cfg = c
	cs.mu.Unlock()
	return cs.write()
}

func (cs *ConfigStore) write() error {
	cs.mu.RLock()
	enc, err := protectSecret(cs.cfg.LLM.APIKey)
	if err != nil {
		cs.mu.RUnlock()
		return err
	}
	b, err := json.MarshalIndent(cs.masked(), "", "  ")
	cs.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.WriteFile(cs.path, b, 0o644); err != nil {
		return err
	}
	if enc != "" {
		if err := os.WriteFile(cs.secretPath, []byte(enc), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// HasAPIKey 供接口判断「本机是否已存了 Key」（不返回明文）
func (cs *ConfigStore) HasAPIKey() bool {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.cfg.LLM.APIKey != ""
}
