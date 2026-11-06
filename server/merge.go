package main

import (
	"sync"
	"time"
)

// Merger 防刷合并：1 秒窗口内同类特效聚合成一次，按权重分级
type mergeBucket struct {
	Key     string
	Count   int
	Weight  float64
	Sample  EffectCmd
	Users   map[string]int
	FirstTS int64
}

// Merger 窗口聚合器
type Merger struct {
	mu      sync.Mutex
	buckets map[string]*mergeBucket
	window  time.Duration
	emit    func(*mergeBucket)
	stop    chan struct{}
	Total   int64 // 累计合并次数
	Saved   int64 // 因合并省掉的派发次数
}

func newMerger(window time.Duration, emit func(*mergeBucket)) *Merger {
	return &Merger{
		buckets: map[string]*mergeBucket{},
		window:  window,
		emit:    emit,
		stop:    make(chan struct{}),
	}
}

func (m *Merger) Start() {
	ticker := time.NewTicker(m.window)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.flush()
			case <-m.stop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (m *Merger) Close() { close(m.stop) }

// Add 投递一次待合并的特效
func (m *Merger) Add(key string, cmd EffectCmd, weight float64, user string) {
	if weight <= 0 {
		weight = 1
	}
	m.mu.Lock()
	b := m.buckets[key]
	if b == nil {
		b = &mergeBucket{Key: key, Sample: cmd, FirstTS: time.Now().UnixMilli(), Users: map[string]int{}}
		m.buckets[key] = b
	}
	b.Count++
	b.Weight += weight
	if user != "" {
		b.Users[user] += int(weight)
	}
	m.mu.Unlock()
}

func (m *Merger) flush() {
	m.mu.Lock()
	if len(m.buckets) == 0 {
		m.mu.Unlock()
		return
	}
	list := make([]*mergeBucket, 0, len(m.buckets))
	for _, b := range m.buckets {
		list = append(list, b)
	}
	m.buckets = map[string]*mergeBucket{}
	m.mu.Unlock()

	for _, b := range list {
		cmd := b.Sample
		cmd.Count = b.Count
		cmd.Strength = 1
		if b.Count > 1 {
			cmd.Strength = strengthLevel(b.Weight)
			if top := topUser(b.Users); top != nil && top.User != "" {
				cmd.User = top.User
			}
			m.mu.Lock()
			m.Total++
			m.Saved += int64(b.Count - 1)
			m.mu.Unlock()
		}
		b.Sample = cmd
		if m.emit != nil {
			m.emit(b)
		}
	}
}

func strengthLevel(weight float64) int {
	switch {
	case weight >= 200:
		return 4
	case weight >= 50:
		return 3
	case weight >= 10:
		return 2
	default:
		return 1
	}
}

func topUser(users map[string]int) *struct{ User, Text string } {
	if len(users) == 0 {
		return nil
	}
	var name string
	var best int
	for u, v := range users {
		if v > best {
			name, best = u, v
		}
	}
	if name == "" {
		return nil
	}
	return &struct{ User, Text string }{User: name, Text: ""}
}

// MergeStats 供控制台展示防刷效果
func (m *Merger) MergeStats() map[string]int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]int64{"merged": m.Total, "saved": m.Saved}
}
