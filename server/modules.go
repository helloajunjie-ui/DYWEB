package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Module 功能模块：分类化开关 + 参数容器
type Module struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Category string         `json:"category"`
	Desc     string         `json:"desc"`
	Ready    bool           `json:"ready"`
	Enabled  bool           `json:"enabled"`
	Testable bool           `json:"testable"`
	Params   map[string]any `json:"params"`
}

// Category 分类：小白按"直播哪个环节"理解，而不是按技术模块名
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
}

var categoryList = []Category{
	{ID: "ingest", Name: "① 接数据", Desc: "别家工具把弹幕、礼物、进场推给这里"},
	{ID: "talk", Name: "② 说话：AI 与语音", Desc: "AI 写台词、把话念出来、记住老粉"},
	{ID: "visual", Name: "③ 画面：礼物特效", Desc: "观众送礼时大屏放什么"},
	{ID: "game", Name: "④ 玩法：抽奖和整活", Desc: "转盘、猜数字、抓叛徒这类互动"},
	{ID: "audio", Name: "⑤ 声音：音效与提示", Desc: "叮、boom、倒计时这些声音"},
	{ID: "hardware", Name: "⑥ 外设：灯光和按键", Desc: "接灯、接键盘热键（要硬件才用得上）"},
	{ID: "ops", Name: "⑦ 后台：自动修复", Desc: "掉线、卡顿自己恢复，平时不用管"},
}

// ModuleStore 模块注册表
type ModuleStore struct {
	mu   sync.Mutex
	list []Module
	path string
}

func newModuleStore(dataDir string) *ModuleStore {
	ms := &ModuleStore{path: filepath.Join(dataDir, "modules.json"), list: defaultModules()}
	if b, err := os.ReadFile(ms.path); err == nil {
		var saved []Module
		if json.Unmarshal(b, &saved) == nil && len(saved) > 0 {
			// 用保存的启用态/参数覆盖默认注册表
			byID := map[string]Module{}
			for _, m := range saved {
				byID[m.ID] = m
			}
			for i := range ms.list {
				if s, ok := byID[ms.list[i].ID]; ok {
					ms.list[i].Enabled = s.Enabled
					if s.Params != nil {
						for k, v := range s.Params {
							ms.list[i].Params[k] = v
						}
					}
				}
			}
		}
	}
	return ms
}

func (ms *ModuleStore) List() []Module {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	out := make([]Module, len(ms.list))
	copy(out, ms.list)
	return out
}

func (ms *ModuleStore) Categories() []Category { return categoryList }

func (ms *ModuleStore) IsEnabled(id string) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, m := range ms.list {
		if m.ID == id {
			return m.Enabled
		}
	}
	return false
}

func (ms *ModuleStore) SetEnabled(id string, on bool) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for i := range ms.list {
		if ms.list[i].ID == id {
			ms.list[i].Enabled = on
			ms.saveLocked()
			return true
		}
	}
	return false
}

func (ms *ModuleStore) SetParams(id string, p map[string]any) bool {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for i := range ms.list {
		if ms.list[i].ID == id {
			if ms.list[i].Params == nil {
				ms.list[i].Params = map[string]any{}
			}
			for k, v := range p {
				ms.list[i].Params[k] = v
			}
			ms.saveLocked()
			return true
		}
	}
	return false
}

func (ms *ModuleStore) Get(id string) (Module, bool) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	for _, m := range ms.list {
		if m.ID == id {
			return m, true
		}
	}
	return Module{}, false
}

