package config

import (
	"os"
	"strings"
)

// ProviderPreset is a Grok/OpenCode/Pi-style catalog entry:
// provider (base URL + env_key) is separate from the model id sent to /chat/completions.
type ProviderPreset struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	BaseURL         string   `json:"base_url"`
	EnvKey          string   `json:"env_key"`
	DefaultModel    string   `json:"default_model"`
	SuggestedModels []string `json:"suggested_models"`
	Local           bool     `json:"local"`
	TimeoutSeconds  int      `json:"timeout_seconds"`
	Notes           string   `json:"notes,omitempty"`
}

// Catalog returns built-in OpenAI-compatible providers.
// Keys stay out of the catalog; use env_key or the SQLite api_key field.
func Catalog() []ProviderPreset {
	return []ProviderPreset{
		{
			ID:              "deepseek",
			Name:            "DeepSeek",
			BaseURL:         "https://api.deepseek.com/v1",
			EnvKey:          "DEEPSEEK_API_KEY",
			DefaultModel:    "deepseek-chat",
			SuggestedModels: []string{"deepseek-chat", "deepseek-reasoner"},
			TimeoutSeconds:  600,
		},
		{
			ID:              "qwen",
			Name:            "Qwen (DashScope)",
			BaseURL:         "https://dashscope.aliyuncs.com/compatible-mode/v1",
			EnvKey:          "DASHSCOPE_API_KEY",
			DefaultModel:    "qwen3-max",
			SuggestedModels: []string{"qwen3-max", "qwen-plus", "qwen-turbo"},
			TimeoutSeconds:  600,
		},
		{
			ID:              "nvidia",
			Name:            "NVIDIA NIM",
			BaseURL:         "https://integrate.api.nvidia.com/v1",
			EnvKey:          "NVIDIA_API_KEY",
			DefaultModel:    "",
			SuggestedModels: []string{},
			TimeoutSeconds:  600,
			Notes:           "Do not use retired ids such as z-ai/glm-5.2 (EOL 2026-08-21). Probe /v1/models and pick a live id.",
		},
		{
			ID:              "openrouter",
			Name:            "OpenRouter",
			BaseURL:         "https://openrouter.ai/api/v1",
			EnvKey:          "OPENROUTER_API_KEY",
			DefaultModel:    "deepseek/deepseek-chat",
			SuggestedModels: []string{"deepseek/deepseek-chat", "anthropic/claude-sonnet-4", "openai/gpt-4o"},
			TimeoutSeconds:  600,
		},
		{
			ID:              "openai",
			Name:            "OpenAI",
			BaseURL:         "https://api.openai.com/v1",
			EnvKey:          "OPENAI_API_KEY",
			DefaultModel:    "gpt-4o",
			SuggestedModels: []string{"gpt-4o", "gpt-4o-mini"},
			TimeoutSeconds:  600,
		},
		{
			ID:              "lmstudio",
			Name:            "LM Studio",
			BaseURL:         "http://127.0.0.1:1234/v1",
			EnvKey:          "LMSTUDIO_API_KEY",
			DefaultModel:    "",
			SuggestedModels: []string{"google/gemma-4-12b", "qwen-agentworld-35b-a3b"},
			Local:           true,
			TimeoutSeconds:  1200,
			Notes:           "Inside Docker, 127.0.0.1 is rewritten to host.docker.internal.",
		},
		{
			ID:              "omlx",
			Name:            "oMLX",
			BaseURL:         "http://127.0.0.1:1216/v1",
			EnvKey:          "OMLX_API_KEY",
			DefaultModel:    "Qwen3.6-35B-A3B-OptiQ-4bit",
			SuggestedModels: []string{"Qwen3.6-35B-A3B-OptiQ-4bit", "Laguna-S-2.1-oQ4e-fast"},
			Local:           true,
			TimeoutSeconds:  1200,
		},
		{
			ID:              "ds4",
			Name:            "DwarfStar 4",
			BaseURL:         "http://127.0.0.1:1235/v1",
			EnvKey:          "DS4_API_KEY",
			DefaultModel:    "deepseek-v4-flash",
			SuggestedModels: []string{"deepseek-v4-flash"},
			Local:           true,
			TimeoutSeconds:  1200,
		},
		{
			ID:              "rtx5090",
			Name:            "RTX5090PC",
			BaseURL:         "http://stevewin11pcrtx5090.tailb35ec3.ts.net:9810/v1",
			EnvKey:          "RTX5090_API_KEY",
			DefaultModel:    "qwen3.8-27b",
			SuggestedModels: []string{"qwen3.8-27b"},
			TimeoutSeconds:  1200,
		},
		{
			ID:              "ollama",
			Name:            "Ollama",
			BaseURL:         "http://127.0.0.1:11434/v1",
			EnvKey:          "OLLAMA_API_KEY",
			DefaultModel:    "llama3.1",
			SuggestedModels: []string{"llama3.1", "qwen2.5"},
			Local:           true,
			TimeoutSeconds:  1200,
		},
		{
			ID:             "custom",
			Name:           "Custom (OpenAI-compatible)",
			BaseURL:        "",
			EnvKey:         "",
			TimeoutSeconds: 1200,
			Notes:          "Any /v1/chat/completions endpoint. Fill Base URL and model id yourself, or probe /v1/models.",
		},
	}
}

