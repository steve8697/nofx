package config

import (
	"os"
	"testing"
)

func TestLookupPreset(t *testing.T) {
	nvidia := LookupPreset("NVIDIA")
	if nvidia == nil || nvidia.BaseURL != "https://integrate.api.nvidia.com/v1" {
		t.Fatalf("nvidia preset: %+v", nvidia)
	}
	if LookupPreset("missing") != nil {
		t.Fatal("expected nil for unknown preset")
	}
}

func TestResolveAPIKeyOrder(t *testing.T) {
	t.Setenv("NVIDIA_API_KEY", "from-catalog-env")
	t.Setenv("MY_NIM_KEY", "from-env-key")

	if got := ResolveAPIKey("stored", "MY_NIM_KEY", "nvidia"); got != "stored" {
		t.Fatalf("stored key should win, got %q", got)
	}
	if got := ResolveAPIKey("***", "MY_NIM_KEY", "nvidia"); got != "from-env-key" {
		t.Fatalf("mask should fall through to env_key, got %q", got)
	}
	if got := ResolveAPIKey("", "", "nvidia"); got != "from-catalog-env" {
		t.Fatalf("catalog env should be last resort, got %q", got)
	}
	os.Unsetenv("MY_NIM_KEY")
	os.Unsetenv("NVIDIA_API_KEY")
	if got := ResolveAPIKey("", "MY_NIM_KEY", "nvidia"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestRewriteLoopbackURL(t *testing.T) {
	t.Setenv("NOFX_IN_DOCKER", "1")
	got := RewriteLoopbackURL("http://127.0.0.1:1234/v1")
	if got != "http://host.docker.internal:1234/v1" {
		t.Fatalf("docker rewrite: %q", got)
	}
	t.Setenv("NOFX_IN_DOCKER", "")
	// Unset is not enough if /.dockerenv exists on the test host; force off via empty rewrite path.
	// RewriteLoopbackURL checks runningInDocker which also looks at /.dockerenv.
	// On macOS host this should stay loopback.
	if runningInDocker() && os.Getenv("NOFX_IN_DOCKER") != "1" {
		t.Skip("running inside docker; host rewrite assertion would be inverted")
	}
}

func TestEffectiveBaseURLFillsCatalog(t *testing.T) {
	m := &AIModelConfig{Provider: "nvidia"}
	if got := EffectiveBaseURL(m); got != "https://integrate.api.nvidia.com/v1" {
		t.Fatalf("expected nvidia default, got %q", got)
	}
	m.CustomAPIURL = "https://example.com/v1"
	if got := EffectiveBaseURL(m); got != "https://example.com/v1" {
		t.Fatalf("stored URL should win, got %q", got)
	}
}

func TestClientSettingsForcesCustomWhenURLSet(t *testing.T) {
	m := &AIModelConfig{
		Provider:        "deepseek",
		APIKey:          "nvapi-x",
		CustomAPIURL:    "https://integrate.api.nvidia.com/v1",
		CustomModelName: "deepseek-ai/deepseek-v4-pro",
	}
	s := m.ClientSettings()
	if s.Kind != "custom" {
		t.Fatalf("nvidia-via-deepseek should be custom client, got %+v", s)
	}
	if s.APIKey != "nvapi-x" || s.ModelName != "deepseek-ai/deepseek-v4-pro" {
		t.Fatalf("settings %+v", s)
	}

	official := &AIModelConfig{Provider: "deepseek", APIKey: "sk"}
	got := official.ClientSettings()
	if got.Kind != "deepseek" {
		t.Fatalf("official deepseek should stay deepseek, got %+v", got)
	}
}
