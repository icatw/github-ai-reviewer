package review

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestServiceProcessesJobEndToEnd(t *testing.T) {
	gh := &fakeGitHub{
		files: []FileChange{{Filename: "main.go", Status: "modified", Additions: 1, Patch: "@@ test"}},
	}
	risk := 10
	llm := &fakeLLM{result: ReviewResult{Summary: "Looks focused.", RiskScore: &risk}}
	reporter := &fakeReporter{}
	svc := NewService(gh, llm, reporter, nil)

	job := Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc", Action: "opened", DeliveryID: "d1"}
	if err := svc.Process(context.Background(), job); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if gh.installationID != 42 || gh.owner != "octo" || gh.repo != "repo" || gh.pullNumber != 7 {
		t.Fatalf("github fake = %+v", gh)
	}
	if llm.prompt == "" || reporter.result.Summary == "" || reporter.started != 1 || reporter.completed != 1 || reporter.job.InstallationID != 42 {
		t.Fatalf("llm/reporter fakes not called: %+v %+v", llm, reporter)
	}
}

func TestServiceReportsInfrastructureFailure(t *testing.T) {
	gh := &fakeGitHub{err: errors.New("github failed")}
	llm := &fakeLLM{}
	reporter := &fakeReporter{}
	svc := NewService(gh, llm, reporter, nil)

	err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7})
	if err == nil {
		t.Fatal("Process() error = nil")
	}
	if reporter.started != 1 || reporter.failed != 1 || reporter.failure.Category != FailureCategoryGitHub {
		t.Fatalf("reporter = %+v", reporter)
	}
	if llm.prompt != "" {
		t.Fatalf("llm should not run, prompt = %q", llm.prompt)
	}
}

func TestServiceStopsBeforeLLMWhenGitHubFails(t *testing.T) {
	gh := &fakeGitHub{err: errors.New("token exchange failed")}
	llm := &fakeLLM{}
	reporter := &fakeReporter{}
	svc := NewService(gh, llm, reporter, nil)

	err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7})
	if err == nil {
		t.Fatal("Process() error = nil")
	}
	if llm.prompt != "" || reporter.completed != 0 || reporter.failed != 1 {
		t.Fatalf("downstream should not run: %+v %+v", llm, reporter)
	}
}

func TestServiceSuppressesEmptyLLMOutput(t *testing.T) {
	gh := &fakeGitHub{files: []FileChange{{Filename: "main.go"}}}
	llm := &fakeLLM{result: ReviewResult{}}
	reporter := &fakeReporter{}
	svc := NewService(gh, llm, reporter, nil)

	if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if reporter.suppressed != 1 || reporter.completed != 0 {
		t.Fatalf("reporter = %+v", reporter)
	}
}

func TestServiceStopsWithoutPublishingWhenLLMParseFails(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	gh := &fakeGitHub{files: []FileChange{{Filename: "main.go"}}}
	llm := &fakeLLM{err: ErrMalformedResult}
	reporter := &fakeReporter{}
	svc := NewService(gh, llm, reporter, logger)

	err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, DeliveryID: "d1"})
	if !errors.Is(err, ErrMalformedResult) {
		t.Fatalf("Process() error = %v, want ErrMalformedResult", err)
	}
	if reporter.completed != 0 || reporter.failed != 1 || reporter.failure.Category != FailureCategoryLLM {
		t.Fatalf("reporter = %+v", reporter)
	}
	if !strings.Contains(buf.String(), "stage=llm") || !strings.Contains(buf.String(), "category=malformed_result") {
		t.Fatalf("log line = %q", buf.String())
	}
}

func TestServiceStopsWithoutPublishingWhenValidationFails(t *testing.T) {
	gh := &fakeGitHub{files: []FileChange{{Filename: "main.go"}}}
	llm := &fakeLLM{err: ErrInvalidSeverity}
	reporter := &fakeReporter{}
	svc := NewService(gh, llm, reporter, nil)

	err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7})
	if !errors.Is(err, ErrInvalidSeverity) {
		t.Fatalf("Process() error = %v, want ErrInvalidSeverity", err)
	}
	if reporter.completed != 0 || reporter.failed != 1 {
		t.Fatalf("reporter = %+v", reporter)
	}
}