// LookupPreset finds a catalog entry by id (case-insensitive).
func LookupPreset(id string) *ProviderPreset {
	want := strings.ToLower(strings.TrimSpace(id))
	if want == "" {
		return nil
	}
	for _, p := range Catalog() {
		if strings.ToLower(p.ID) == want {
			cp := p
			return &cp
		}
	}
	return nil
}

// ResolveAPIKey follows Grok's order: stored api_key, then env_key, then catalog env for provider.
// "***" is treated as empty (UI mask, never a real key).
func ResolveAPIKey(storedKey, envKey, provider string) string {
	if key := strings.TrimSpace(storedKey); key != "" && key != "***" {
		return key
	}
	for _, name := range envKeyNames(envKey, provider) {
		if val := strings.TrimSpace(os.Getenv(name)); val != "" {
			return val
		}
	}
	return ""
}

func envKeyNames(envKey, provider string) []string {
	var names []string
	seen := map[string]bool{}
	add := func(raw string) {
		for _, part := range strings.Split(raw, ",") {
			name := strings.TrimSpace(part)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	add(envKey)
	if p := LookupPreset(provider); p != nil {
		add(p.EnvKey)
	}
	return names
}

// EffectiveBaseURL returns the stored URL, else the catalog default, with Docker loopback rewrite.
func EffectiveBaseURL(model *AIModelConfig) string {
	if model == nil {
		return ""
	}
	url := strings.TrimSpace(model.CustomAPIURL)
	if url == "" {
		if p := LookupPreset(model.Provider); p != nil {
			url = p.BaseURL
		}
	}
	return RewriteLoopbackURL(url)
}

// RewriteLoopbackURL maps localhost/127.0.0.1 to host.docker.internal when running in Docker
// so LM Studio / oMLX / ds4 on the host remain reachable from the backend container.
func RewriteLoopbackURL(raw string) string {
	url := strings.TrimSpace(raw)
	if url == "" || !runningInDocker() {
		return url
	}
	replacer := strings.NewReplacer(
		"http://127.0.0.1", "http://host.docker.internal",
		"https://127.0.0.1", "https://host.docker.internal",
		"http://localhost", "http://host.docker.internal",
		"https://localhost", "https://host.docker.internal",
		"http://[::1]", "http://host.docker.internal",
	)
	return replacer.Replace(url)
}

func runningInDocker() bool {
	if os.Getenv("NOFX_IN_DOCKER") == "1" {
		return true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// HasUsableAIKey reports whether a model can authenticate (stored key or env).
func HasUsableAIKey(model *AIModelConfig) bool {
	if model == nil {
		return false
	}
	return ResolveAPIKey(model.APIKey, model.EnvKey, model.Provider) != ""
}

// AIClientSettings is the resolved OpenAI-compatible client config for a trader.
type AIClientSettings struct {
	Kind      string // "deepseek", "qwen", or "custom"
	BaseURL   string
	APIKey    string
	ModelName string
}

// ClientSettings maps a stored AIModelConfig onto the MCP client knobs.
// A non-empty custom URL (or a catalog URL for non-official providers) uses the custom client.
func (m *AIModelConfig) ClientSettings() AIClientSettings {
	if m == nil {
		return AIClientSettings{}
	}
	key := ResolveAPIKey(m.APIKey, m.EnvKey, m.Provider)
	url := EffectiveBaseURL(m)
	name := strings.TrimSpace(m.CustomModelName)
	if name == "" {
		if p := LookupPreset(m.Provider); p != nil {
			name = p.DefaultModel
		}
	}

	official := m.Provider == "deepseek" || m.Provider == "qwen"
	if !official || strings.TrimSpace(m.CustomAPIURL) != "" {
		return AIClientSettings{Kind: "custom", BaseURL: url, APIKey: key, ModelName: name}
	}
	return AIClientSettings{Kind: m.Provider, BaseURL: strings.TrimSpace(m.CustomAPIURL), APIKey: key, ModelName: name}
}
