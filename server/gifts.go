package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Gift 一个礼物的价目：Value 为单价（钻石，与抖币口径一致）
type Gift struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// GiftStore 礼物价目表：第三方工具常常只推礼物名，靠这张表算出价值
type GiftStore struct {
	mu    sync.RWMutex
	items []Gift
	index map[string]float64
	path  string
}

func newGiftStore(dataDir string) *GiftStore {
	g := &GiftStore{path: filepath.Join(dataDir, "gifts.json")}
	g.load()
	return g
}

func (g *GiftStore) load() {
	b, err := os.ReadFile(g.path)
	if err == nil {
		var list []Gift
		if json.Unmarshal(b, &list) == nil && len(list) > 0 {
			g.setItems(list)
			return
		}
	}
	g.setItems(defaultGifts())
	g.save()
}

func (g *GiftStore) setItems(list []Gift) {
	idx := make(map[string]float64, len(list))
	for _, it := range list {
		if k := giftKey(it.Name); k != "" {
			idx[k] = it.Value
		}
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Value == list[j].Value {
			return list[i].Name < list[j].Name
		}
		return list[i].Value < list[j].Value
	})
	g.mu.Lock()
	g.items = list
	g.index = idx
	g.mu.Unlock()
}

func (g *GiftStore) save() {
	g.mu.RLock()
	list := g.items
	g.mu.RUnlock()
	b, _ := json.MarshalIndent(list, "", "  ")
	_ = os.WriteFile(g.path, b, 0o644)
}

func (g *GiftStore) List() []Gift {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]Gift, len(g.items))
	copy(out, g.items)
	return out
}

// Replace 整表替换（控制台保存时用）
func (g *GiftStore) Replace(list []Gift) {
	g.setItems(list)
	g.save()
}

