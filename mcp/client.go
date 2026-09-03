package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Provider AI提供商类型
type Provider string

const (
	ProviderDeepSeek Provider = "deepseek"
	ProviderQwen     Provider = "qwen"
	ProviderCustom   Provider = "custom"
	ProviderMock     Provider = "mock"
)

// Client AI API配置
type Client struct {
	Provider   Provider
	APIKey     string
	BaseURL    string
	Model      string
	Timeout    time.Duration
	UseFullURL bool         // 是否使用完整URL（不添加/chat/completions）
	MaxTokens  int          // AI响应的最大token数
	httpClient *http.Client // 🔧 共享HTTP客戶端，避免連接池耗盡
}

// newHTTPClient 創建優化的HTTP客戶端，解決長時間運行後連接池耗盡問題
func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			// 連接池配置
			MaxIdleConns:        100,              // 最大空閒連接數
			MaxIdleConnsPerHost: 10,               // 每個主機最大空閒連接數
			MaxConnsPerHost:     20,               // 每個主機最大連接數
			IdleConnTimeout:     90 * time.Second, // 空閒連接超時（清理腐爛連接）

			// 連接建立超時
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				dialer := &net.Dialer{
					Timeout:   30 * time.Second, // 連接超時
					KeepAlive: 30 * time.Second, // Keep-Alive 探測間隔
				}
				return dialer.DialContext(ctx, network, addr)
			},

			// TLS 握手超時
			TLSHandshakeTimeout: 10 * time.Second,

			// 響應頭讀取超時
			// ⚠️ 重要：AI 模型可能需要 2-5 分鐘才開始輸出，設為 0 由整體 Timeout 控制
			ResponseHeaderTimeout: 0,

			// 期望 100-continue 超時
			ExpectContinueTimeout: 1 * time.Second,

			// 強制關閉空閒連接（防止連接腐爛）
			DisableKeepAlives: false, // 保持 Keep-Alive 以提高效率
			ForceAttemptHTTP2: true,  // 嘗試使用 HTTP/2
		},
	}
}

func New() *Client {
	// 从环境变量读取 MaxTokens，默认 32000（足夠容納完整的思維鏈+JSON）
	maxTokens := 32000
	if envMaxTokens := os.Getenv("AI_MAX_TOKENS"); envMaxTokens != "" {
		if parsed, err := strconv.Atoi(envMaxTokens); err == nil && parsed > 0 {
			maxTokens = parsed
			log.Printf("🔧 [MCP] 使用环境变量 AI_MAX_TOKENS: %d", maxTokens)
		} else {
			log.Printf("⚠️  [MCP] 环境变量 AI_MAX_TOKENS 无效 (%s)，使用默认值: %d", envMaxTokens, maxTokens)
		}
	}

	timeout := 600 * time.Second // 增加到600秒，防止DeepSeek/Qwen分析大量数据时超时

	// 默认配置
	return &Client{
		Provider:   ProviderDeepSeek,
		BaseURL:    "https://api.deepseek.com/v1",
		Model:      "deepseek-chat",
		Timeout:    timeout,
		MaxTokens:  maxTokens,
		httpClient: newHTTPClient(timeout), // 🔧 使用共享HTTP客戶端
	}
}

// SetMock 启用Mock模式
func (client *Client) SetMock() {
	client.Provider = ProviderMock
	client.APIKey = "mock_key" // Set dummy key to pass validation
	log.Printf("🔧 [MCP] 启用 Mock 模式")
}

// setBaseURL 设置BaseURL并智能检测是否为完整URL
func (client *Client) setBaseURL(url string) {
	// 去除末尾的斜杠
	url = strings.TrimSuffix(url, "/")

	// 智能检测：如果用户已经填了完整路径（以 /chat/completions 结尾）
	if strings.HasSuffix(url, "/chat/completions") {
		client.BaseURL = url
		client.UseFullURL = true
		log.Printf("🔧 [MCP] 检测到完整URL路径，将直接使用: %s", url)
	} else if strings.HasSuffix(url, "#") {
		// 手动标记使用完整URL
		client.BaseURL = strings.TrimSuffix(url, "#")
		client.UseFullURL = true
	} else {
		client.BaseURL = url
		client.UseFullURL = false
	}
}

