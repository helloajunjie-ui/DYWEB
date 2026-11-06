package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMClient OpenAI 兼容对话接口（方舟 / DeepSeek / Ollama / OneAPI 都吃这套）
type LLMClient struct {
	get func() LLMConfig
}

func newLLMClient(get func() LLMConfig) *LLMClient {
	return &LLMClient{get: get}
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []chatMsg `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMsg `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat 返回模型回复文本
func (c *LLMClient) Chat(userMsg string) (string, error) {
	return c.ChatWith(userMsg, "")
}

// ChatWith extraSystem 会追加到人设后面（用于把话术库片段喂给模型参考）
func (c *LLMClient) ChatWith(userMsg, extraSystem string) (string, error) {
	return c.ChatWithTimeout(userMsg, extraSystem, 0)
}

// ChatWithTimeout 同上，但可以压一个更短的超时（直播里等 8 秒太久）
func (c *LLMClient) ChatWithTimeout(userMsg, extraSystem string, maxWait time.Duration) (string, error) {
	return c.chatWithMode("", userMsg, extraSystem, maxWait)
}

// ChatMode 按当前直播定位换人设（娱乐 / 无人娱乐 / 电商 / 无人电商 各一套）
func (c *LLMClient) ChatMode(mode, userMsg, extraSystem string) (string, error) {
	return c.chatWithMode(mode, userMsg, extraSystem, 0)
}

// ChatModeTimeout 带超时的分场景版本
func (c *LLMClient) ChatModeTimeout(mode, userMsg, extraSystem string, maxWait time.Duration) (string, error) {
	return c.chatWithMode(mode, userMsg, extraSystem, maxWait)
}

func (c *LLMClient) chatWithMode(mode, userMsg, extraSystem string, maxWait time.Duration) (string, error) {
	cfg := c.get()
	if !cfg.Enabled {
		return "", errors.New("LLM 未启用（功能中心 → 话术 AI 里打开）")
	}
	if cfg.BaseURL == "" || cfg.Model == "" {
		return "", errors.New("LLM 缺少 BaseURL 或 Model")
	}
	system := cfg.System
	if mode != "" {
		system = cfg.PromptFor(mode)
	}
	if strings.TrimSpace(extraSystem) != "" {
		system += "\n\n" + extraSystem
	}

	url := strings.TrimRight(cfg.BaseURL, "/")
	if !strings.HasSuffix(url, "/chat/completions") {
		url += "/chat/completions"
	}

	body := chatRequest{
		Model:       cfg.Model,
		Temperature: cfg.Temperature,
		Messages: []chatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: userMsg},
		},
	}
	b, _ := json.Marshal(body)

	timeout := time.Duration(defaultInt(cfg.TimeoutMS, 2500)) * time.Millisecond
	if maxWait > 0 && maxWait < timeout {
		timeout = maxWait
	}
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest("POST", url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New("LLM 返回 " + resp.Status + ": " + truncate(string(raw), 200))
	}
	var r chatResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	if r.Error != nil {
		return "", errors.New(r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return "", errors.New("LLM 返回内容为空")
	}
	out := strings.TrimSpace(r.Choices[0].Message.Content)
	out = strings.Trim(out, "\"“”")
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// ListModels 拉取 OpenAI 兼容的模型列表（GET {base}/models）
func ListModels(baseURL, apiKey string) ([]string, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, errors.New("没填 BaseURL")
	}
	// 用户可能填了 .../chat/completions，这里统一回退到 .../models
	base = strings.TrimSuffix(base, "/chat/completions")
	url := base + "/models"

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("连不上这个地址：" + err.Error())
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New(explainHTTP(resp.StatusCode, string(raw)))
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"models"` // Ollama 风格
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, errors.New("返回内容不是模型列表，检查 BaseURL 是不是少写了 /v1")
	}
	out := make([]string, 0, len(body.Data)+len(body.Models))
	for _, m := range body.Data {
		if m.ID != "" {
			out = append(out, m.ID)
		}
	}
	for _, m := range body.Models {
		id := firstNonEmpty(m.ID, m.Name)
		if id != "" {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("这个地址没有返回任何模型")
	}
	return out, nil
}

// ProbeLLM 用给定配置真发一句，验证 key / 地址 / 模型名都对
func ProbeLLM(cfg LLMConfig) (string, error) {
	if cfg.BaseURL == "" {
		return "", errors.New("没填 BaseURL")
	}
	if cfg.Model == "" {
		return "", errors.New("没选模型")
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if !strings.HasSuffix(base, "/chat/completions") {
		base += "/chat/completions"
	}
	body, _ := json.Marshal(chatRequest{
		Model:       cfg.Model,
		Temperature: 0.3,
		Messages: []chatMsg{
			{Role: "user", Content: "只说两个字：在的"},
		},
	})
	timeout := time.Duration(defaultInt(cfg.TimeoutMS, 8000)) * time.Millisecond
	if timeout < 5*time.Second {
		timeout = 5 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequest("POST", base, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New("连不上这个地址，检查网络或 BaseURL")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New(explainHTTP(resp.StatusCode, string(raw)))
	}
	var r chatResponse
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", errors.New("返回内容看不懂，检查 BaseURL 是否为 OpenAI 兼容接口")
	}
	if r.Error != nil {
		return "", errors.New(r.Error.Message)
	}
	if len(r.Choices) == 0 {
		return "", errors.New("模型没返回内容")
	}
	return strings.TrimSpace(r.Choices[0].Message.Content), nil
}

// modelSuggestions 拉不到列表时给点常识型建议（按厂商猜）
func modelSuggestions(baseURL string) []string {
	l := strings.ToLower(baseURL)
	switch {
	case strings.Contains(l, "deepseek"):
		return []string{"deepseek-chat", "deepseek-reasoner"}
	case strings.Contains(l, "dashscope") || strings.Contains(l, "aliyun"):
		return []string{"qwen-plus", "qwen-turbo", "qwen-max"}
	case strings.Contains(l, "volces") || strings.Contains(l, "ark"):
		return []string{"（方舟要填推理接入点 ep-xxxx，在方舟控制台创建）"}
	case strings.Contains(l, "11434") || strings.Contains(l, "ollama"):
		return []string{"qwen2.5", "llama3.1", "deepseek-r1"}
	case strings.Contains(l, "moonshot"):
		return []string{"moonshot-v1-8k"}
	case strings.Contains(l, "zhipu") || strings.Contains(l, "bigmodel"):
		return []string{"glm-4-flash", "glm-4"}
	case strings.Contains(l, "siliconflow"):
		return []string{"Qwen/Qwen2.5-7B-Instruct"}
	}
	return []string{"deepseek-chat", "qwen-plus", "gpt-4o-mini"}
}

// explainHTTP 把各家 HTTP 错误翻成小白能懂的中文
func explainHTTP(code int, raw string) string {
	switch code {
	case 401:
		return "API Key 不对或已失效，去控制台重新复制一个"
	case 402:
		return "账户余额不足，先去充值"
	case 403:
		return "这个 Key 没有调用权限（或者没开通该模型）"
	case 404:
		return "地址写错或模型名不存在，BaseURL 一般以 /v1 结尾"
	case 429:
		return "请求太频繁或额度用完了，等一会儿再试"
	}
	if code >= 500 {
		return "对方服务器出错（" + http.StatusText(code) + "），稍后重试"
	}
	return "请求失败（HTTP " + itoa(code) + "）：" + truncate(raw, 120)
}
