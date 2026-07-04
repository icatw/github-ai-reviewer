package review

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseRepositoryConfigValidAndOmittedFields(t *testing.T) {
	cfg, err := ParseRepositoryConfig([]byte(`
enabled: false
language: zh-CN
summary_comment:
  enabled: false
check_run:
  enabled: false
inline_comments:
  enabled: false
  max_comments: 3
  severity_threshold: warning
  confidence_threshold: 0.85
path_ignore:
  - docs/**
  - vendor/
  - README.md
go_analyzer:
  enabled: false
`))
	if err != nil {
		t.Fatalf("ParseRepositoryConfig() error = %v", err)
	}
	if cfg.Enabled == nil || *cfg.Enabled {
		t.Fatalf("Enabled = %#v, want explicit false", cfg.Enabled)
	}
	if cfg.Language == nil || *cfg.Language != LanguageSimplifiedChinese {
		t.Fatalf("Language = %#v", cfg.Language)
	}
	if cfg.SummaryComment.Enabled == nil || *cfg.SummaryComment.Enabled {
		t.Fatalf("SummaryComment.Enabled = %#v", cfg.SummaryComment.Enabled)
	}
	if cfg.CheckRun.Enabled == nil || *cfg.CheckRun.Enabled {
		t.Fatalf("CheckRun.Enabled = %#v", cfg.CheckRun.Enabled)
	}
	if cfg.InlineComments.Enabled == nil || *cfg.InlineComments.Enabled {
		t.Fatalf("InlineComments.Enabled = %#v", cfg.InlineComments.Enabled)
	}
	if cfg.InlineComments.MaxComments == nil || *cfg.InlineComments.MaxComments != 3 {
		t.Fatalf("InlineComments.MaxComments = %#v", cfg.InlineComments.MaxComments)
	}
	if cfg.InlineComments.SeverityThreshold == nil || *cfg.InlineComments.SeverityThreshold != SeverityWarning {
		t.Fatalf("InlineComments.SeverityThreshold = %#v", cfg.InlineComments.SeverityThreshold)
	}
	if cfg.InlineComments.ConfidenceThreshold == nil || *cfg.InlineComments.ConfidenceThreshold != 0.85 {
		t.Fatalf("InlineComments.ConfidenceThreshold = %#v", cfg.InlineComments.ConfidenceThreshold)
	}
	if cfg.GoAnalyzer.Enabled == nil || *cfg.GoAnalyzer.Enabled {
		t.Fatalf("GoAnalyzer.Enabled = %#v", cfg.GoAnalyzer.Enabled)
	}
	for _, path := range []string{"docs/guide.md", "vendor/lib/a.go", "README.md"} {
		if !cfg.PathIgnore.Matches(path) {
			t.Fatalf("path_ignore did not match %q", path)
		}
	}

	empty, err := ParseRepositoryConfig([]byte(`{}`))
	if err != nil {
		t.Fatalf("ParseRepositoryConfig({}) error = %v", err)
	}
	if empty.Enabled != nil || empty.Language != nil || empty.SummaryComment.Enabled != nil || len(empty.PathIgnore) != 0 {
		t.Fatalf("omitted fields should remain unset: %+v", empty)
	}
}

func TestParseRepositoryConfigRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed yaml", body: "enabled: ["},
		{name: "unknown field", body: "request_changes: true"},
		{name: "invalid type", body: "enabled: nope"},
		{name: "unsupported language", body: "language: fr"},
		{name: "unsupported severity", body: "inline_comments:\n  severity_threshold: critical"},
		{name: "max comments too high", body: "inline_comments:\n  max_comments: 99"},
		{name: "negative confidence", body: "inline_comments:\n  confidence_threshold: -0.1"},
		{name: "unsafe absolute path", body: "path_ignore:\n  - /etc/passwd"},
		{name: "unsafe parent path", body: "path_ignore:\n  - ../secret"},
		{name: "unsupported glob", body: "path_ignore:\n  - src/[abc].go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseRepositoryConfig([]byte(tt.body)); err == nil {
				t.Fatal("ParseRepositoryConfig() error = nil")
			}
		})
	}
}

