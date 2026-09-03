package config

import (
	"path/filepath"
	"testing"
)

func TestUpdateAIModelPersistsByIDNotProvider(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDatabase(filepath.Join(dir, "cfg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.UpdateAIModel("admin", "1001_nvidia", true, "k1", "https://a.example/v1", "model-a", "NVIDIA_API_KEY", "NIM A", "nvidia"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateAIModel("admin", "1002_nvidia", true, "k2", "https://b.example/v1", "model-b", "NVIDIA_API_KEY", "NIM B", "nvidia"); err != nil {
		t.Fatal(err)
	}

	models, err := db.GetAIModels("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 {
		t.Fatalf("want 2 nvidia rows, got %d", len(models))
	}

	// Masked PUT on one id must not wipe the other.
	if err := db.UpdateAIModel("admin", "1001_nvidia", true, "***", "https://a.example/v1", "model-a2", "NVIDIA_API_KEY", "NIM A", "nvidia"); err != nil {
		t.Fatal(err)
	}
	models, err = db.GetAIModels("admin")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*AIModelConfig{}
	for _, m := range models {
		byID[m.ID] = m
	}
	if byID["1001_nvidia"].APIKey != "k1" {
		t.Fatalf("masked update dropped key: %+v", byID["1001_nvidia"])
	}
	if byID["1001_nvidia"].CustomModelName != "model-a2" {
		t.Fatalf("model name not persisted: %+v", byID["1001_nvidia"])
	}
	if byID["1001_nvidia"].EnvKey != "NVIDIA_API_KEY" {
		t.Fatalf("env_key not persisted: %+v", byID["1001_nvidia"])
	}
	if byID["1002_nvidia"].APIKey != "k2" || byID["1002_nvidia"].CustomModelName != "model-b" {
		t.Fatalf("second nvidia row clobbered: %+v", byID["1002_nvidia"])
	}
}

func TestLegacyProviderKeyStillUpdatesDeepSeek(t *testing.T) {
	dir := t.TempDir()
	db, err := NewDatabase(filepath.Join(dir, "cfg.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.UpdateAIModel("admin", "admin_deepseek", true, "old", "https://integrate.api.nvidia.com/v1", "z-ai/glm-5.2", "", "DeepSeek", "deepseek"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateAIModel("admin", "deepseek", true, "new", "https://integrate.api.nvidia.com/v1", "kept", "", "", "deepseek"); err != nil {
		t.Fatal(err)
	}
	models, err := db.GetAIModels("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 {
		t.Fatalf("legacy deepseek key should update existing row, got %d", len(models))
	}
	if models[0].ID != "admin_deepseek" || models[0].APIKey != "new" || models[0].CustomModelName != "kept" {
		t.Fatalf("legacy update: %+v", models[0])
	}
}
