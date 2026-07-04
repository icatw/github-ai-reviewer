package review

import (
	"context"
	"errors"
	"log"
	"strings"
)

type Job struct {
	InstallationID  int64
	Owner           string
	Repo            string
	PullNumber      int
	HeadSHA         string
	Action          string
	DeliveryID      string
	EffectiveConfig *EffectiveReviewConfig
}

type CleanupState string

const (
	CleanupStateClosed CleanupState = "closed"
	CleanupStateMerged CleanupState = "merged"
)

type CleanupJob struct {
	InstallationID int64
	Owner          string
	Repo           string
	PullNumber     int
	HeadSHA        string
	Action         string
	DeliveryID     string
	State          CleanupState
}

type PullRequestMetadata struct {
	HeadSHA string
	State   string
	Merged  bool
}

type FileChange struct {
	Filename  string
	Status    string
	Additions int
	Deletions int
	Patch     string
}

type GitHubClient interface {
	FetchPullRequestFiles(ctx context.Context, installationID int64, owner, repo string, pullNumber int) ([]FileChange, error)
}

type LLMClient interface {
	Review(ctx context.Context, prompt string) (ReviewResult, error)
}

type Language string

const (
	LanguageEnglish           Language = "en"
	LanguageSimplifiedChinese Language = "zh-CN"
)

func NormalizeLanguage(language string) Language {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "zh-cn", "zh_hans", "zh-hans", "chinese", "中文":
		return LanguageSimplifiedChinese
	default:
		return LanguageEnglish
	}
}

type Analyzer interface {
	Analyze(ctx context.Context, job Job, repoContext RepoContext) GoAnalyzerResult
}

type Service struct {
	github     GitHubClient
	llm        LLMClient
	reporter   Reporter
	logger     *log.Logger
	goAnalyzer Analyzer
	language   Language
	global     GlobalReviewConfig
}

type ServiceOptions struct {
	Language     Language
	GlobalConfig GlobalReviewConfig
}

func NewService(github GitHubClient, llm LLMClient, reporter Reporter, logger *log.Logger) *Service {
	return NewServiceWithOptions(github, llm, reporter, logger, ServiceOptions{})
}

func NewServiceWithOptions(github GitHubClient, llm LLMClient, reporter Reporter, logger *log.Logger, opts ServiceOptions) *Service {
	language := opts.Language
	if language == "" {
		language = LanguageEnglish
	}
	global := opts.GlobalConfig
	if !global.ReviewEnabled && global == (GlobalReviewConfig{}) {
		global = DefaultGlobalReviewConfig(language)
	}
	if global.Language == "" {
		global.Language = language
	}
	return &Service{github: github, llm: llm, reporter: reporter, logger: logger, goAnalyzer: NewGoAnalyzer(nil, nil, GoAnalyzerOptions{}), language: language, global: global}
}

func NewServiceWithWorkspaceProvider(github GitHubClient, llm LLMClient, reporter Reporter, logger *log.Logger, provider GoWorkspaceProvider) *Service {
	svc := NewServiceWithWorkspaceProviderAndOptions(github, llm, reporter, logger, provider, ServiceOptions{})
	return svc
}

func NewServiceWithWorkspaceProviderAndOptions(github GitHubClient, llm LLMClient, reporter Reporter, logger *log.Logger, provider GoWorkspaceProvider, opts ServiceOptions) *Service {
	svc := NewServiceWithOptions(github, llm, reporter, logger, opts)
	svc.SetGoAnalyzer(NewGoAnalyzer(provider, nil, GoAnalyzerOptions{}))
	return svc
}

func (s *Service) SetGoAnalyzer(analyzer Analyzer) {
	s.goAnalyzer = analyzer
}

