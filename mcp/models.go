package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ModelInfo is one entry from an OpenAI-compatible GET /v1/models response.
type ModelInfo struct {
	ID string `json:"id"`
}

// ModelsListURL builds {base}/models, stripping a trailing /chat/completions if present.
func ModelsListURL(baseURL string) string {
	url := strings.TrimSpace(baseURL)
	url = strings.TrimSuffix(url, "#")
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, "/chat/completions")
	url = strings.TrimSuffix(url, "/")
	if url == "" {
		return ""
	}
	return url + "/models"
}

// ListModels probes GET /v1/models. It does not place trades or call chat completions.
func ListModels(baseURL, apiKey string) ([]ModelInfo, error) {
	listURL := ModelsListURL(baseURL)
	if listURL == "" {
		return nil, fmt.Errorf("empty base URL")
	}

	req, err := http.NewRequest(http.MethodGet, listURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(apiKey) != "" && apiKey != "***" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		preview := strings.TrimSpace(string(body))
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		return nil, fmt.Errorf("GET %s status %d: %s", listURL, resp.StatusCode, preview)
	}

	ids, err := parseModelsList(body)
	if err != nil {
		return nil, err
	}
	if len(ids) > 100 {
		ids = ids[:100]
	}
	return ids, nil
}

func parseModelsList(body []byte) ([]ModelInfo, error) {
	var wrapped struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && len(wrapped.Data) > 0 {
		return uniqueModels(wrapped.Data), nil
	}

	var alt struct {
		Models []json.RawMessage `json:"models"`
	}
	if err := json.Unmarshal(body, &alt); err == nil && len(alt.Models) > 0 {
		out := make([]ModelInfo, 0, len(alt.Models))
		for _, raw := range alt.Models {
			var asObj ModelInfo
			if json.Unmarshal(raw, &asObj) == nil && asObj.ID != "" {
				out = append(out, asObj)
				continue
			}
			var asStr string
			if json.Unmarshal(raw, &asStr) == nil && asStr != "" {
				out = append(out, ModelInfo{ID: asStr})
			}
		}
		if len(out) > 0 {
			return uniqueModels(out), nil
		}
	}

	var arr []ModelInfo
	if err := json.Unmarshal(body, &arr); err == nil && len(arr) > 0 {
		return uniqueModels(arr), nil
	}

	return nil, fmt.Errorf("unrecognized /models payload")
}

func uniqueModels(in []ModelInfo) []ModelInfo {
	seen := map[string]bool{}
	out := make([]ModelInfo, 0, len(in))
	for _, m := range in {
		id := strings.TrimSpace(m.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, ModelInfo{ID: id})
	}
	return out
}
