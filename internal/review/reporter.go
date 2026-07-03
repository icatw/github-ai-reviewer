package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	FailureCategoryGitHub   = "github_error"
	FailureCategoryLLM      = "llm_error"
	FailureCategoryReporter = "reporter_error"
)

type GitHubStatusClassifier interface {
	GitHubStatusCode() int
}

type Failure struct {
	Category string
	Message  string
}

type Reporter interface {
	Name() string
	JobStarted(ctx context.Context, job Job) error
	ReviewCompleted(ctx context.Context, job Job, result ReviewResult) error
	OutputSuppressed(ctx context.Context, job Job, result ReviewResult) error
	JobFailed(ctx context.Context, job Job, failure Failure) error
}

type MultiReporter []Reporter

func (m MultiReporter) Name() string { return "multi" }

func (m MultiReporter) JobStarted(ctx context.Context, job Job) error {
	return m.fanOut("job_started", func(reporter Reporter) error {
		return reporter.JobStarted(ctx, job)
	})
}

func (m MultiReporter) ReviewCompleted(ctx context.Context, job Job, result ReviewResult) error {
	return m.fanOut("review_completed", func(reporter Reporter) error {
		return reporter.ReviewCompleted(ctx, job, result)
	})
}

func (m MultiReporter) OutputSuppressed(ctx context.Context, job Job, result ReviewResult) error {
	return m.fanOut("output_suppressed", func(reporter Reporter) error {
		return reporter.OutputSuppressed(ctx, job, result)
	})
}

func (m MultiReporter) JobFailed(ctx context.Context, job Job, failure Failure) error {
	return m.fanOut("job_failed", func(reporter Reporter) error {
		return reporter.JobFailed(ctx, job, failure)
	})
}

func (m MultiReporter) fanOut(event string, call func(Reporter) error) error {
	var failures []ReporterFailure
	for _, reporter := range m {
		if reporter == nil {
			continue
		}
		if err := call(reporter); err != nil {
			failures = append(failures, safeReporterFailure(reporter.Name(), event, err))
		}
	}
	if len(failures) > 0 {
		return ReporterError{Failures: failures}
	}
	return nil
}

type ReporterError struct {
	Failures []ReporterFailure
}

func (e ReporterError) Error() string {
	if len(e.Failures) == 0 {
		return "reporter fan-out failed"
	}
	parts := make([]string, 0, len(e.Failures))
	for _, failure := range e.Failures {
		parts = append(parts, failure.Error())
	}
	return "reporter fan-out failed: " + strings.Join(parts, "; ")
}

type ReporterFailure struct {
	Reporter string
	Event    string
	Category string
}

func (f ReporterFailure) Error() string {
	return fmt.Sprintf("reporter=%s event=%s category=%s", f.Reporter, f.Event, f.Category)
}

func safeReporterFailure(reporter, event string, err error) ReporterFailure {
	var failure ReporterFailure
	if errors.As(err, &failure) {
		if failure.Reporter == "" {
			failure.Reporter = reporter
		}
		if failure.Event == "" {
			failure.Event = event
		}
		if failure.Category == "" {
			failure.Category = FailureCategoryReporter
		}
		return failure
	}
	return ReporterFailure{Reporter: reporter, Event: event, Category: FailureCategoryReporter}
}