func (ms *ModuleStore) saveLocked() {
	b, err := json.MarshalIndent(ms.list, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(ms.path, b, 0o644)
}

// moduleForAction 规则动作 → 归属模块，用于能力开关
func moduleForAction(a Action) string {
	switch a.Type {
	case "tts":
		return "tts_local"
	case "sound":
		return "sfx_play"
	case "effect":
		if a.Preset == "" {
			return "fx_banner"
		}
		return "fx_" + a.Preset
	case "banner":
		return "fx_banner"
	case "webhook":
		return "hook_webhook"
	case "hotkey":
		return "key_hotkey"
	case "llm":
		return "llm_chat"
	default:
		return ""
	}
}

func defaultModules() []Module {
	fx := func(id, name, desc string) Module {
		return Module{ID: id, Name: name, Category: "visual", Desc: desc, Ready: true, Enabled: true, Testable: true,
			Params: map[string]any{"durationMs": 4000}}
	}
	return []Module{
		{ID: "http_push", Name: "HTTP 推送接入", Category: "ingest", Desc: "第三方工具 POST 到 /push", Ready: true, Enabled: true, Testable: true},
		{ID: "ws_ingest", Name: "WebSocket 推送接入", Category: "ingest", Desc: "长连接推送到 /ws/ingest", Ready: true, Enabled: true},
		{ID: "field_mapping", Name: "字段映射", Category: "ingest", Desc: "把工具的字段映射成统一事件", Ready: true, Enabled: true, Testable: true},

		{ID: "tts_local", Name: "本机语音播报", Category: "talk", Desc: "负责把字念出来（嗓子），和 AI 不冲突", Ready: true, Enabled: true, Testable: true,
			Params: map[string]any{"rate": 0, "volume": 90, "mode": "local"}},
		{ID: "tts_browser", Name: "网页语音朗读", Category: "talk", Desc: "换一种嗓音念，和本机播报二选一，别同时开", Ready: true, Enabled: false},
		{ID: "llm_chat", Name: "AI 话术生成", Category: "talk", Desc: "OpenAI 兼容接口，生成后自动播报", Ready: true, Enabled: false, Testable: true},
		{ID: "fan_memory", Name: "观众画像记忆", Category: "talk", Desc: "老粉识别、进场提醒、{fanNote} 注入", Ready: true, Enabled: true, Testable: true},
		{ID: "cloud_tts", Name: "云 TTS 音色", Category: "talk", Desc: "火山/阿里云音色，需填写凭证", Ready: false, Enabled: false},
		{ID: "voice_clone", Name: "主播音色克隆", Category: "talk", Desc: "本地 GPT-SoVITS 服务对接", Ready: false, Enabled: false},

		fx("fx_fireworks", "烟花", "大礼物主特效"),
		fx("fx_hearts", "爱心雨", "小额礼物、连击"),
		fx("fx_meteor", "流星", "快速飞过，适合进场"),
		fx("fx_confetti", "彩带", "成就/中奖"),
		fx("fx_shake", "画面抖动", "重击感"),
		fx("fx_spotlight", "聚光扫过", "点名/聚焦"),
		{ID: "fx_banner", Name: "大字横幅", Category: "visual", Desc: "感谢/进场大字", Ready: true, Enabled: true, Testable: true,
			Params: map[string]any{"durationMs": 5000, "color": "#FFD24A"}},
		{ID: "fx_physics", Name: "物理挂件", Category: "visual", Desc: "Matter.js 落体堆叠重砸", Ready: false, Enabled: false},
		{ID: "fx_media", Name: "素材叠加", Category: "visual", Desc: "绿幕/鬼畜视频贴图", Ready: false, Enabled: false},

		{ID: "game_pusher", Name: "推币机", Category: "game", Desc: "挂机盲盒/云推币", Ready: false, Enabled: false},
		{ID: "game_tug", Name: "阵营拔河", Category: "game", Desc: "红蓝能量对抗", Ready: false, Enabled: false},
		{ID: "game_box", Name: "宝箱能量条", Category: "game", Desc: "弹幕+礼物攒能量开奖", Ready: false, Enabled: false},
		{ID: "game_woodfish", Name: "木鱼功德", Category: "game", Desc: "扣1敲木鱼，飘字功德", Ready: false, Enabled: false},
		{ID: "game_debate", Name: "AI 双人对线", Category: "game", Desc: "两个 AI 就弹幕辩题互怼", Ready: false, Enabled: false},

		{ID: "sfx_play", Name: "音效播放", Category: "audio", Desc: "本地音效库，本机+页面双通道", Ready: true, Enabled: true, Testable: true},
		{ID: "bgm", Name: "背景音乐切换", Category: "audio", Desc: "按事件切换 BGM", Ready: false, Enabled: false},
		{ID: "audio_hrtf", Name: "空间音频", Category: "audio", Desc: "PannerNode HRTF 双耳定位", Ready: false, Enabled: false},

		{ID: "hook_webhook", Name: "Webhook 联动", Category: "hardware", Desc: "灯光/继电器/机械臂", Ready: true, Enabled: true, Testable: true},
		{ID: "key_hotkey", Name: "模拟按键", Category: "hardware", Desc: "礼物触发键盘组合键", Ready: true, Enabled: true, Testable: true},
		{ID: "obs_ws", Name: "OBS 联动", Category: "hardware", Desc: "换场景/开关录制（obs-websocket v5）", Ready: false, Enabled: false},
		{ID: "iot_hue", Name: "飞利浦 Hue", Category: "hardware", Desc: "房间灯光变色", Ready: false, Enabled: false},
		{ID: "iot_mijia", Name: "米家", Category: "hardware", Desc: "智能家居联动", Ready: false, Enabled: false},

		{ID: "merge_antispam", Name: "特效合并防刷", Category: "ops", Desc: "1s 窗口聚合，强度分级", Ready: true, Enabled: true, Testable: true},
		{ID: "overlay_watchdog", Name: "overlay 看门狗", Category: "ops", Desc: "30s 无消息自动刷新", Ready: true, Enabled: true},
		{ID: "fps_guard", Name: "帧率护栏", Category: "ops", Desc: "掉帧自动降低粒子上限", Ready: true, Enabled: true},
	}
}
