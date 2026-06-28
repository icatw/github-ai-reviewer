package review

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCheckRunReporterCreatesInProgressWhenNoMatchExists(t *testing.T) {
	client := &fakeCheckRunClient{}
	reporter := NewCheckRunReporter(client)
	job := Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}

	if err := reporter.JobStarted(context.Background(), job); err != nil {
		t.Fatalf("JobStarted() error = %v", err)
	}
	if len(client.created) != 1 {
		t.Fatalf("created = %+v", client.created)
	}
	req := client.created[0]
	if req.Name != CheckRunName || req.HeadSHA != "abc" || req.Status != CheckRunStatusInProgress {
		t.Fatalf("create request = %+v", req)
	}
	if strings.Contains(req.Output.Summary, "abc") {
		t.Fatalf("summary should stay concise and not include raw SHA detail: %q", req.Output.Summary)
	}
}

func TestCheckRunReporterUpdatesNewestMatchingCheckRun(t *testing.T) {
	client := &fakeCheckRunClient{
		runs: []CheckRun{
			{ID: 10, Name: CheckRunName, HeadSHA: "abc"},
			{ID: 11, Name: "Other", HeadSHA: "abc"},
			{ID: 12, Name: CheckRunName, HeadSHA: "abc"},
		},
	}
	reporter := NewCheckRunReporter(client)

	if err := reporter.ReviewCompleted(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}, ReviewResult{Summary: "Done", Findings: []Finding{{
		Severity:        "blocker",
		Title:           "Advisory only",
		Evidence:        "evidence",
		FailureScenario: "scenario",
		Suggestion:      "suggestion",
	}}}); err != nil {
		t.Fatalf("ReviewCompleted() error = %v", err)
	}
	if len(client.updated) != 1 || client.updated[0].ID != 12 {
		t.Fatalf("updated = %+v", client.updated)
	}
	req := client.updated[0].Request
	if req.Status != CheckRunStatusCompleted || req.Conclusion != CheckRunConclusionNeutral {
		t.Fatalf("update request = %+v", req)
	}
	if strings.Contains(req.Output.Summary, "Advisory only") || strings.Contains(req.Output.Summary, "evidence") {
		t.Fatalf("summary leaked finding details: %q", req.Output.Summary)
	}
}

func TestCheckRunReporterCreatesCompletedCheckWithConclusionWhenNoMatchExists(t *testing.T) {
	client := &fakeCheckRunClient{}
	reporter := NewCheckRunReporter(client)

	if err := reporter.ReviewCompleted(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}, ReviewResult{Summary: "Done"}); err != nil {
		t.Fatalf("ReviewCompleted() error = %v", err)
	}
	if len(client.created) != 1 {
		t.Fatalf("created = %+v", client.created)
	}
	req := client.created[0]
	if req.Status != CheckRunStatusCompleted || req.Conclusion != CheckRunConclusionNeutral {
		t.Fatalf("create request = %+v", req)
	}
}

func TestCheckRunReporterFailsCheckOnlyForInfrastructureFailure(t *testing.T) {
	client := &fakeCheckRunClient{runs: []CheckRun{{ID: 10, Name: CheckRunName, HeadSHA: "abc"}}}
	reporter := NewCheckRunReporter(client)

	err := reporter.JobFailed(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}, Failure{
		Category: FailureCategoryLLM,
		Message:  "llm request failed: token secret should not appear",
	})
	if err != nil {
		t.Fatalf("JobFailed() error = %v", err)
	}
	if len(client.updated) != 1 || client.updated[0].Request.Conclusion != CheckRunConclusionFailure {
		t.Fatalf("updated = %+v", client.updated)
	}
	summary := client.updated[0].Request.Output.Summary
	if !strings.Contains(summary, "llm_error") {
		t.Fatalf("summary = %q, want safe category", summary)
	}
	for _, disallowed := range []string{"token", "secret", "llm request failed"} {
		if strings.Contains(summary, disallowed) {
			t.Fatalf("summary %q contains %q", summary, disallowed)
		}
	}
}

func TestCheckRunReporterReturnsSafeCategoryWhenClientFails(t *testing.T) {
	reporter := NewCheckRunReporter(&fakeCheckRunClient{listErr: errors.New("github token failure")})

	err := reporter.JobStarted(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"})
	if err == nil {
		t.Fatal("JobStarted() error = nil")
	}
	var failure ReporterFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error = %T, want ReporterFailure", err)
	}
	if failure.Reporter != "github_check_run" || failure.Category != FailureCategoryGitHub {
		t.Fatalf("failure = %+v", failure)
	}
	if strings.Contains(failure.Error(), "token") {
		t.Fatalf("error leaked raw client message: %q", failure.Error())
	}
}

type fakeCheckRunClient struct {
	runs    []CheckRun
	created []CheckRunCreateRequest
	updated []fakeCheckRunUpdate
	listErr error
}

func (f *fakeCheckRunClient) ListCheckRuns(ctx context.Context, installationID int64, owner, repo, ref string) ([]CheckRun, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.runs, nil
}

func (f *fakeCheckRunClient) CreateCheckRun(ctx context.Context, installationID int64, owner, repo string, req CheckRunCreateRequest) (CheckRun, error) {
	f.created = append(f.created, req)
	run := CheckRun{ID: int64(100 + len(f.created)), Name: req.Name, HeadSHA: req.HeadSHA}
	f.runs = append(f.runs, run)
	return run, nil
}

func (f *fakeCheckRunClient) UpdateCheckRun(ctx context.Context, installationID int64, owner, repo string, id int64, req CheckRunUpdateRequest) error {
	f.updated = append(f.updated, fakeCheckRunUpdate{ID: id, Request: req})
	return nil
}

type fakeCheckRunUpdate struct {
	ID      int64
	Request CheckRunUpdateRequest
}