// Value 查价：先精确、再包含（长名优先，「蝶连理枝」也能命中「蝶·连理枝」）
func (g *GiftStore) Value(name string) (float64, bool) {
	k := giftKey(name)
	if k == "" {
		return 0, false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if v, ok := g.index[k]; ok {
		return v, true
	}
	best := ""
	bestVal := 0.0
	for n, v := range g.index {
		if strings.Contains(k, n) || strings.Contains(n, k) {
			if len(n) > len(best) {
				best, bestVal = n, v
			}
		}
	}
	if best != "" {
		return bestVal, true
	}
	return 0, false
}

// giftKey 归一化礼物名：去空格、去符号、小写
func giftKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '·', '・', '.', '-', '_', '/', '\\', '（', '）', '(', ')', '「', '」':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// defaultGifts 解析内置价目清单（格式：名称 空格 单价，一行一个）
func defaultGifts() []Gift {
	seen := map[string]bool{}
	list := make([]Gift, 0, 320)
	for _, line := range strings.Split(giftCatalogText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseFloat(fields[len(fields)-1], 64)
		if err != nil {
			continue
		}
		name := strings.Join(fields[:len(fields)-1], "")
		k := giftKey(name)
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		list = append(list, Gift{Name: name, Value: v})
	}
	return list
}

// giftCatalogText 内置抖音礼物价目（钻石），可在控制台的「礼物库」里改
const giftCatalogText = `
玫瑰 1
小心心 1
人气票 1
番茄 1
抖音 1
大啤酒 2
你最好看 2
棒棒糖 9
一起打call 9
心疼 9
给到夯 9
服了 9
鲜花 10
加油鸭 15
送你花花 49
爱你哟 52
亲吻 99
Thuglife 99
闪耀星辰 99
捏捏小脸 99
荧光律动 99
龙抬头 99
工坊宝箱 99
荧光棒 99
趣玩泡泡 99
黄桃罐头 99
爱的纸鹤 99
多喝热水 126
真爱誓言 199
礼花筒 199
比心 199
为你爆灯 199
为你举牌 199
拳拳出击 199
比心兔兔 299
可以可以 299
不行不行 299
爱的守护 299
岁月情书 299
ONE礼挑一 299
星星点灯 268
蝶·连理枝 280
真爱玫瑰 366
一束花开 366
花开烂漫 466
真的爱你 520
热气球 520
永生花 520
娶你回家 599
浪漫花火 599
爱情树下 599
纪念派对 600
PK宝箱 600
灵龙现世 600
万象烟花 688
日出相伴 726
掌上明珠 888
璀璨舞台 899
纸短情长 921
一千零一夜 1001
真爱钻戒 1314
跑车 1200
无限热爱 1200
真爱永恒 8999
真爱至上 9999
千金归来 10000
抖音1号 10001
为爱启航 10001
一千零一夜 1001
花落长亭 1588
浪漫营地 1699
一路有你 17999
倾心于你 1888
浪漫满怀 1888
一眼万年 1888
绣球传情 1888
人鱼之恋 1888
花间浪漫 1888
炫彩射击 1888
单车恋人 1899
许愿天灯 1999
浪漫恋人 1999
光之祝福 1999
奇幻八音盒 2399
花海泛舟 2800
环游世界 3000
私人飞机 3000
冰冻战车 3000
直升机 2999
薰衣草庄园 3300
爱意久久 2999
海上生明月 4166
心动丘比特 4321
火龙爆发 5000
大小姐驾到 5000
春日秘境 5200
一眼心动 5200
为你心动 5200
豪华邮轮 6000
天空之镜 6399
月下瀑布 6666
夜色倾心 6999
星际玫瑰 7500
云霄大厦 7888
摩天大厦 8222
情定三生 9666
抖音飞艇 20000
蔷薇骑士 20000
梦幻城堡 28888
浪漫马车 28888
嘉年华 30000
云中秘境 13140
奏响人生 3666
哪吒发髻 99
石矶娘娘 199
蓝色龙角 99
暖冬套装 99
洗把脸 99
情丝梳理 99
爱的通话 99
节奏快板 99
掉星星啦 299
爆米花礼帽 299
流光翼彩 299
花朵精灵帽 199
大拉翅旗头 199
刷个牙 199
蝶翼灵辉 199
海星QQ糖 299
甜蜜猫耳 99
八戒帽 199
成为悟空 199
情侣杯 199
爱心冰激凌 99
鼻尖冰淇淋 99
西瓜头盔 199
如蕉似漆 99
冰爽果饮 199
夏威夷花环 99
鸭鸭泳圈 99
葡式蛋挞 99
章鱼小丸子 99
爱心便当 99
爽口牛排 399
串串香 199
手打柠檬茶 99
寿司 99
小龙虾 299
水果捞 99
红烧排骨 99
奶油可颂泡芙 99
肉夹馍 99
东坡肉 199
奥尔良鸡翅 99
土豆牛肉 99
顶榴 99
为你画心 99
六月的雨 199
电竞耳机 99
学士帽 199
簪花学士帽 199
花之蝶舞 99
亚比囧囧囧 99
晚安贴贴 99
锅包肉 99
草莓蛋糕 99
热辣拳拳 99
甜酷耳机 99
烤个地瓜 199
我是大厨 199
金箍咒 99
吃口鱼粉 99
爱的花花 99
甜蜜小熊猫 199
爱意UFO 99
飞吻狗狗 199
人鱼发饰 199
吃个汉堡 299
宝莲灯 99
松鼠发箍 199
得闲饮茶 99
肥牛火锅 99
喝杯咖啡 99
赠你玫瑰 99
春日花环 199
莲花彩灯 199
太阳花 299
爱心小拳拳 199
上电视了 199
拳击套 99
可爱发帽 99
兔兔甜甜圈 99
为你亮灯 99
黏人小喵 199
水壶头盔 199
光明之角 99
喝杯茶呀 99
塑胶黄鸭 199
让人头大 99
童趣纸盒 299
小猫咪 299
好运莲莲鸭 99
灵动小鸟 199
大侠来了 199
犀牛头套 299
小提琴家 99
鸭鸭帽 199
陕北毛巾 99
六六六 99
恶霸狗狗 299
爱心礼盒 99
牛奶盒子 299
小驴驴 199
火烈鸟帽 199
吃臭豆腐 99
喝杯热水 99
飞碟入侵 199
小松鼠 299
荣耀之角 99
状元帽 299
橘猫帽 199
小熊猫帽 99
三角龙 299
有钱花 99
有蕉一日 99
铜锅涮肉 99
华丽萨克斯 99
鲨鱼头套 299
梦幻椰子岛 99
喝个椰汁 99
独角兽发箍 99
爱意提琴 99
爱心光碟 99
菌菇发卡 99
花布头巾 199
东北时尚帽 199
音你闪耀 99
拍立得 99
粉兔兔 199
月光女神 99
小蘑菇 99
二哈狗狗 299
小丑鱼 299
生煎 99
来条肠粉 99
绅士帽 99
狗皮帽 99
牛角尖尖 99
绿龙龙 299
萌虎帽 199
注意保暖 199
猜猜我是谁 99
诉说情话 99
鹿仙子 99
麋鹿头套 199
蘑菇汤 99
动感DJ 99
大厨煎蛋 199
鹿角发箍 99
拍拍小手 99
老虎帽 99
花见花开 99
鹤彩 99
口红示爱 99
复古头巾 99
遇见飞天 499
为你撑伞 299
捶捶肩膀 99
燃起来了 288
展开说说 288
小鸡跳跳 288
懒懒猫 288
躲猫猫 288
做我的猫 99
爱的抱抱 288
我保护你 288
为你演奏 99
我想开了 99
虎虎生风 99
啄木鸟 199
治愈贴贴 288
拳拳机 99
蝴蝶仙子 99
女神花环 99
泡泡机 99
要喝奶茶 288
大明星 199
丘比特之箭 99
兔耳朵 99
小红帽 99
芭比帽 99
小恶魔 99
为爱唤醒 299
生日眼镜 99
假笑 199
`