// SetDeepSeekAPIKey 设置DeepSeek API密钥
// customURL 为空时使用默认URL，customModel 为空时使用默认模型
func (client *Client) SetDeepSeekAPIKey(apiKey string, customURL string, customModel string) {
	client.Provider = ProviderDeepSeek
	client.APIKey = apiKey
	if customURL != "" {
		client.setBaseURL(customURL)
		log.Printf("🔧 [MCP] DeepSeek 使用自定义 BaseURL: %s", client.BaseURL)
	} else {
		client.BaseURL = "https://api.deepseek.com/v1"
		client.UseFullURL = false
		log.Printf("🔧 [MCP] DeepSeek 使用默认 BaseURL: %s", client.BaseURL)
	}
	if customModel != "" {
		client.Model = customModel
		log.Printf("🔧 [MCP] DeepSeek 使用自定义 Model: %s", customModel)
	} else {
		client.Model = "deepseek-chat"
		log.Printf("🔧 [MCP] DeepSeek 使用默认 Model: %s", client.Model)
	}
	// 打印 API Key 的前后各4位用于验证
	if len(apiKey) > 8 {
		log.Printf("🔧 [MCP] DeepSeek API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
}

// SetQwenAPIKey 设置阿里云Qwen API密钥
// customURL 为空时使用默认URL，customModel 为空时使用默认模型
func (client *Client) SetQwenAPIKey(apiKey string, customURL string, customModel string) {
	client.Provider = ProviderQwen
	client.APIKey = apiKey
	if customURL != "" {
		client.setBaseURL(customURL)
		log.Printf("🔧 [MCP] Qwen 使用自定义 BaseURL: %s", client.BaseURL)
	} else {
		client.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		client.UseFullURL = false
		log.Printf("🔧 [MCP] Qwen 使用默认 BaseURL: %s", client.BaseURL)
	}
	if customModel != "" {
		client.Model = customModel
		log.Printf("🔧 [MCP] Qwen 使用自定义 Model: %s", customModel)
	} else {
		client.Model = "qwen3-max"
		log.Printf("🔧 [MCP] Qwen 使用默认 Model: %s", client.Model)
	}
	// 打印 API Key 的前后各4位用于验证
	if len(apiKey) > 8 {
		log.Printf("🔧 [MCP] Qwen API Key: %s...%s", apiKey[:4], apiKey[len(apiKey)-4:])
	}
}

// SetCustomAPI 设置自定义OpenAI兼容API
func (client *Client) SetCustomAPI(apiURL, apiKey, modelName string) {
	client.Provider = ProviderCustom
	client.APIKey = apiKey

	client.setBaseURL(apiURL)

	client.Model = modelName
	client.Timeout = 1200 * time.Second // 本地模型可能非常慢，给予20分钟超时时间

	// 🔧 關鍵修復：重新創建 httpClient 以使用新的超時設置
	client.httpClient = newHTTPClient(client.Timeout)

	// 🔥 Custom API（如 OpenRouter）需要更大的 MaxTokens
	client.MaxTokens = 32000
	log.Printf("🔧 [MCP] CustomAPI 配置: Timeout=%v, MaxTokens=%d, httpClient已重新初始化", client.Timeout, client.MaxTokens)
}

// SetClient 设置完整的AI配置（高级用户）
func (client *Client) SetClient(newClient Client) {
	if newClient.Timeout == 0 {
		newClient.Timeout = 600 * time.Second
	}
	// 複製所有字段
	client.Provider = newClient.Provider
	client.APIKey = newClient.APIKey
	client.BaseURL = newClient.BaseURL
	client.Model = newClient.Model
	client.Timeout = newClient.Timeout
	client.UseFullURL = newClient.UseFullURL
	client.MaxTokens = newClient.MaxTokens

	// 🔧 確保 httpClient 使用正確的超時設置
	client.httpClient = newHTTPClient(client.Timeout)
	log.Printf("🔧 [MCP] SetClient 完成: Timeout=%v, httpClient已重新初始化", client.Timeout)
}

// CallWithMessages 使用 system + user prompt 调用AI API（推荐）
func (client *Client) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	if client.Provider == ProviderMock {
		log.Printf("🧪 [Mock Mode] Returning dummy AI response")
		return "```json\n[\n  {\n    \"symbol\": \"BTCUSDT\",\n    \"action\": \"wait\",\n    \"reasoning\": \"Mock decision for verification. Data Visible!\"\n  }\n]\n```", nil
	}

	if client.APIKey == "" {
		return "", fmt.Errorf("AI API密钥未设置：填写 api_key，或设置 env_key / 对应环境变量")
	}

	// 重试配置
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("⚠️  AI API调用失败，正在重试 (%d/%d)...\n", attempt, maxRetries)
		}

		result, err := client.callOnce(systemPrompt, userPrompt)
		if err == nil {
			if attempt > 1 {
				fmt.Printf("✓ AI API重试成功\n")
			}
			return result, nil
		}

		lastErr = err
		// 如果不是网络错误，不重试
		if !isRetryableError(err) {
			return "", err
		}

		// 重试前等待
		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * 2 * time.Second
			fmt.Printf("⏳ 等待%v后重试...\n", waitTime)
			time.Sleep(waitTime)
		}
	}

	return "", fmt.Errorf("重试%d次后仍然失败: %w", maxRetries, lastErr)
}

