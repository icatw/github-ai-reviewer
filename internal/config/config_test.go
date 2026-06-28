package config

import (
	"strings"
	"testing"
)

func TestLoadFromEnvSucceeds(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "key")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("LLM_API_KEY", "api-key")
	t.Setenv("LLM_MODEL", "review-model")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.HTTPAddr != ":9090" || cfg.GitHub.AppID != 123 || cfg.LLM.Model != "review-model" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadFromEnvReportsMissingRequiredConfig(t *testing.T) {
	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want missing config error")
	}
	msg := err.Error()
	for _, field := range []string{"GITHUB_WEBHOOK_SECRET", "GITHUB_APP_ID", "GITHUB_APP_PRIVATE_KEY or GITHUB_APP_PRIVATE_KEY_PATH", "LLM_BASE_URL", "LLM_API_KEY", "LLM_MODEL"} {
		if !strings.Contains(msg, field) {
			t.Fatalf("error %q does not mention %s", msg, field)
		}
	}
}