func (s *Service) Process(ctx context.Context, job Job) error {
	effective, limitations := s.effectiveConfig(ctx, job)
	if !effective.Enabled {
		s.logf("review job skipped by repository config delivery=%s repo=%s/%s pull=%d category=repo_config_disabled", job.DeliveryID, job.Owner, job.Repo, job.PullNumber)
		return nil
	}
	if err := s.reportJobStarted(ctx, job, effective); err != nil {
		s.logReporterError("job_started", job, err)
	}
	files, err := s.github.FetchPullRequestFiles(ctx, job.InstallationID, job.Owner, job.Repo, job.PullNumber)
	if err != nil {
		s.logf("review job failed at github files delivery=%s repo=%s/%s pull=%d error=%v", job.DeliveryID, job.Owner, job.Repo, job.PullNumber, err)
		if reportErr := s.reportJobFailed(ctx, job, Failure{Category: FailureCategoryGitHub, Message: "fetch_pull_request_files"}, effective); reportErr != nil {
			s.logReporterError("job_failed", job, reportErr)
		}
		return err
	}
	files, ignoredFiles := filterIgnoredFiles(files, effective.PathIgnore)
	repoContext := BuildPatchContext(files, DefaultContextBudgets.MaxPatchBytes)
	repoContext.Omitted = append(repoContext.Omitted, ignoredFiles...)
	if reader, ok := s.github.(RepositoryReader); ok {
		repoContext = BuildRepoContext(ctx, job, files, reader, DefaultContextBudgets)
		repoContext.Omitted = append(repoContext.Omitted, ignoredFiles...)
	}
	repoContext = filterIgnoredRepoContext(repoContext, effective.PathIgnore)
	s.logf("review context built delivery=%s repo=%s/%s pull=%d patches=%d full_files=%d related_sources=%d related_tests=%d repo_docs=%d omitted=%d", job.DeliveryID, job.Owner, job.Repo, job.PullNumber, len(repoContext.Patches), len(repoContext.FullFiles), len(repoContext.RelatedSources), len(repoContext.RelatedTests), len(repoContext.RepoDocs), len(repoContext.Omitted))
	if s.goAnalyzer != nil && effective.GoAnalyzerEnabled {
		analyzerResult := s.goAnalyzer.Analyze(ctx, job, repoContext)
		repoContext.StaticChecks = append(repoContext.StaticChecks, analyzerResult.Evidence...)
		s.logAnalyzerStats(job, analyzerResult.Stats)
	}
	result, err := s.llm.Review(ctx, BuildPromptWithContextAndLanguage(job, repoContext, effective.Language))
	if err != nil {
		category := reviewErrorCategory(err)
		s.logf("review job failed stage=llm category=%s delivery=%s repo=%s/%s pull=%d error=%v", category, job.DeliveryID, job.Owner, job.Repo, job.PullNumber, err)
		if reportErr := s.reportJobFailed(ctx, job, Failure{Category: FailureCategoryLLM, Message: category}, effective); reportErr != nil {
			s.logReporterError("job_failed", job, reportErr)
		}
		return err
	}
	result.Limitations = append(result.Limitations, limitations...)
	result, stats := VerifyReviewResult(result, repoContext)
	s.logVerificationStats(job, stats)
	if !result.HasUsefulContent() {
		s.logf("review job suppressed empty result delivery=%s repo=%s/%s pull=%d", job.DeliveryID, job.Owner, job.Repo, job.PullNumber)
		if err := s.reportOutputSuppressed(ctx, job, result, effective); err != nil {
			s.logReporterError("output_suppressed", job, err)
		}
		return nil
	}
	if err := s.reportReviewCompleted(ctx, job, result, effective); err != nil {
		s.logReporterError("review_completed", job, err)
	}
	return nil
}

func BuildPrompt(job Job, files []FileChange, maxPatchChars int) string {
	return BuildPromptWithContext(job, BuildPatchContext(files, maxPatchChars))
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}