func TestEffectiveReviewConfigKeepsGlobalConfigAsUpperBoundary(t *testing.T) {
	enabled := true
	disabled := false
	max := 4
	confidence := 0.9
	severity := SeverityWarning
	repo := RepositoryConfig{
		SummaryComment: RepositoryFeatureConfig{Enabled: &disabled},
		CheckRun:       RepositoryFeatureConfig{Enabled: &enabled},
		InlineComments: RepositoryInlineConfig{
			Enabled:             &enabled,
			MaxComments:         &max,
			SeverityThreshold:   &severity,
			ConfidenceThreshold: &confidence,
		},
		GoAnalyzer: RepositoryFeatureConfig{Enabled: &enabled},
	}
	global := GlobalReviewConfig{
		ReviewEnabled:             true,
		SummaryCommentEnabled:     true,
		CheckRunEnabled:           false,
		InlineCommentsEnabled:     false,
		GoAnalyzerEnabled:         false,
		SafeCheckoutEnabled:       false,
		Language:                  LanguageEnglish,
		InlineMaxComments:         10,
		InlineSeverityThreshold:   SeveritySuggestion,
		InlineConfidenceThreshold: 0.7,
	}
	effective := MergeEffectiveReviewConfig(global, &repo)
	if !effective.Enabled {
		t.Fatal("review unexpectedly disabled")
	}
	if effective.SummaryCommentEnabled {
		t.Fatal("repo config failed to disable summary comments")
	}
	if effective.CheckRunEnabled || effective.InlineCommentsEnabled || effective.GoAnalyzerEnabled {
		t.Fatalf("repo config enabled globally disabled features: %+v", effective)
	}
	if effective.SafeCheckoutEnabled {
		t.Fatal("safe checkout should stay globally disabled")
	}
	if effective.InlineMaxComments != 4 || effective.InlineSeverityThreshold != SeverityWarning || effective.InlineConfidenceThreshold != 0.9 {
		t.Fatalf("inline tightening not applied: %+v", effective)
	}
	if effective.BlockingPolicyEnabled {
		t.Fatal("repo config must not enable blocking policy")
	}
}

func TestEffectiveReviewConfigRejectsLooserInlineRepoSettings(t *testing.T) {
	max := 99
	confidence := 0.2
	severity := SeveritySuggestion
	repo := RepositoryConfig{InlineComments: RepositoryInlineConfig{
		MaxComments:         &max,
		SeverityThreshold:   &severity,
		ConfidenceThreshold: &confidence,
	}}
	global := GlobalReviewConfig{
		ReviewEnabled: true, SummaryCommentEnabled: true, CheckRunEnabled: true, InlineCommentsEnabled: true,
		GoAnalyzerEnabled: true, SafeCheckoutEnabled: true, Language: LanguageEnglish,
		InlineMaxComments: 5, InlineSeverityThreshold: SeverityWarning, InlineConfidenceThreshold: 0.8,
	}
	effective := MergeEffectiveReviewConfig(global, &repo)
	if effective.InlineMaxComments != 5 || effective.InlineSeverityThreshold != SeverityWarning || effective.InlineConfidenceThreshold != 0.8 {
		t.Fatalf("looser repo settings should be ignored: %+v", effective)
	}
}

func TestDiscoverRepositoryConfigPrecedenceAndMissing(t *testing.T) {
	reader := &fakeConfigReader{contents: map[string]string{
		".github/ai-review.yml":  "enabled: false",
		".github/ai-review.yaml": "enabled: true",
	}}
	candidate, err := DiscoverRepositoryConfig(context.Background(), reader, Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"})
	if err != nil {
		t.Fatalf("DiscoverRepositoryConfig() error = %v", err)
	}
	if !candidate.Found || candidate.Path != ".github/ai-review.yml" || !strings.Contains(candidate.Content, "enabled: false") {
		t.Fatalf("candidate = %+v", candidate)
	}
	if len(reader.paths) != 1 {
		t.Fatalf("primary config should stop discovery, paths=%v", reader.paths)
	}

	reader = &fakeConfigReader{contents: map[string]string{".github/ai-review.yaml": "enabled: false"}}
	candidate, err = DiscoverRepositoryConfig(context.Background(), reader, Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"})
	if err != nil {
		t.Fatalf("DiscoverRepositoryConfig() fallback error = %v", err)
	}
	if !candidate.Found || candidate.Path != ".github/ai-review.yaml" || len(reader.paths) != 2 {
		t.Fatalf("fallback candidate = %+v paths=%v", candidate, reader.paths)
	}

	reader = &fakeConfigReader{}
	candidate, err = DiscoverRepositoryConfig(context.Background(), reader, Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"})
	if err != nil || candidate.Found || candidate.Limitation != "" {
		t.Fatalf("missing config should be normal: candidate=%+v err=%v", candidate, err)
	}
}

func TestDiscoverRepositoryConfigFetchFailureIsSafeLimitation(t *testing.T) {
	reader := &fakeConfigReader{err: errors.New("private body: secret-token")}
	candidate, err := DiscoverRepositoryConfig(context.Background(), reader, Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"})
	if err != nil {
		t.Fatalf("DiscoverRepositoryConfig() error = %v", err)
	}
	if candidate.Found || candidate.Limitation != RepoConfigUnavailable {
		t.Fatalf("candidate = %+v", candidate)
	}
	if strings.Contains(candidate.Limitation, "secret-token") {
		t.Fatalf("limitation leaked raw error: %+v", candidate)
	}
}

type fakeConfigReader struct {
	contents map[string]string
	paths    []string
	err      error
}

func (f *fakeConfigReader) FetchFileContent(ctx context.Context, installationID int64, owner, repo, ref, path string) (string, error) {
	f.paths = append(f.paths, path)
	if f.err != nil {
		return "", f.err
	}
	content, ok := f.contents[path]
	if !ok {
		return "", ErrRepositoryContentNotFound
	}
	return content, nil
}