// callOnce 单次调用AI API（内部使用）
func (client *Client) callOnce(systemPrompt, userPrompt string) (string, error) {
	// 🔧 防禦性檢查：確保 httpClient 已初始化
	if client.httpClient == nil {
		log.Printf("⚠️ [MCP] httpClient 未初始化，正在創建...")
		client.httpClient = newHTTPClient(client.Timeout)
	}

	// 打印当前 AI 配置
	log.Printf("📡 [MCP] AI 请求配置:")
	log.Printf("   Provider: %s", client.Provider)
	log.Printf("   BaseURL: %s", client.BaseURL)
	log.Printf("   Model: %s", client.Model)
	log.Printf("   UseFullURL: %v", client.UseFullURL)
	if len(client.APIKey) > 8 {
		log.Printf("   API Key: %s...%s", client.APIKey[:4], client.APIKey[len(client.APIKey)-4:])
	}

	// 构建 messages 数组
	messages := []map[string]string{}

	// 如果有 system prompt，添加 system message
	if systemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": systemPrompt,
		})
	}

	// 添加 user message
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userPrompt,
	})

	// 注意：temperature 设为 0.5 以提高 JSON 格式稳定性
	requestBody := map[string]interface{}{
		"model":       client.Model,
		"messages":    messages,
		"temperature": 0.5, // 降低temperature以提高JSON格式输出稳定性
		"max_tokens":  client.MaxTokens,
	}
	// 🔥 關鍵修復：為 OpenRouter 等模型啟用 reasoning 模式（其它相容接口不添加以避免 400 錯誤）
	// 這允許模型正確分配思考鏈和最終輸出的 token 預算
	if client.Provider == ProviderCustom && strings.Contains(strings.ToLower(client.BaseURL), "openrouter") {
		requestBody["reasoning"] = map[string]interface{}{
			"enabled": true,
		}
		log.Printf("🧠 [MCP] 啟用 Reasoning 模式（OpenRouter/Custom API）")
	}
	// 注意：response_format 参数仅 OpenAI 支持，DeepSeek/Qwen 不支持
	// 我们通过强化 prompt 和后处理来确保 JSON 格式正确

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建HTTP请求
	var url string
	if client.UseFullURL {
		// 使用完整URL，不添加/chat/completions
		url = client.BaseURL
	} else {
		// 默认行为：添加/chat/completions
		url = fmt.Sprintf("%s/chat/completions", client.BaseURL)
	}
	log.Printf("📡 [MCP] 请求 URL: %s", url)
	log.Printf("⏳ [MCP] 当前超时设置: %v", client.Timeout)
	log.Printf("📦 [MCP] 请求 Payload 大小: %.2f KB", float64(len(jsonData))/1024.0)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 根据不同的Provider设置认证方式
	switch client.Provider {
	case ProviderDeepSeek:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
	case ProviderQwen:
		// 阿里云Qwen使用API-Key认证
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
		// 注意：如果使用的不是兼容模式，可能需要不同的认证方式
	default:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.APIKey))
	}

	// 发送请求 - 使用共享的 httpClient（避免連接池耗盡）
	resp, err := client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误 (status %d): %s", resp.StatusCode, string(body))
	}

	// 🔍 調試日誌：記錄原始 API 響應體（前 2000 字符）
	bodyStr := string(body)
	bodyPreview := bodyStr
	if len(bodyPreview) > 2000 {
		bodyPreview = bodyPreview[:2000] + "..."
	}
	log.Printf("📥 [MCP] API 原始響應體長度: %d 字節", len(body))
	log.Printf("📥 [MCP] API 原始響應體預覽（前2000字符）:\n%s", bodyPreview)

	// 🔍 先嘗試解析為更通用的結構來診斷問題
	var rawResult map[string]interface{}
	if err := json.Unmarshal(body, &rawResult); err != nil {
		log.Printf("❌ [MCP] 無法解析為 JSON: %v", err)
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	// 打印頂層 keys
	var keys []string
	for k := range rawResult {
		keys = append(keys, k)
	}
	log.Printf("📥 [MCP] JSON 頂層 keys: %v", keys)

	// 解析响应
	var result struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"` // 支持思维链字段
			} `json:"message"`
			// 某些 API 可能使用 text 而非 message.content
			Text string `json:"text"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	log.Printf("📥 [MCP] choices 數組長度: %d", len(result.Choices))

	if len(result.Choices) == 0 {
		log.Printf("❌ API返回空响应，原始Body: %s", string(body))
		return "", fmt.Errorf("API返回空响应")
	}

	// 打印第一個 choice 的內容
	log.Printf("📥 [MCP] choices[0].Message.Content 長度: %d", len(result.Choices[0].Message.Content))
	log.Printf("📥 [MCP] choices[0].Message.ReasoningContent 長度: %d", len(result.Choices[0].Message.ReasoningContent))
	log.Printf("📥 [MCP] choices[0].Text 長度: %d", len(result.Choices[0].Text))

	content := result.Choices[0].Message.Content
	reasoning := result.Choices[0].Message.ReasoningContent

	// 🔧 新增：如果 Message.Content 為空但 Text 有值，使用 Text
	if content == "" && result.Choices[0].Text != "" {
		log.Printf("⚠️ [MCP] Content 为空，使用 Text 字段替代")
		content = result.Choices[0].Text
	}

	// 如果 Content 为空但 ReasoningContent 有值，优先使用 ReasoningContent
	// 或者如果两者都有值，将它们合并（视具体模型行为而定，但在当前 case 中 Content 是空的）
	if content == "" && reasoning != "" {
		log.Printf("⚠️ [MCP] Content 为空，使用 ReasoningContent 替代")
		content = reasoning
	} else if reasoning != "" {
		// 如果两者都有，通常 ReasoningContent 是思考过程，Content 是最终结果
		// 我们将它们拼接，以便后续逻辑可以提取 CoT
		log.Printf("ℹ️ [MCP] 检测到 ReasoningContent，已拼接到响应中")
		content = fmt.Sprintf("%s\n\n%s", reasoning, content)
	}

	return content, nil
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	errStr := err.Error()
	// 网络错误、超时、EOF等可以重试
	retryableErrors := []string{
		"EOF",
		"timeout",
		"context deadline exceeded", // 超時錯誤（常見）
		"Client.Timeout",            // HTTP 客戶端超時
		"connection reset",
		"connection refused",
		"temporary failure",
		"no such host",
		"stream error",   // HTTP/2 stream 错误
		"INTERNAL_ERROR", // 服务端内部错误
		"502",            // Bad Gateway（API 閘道問題）
		"503",            // Service Unavailable
		"504",            // Gateway Timeout
		"429",            // Too Many Requests (Rate Limit) - 必須重試
		"解析响应失败",         // AI返回非JSON格式 (抽风) - 應該重試
		"unexpected EOF", // 讀取響應時中斷
	}
	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	return false
}