func (s *Service) effectiveConfig(ctx context.Context, job Job) (EffectiveReviewConfig, []string) {
	var limitations []string
	var repoConfig *RepositoryConfig
	var contextIgnoredPaths []string
	if reader, ok := s.github.(RepositoryConfigReader); ok {
		candidate, _ := DiscoverRepositoryConfig(ctx, reader, job)
		if candidate.Limitation != "" {
			limitations = append(limitations, candidate.Limitation)
		}
		if candidate.Found {
			cfg, err := ParseRepositoryConfig([]byte(candidate.Content))
			if err != nil {
				limitations = append(limitations, RepoConfigInvalid)
				contextIgnoredPaths = append(contextIgnoredPaths, candidate.Path)
				s.logf("repository config ignored delivery=%s repo=%s/%s pull=%d path=%s category=%s", job.DeliveryID, job.Owner, job.Repo, job.PullNumber, candidate.Path, RepoConfigInvalid)
			} else {
				repoConfig = &cfg
			}
		}
	}
	effective := MergeEffectiveReviewConfig(s.global, repoConfig)
	effective.PathIgnore = append(effective.PathIgnore, exactPathIgnorePatterns(contextIgnoredPaths)...)
	return effective, limitations
}

func (s *Service) reporterForConfig(effective EffectiveReviewConfig) Reporter {
	if s.reporter == nil {
		return nil
	}
	return FilterReporter(s.reporter, effective)
}

func jobWithEffectiveConfig(job Job, effective EffectiveReviewConfig) Job {
	job.EffectiveConfig = &effective
	return job
}

func (s *Service) reportJobStarted(ctx context.Context, job Job, effective EffectiveReviewConfig) error {
	reporter := s.reporterForConfig(effective)
	if reporter == nil {
		return nil
	}
	return reporter.JobStarted(ctx, jobWithEffectiveConfig(job, effective))
}

func (s *Service) reportReviewCompleted(ctx context.Context, job Job, result ReviewResult, effective EffectiveReviewConfig) error {
	reporter := s.reporterForConfig(effective)
	if reporter == nil {
		return nil
	}
	return reporter.ReviewCompleted(ctx, jobWithEffectiveConfig(job, effective), result)
}

func (s *Service) reportOutputSuppressed(ctx context.Context, job Job, result ReviewResult, effective EffectiveReviewConfig) error {
	reporter := s.reporterForConfig(effective)
	if reporter == nil {
		return nil
	}
	return reporter.OutputSuppressed(ctx, jobWithEffectiveConfig(job, effective), result)
}

func (s *Service) reportJobFailed(ctx context.Context, job Job, failure Failure, effective EffectiveReviewConfig) error {
	reporter := s.reporterForConfig(effective)
	if reporter == nil {
		return nil
	}
	return reporter.JobFailed(ctx, jobWithEffectiveConfig(job, effective), failure)
}

func (s *Service) logReporterError(event string, job Job, err error) {
	s.logf("review reporter failed event=%s delivery=%s repo=%s/%s pull=%d error=%v", event, job.DeliveryID, job.Owner, job.Repo, job.PullNumber, err)
}

func (s *Service) logVerificationStats(job Job, stats VerificationStats) {
	s.logf("finding verification completed delivery=%s repo=%s/%s pull=%d %s", job.DeliveryID, job.Owner, job.Repo, job.PullNumber, stats.String())
}

func (s *Service) logAnalyzerStats(job Job, stats GoAnalyzerStats) {
	s.logf("go analyzer completed delivery=%s repo=%s/%s pull=%d %s", job.DeliveryID, job.Owner, job.Repo, job.PullNumber, stats.String())
}

func reviewErrorCategory(err error) string {
	switch {
	case errors.Is(err, ErrMalformedResult):
		return "malformed_result"
	case errors.Is(err, ErrNoUsefulContent):
		return "no_useful_content"
	case errors.Is(err, ErrInvalidSeverity):
		return "invalid_severity"
	case errors.Is(err, ErrInvalidRiskScore):
		return "invalid_risk_score"
	case errors.Is(err, ErrInvalidConfidence):
		return "invalid_confidence"
	case errors.Is(err, ErrInvalidFinding):
		return "invalid_finding"
	default:
		return "provider_error"
	}
}
