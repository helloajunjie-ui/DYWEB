package main

// Event 归一化后的直播事件
type Event struct {
	ID        int64   `json:"id"`
	TS        int64   `json:"ts"`
	Type      string  `json:"type"` // danmaku|gift|like|enter|follow|guard|fansclub|unknown
	User      string  `json:"user"`
	UserID    string  `json:"userId"`
	Avatar    string  `json:"avatar"`
	Level     int     `json:"level"`
	GuardLevel int    `json:"guardLevel"` // 星守护等级 1~3，0=非守护
	FansClubLevel int `json:"fansClubLevel"` // 粉丝团等级，0=没加团
	Text      string  `json:"text"`
	GiftName  string  `json:"giftName"`
	GiftCount int     `json:"giftCount"`
	GiftValue float64 `json:"giftValue"` // 单价，抖币
	Total     float64 `json:"total"`     // 单价 × 数量
	Raw       string  `json:"raw"`

	// 运行时填充
	HitRules  []string `json:"hitRules,omitempty"`
	FanNote   string   `json:"fanNote,omitempty"`
	IsRegular bool     `json:"isRegular,omitempty"`
}

// EffectCmd 发给 overlay 的特效指令
type EffectCmd struct {
	Preset     string `json:"preset"` // fireworks|hearts|meteor|confetti|shake|spotlight|banner
	DurationMS int    `json:"durationMs"`
	Text       string `json:"text"`
	User       string `json:"user"`
	Avatar     string `json:"avatar"`
	Gift       string `json:"gift"`
	Count      int    `json:"count,omitempty"`
	Color      string `json:"color,omitempty"`
	Sound      string `json:"sound,omitempty"`
	Strength   int    `json:"strength,omitempty"` // 合并强度分级 1~4
}

// Rule 规则
type Rule struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Enabled    bool     `json:"enabled"`
	Match      MatchSpec `json:"match"`
	Actions    []Action `json:"actions"`
	CooldownMS int      `json:"cooldownMs"`
	Priority   int      `json:"priority"`
	HitCount   int64    `json:"hitCount"`
	LastHit    int64    `json:"lastHit"`
}

// MatchSpec 匹配条件
type MatchSpec struct {
	Types        []string `json:"types"`        // 空=不限
	Keyword      string   `json:"keyword"`      // 弹幕关键词
	KeywordMode  string   `json:"keywordMode"`  // contains|regex|equals|prefix
	GiftNames    []string `json:"giftNames"`    // 礼物名，支持包含匹配；空=不限
	MinValue     float64  `json:"minValue"`     // 礼物最小单价
	MinCount     int      `json:"minCount"`     // 最小数量
	MinLevel     int      `json:"minLevel"`     // 观众最低等级
	UserName     string   `json:"userName"`     // 指定观众（昵称包含）
	ComboWithinMS int     `json:"comboWithinMs"` // 连击累计窗口(ms)
	OnlyRegular  bool     `json:"onlyRegular"`  // 仅老粉（互动>=N次）
	RegularMin   int      `json:"regularMin"`   // 老粉判定阈值
}

// Action 动作
type Action struct {
	Type       string            `json:"type"` // tts|sound|effect|banner|webhook|llm|hotkey
	Template   string            `json:"template"`
	File       string            `json:"file"`
	Volume     float64           `json:"volume"`
	Preset     string            `json:"preset"`
	DurationMS int               `json:"durationMs"`
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers,omitempty"`
	Keys       string            `json:"keys"`   // hotkey: 如 ctrl+space / f5
	LLMPrompt  string            `json:"llmPrompt"` // llm: 追加提示词
	Color      string            `json:"color"`
	Merge      bool              `json:"merge"` // 1s 窗口合并，防刷屏
}

// Stats 统计快照
type Stats struct {
	StartedAt   int64            `json:"startedAt"`
	TotalEvents int64            `json:"totalEvents"`
	Danmaku     int64            `json:"danmaku"`
	Gifts       int64            `json:"gifts"`
	Likes       int64            `json:"likes"`
	Enters      int64            `json:"enters"`
	Follows     int64            `json:"follows"`
	Guards      int64            `json:"guards"`
	FansClubs   int64            `json:"fansClubs"`
	GiftValue   float64          `json:"giftValue"`
	PerMinute   float64          `json:"perMinute"`
	RuleHits    int64            `json:"ruleHits"`
	TopGifters   []GifterStat     `json:"topGifters"`
	LastEventTS int64            `json:"lastEventTs"`
	ByType      map[string]int64 `json:"byType"`
}

// GifterStat 送礼榜条目
type GifterStat struct {
	User   string  `json:"user"`
	Value  float64 `json:"value"`
	Count  int     `json:"count"`
	Avatar string  `json:"avatar"`
}

// FanNote 一条人工备注（带时间，方便回看）
type FanNote struct {
	TS   int64  `json:"ts"`
	Text string `json:"text"`
}

// Fan 观众画像记忆节点
type Fan struct {
	Key        string  `json:"key"` // 归一化后的 key
	Name       string  `json:"name"`
	Avatar     string  `json:"avatar"`
	Level      int     `json:"level"`
	MsgCount   int     `json:"msgCount"`
	GiftCount  int     `json:"giftCount"`
	GiftValue  float64 `json:"giftValue"`
	FirstSeen  int64   `json:"firstSeen"`
	LastSeen   int64   `json:"lastSeen"`
	Note       string  `json:"note"`   // 人工备注（最近一条，注入 {fanNote}）
	Notes      []FanNote `json:"notes"` // 备注历史：主播可以反复补，不被覆盖
	Tags       []string `json:"tags"`  // 标签
	Regular    bool    `json:"regular"` // 自动判定老粉
	GuardLevel int     `json:"guardLevel"` // 星守护等级（取最高）
	FansClub   bool    `json:"fansClub"`   // 已加入粉丝团
	FansClubLevel int  `json:"fansClubLevel"` // 粉丝团等级
	Followed   bool    `json:"followed"`   // 已关注主播
}
