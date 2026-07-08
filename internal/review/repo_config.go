package review

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	RepoConfigInvalid     = "repo_config_invalid"
	RepoConfigUnavailable = "repo_config_unavailable"

	maxRepoConfigBytes = 64 * 1024
)

var repositoryConfigPaths = []string{".github/ai-review.yml", ".github/ai-review.yaml"}

type SeverityThreshold string

const (
	SeverityBlocker    SeverityThreshold = "blocker"
	SeverityWarning    SeverityThreshold = "warning"
	SeveritySuggestion SeverityThreshold = "suggestion"
	SeverityQuestion   SeverityThreshold = "question"
)

type RepositoryConfig struct {
	Enabled        *bool                   `yaml:"enabled,omitempty"`
	Language       *Language               `yaml:"language,omitempty"`
	SummaryComment RepositoryFeatureConfig `yaml:"summary_comment,omitempty"`
	CheckRun       RepositoryFeatureConfig `yaml:"check_run,omitempty"`
	InlineComments RepositoryInlineConfig  `yaml:"inline_comments,omitempty"`
	PathIgnore     PathIgnorePatterns      `yaml:"path_ignore,omitempty"`
	GoAnalyzer     RepositoryFeatureConfig `yaml:"go_analyzer,omitempty"`
}

type RepositoryFeatureConfig struct {
	Enabled *bool `yaml:"enabled,omitempty"`
}

type RepositoryInlineConfig struct {
	Enabled             *bool              `yaml:"enabled,omitempty"`
	MaxComments         *int               `yaml:"max_comments,omitempty"`
	SeverityThreshold   *SeverityThreshold `yaml:"severity_threshold,omitempty"`
	ConfidenceThreshold *float64           `yaml:"confidence_threshold,omitempty"`
}

type PathIgnorePatterns []PathIgnorePattern

type PathIgnorePattern struct {
	Raw  string
	kind pathIgnoreKind
}

type pathIgnoreKind int

const (
	pathIgnoreExact pathIgnoreKind = iota
	pathIgnorePrefix
)

type GlobalReviewConfig struct {
	ReviewEnabled             bool
	SummaryCommentEnabled     bool
	CheckRunEnabled           bool
	InlineCommentsEnabled     bool
	GoAnalyzerEnabled         bool
	SafeCheckoutEnabled       bool
	Language                  Language
	InlineMaxComments         int
	InlineSeverityThreshold   SeverityThreshold
	InlineConfidenceThreshold float64
}

type EffectiveReviewConfig struct {
	Enabled                   bool
	SummaryCommentEnabled     bool
	CheckRunEnabled           bool
	InlineCommentsEnabled     bool
	GoAnalyzerEnabled         bool
	SafeCheckoutEnabled       bool
	BlockingPolicyEnabled     bool
	Language                  Language
	InlineMaxComments         int
	InlineSeverityThreshold   SeverityThreshold
	InlineConfidenceThreshold float64
	PathIgnore                PathIgnorePatterns
}

type RepositoryConfigCandidate struct {
	Found      bool
	Path       string
	Content    string
	Limitation string
}

func DefaultGlobalReviewConfig(language Language) GlobalReviewConfig {
	return GlobalReviewConfig{
		ReviewEnabled:             true,
		SummaryCommentEnabled:     true,
		CheckRunEnabled:           true,
		InlineCommentsEnabled:     true,
		GoAnalyzerEnabled:         true,
		SafeCheckoutEnabled:       true,
		Language:                  normalizeEffectiveLanguage(language),
		InlineMaxComments:         10,
		InlineSeverityThreshold:   SeverityBlocker,
		InlineConfidenceThreshold: 0.70,
	}
}

type RepositoryConfigReader interface {
	FetchFileContent(ctx context.Context, installationID int64, owner, repo, ref, path string) (string, error)
}