func TestServiceLogsReporterPublishFailureWithoutFailingJob(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	gh := &fakeGitHub{files: []FileChange{{Filename: "main.go"}}}
	llm := &fakeLLM{result: ReviewResult{Summary: "Looks focused."}}
	reporter := &fakeReporter{completeErr: errors.New("reporter failed")}
	svc := NewService(gh, llm, reporter, logger)

	if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, DeliveryID: "d1"}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if reporter.completed != 1 {
		t.Fatalf("reporter = %+v", reporter)
	}
	if !strings.Contains(buf.String(), "review reporter failed event=review_completed") {
		t.Fatalf("log line = %q", buf.String())
	}
}

func TestServiceContinuesWhenJobStartedReporterFails(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	gh := &fakeGitHub{files: []FileChange{{Filename: "main.go"}}}
	llm := &fakeLLM{result: ReviewResult{Summary: "Looks focused."}}
	reporter := &fakeReporter{startedErr: errors.New("check run permission denied")}
	svc := NewService(gh, llm, reporter, logger)

	if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, DeliveryID: "d1"}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if gh.pullNumber != 7 || llm.prompt == "" || reporter.completed != 1 {
		t.Fatalf("process did not continue: gh=%+v llm=%+v reporter=%+v", gh, llm, reporter)
	}
	if !strings.Contains(buf.String(), "review reporter failed event=job_started") {
		t.Fatalf("log line = %q", buf.String())
	}
}

func TestServiceLogsDownstreamErrorDetails(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	svc := NewService(&fakeGitHub{files: []FileChange{{Filename: "main.go"}}}, &fakeLLM{err: errors.New("llm request failed: status 401")}, &fakeReporter{}, logger)

	err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, DeliveryID: "d1"})
	if err == nil {
		t.Fatal("Process() error = nil")
	}
	logLine := buf.String()
	for _, want := range []string{"review job failed stage=llm", "category=provider_error", "delivery=d1", "repo=octo/repo", "pull=7", "error=llm request failed: status 401"} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log line %q missing %q", logLine, want)
		}
	}
}

type fakeGitHub struct {
	installationID int64
	owner          string
	repo           string
	pullNumber     int
	files          []FileChange
	err            error
}

func (f *fakeGitHub) FetchPullRequestFiles(ctx context.Context, installationID int64, owner, repo string, pullNumber int) ([]FileChange, error) {
	f.installationID, f.owner, f.repo, f.pullNumber = installationID, owner, repo, pullNumber
	return f.files, f.err
}

type fakeLLM struct {
	prompt string
	result ReviewResult
	err    error
}

func (f *fakeLLM) Review(ctx context.Context, prompt string) (ReviewResult, error) {
	f.prompt = prompt
	return f.result, f.err
}

type fakeComments struct {
	installationID int64
	owner          string
	repo           string
	number         int
	result         ReviewResult
	err            error
}

func (f *fakeComments) Publish(ctx context.Context, installationID int64, owner, repo string, number int, result ReviewResult) error {
	f.installationID, f.owner, f.repo, f.number, f.result = installationID, owner, repo, number, result
	return f.err
}

type fakeReporter struct {
	started     int
	completed   int
	suppressed  int
	failed      int
	job         Job
	result      ReviewResult
	failure     Failure
	startedErr  error
	completeErr error
}

func (f *fakeReporter) Name() string { return "fake" }

func (f *fakeReporter) JobStarted(ctx context.Context, job Job) error {
	f.started++
	f.job = job
	return f.startedErr
}

func (f *fakeReporter) ReviewCompleted(ctx context.Context, job Job, result ReviewResult) error {
	f.completed++
	f.job = job
	f.result = result
	return f.completeErr
}

func (f *fakeReporter) OutputSuppressed(ctx context.Context, job Job, result ReviewResult) error {
	f.suppressed++
	f.job = job
	f.result = result
	return nil
}

func (f *fakeReporter) JobFailed(ctx context.Context, job Job, failure Failure) error {
	f.failed++
	f.job = job
	f.failure = failure
	return nil
}
