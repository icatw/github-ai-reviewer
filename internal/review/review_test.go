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
	comments := &fakeComments{}
	svc := NewService(gh, llm, comments, nil)

	job := Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc", Action: "opened", DeliveryID: "d1"}
	if err := svc.Process(context.Background(), job); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if gh.installationID != 42 || gh.owner != "octo" || gh.repo != "repo" || gh.pullNumber != 7 {
		t.Fatalf("github fake = %+v", gh)
	}
	if llm.prompt == "" || comments.result.Summary == "" || comments.installationID != 42 || comments.number != 7 {
		t.Fatalf("llm/comment fakes not called: %+v %+v", llm, comments)
	}
}

func TestServiceStopsBeforeLLMWhenGitHubFails(t *testing.T) {
	gh := &fakeGitHub{err: errors.New("token exchange failed")}
	llm := &fakeLLM{}
	comments := &fakeComments{}
	svc := NewService(gh, llm, comments, nil)

	err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7})
	if err == nil {
		t.Fatal("Process() error = nil")
	}
	if llm.prompt != "" || comments.result.HasUsefulContent() {
		t.Fatalf("downstream should not run: %+v %+v", llm, comments)
	}
}

func TestServiceSuppressesEmptyLLMOutput(t *testing.T) {
	gh := &fakeGitHub{files: []FileChange{{Filename: "main.go"}}}
	llm := &fakeLLM{result: ReviewResult{}}
	comments := &fakeComments{}
	svc := NewService(gh, llm, comments, nil)

	if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if comments.result.HasUsefulContent() {
		t.Fatalf("comment should be suppressed, got %+v", comments.result)
	}
}

func TestServiceStopsWithoutPublishingWhenLLMParseFails(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	gh := &fakeGitHub{files: []FileChange{{Filename: "main.go"}}}
	llm := &fakeLLM{err: ErrMalformedResult}
	comments := &fakeComments{}
	svc := NewService(gh, llm, comments, logger)

	err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, DeliveryID: "d1"})
	if !errors.Is(err, ErrMalformedResult) {
		t.Fatalf("Process() error = %v, want ErrMalformedResult", err)
	}
	if comments.result.HasUsefulContent() {
		t.Fatalf("comment should not publish, got %+v", comments.result)
	}
	if !strings.Contains(buf.String(), "stage=llm") || !strings.Contains(buf.String(), "category=malformed_result") {
		t.Fatalf("log line = %q", buf.String())
	}
}

func TestServiceStopsWithoutPublishingWhenValidationFails(t *testing.T) {
	gh := &fakeGitHub{files: []FileChange{{Filename: "main.go"}}}
	llm := &fakeLLM{err: ErrInvalidSeverity}
	comments := &fakeComments{}
	svc := NewService(gh, llm, comments, nil)

	err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7})
	if !errors.Is(err, ErrInvalidSeverity) {
		t.Fatalf("Process() error = %v, want ErrInvalidSeverity", err)
	}
	if comments.result.HasUsefulContent() {
		t.Fatalf("comment should not publish, got %+v", comments.result)
	}
}

func TestServiceReturnsCommentPublishFailure(t *testing.T) {
	gh := &fakeGitHub{files: []FileChange{{Filename: "main.go"}}}
	llm := &fakeLLM{result: ReviewResult{Summary: "Looks focused."}}
	comments := &fakeComments{err: errors.New("comment failed")}
	svc := NewService(gh, llm, comments, nil)

	err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7})
	if err == nil || err.Error() != "comment failed" {
		t.Fatalf("Process() error = %v", err)
	}
}

func TestServiceLogsDownstreamErrorDetails(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	svc := NewService(&fakeGitHub{files: []FileChange{{Filename: "main.go"}}}, &fakeLLM{err: errors.New("llm request failed: status 401")}, &fakeComments{}, logger)

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
