package review

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
)

type Job struct {
	InstallationID int64
	Owner          string
	Repo           string
	PullNumber     int
	HeadSHA        string
	Action         string
	DeliveryID     string
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

type Service struct {
	github   GitHubClient
	llm      LLMClient
	reporter Reporter
	logger   *log.Logger
}

func NewService(github GitHubClient, llm LLMClient, reporter Reporter, logger *log.Logger) *Service {
	return &Service{github: github, llm: llm, reporter: reporter, logger: logger}
}

func (s *Service) Process(ctx context.Context, job Job) error {
	if err := s.reportJobStarted(ctx, job); err != nil {
		s.logReporterError("job_started", job, err)
	}
	files, err := s.github.FetchPullRequestFiles(ctx, job.InstallationID, job.Owner, job.Repo, job.PullNumber)
	if err != nil {
		s.logf("review job failed at github files delivery=%s repo=%s/%s pull=%d error=%v", job.DeliveryID, job.Owner, job.Repo, job.PullNumber, err)
		if reportErr := s.reportJobFailed(ctx, job, Failure{Category: FailureCategoryGitHub, Message: "fetch_pull_request_files"}); reportErr != nil {
			s.logReporterError("job_failed", job, reportErr)
		}
		return err
	}
	result, err := s.llm.Review(ctx, BuildPrompt(job, files, 12000))
	if err != nil {
		category := reviewErrorCategory(err)
		s.logf("review job failed stage=llm category=%s delivery=%s repo=%s/%s pull=%d error=%v", category, job.DeliveryID, job.Owner, job.Repo, job.PullNumber, err)
		if reportErr := s.reportJobFailed(ctx, job, Failure{Category: FailureCategoryLLM, Message: category}); reportErr != nil {
			s.logReporterError("job_failed", job, reportErr)
		}
		return err
	}
	if !result.HasUsefulContent() {
		s.logf("review job suppressed empty result delivery=%s repo=%s/%s pull=%d", job.DeliveryID, job.Owner, job.Repo, job.PullNumber)
		if err := s.reportOutputSuppressed(ctx, job, result); err != nil {
			s.logReporterError("output_suppressed", job, err)
		}
		return nil
	}
	if err := s.reportReviewCompleted(ctx, job, result); err != nil {
		s.logReporterError("review_completed", job, err)
	}
	return nil
}

func BuildPrompt(job Job, files []FileChange, maxPatchChars int) string {
	if maxPatchChars <= 0 {
		maxPatchChars = 12000
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Review pull request %s/%s#%d at head %s.\n", job.Owner, job.Repo, job.PullNumber, job.HeadSHA)
	fmt.Fprintf(&b, "Webhook action: %s.\n", job.Action)
	b.WriteString("Return JSON-only output matching this schema: {\"summary\": string, \"risk_score\": integer 0-100, \"findings\": [{\"severity\": \"blocker|warning|suggestion|question\", \"category\": string, \"file\": string, \"line\": integer, \"title\": string, \"evidence\": string, \"failure_scenario\": string, \"suggestion\": string, \"confidence\": number 0.0-1.0}], \"missing_tests\": [string], \"limitations\": [string]}.\n")
	b.WriteString("Findings are advisory and non-blocking. Be concise and evidence-based. Use only the context below; if context is insufficient, record the limitation instead of fabricating unavailable context.\n\n")
	remaining := maxPatchChars
	omitted := false
	for _, f := range files {
		fmt.Fprintf(&b, "File: %s\nStatus: %s\nAdditions: %d Deletions: %d\n", f.Filename, f.Status, f.Additions, f.Deletions)
		patch := f.Patch
		if patch != "" {
			if remaining <= 0 {
				omitted = true
				b.WriteString("Patch omitted due to prompt budget.\n\n")
				continue
			}
			if len(patch) > remaining {
				patch = patch[:remaining]
				omitted = true
			}
			remaining -= len(patch)
			b.WriteString("Patch:\n")
			b.WriteString(patch)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	if omitted {
		b.WriteString("Some patch context was omitted due to the prompt budget. Mention this limitation if it affects confidence.\n")
	}
	return b.String()
}

func (s *Service) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}

func (s *Service) reportJobStarted(ctx context.Context, job Job) error {
	if s.reporter == nil {
		return nil
	}
	return s.reporter.JobStarted(ctx, job)
}

func (s *Service) reportReviewCompleted(ctx context.Context, job Job, result ReviewResult) error {
	if s.reporter == nil {
		return nil
	}
	return s.reporter.ReviewCompleted(ctx, job, result)
}

func (s *Service) reportOutputSuppressed(ctx context.Context, job Job, result ReviewResult) error {
	if s.reporter == nil {
		return nil
	}
	return s.reporter.OutputSuppressed(ctx, job, result)
}

func (s *Service) reportJobFailed(ctx context.Context, job Job, failure Failure) error {
	if s.reporter == nil {
		return nil
	}
	return s.reporter.JobFailed(ctx, job, failure)
}

func (s *Service) logReporterError(event string, job Job, err error) {
	s.logf("review reporter failed event=%s delivery=%s repo=%s/%s pull=%d error=%v", event, job.DeliveryID, job.Owner, job.Repo, job.PullNumber, err)
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
