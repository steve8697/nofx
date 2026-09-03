package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModelsListURL(t *testing.T) {
	cases := map[string]string{
		"https://integrate.api.nvidia.com/v1":        "https://integrate.api.nvidia.com/v1/models",
		"https://integrate.api.nvidia.com/v1/":       "https://integrate.api.nvidia.com/v1/models",
		"https://api.openai.com/v1/chat/completions": "https://api.openai.com/v1/models",
		"http://127.0.0.1:1234/v1/chat/completions#": "http://127.0.0.1:1234/v1/models",
		"": "",
	}
	for in, want := range cases {
		if got := ModelsListURL(in); got != want {
			t.Fatalf("ModelsListURL(%q)=%q want %q", in, got, want)
		}
	}
}

func TestListModelsOpenAIShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer k" {
			t.Fatalf("auth %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "deepseek-ai/deepseek-v4-pro"},
				{"id": "z-ai/glm-5.2"},
				{"id": "deepseek-ai/deepseek-v4-pro"},
			},
		})
	}))
	defer srv.Close()

	got, err := ListModels(srv.URL+"/v1", "k")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "deepseek-ai/deepseek-v4-pro" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseModelsListVariants(t *testing.T) {
	openai, err := parseModelsList([]byte(`{"data":[{"id":"a"},{"id":"b"}]}`))
	if err != nil || len(openai) != 2 {
		t.Fatalf("openai: %v %+v", err, openai)
	}
	ollama, err := parseModelsList([]byte(`{"models":["llama3.1","qwen2.5"]}`))
	if err != nil || len(ollama) != 2 {
		t.Fatalf("ollama strings: %v %+v", err, ollama)
	}
}
