package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr    string
	GitHub      GitHubConfig
	LLM         LLMConfig
	GoWorkspace GoWorkspaceConfig
}

type GitHubConfig struct {
	WebhookSecret  string
	AppID          int64
	PrivateKey     string
	PrivateKeyPath string
}

type LLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

type GoWorkspaceConfig struct {
	Enabled          bool
	Root             string
	CheckoutTimeout  time.Duration
	OutputLimitBytes int
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		HTTPAddr: envOrDefault("HTTP_ADDR", ":8088"),
		GitHub: GitHubConfig{
			WebhookSecret:  os.Getenv("GITHUB_WEBHOOK_SECRET"),
			PrivateKey:     os.Getenv("GITHUB_APP_PRIVATE_KEY"),
			PrivateKeyPath: os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH"),
		},
		LLM: LLMConfig{
			BaseURL: os.Getenv("LLM_BASE_URL"),
			APIKey:  os.Getenv("LLM_API_KEY"),
			Model:   os.Getenv("LLM_MODEL"),
		},
		GoWorkspace: GoWorkspaceConfig{
			Enabled:          parseBoolEnv("GO_WORKSPACE_PROVIDER_ENABLED"),
			Root:             os.Getenv("GO_WORKSPACE_ROOT"),
			CheckoutTimeout:  30 * time.Second,
			OutputLimitBytes: 16 * 1024,
		},
	}
	if appID := os.Getenv("GITHUB_APP_ID"); appID != "" {
		id, err := strconv.ParseInt(appID, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("GITHUB_APP_ID must be an integer")
		}
		cfg.GitHub.AppID = id
	}
	if timeout := os.Getenv("GO_WORKSPACE_CHECKOUT_TIMEOUT"); timeout != "" {
		d, err := time.ParseDuration(timeout)
		if err != nil {
			return Config{}, fmt.Errorf("GO_WORKSPACE_CHECKOUT_TIMEOUT must be a duration")
		}
		cfg.GoWorkspace.CheckoutTimeout = d
	}
	if limit := os.Getenv("GO_WORKSPACE_OUTPUT_LIMIT_BYTES"); limit != "" {
		v, err := strconv.Atoi(limit)
		if err != nil {
			return Config{}, fmt.Errorf("GO_WORKSPACE_OUTPUT_LIMIT_BYTES must be an integer")
		}
		cfg.GoWorkspace.OutputLimitBytes = v
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	var missing []string
	if c.GitHub.WebhookSecret == "" {
		missing = append(missing, "GITHUB_WEBHOOK_SECRET")
	}
	if c.GitHub.AppID == 0 {
		missing = append(missing, "GITHUB_APP_ID")
	}
	if c.GitHub.PrivateKey == "" && c.GitHub.PrivateKeyPath == "" {
		missing = append(missing, "GITHUB_APP_PRIVATE_KEY or GITHUB_APP_PRIVATE_KEY_PATH")
	}
	if c.LLM.BaseURL == "" {
		missing = append(missing, "LLM_BASE_URL")
	}
	if c.LLM.APIKey == "" {
		missing = append(missing, "LLM_API_KEY")
	}
	if c.LLM.Model == "" {
		missing = append(missing, "LLM_MODEL")
	}
	if c.GoWorkspace.Enabled {
		if strings.TrimSpace(c.GoWorkspace.Root) == "" {
			missing = append(missing, "GO_WORKSPACE_ROOT")
		} else if !filepath.IsAbs(c.GoWorkspace.Root) {
			return errors.New("invalid GO_WORKSPACE_ROOT: must be an absolute path")
		}
		if c.GoWorkspace.CheckoutTimeout <= 0 {
			return errors.New("invalid GO_WORKSPACE_CHECKOUT_TIMEOUT: must be positive")
		}
		if c.GoWorkspace.OutputLimitBytes <= 0 {
			return errors.New("invalid GO_WORKSPACE_OUTPUT_LIMIT_BYTES: must be positive")
		}
	}
	if len(missing) > 0 {
		return errors.New("missing required config: " + strings.Join(missing, ", "))
	}
	return nil
}

func (c GitHubConfig) PrivateKeyPEM() (string, error) {
	if c.PrivateKey != "" {
		return c.PrivateKey, nil
	}
	b, err := os.ReadFile(c.PrivateKeyPath)
	if err != nil {
		return "", fmt.Errorf("read GITHUB_APP_PRIVATE_KEY_PATH: %w", err)
	}
	return string(b), nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseBoolEnv(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