func ParseRepositoryConfig(content []byte) (RepositoryConfig, error) {
	if len(content) > maxRepoConfigBytes {
		return RepositoryConfig{}, fmt.Errorf("%s: content too large", RepoConfigInvalid)
	}
	var raw repositoryConfigYAML
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return RepositoryConfig{}, fmt.Errorf("%s: %w", RepoConfigInvalid, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil && extra != nil {
		return RepositoryConfig{}, fmt.Errorf("%s: multiple yaml documents are not supported", RepoConfigInvalid)
	}
	cfg := RepositoryConfig{
		Enabled:        raw.Enabled,
		SummaryComment: RepositoryFeatureConfig{Enabled: raw.SummaryComment.Enabled},
		CheckRun:       RepositoryFeatureConfig{Enabled: raw.CheckRun.Enabled},
		InlineComments: RepositoryInlineConfig{
			Enabled:             raw.InlineComments.Enabled,
			MaxComments:         raw.InlineComments.MaxComments,
			ConfidenceThreshold: raw.InlineComments.ConfidenceThreshold,
		},
		GoAnalyzer: RepositoryFeatureConfig{Enabled: raw.GoAnalyzer.Enabled},
	}
	if raw.Language != nil {
		language, err := parseRepositoryLanguage(*raw.Language)
		if err != nil {
			return RepositoryConfig{}, err
		}
		cfg.Language = &language
	}
	if raw.InlineComments.SeverityThreshold != nil {
		severity, err := parseSeverityThreshold(*raw.InlineComments.SeverityThreshold)
		if err != nil {
			return RepositoryConfig{}, err
		}
		cfg.InlineComments.SeverityThreshold = &severity
	}
	patterns, err := parsePathIgnorePatterns(raw.PathIgnore)
	if err != nil {
		return RepositoryConfig{}, err
	}
	cfg.PathIgnore = patterns
	if err := cfg.Validate(); err != nil {
		return RepositoryConfig{}, err
	}
	return cfg, nil
}

func (c RepositoryConfig) Validate() error {
	if c.InlineComments.MaxComments != nil && (*c.InlineComments.MaxComments < 0 || *c.InlineComments.MaxComments > 10) {
		return fmt.Errorf("%s: inline_comments.max_comments must be between 0 and 10", RepoConfigInvalid)
	}
	if c.InlineComments.ConfidenceThreshold != nil && (*c.InlineComments.ConfidenceThreshold < 0 || *c.InlineComments.ConfidenceThreshold > 1) {
		return fmt.Errorf("%s: inline_comments.confidence_threshold must be between 0 and 1", RepoConfigInvalid)
	}
	return nil
}

func MergeEffectiveReviewConfig(global GlobalReviewConfig, repo *RepositoryConfig) EffectiveReviewConfig {
	effective := EffectiveReviewConfig{
		Enabled:                   global.ReviewEnabled,
		SummaryCommentEnabled:     global.SummaryCommentEnabled,
		CheckRunEnabled:           global.CheckRunEnabled,
		InlineCommentsEnabled:     global.InlineCommentsEnabled,
		GoAnalyzerEnabled:         global.GoAnalyzerEnabled && global.SafeCheckoutEnabled,
		SafeCheckoutEnabled:       global.SafeCheckoutEnabled,
		Language:                  normalizeEffectiveLanguage(global.Language),
		InlineMaxComments:         normalizeInlineMax(global.InlineMaxComments),
		InlineSeverityThreshold:   normalizeSeverityThreshold(global.InlineSeverityThreshold),
		InlineConfidenceThreshold: normalizeInlineConfidence(global.InlineConfidenceThreshold),
	}
	if repo == nil {
		return effective
	}
	if repo.Enabled != nil && !*repo.Enabled {
		effective.Enabled = false
	}
	if repo.Language != nil {
		effective.Language = normalizeEffectiveLanguage(*repo.Language)
	}
	if repo.SummaryComment.Enabled != nil && !*repo.SummaryComment.Enabled {
		effective.SummaryCommentEnabled = false
	}
	if repo.CheckRun.Enabled != nil && !*repo.CheckRun.Enabled {
		effective.CheckRunEnabled = false
	}
	if repo.InlineComments.Enabled != nil && !*repo.InlineComments.Enabled {
		effective.InlineCommentsEnabled = false
	}
	if repo.GoAnalyzer.Enabled != nil && !*repo.GoAnalyzer.Enabled {
		effective.GoAnalyzerEnabled = false
	}
	if repo.InlineComments.MaxComments != nil && *repo.InlineComments.MaxComments < effective.InlineMaxComments {
		effective.InlineMaxComments = *repo.InlineComments.MaxComments
	}
	if repo.InlineComments.SeverityThreshold != nil && severityRank(*repo.InlineComments.SeverityThreshold) <= severityRank(effective.InlineSeverityThreshold) {
		effective.InlineSeverityThreshold = *repo.InlineComments.SeverityThreshold
	}
	if repo.InlineComments.ConfidenceThreshold != nil && *repo.InlineComments.ConfidenceThreshold > effective.InlineConfidenceThreshold {
		effective.InlineConfidenceThreshold = *repo.InlineComments.ConfidenceThreshold
	}
	effective.PathIgnore = repo.PathIgnore
	return effective
}

func DiscoverRepositoryConfig(ctx context.Context, reader RepositoryConfigReader, job Job) (RepositoryConfigCandidate, error) {
	if reader == nil || strings.TrimSpace(job.HeadSHA) == "" {
		return RepositoryConfigCandidate{}, nil
	}
	for _, configPath := range repositoryConfigPaths {
		content, err := reader.FetchFileContent(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, configPath)
		if errors.Is(err, ErrRepositoryContentNotFound) {
			continue
		}
		if err != nil {
			return RepositoryConfigCandidate{Limitation: RepoConfigUnavailable}, nil
		}
		if len(content) > maxRepoConfigBytes {
			return RepositoryConfigCandidate{Limitation: RepoConfigUnavailable}, nil
		}
		return RepositoryConfigCandidate{Found: true, Path: configPath, Content: content}, nil
	}
	return RepositoryConfigCandidate{}, nil
}

func filterIgnoredFiles(files []FileChange, patterns PathIgnorePatterns) ([]FileChange, []OmittedContext) {
	if len(patterns) == 0 {
		return files, nil
	}
	filtered := make([]FileChange, 0, len(files))
	var omitted []OmittedContext
	for _, file := range files {
		if patterns.Matches(file.Filename) {
			omitted = append(omitted, OmittedContext{Path: file.Filename, Section: SectionPatch, Reason: OmitRepoConfigIgnored})
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered, omitted
}

func exactPathIgnorePatterns(paths []string) PathIgnorePatterns {
	out := make(PathIgnorePatterns, 0, len(paths))
	for _, filePath := range paths {
		normalized := normalizeConfigPath(filePath)
		if normalized == "" {
			continue
		}
		out = append(out, PathIgnorePattern{Raw: normalized, kind: pathIgnoreExact})
	}
	return out
}

func filterIgnoredRepoContext(ctx RepoContext, patterns PathIgnorePatterns) RepoContext {
	if len(patterns) == 0 {
		return ctx
	}
	ctx.Patches = filterIgnoredPatches(ctx.Patches, patterns, &ctx.Omitted)
	ctx.FullFiles = filterIgnoredFileContexts(ctx.FullFiles, patterns, SectionFullFile, &ctx.Omitted)
	ctx.RelatedSources = filterIgnoredFileContexts(ctx.RelatedSources, patterns, SectionRelatedSrc, &ctx.Omitted)
	ctx.RelatedTests = filterIgnoredFileContexts(ctx.RelatedTests, patterns, SectionRelatedTest, &ctx.Omitted)
	ctx.RepoDocs = filterIgnoredFileContexts(ctx.RepoDocs, patterns, SectionRepoDocs, &ctx.Omitted)
	return ctx
}

func filterIgnoredPatches(items []PatchContext, patterns PathIgnorePatterns, omitted *[]OmittedContext) []PatchContext {
	out := make([]PatchContext, 0, len(items))
	for _, item := range items {
		if patterns.Matches(item.Path) {
			*omitted = append(*omitted, OmittedContext{Path: item.Path, Section: SectionPatch, Reason: OmitRepoConfigIgnored})
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterIgnoredFileContexts(items []FileContext, patterns PathIgnorePatterns, section ContextSection, omitted *[]OmittedContext) []FileContext {
	out := make([]FileContext, 0, len(items))
	for _, item := range items {
		if patterns.Matches(item.Path) {
			*omitted = append(*omitted, OmittedContext{Path: item.Path, Section: section, Reason: OmitRepoConfigIgnored})
			continue
		}
		out = append(out, item)
	}
	return out
}

func (p PathIgnorePatterns) Matches(filePath string) bool {
	normalized := normalizeConfigPath(filePath)
	if normalized == "" {
		return false
	}
	for _, pattern := range p {
		if pattern.matches(normalized) {
			return true
		}
	}
	return false
}

func (p PathIgnorePattern) matches(filePath string) bool {
	raw := p.Raw
	switch p.kind {
	case pathIgnorePrefix:
		return strings.HasPrefix(filePath, raw)
	default:
		return filePath == raw
	}
}

func parsePathIgnorePatterns(values []string) (PathIgnorePatterns, error) {
	out := make(PathIgnorePatterns, 0, len(values))
	for _, value := range values {
		pattern, err := parsePathIgnorePattern(value)
		if err != nil {
			return nil, err
		}
		out = append(out, pattern)
	}
	return out, nil
}

func parsePathIgnorePattern(value string) (PathIgnorePattern, error) {
	raw := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if raw == "" {
		return PathIgnorePattern{}, fmt.Errorf("%s: path_ignore entries must not be empty", RepoConfigInvalid)
	}
	if strings.HasPrefix(raw, "/") || raw == ".." || strings.HasPrefix(raw, "../") || strings.Contains(raw, "/../") || strings.HasSuffix(raw, "/..") {
		return PathIgnorePattern{}, fmt.Errorf("%s: path_ignore entries must be repository-relative", RepoConfigInvalid)
	}
	if strings.ContainsAny(raw, "[]{}?!") || strings.Count(raw, "*") > 2 || (strings.Contains(raw, "*") && !strings.HasSuffix(raw, "/**")) {
		return PathIgnorePattern{}, fmt.Errorf("%s: unsupported path_ignore pattern", RepoConfigInvalid)
	}
	raw = strings.TrimPrefix(raw, "./")
	if strings.HasSuffix(raw, "/**") {
		prefix := strings.TrimSuffix(raw, "**")
		if prefix == "" || strings.Contains(prefix, "*") {
			return PathIgnorePattern{}, fmt.Errorf("%s: unsupported path_ignore pattern", RepoConfigInvalid)
		}
		return PathIgnorePattern{Raw: prefix, kind: pathIgnorePrefix}, nil
	}
	if strings.HasSuffix(raw, "/") {
		return PathIgnorePattern{Raw: raw, kind: pathIgnorePrefix}, nil
	}
	clean := path.Clean(raw)
	if clean == "." || clean != raw {
		return PathIgnorePattern{}, fmt.Errorf("%s: path_ignore entries must be clean paths", RepoConfigInvalid)
	}
	return PathIgnorePattern{Raw: raw, kind: pathIgnoreExact}, nil
}

func parseRepositoryLanguage(value string) (Language, error) {
	trimmed := strings.TrimSpace(value)
	switch strings.ToLower(trimmed) {
	case "en", "en-us":
		return LanguageEnglish, nil
	case "zh-cn", "zh_hans", "zh-hans", "chinese", "中文":
		return LanguageSimplifiedChinese, nil
	default:
		return "", fmt.Errorf("%s: unsupported language", RepoConfigInvalid)
	}
}

func parseSeverityThreshold(value string) (SeverityThreshold, error) {
	severity := SeverityThreshold(strings.ToLower(strings.TrimSpace(value)))
	switch severity {
	case SeverityBlocker, SeverityWarning, SeveritySuggestion, SeverityQuestion:
		return severity, nil
	default:
		return "", fmt.Errorf("%s: unsupported inline_comments.severity_threshold", RepoConfigInvalid)
	}
}

func normalizeEffectiveLanguage(language Language) Language {
	if language == LanguageSimplifiedChinese {
		return LanguageSimplifiedChinese
	}
	return LanguageEnglish
}

func normalizeSeverityThreshold(severity SeverityThreshold) SeverityThreshold {
	switch severity {
	case SeverityBlocker, SeverityWarning, SeveritySuggestion, SeverityQuestion:
		return severity
	default:
		return SeverityWarning
	}
}

func normalizeInlineMax(value int) int {
	if value <= 0 || value > 10 {
		return 10
	}
	return value
}

func normalizeInlineConfidence(value float64) float64 {
	if value < 0 || value > 1 {
		return 0.70
	}
	return value
}

func normalizeConfigPath(filePath string) string {
	filePath = strings.TrimSpace(strings.ReplaceAll(filePath, "\\", "/"))
	for strings.HasPrefix(filePath, "./") {
		filePath = strings.TrimPrefix(filePath, "./")
	}
	clean := path.Clean(filePath)
	if clean == "." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") {
		return ""
	}
	return clean
}

func severityRank(severity SeverityThreshold) int {
	switch severity {
	case SeverityBlocker:
		return 0
	case SeverityWarning:
		return 1
	case SeveritySuggestion:
		return 2
	case SeverityQuestion:
		return 3
	default:
		return 0
	}
}

type repositoryConfigYAML struct {
	Enabled        *bool                       `yaml:"enabled,omitempty"`
	Language       *string                     `yaml:"language,omitempty"`
	SummaryComment repositoryFeatureConfigYAML `yaml:"summary_comment,omitempty"`
	CheckRun       repositoryFeatureConfigYAML `yaml:"check_run,omitempty"`
	InlineComments repositoryInlineConfigYAML  `yaml:"inline_comments,omitempty"`
	PathIgnore     []string                    `yaml:"path_ignore,omitempty"`
	GoAnalyzer     repositoryFeatureConfigYAML `yaml:"go_analyzer,omitempty"`
}

type repositoryFeatureConfigYAML struct {
	Enabled *bool `yaml:"enabled,omitempty"`
}

type repositoryInlineConfigYAML struct {
	Enabled             *bool    `yaml:"enabled,omitempty"`
	MaxComments         *int     `yaml:"max_comments,omitempty"`
	SeverityThreshold   *string  `yaml:"severity_threshold,omitempty"`
	ConfidenceThreshold *float64 `yaml:"confidence_threshold,omitempty"`
}
