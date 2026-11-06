package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store 事件环缓冲 + 统计 + 观众画像
type Store struct {
	mu       sync.Mutex
	events   []Event
	capacity int
	seq      int64
	stats    Stats
	gifters  map[string]*GifterStat
	fans     map[string]*Fan
	fansPath string
	dirty    bool
}

func newStore(dataDir string) *Store {
	s := &Store{
		capacity: 300,
		gifters:  map[string]*GifterStat{},
		fans:     map[string]*Fan{},
		fansPath: filepath.Join(dataDir, "fans.json"),
	}
	s.stats.StartedAt = time.Now().UnixMilli()
	s.stats.ByType = map[string]int64{}
	s.loadFans()
	return s
}

// Add 写入一条事件，返回补全 ID 后的事件
func (s *Store) Add(e Event) Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	e.ID = s.seq
	if e.TS == 0 {
		e.TS = time.Now().UnixMilli()
	}
	if len(s.events) >= s.capacity {
		s.events = s.events[1:]
	}
	s.events = append(s.events, e)

	s.stats.TotalEvents++
	s.stats.LastEventTS = e.TS
	switch e.Type {
	case "danmaku":
		s.stats.Danmaku++
	case "gift":
		s.stats.Gifts++
		s.stats.GiftValue += e.Total
	case "like":
		s.stats.Likes++
	case "enter":
		s.stats.Enters++
	case "follow":
		s.stats.Follows++
	case "guard":
		s.stats.Guards++
	case "fansclub":
		s.stats.FansClubs++
	}
	if s.stats.ByType == nil {
		s.stats.ByType = map[string]int64{}
	}
	s.stats.ByType[e.Type]++

	if e.Type == "gift" && e.User != "" {
		g := s.gifters[e.User]
		if g == nil {
			g = &GifterStat{User: e.User, Avatar: e.Avatar}
			s.gifters[e.User] = g
		}
		g.Value += e.Total
		g.Count += e.GiftCount
		if e.Avatar != "" {
			g.Avatar = e.Avatar
		}
	}

	// 观众画像
	key := e.UserID
	if key == "" {
		key = e.User
	}
	if key != "" && e.User != "" {
		f := s.fans[key]
		if f == nil {
			f = &Fan{Key: key, Name: e.User, FirstSeen: e.TS}
			s.fans[key] = f
			s.dirty = true
		}
		f.Name = e.User
		if e.Avatar != "" {
			f.Avatar = e.Avatar
		}
		if e.Level > f.Level {
			f.Level = e.Level
		}
		f.LastSeen = e.TS
		if e.Type == "danmaku" || e.Type == "like" || e.Type == "enter" {
			f.MsgCount++
		}
		if e.Type == "gift" {
			f.GiftCount += e.GiftCount
			f.GiftValue += e.Total
		}
		if e.Type == "follow" {
			f.Followed = true
		}
		if e.Type == "fansclub" {
			f.FansClub = true
		}
		if e.FansClubLevel > f.FansClubLevel {
			f.FansClubLevel = e.FansClubLevel
			f.FansClub = true
		}
		if e.GuardLevel > f.GuardLevel {
			f.GuardLevel = e.GuardLevel
		}
		if f.MsgCount >= 5 {
			f.Regular = true
		}
		f.LastSeen = e.TS
		e.FanNote = f.Note
		e.IsRegular = f.Regular
		e.Avatar = firstNonEmpty(e.Avatar, f.Avatar)
	}
	return e
}

func (s *Store) Recent(n int) []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n <= 0 || n > len(s.events) {
		n = len(s.events)
	}
	out := make([]Event, n)
	copy(out, s.events[len(s.events)-n:])
	// 倒序：最新在前
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (s *Store) Snapshot() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats
	now := time.Now().UnixMilli()
	var recent int64
	for _, e := range s.events {
		if now-e.TS <= 60*1000 {
			recent++
		}
	}
	st.PerMinute = float64(recent)

	list := make([]GifterStat, 0, len(s.gifters))
	for _, g := range s.gifters {
		list = append(list, *g)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Value > list[j].Value })
	if len(list) > 10 {
		list = list[:10]
	}
	st.TopGifters = list
	return st
}

func (s *Store) IncRuleHits() {
	s.mu.Lock()
	s.stats.RuleHits++
	s.mu.Unlock()
}

func (s *Store) Fans() []Fan {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := make([]Fan, 0, len(s.fans))
	for _, f := range s.fans {
		list = append(list, *f)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].GiftValue == list[j].GiftValue {
			return list[i].LastSeen > list[j].LastSeen
		}
		return list[i].GiftValue > list[j].GiftValue
	})
	if len(list) > 500 {
		list = list[:500]
	}
	return list
}

func (s *Store) SetFanNote(key, note string, tags []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.fans[key]
	if f == nil {
		return false
	}
	if note != "" && note != f.Note {
		f.Notes = append(f.Notes, FanNote{TS: time.Now().UnixMilli(), Text: note})
		if len(f.Notes) > 20 {
			f.Notes = f.Notes[len(f.Notes)-20:]
		}
	}
	f.Note = note
	if tags != nil {
		f.Tags = tags
	}
	s.dirty = true
	return true
}

func (s *Store) loadFans() {
	b, err := os.ReadFile(s.fansPath)
	if err != nil {
		return
	}
	var list []Fan
	if json.Unmarshal(b, &list) != nil {
		return
	}
	for i := range list {
		f := list[i]
		s.fans[f.Key] = &f
	}
}

func (s *Store) SaveFansIfDirty() {
	s.mu.Lock()
	if !s.dirty {
		s.mu.Unlock()
		return
	}
	list := make([]Fan, 0, len(s.fans))
	for _, f := range s.fans {
		list = append(list, *f)
	}
	s.dirty = false
	s.mu.Unlock()

	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.fansPath, b, 0o644)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
