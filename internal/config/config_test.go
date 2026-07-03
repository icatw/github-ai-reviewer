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
	if cfg.LLM.Language != "en" {
		t.Fatalf("LLM.Language = %q, want default en", cfg.LLM.Language)
	}
	if cfg.GoWorkspace.Enabled {
		t.Fatalf("GoWorkspace.Enabled = true, want default disabled")
	}
	if !cfg.CheckRun.Enabled {
		t.Fatalf("CheckRun.Enabled = false, want default enabled")
	}
}

func TestLoadFromEnvCanDisableCheckRunReporter(t *testing.T) {
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "key")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("LLM_API_KEY", "api-key")
	t.Setenv("LLM_MODEL", "review-model")
	t.Setenv("CHECK_RUN_ENABLED", "false")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.CheckRun.Enabled {
		t.Fatalf("CheckRun.Enabled = true, want disabled")
	}
}

func TestLoadFromEnvReadsReviewLanguage(t *testing.T) {
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "key")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("LLM_API_KEY", "api-key")
	t.Setenv("LLM_MODEL", "review-model")
	t.Setenv("REVIEW_LANGUAGE", "zh-CN")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.LLM.Language != "zh-CN" {
		t.Fatalf("LLM.Language = %q, want zh-CN", cfg.LLM.Language)
	}
}

func TestLoadFromEnvRejectsUnsupportedReviewLanguage(t *testing.T) {
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "key")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("LLM_API_KEY", "api-key")
	t.Setenv("LLM_MODEL", "review-model")
	t.Setenv("REVIEW_LANGUAGE", "klingon")

	_, err := LoadFromEnv()
	if err == nil || !strings.Contains(err.Error(), "REVIEW_LANGUAGE") {
		t.Fatalf("LoadFromEnv() error = %v, want REVIEW_LANGUAGE validation", err)
	}
}

func TestLoadFromEnvEnablesGoWorkspaceOnlyWhenExplicit(t *testing.T) {
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "key")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("LLM_API_KEY", "api-key")
	t.Setenv("LLM_MODEL", "review-model")
	t.Setenv("GO_WORKSPACE_PROVIDER_ENABLED", "true")
	t.Setenv("GO_WORKSPACE_ROOT", "/tmp/github-ai-reviewer-workspaces")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if !cfg.GoWorkspace.Enabled || cfg.GoWorkspace.Root != "/tmp/github-ai-reviewer-workspaces" {
		t.Fatalf("GoWorkspace = %+v, want explicit enabled root", cfg.GoWorkspace)
	}
}

func TestLoadFromEnvParsesGoWorkspaceBounds(t *testing.T) {
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "key")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("LLM_API_KEY", "api-key")
	t.Setenv("LLM_MODEL", "review-model")
	t.Setenv("GO_WORKSPACE_PROVIDER_ENABLED", "true")
	t.Setenv("GO_WORKSPACE_ROOT", "/tmp/github-ai-reviewer-workspaces")
	t.Setenv("GO_WORKSPACE_CHECKOUT_TIMEOUT", "7s")
	t.Setenv("GO_WORKSPACE_OUTPUT_LIMIT_BYTES", "4096")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.GoWorkspace.CheckoutTimeout.String() != "7s" || cfg.GoWorkspace.OutputLimitBytes != 4096 {
		t.Fatalf("GoWorkspace bounds = %+v, want env overrides", cfg.GoWorkspace)
	}
}

func TestLoadFromEnvValidatesEnabledGoWorkspace(t *testing.T) {
	t.Setenv("GITHUB_WEBHOOK_SECRET", "secret")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_APP_PRIVATE_KEY", "key")
	t.Setenv("LLM_BASE_URL", "https://llm.example/v1")
	t.Setenv("LLM_API_KEY", "api-key")
	t.Setenv("LLM_MODEL", "review-model")
	t.Setenv("GO_WORKSPACE_PROVIDER_ENABLED", "true")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("LoadFromEnv() error = nil, want workspace config validation error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "GO_WORKSPACE_ROOT") {
		t.Fatalf("error %q does not mention GO_WORKSPACE_ROOT", msg)
	}
	for _, secret := range []string{"secret", "api-key", "key"} {
		if strings.Contains(msg, secret) {
			t.Fatalf("workspace validation error leaked secret %q: %s", secret, msg)
		}
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
