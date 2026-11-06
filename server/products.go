package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Product 直播商品：录入卖点/人群/避坑，AI 才能讲得像懂行的
type Product struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Origin   float64 `json:"origin"`   // 原价/划线价，0 表示没有
	Spec     string  `json:"spec"`     // 规格：500ml / 均码 / 2 件装
	Points   string  `json:"points"`   // 卖点，一行一条
	Suitable string  `json:"suitable"` // 适合谁
	Avoid    string  `json:"avoid"`    // 不适合谁（帮观众避坑，降退货）
	Faq      string  `json:"faq"`      // 常见问题，一行一条
	Script   string  `json:"script"`   // 主播自留话术
	Active   bool    `json:"active"`   // 当前主推：AI 会优先讲这个
	Updated  int64   `json:"updated"`
}

// ProductStore 产品库
type ProductStore struct {
	mu   sync.Mutex
	list []Product
	path string
}

func newProductStore(dataDir string) *ProductStore {
	p := &ProductStore{path: filepath.Join(dataDir, "products.json")}
	if b, err := os.ReadFile(p.path); err == nil {
		var saved []Product
		if json.Unmarshal(b, &saved) == nil {
			p.list = saved
		}
	}
	return p
}

func (p *ProductStore) List() []Product {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Product, len(p.list))
	copy(out, p.list)
	return out
}

// Replace 整表替换，只保留一个主推
func (p *ProductStore) Replace(list []Product) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	clean := make([]Product, 0, len(list))
	activeSeen := false
	for _, it := range list {
		it.Name = strings.TrimSpace(it.Name)
		if it.Name == "" {
			continue
		}
		if it.ID == "" {
			it.ID = "p_" + itoa(len(clean)+1)
		}
		if it.Active {
			if activeSeen {
				it.Active = false
			} else {
				activeSeen = true
			}
		}
		clean = append(clean, it)
	}
	p.list = clean
	b, _ := json.MarshalIndent(clean, "", "  ")
	return os.WriteFile(p.path, b, 0o644)
}

// Get 按 id 找商品
func (p *ProductStore) Get(id string) (Product, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, it := range p.list {
		if it.ID == id {
			return it, true
		}
	}
	return Product{}, false
}

// ActiveOne 当前主推商品（没有返回 false）
func (p *ProductStore) ActiveOne() (Product, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, it := range p.list {
		if it.Active {
			return it, true
		}
	}
	return Product{}, false
}

// Brief 给大模型看的商品资料（紧凑、不啰嗦）
func (p Product) Brief() string {
	var sb strings.Builder
	sb.WriteString("商品：" + p.Name)
	if p.Spec != "" {
		sb.WriteString("（" + p.Spec + "）")
	}
	if p.Price > 0 {
		sb.WriteString("，现价 " + trimFloat(p.Price) + " 元")
		if p.Origin > p.Price {
			sb.WriteString("，原价 " + trimFloat(p.Origin) + " 元")
		}
	}
	if p.Points != "" {
		sb.WriteString("\n卖点：" + strings.ReplaceAll(strings.TrimSpace(p.Points), "\n", "；"))
	}
	if p.Suitable != "" {
		sb.WriteString("\n适合：" + strings.TrimSpace(p.Suitable))
	}
	if p.Avoid != "" {
		sb.WriteString("\n不适合（要劝退）：" + strings.TrimSpace(p.Avoid))
	}
	if p.Faq != "" {
		sb.WriteString("\n常见问题：" + strings.ReplaceAll(strings.TrimSpace(p.Faq), "\n", "；"))
	}
	return sb.String()
}
