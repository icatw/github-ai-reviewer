package main

import (
	"context"
	"strings"
	"testing"

	"github-ai-reviewer/internal/config"
	"github-ai-reviewer/internal/review"
)

func TestBuildReviewServiceLeavesWorkspaceProviderDisabledByDefault(t *testing.T) {
	gh := &fakeGitHubClient{files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +1 @@\n+package main\n"}}}
	llm := &recordingLLM{result: review.ReviewResult{Summary: "review summary"}}
	tokens := &countingTokenSource{token: "sentinel-checkout-token"}
	svc := buildReviewService(config.Config{}, reviewServiceDeps{
		github:             gh,
		installationTokens: tokens,
		llm:                llm,
		reporter:           noopReporter{},
	})

	err := svc.Process(context.Background(), review.Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 1, HeadSHA: strings.Repeat("a", 40)})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if tokens.calls != 0 {
		t.Fatalf("installation token calls = %d, want none when workspace disabled", tokens.calls)
	}
	if !strings.Contains(llm.prompt, "provider_disabled") {
		t.Fatalf("LLM prompt did not include disabled analyzer limitation: %s", llm.prompt)
	}
}

func TestBuildReviewServiceWiresWorkspaceProviderWhenEnabled(t *testing.T) {
	gh := &fakeGitHubClient{files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +1 @@\n+package main\n"}}}
	llm := &recordingLLM{result: review.ReviewResult{Summary: "review summary"}}
	tokens := &countingTokenSource{err: review.ErrCheckoutCredentialAuth}
	svc := buildReviewService(config.Config{GoWorkspace: config.GoWorkspaceConfig{
		Enabled:          true,
		Root:             t.TempDir(),
		CheckoutTimeout:  1,
		OutputLimitBytes: 1024,
	}}, reviewServiceDeps{
		github:             gh,
		installationTokens: tokens,
		llm:                llm,
		reporter:           noopReporter{},
	})

	err := svc.Process(context.Background(), review.Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 1, HeadSHA: strings.Repeat("a", 40)})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if tokens.calls != 1 {
		t.Fatalf("installation token calls = %d, want one when workspace enabled", tokens.calls)
	}
	if !strings.Contains(llm.prompt, string(review.GoAnalyzerCredentialUnavailable)) {
		t.Fatalf("LLM prompt did not include credential limitation: %s", llm.prompt)
	}
}

type fakeGitHubClient struct {
	files []review.FileChange
}

func (f *fakeGitHubClient) FetchPullRequestFiles(context.Context, int64, string, string, int) ([]review.FileChange, error) {
	return f.files, nil
}

type recordingLLM struct {
	result review.ReviewResult
	prompt string
}

func (r *recordingLLM) Review(ctx context.Context, prompt string) (review.ReviewResult, error) {
	r.prompt = prompt
	return r.result, nil
}

type countingTokenSource struct {
	calls int
	token string
	err   error
}

func (c *countingTokenSource) InstallationToken(ctx context.Context, installationID int64) (string, error) {
	c.calls++
	if c.err != nil {
		return "", c.err
	}
	return c.token, nil
}

type noopReporter struct{}

func (noopReporter) Name() string                                 { return "noop" }
func (noopReporter) JobStarted(context.Context, review.Job) error { return nil }
func (noopReporter) ReviewCompleted(context.Context, review.Job, review.ReviewResult) error {
	return nil
}
func (noopReporter) OutputSuppressed(context.Context, review.Job, review.ReviewResult) error {
	return nil
}
func (noopReporter) JobFailed(context.Context, review.Job, review.Failure) error { return nil }
