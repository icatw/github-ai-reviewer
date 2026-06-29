package config

import (
	"os"
	"strings"
	"testing"
)

func TestGitignoreCoversLocalSecretsAndArtifacts(t *testing.T) {
	content, err := os.ReadFile("../../.gitignore")
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	text := string(content)
	for _, pattern := range []string{
		".env",
		".env.*",
		"!.env.example",
		"*.pem",
		"*.key",
		"private-key*.pem",
		"data/",
		"*.db",
		"/server",
	} {
		if !strings.Contains(text, pattern) {
			t.Fatalf(".gitignore does not contain %q", pattern)
		}
	}
}
