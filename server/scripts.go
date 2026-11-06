package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Script 一条主播话术
type Script struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	// Scene 适用场景：""=通用，"fun"=娱乐，"ecom"=卖货，"auto"=无人
	Scene   string `json:"scene"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Builtin bool   `json:"builtin"`
}

type ScriptCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
}

var scriptCategories = []ScriptCategory{
	{ID: "keep", Name: "留人 · 破冰", Desc: "刚进来的别划走：先聊场景，再给看点"},
	{ID: "push", Name: "促单 · 算账", Desc: "把总价拆成每天几毛，帮观众做决定"},
	{ID: "interact", Name: "互动 · 共鸣", Desc: "让公屏动起来，制造归属感"},
	{ID: "thanks", Name: "感谢 · 捧场", Desc: "礼物、关注、粉丝团、守护的回应"},
	{ID: "cold", Name: "冷场 · 救场", Desc: "没人说话时说什么"},
	{ID: "close", Name: "下播 · 收尾", Desc: "临走前留下一次回访理由"},
}

// defaultScripts 2026 主流打法：场景留人、算账促单、平等互动
func defaultScripts() []Script {
	raw := []Script{
		// 留人 · 破冰
		{Category: "keep", Scene: "ecom", Title: "聊天式留人（下班场景）", Text: "刚刚进来的姐妹，如果你也是刚下班、刚接完孩子，先别划走。咱家直播间不卖概念，就聊聊怎么花最少的钱把这事解决掉。"},
		{Category: "keep", Title: "点名头像找话题", Text: "欢迎{user}，你头像这个风景真好看。刚好刚跟进来的老粉在聊今天这天气，你也一起聊两句呗。"},
		{Category: "keep", Scene: "ecom", Title: "钩子式留人（3 分钟）", Text: "进来的宝宝先别划走，看我手上这个版型，今年大牌都在做的设计。今天机制很炸，我只讲 3 分钟，讲完就过。"},
		{Category: "keep", Title: "老粉回流招呼", Text: "{user}你又来啦，上次你说要的那个我给你留着了，一会儿细讲。"},
		{Category: "keep", Title: "新号低人流自救", Text: "现在人不多，正好，人少的时候我最敢说真话。你想问什么直接打在公屏，我一个一个回。"},
		{Category: "keep", Scene: "fun", Title: "娱乐开场", Text: "欢迎来到我的直播间，今天不整别的，就陪你们聊天、唱歌、玩游戏。想听什么、想看什么，直接打在公屏。"},
		{Category: "keep", Scene: "fun", Title: "娱乐留人（下一首更好听）", Text: "刚进来的别急着走，这首歌完了下一首更好听，你们点歌我唱，跑调也算节目效果。"},

		// 促单 · 算账
		{Category: "push", Scene: "ecom", Title: "算账式促单（拆到每天）", Text: "姐妹们别看总价，咱算一笔账。这个东西你最少能用一年，折到每天才几毛钱，一杯奶茶钱换一个改变，还送运费险，试错成本等于零。"},
		{Category: "push", Scene: "ecom", Title: "正向利他（帮避坑）", Text: "大家不用盲目拍，我先帮你们避个坑。你是这种情况就拍 1 号链接，你是那种情况今天别在我这花冤枉钱。我要的是你收到觉得好下次还来。"},
		{Category: "push", Scene: "ecom", Title: "不催单版本", Text: "我不催你们，真的。想清楚再拍，但是机制今天到期，明天恢复原价我也没办法，你们自己衡量。"},
		{Category: "push", Scene: "ecom", Title: "对比替代品", Text: "同样的东西外边卖多少你们心里有数，我这儿不玩虚的，差价就在机制上，剩下的钱你自己揣兜里。"},
		{Category: "push", Scene: "ecom", Title: "退货率友好版", Text: "我实话讲，这个不适合所有人。适合你的我劝你拍，不适合你的我劝你别拍，我不想赚你一次退货的钱。"},

		// 互动 · 共鸣
		{Category: "interact", Scene: "ecom", Title: "公屏归属感", Text: "公屏上打出「想要」的姐妹，你们名字我都看到了。今天哪怕我自己亏，也把这个福利给你们申请下来，咱们自己人绝不吃亏。"},
		{Category: "interact", Title: "找共鸣（扣累）", Text: "觉得平时上班带娃太累的，公屏扣个「累」。今天这儿就是个树洞，我一边陪你们聊，一边给你们找点乐子，不开心的都留在这。"},
		{Category: "interact", Scene: "ecom", Title: "点人互动", Text: "{user}你刚说这个颜色好看，你平时穿什么码？我按你的体型给你选，别买错了。"},
		{Category: "interact", Scene: "ecom", Title: "抛选择题", Text: "两个颜色，扣 1 要黑色，扣 2 要米色，扣得多的那个我直接给你们上机制。"},
		{Category: "interact", Title: "整点小游戏", Text: "来，玩个简单的：公屏扣一个你现在的城市，我抽一个送小礼物，就现在。"},
		{Category: "interact", Scene: "fun", Title: "才艺点歌", Text: "想听歌的扣歌名，今天点歌不限量。唱得好你们刷个小心心，唱得不好你们也别走，看我翻车也是节目。"},
		{Category: "interact", Scene: "fun", Title: "整活惩罚", Text: "输的人接受惩罚，惩罚你们来定：做鬼脸、学猫叫、说一句土味情话，今晚我认了。"},
		{Category: "interact", Scene: "fun", Title: "连麦 / PK 招呼", Text: "有想连麦的扣个「连」，凑够三个我就开麦。别害羞，来这儿就是玩，不上票也行。"},

		// 感谢 · 捧场
		{Category: "thanks", Scene: "fun", Title: "感谢礼物（通用）", Text: "谢谢{user}的{gift}，哥这一下我看见了，真的记住了。"},
		{Category: "thanks", Scene: "fun", Title: "感谢大礼物", Text: "{user}送出{gift}，全场给大哥让让道！今天这场因为你有排面了。"},
		{Category: "thanks", Title: "感谢关注", Text: "感谢{user}的关注，常来玩，我记住你头像了。"},
		{Category: "thanks", Title: "感谢加入粉丝团", Text: "欢迎{user}加入粉丝团，牌牌亮起来，以后你就是自己人了。"},
		{Category: "thanks", Title: "感谢星守护", Text: "恭喜{user}开守护，直播间有守护神了，今天我说什么都有人给我兜底。"},
		{Category: "thanks", Scene: "fun", Title: "榜上点名", Text: "榜上的名字我念一遍：谢谢今天陪着我的每一位，你们不是观众，是我直播间的老熟人。"},

		// 冷场 · 救场
		{Category: "cold", Title: "冷场自嘲", Text: "公屏怎么突然安静了，是不是我说得太干？那换个方式，你们问，我答，问什么答什么。"},
		{Category: "cold", Title: "抛问题救场", Text: "我抛个问题：如果你只能留三样东西，除了手机你留什么？公屏告诉我。"},
		{Category: "cold", Title: "讲个自己的事", Text: "说个我自己的事吧，昨天我也翻车了，正好跟你们说说，别踩我踩过的坑。"},

		// 下播 · 收尾
		{Category: "close", Title: "下播留回访理由", Text: "今天就到这，明天这个点我还开，专门讲你们今天没问完的那几个问题，别到时候找不到我。"},
		{Category: "close", Title: "下播感谢", Text: "谢谢今天陪我到最后的人，名字我都记着，明天见。"},

		// 无人直播 · 循环口播
		{Category: "keep", Scene: "auto", Title: "无人开场（讲清楚在卖什么）", Text: "欢迎进来的朋友，这里全天在线。今天讲的是这几样，想了解哪个直接打编号，我马上给你讲。"},
		{Category: "keep", Scene: "auto", Title: "无人循环引关注", Text: "新来的朋友点个关注，关注之后不迷路，我这儿每天这个点都在，东西和价格都给你摆明白。"},
		{Category: "cold", Scene: "auto", Title: "无人自动应答（没听清）", Text: "刚才有朋友问的问题我再讲一遍，怕新进来的没听到，重点就这一句：适合的再拍，不适合的我也不劝。"},
	}
	out := make([]Script, 0, len(raw))
	for i := range raw {
		raw[i].ID = "s_builtin_" + itoa(i+1)
		raw[i].Builtin = true
		out = append(out, raw[i])
	}
	return out
}

// ScriptStore 内置话术 + 主播自己加的话术
type ScriptStore struct {
	mu     sync.Mutex
	custom []Script
	path   string
}

func newScriptStore(dataDir string) *ScriptStore {
	s := &ScriptStore{path: filepath.Join(dataDir, "scripts.json")}
	if b, err := os.ReadFile(s.path); err == nil {
		var saved []Script
		if json.Unmarshal(b, &saved) == nil {
			s.custom = saved
		}
	}
	return s
}

// List 内置在前，自定义在后
func (s *ScriptStore) List() []Script {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Script{}, defaultScripts()...)
	for _, c := range s.custom {
		c.Builtin = false
		if c.ID == "" {
			c.ID = "s_custom_" + strings.ReplaceAll(c.Title, " ", "_")
		}
		out = append(out, c)
	}
	return out
}

// Relevant 按关键词重合度给 AI 挑几条参考话术；scene 非空时只挑「通用」和该场景
func (s *ScriptStore) Relevant(query string, n int, scene string) []Script {
	all := s.List()
	qt := bigrams(query)
	if len(qt) == 0 || n <= 0 {
		return nil
	}
	type scored struct {
		s Script
		v int
	}
	list := make([]scored, 0, len(all))
	for _, it := range all {
		// 场景隔离：娱乐的话术不该被卖货场景拿去用，反之亦然
		if scene != "" && it.Scene != "" && it.Scene != scene {
			continue
		}
		body := bigrams(it.Title + it.Text)
		hit := 0
		for g := range qt {
			if body[g] {
				hit++
			}
		}
		if hit > 0 {
			list = append(list, scored{it, hit})
		}
	}
	// 简单的选择排序取前 n
	for i := 0; i < len(list) && i < n; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].v > list[i].v {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	out := []Script{}
	for i := 0; i < len(list) && i < n; i++ {
		out = append(out, list[i].s)
	}
	return out
}

func bigrams(s string) map[string]bool {
	s = strings.Map(func(r rune) rune {
		if r < 128 {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				return r
			}
			return -1
		}
		return r
	}, s)
	rs := []rune(s)
	m := map[string]bool{}
	for i := 0; i < len(rs)-1; i++ {
		m[string(rs[i:i+2])] = true
	}
	if len(rs) == 1 {
		m[string(rs)] = true
	}
	return m
}

// 保留一个入口以免未来 store 需要别的检索方式
func (s *ScriptStore) All() []Script { return s.List() }

// SetCustom 整表替换自定义部分
func (s *ScriptStore) SetCustom(list []Script) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	clean := make([]Script, 0, len(list))
	for _, it := range list {
		if strings.TrimSpace(it.Text) == "" {
			continue
		}
		it.Builtin = false
		if it.ID == "" {
			it.ID = "s_custom_" + itoa(len(clean)+1)
		}
		if it.Category == "" {
			it.Category = "keep"
		}
		if it.Scene != "" && it.Scene != "fun" && it.Scene != "ecom" && it.Scene != "auto" {
			it.Scene = ""
		}
		clean = append(clean, it)
	}
	s.custom = clean
	b, _ := json.MarshalIndent(clean, "", "  ")
	return os.WriteFile(s.path, b, 0o644)
}
