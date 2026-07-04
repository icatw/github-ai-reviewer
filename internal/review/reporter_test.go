package review

import (
	"context"
	"errors"
	"testing"
)

func TestMultiReporterFanOutIsOrderedAndCapturesSafeFailures(t *testing.T) {
	ctx := context.Background()
	job := Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}
	result := ReviewResult{Summary: "done"}
	var order []string
	reporters := MultiReporter{
		&recordingReporter{name: "first", order: &order},
		&recordingReporter{name: "second", order: &order, completeErr: errors.New("token abc leaked by fake")},
		&recordingReporter{name: "third", order: &order},
	}

	err := reporters.ReviewCompleted(ctx, job, result)
	if err == nil {
		t.Fatal("ReviewCompleted() error = nil")
	}
	var reportErr ReporterError
	if !errors.As(err, &reportErr) {
		t.Fatalf("ReviewCompleted() error = %T, want ReporterError", err)
	}
	if len(reportErr.Failures) != 1 {
		t.Fatalf("failures = %+v", reportErr.Failures)
	}
	if reportErr.Failures[0].Reporter != "second" || reportErr.Failures[0].Event != "review_completed" || reportErr.Failures[0].Category != FailureCategoryReporter {
		t.Fatalf("failure metadata = %+v", reportErr.Failures[0])
	}
	if reportErr.Error() != "reporter fan-out failed: reporter=second event=review_completed category=reporter_error" {
		t.Fatalf("error string = %q", reportErr.Error())
	}
	want := []string{"first:completed", "second:completed", "third:completed"}
	if !equalStrings(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestMultiReporterStillRunsCheckRunReporterAfterInlineReporterFailure(t *testing.T) {
	ctx := context.Background()
	job := Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}
	result := ReviewResult{Summary: "done"}
	var order []string
	checkRunReporter := &recordingReporter{name: "github_check_run", order: &order}
	reporters := MultiReporter{
		&recordingReporter{name: "pr_summary_comment", order: &order, completeErr: errors.New("inline batch failed")},
		checkRunReporter,
	}

	err := reporters.ReviewCompleted(ctx, job, result)
	if err == nil {
		t.Fatal("ReviewCompleted() error = nil")
	}
	if !equalStrings(order, []string{"pr_summary_comment:completed", "github_check_run:completed"}) {
		t.Fatalf("order = %v", order)
	}
	if !equalStrings(checkRunReporter.events, []string{"completed"}) {
		t.Fatalf("check run events = %v", checkRunReporter.events)
	}
}

func TestMultiReporterSendsAllLifecycleEvents(t *testing.T) {
	ctx := context.Background()
	job := Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7}
	result := ReviewResult{Summary: "done"}
	failure := Failure{Category: FailureCategoryLLM, Message: "provider_error"}
	reporter := &recordingReporter{}
	reporters := MultiReporter{reporter}

	if err := reporters.JobStarted(ctx, job); err != nil {
		t.Fatalf("JobStarted() error = %v", err)
	}
	if err := reporters.ReviewCompleted(ctx, job, result); err != nil {
		t.Fatalf("ReviewCompleted() error = %v", err)
	}
	if err := reporters.OutputSuppressed(ctx, job, result); err != nil {
		t.Fatalf("OutputSuppressed() error = %v", err)
	}
	if err := reporters.JobFailed(ctx, job, failure); err != nil {
		t.Fatalf("JobFailed() error = %v", err)
	}

	want := []string{"started", "completed", "suppressed", "failed"}
	if !equalStrings(reporter.events, want) {
		t.Fatalf("events = %v, want %v", reporter.events, want)
	}
	if reporter.failure.Category != FailureCategoryLLM || reporter.failure.Message != "provider_error" {
		t.Fatalf("failure = %+v", reporter.failure)
	}
}

type recordingReporter struct {
	name        string
	order       *[]string
	events      []string
	failure     Failure
	startErr    error
	completeErr error
}

func (r *recordingReporter) Name() string {
	if r.name == "" {
		return "recording"
	}
	return r.name
}

func (r *recordingReporter) JobStarted(ctx context.Context, job Job) error {
	r.events = append(r.events, "started")
	if r.order != nil {
		*r.order = append(*r.order, r.Name()+":started")
	}
	return r.startErr
}

func (r *recordingReporter) ReviewCompleted(ctx context.Context, job Job, result ReviewResult) error {
	r.events = append(r.events, "completed")
	if r.order != nil {
		*r.order = append(*r.order, r.Name()+":completed")
	}
	return r.completeErr
}

func (r *recordingReporter) OutputSuppressed(ctx context.Context, job Job, result ReviewResult) error {
	r.events = append(r.events, "suppressed")
	return nil
}

func (r *recordingReporter) JobFailed(ctx context.Context, job Job, failure Failure) error {
	r.events = append(r.events, "failed")
	r.failure = failure
	return nil
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
