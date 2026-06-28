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
		contents: map[string]string{
			"main.go":      "package main\n",
			"main_test.go": "package main\nfunc TestMain(t *testing.T) {}\n",
		},
		dirs: map[string][]RepositoryEntry{
			".": {{Path: "main_test.go", Type: RepositoryEntryFile}},
		},
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
	for _, want := range []string{"patch_context", "full_file_context", "related_test_context", "repo_docs_context", "package main"} {
		if !strings.Contains(llm.prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, llm.prompt)
		}
	}
	if gh.ref != "abc" || !contains(gh.fetched, "main.go") || !contains(gh.fetched, "main_test.go") {
		t.Fatalf("repo content was not fetched at head: ref=%q fetched=%v", gh.ref, gh.fetched)
	}
}

func TestServiceLogsSafeContextSummary(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	gh := &fakeGitHub{
		files: []FileChange{{Filename: "main.go", Status: "modified", Additions: 1, Patch: "@@ test"}},
		contents: map[string]string{
			"main.go":   "package main\nconst secret = \"do-not-log\"\n",
			"README.md": "# Repo\n",
		},
	}
	llm := &fakeLLM{result: ReviewResult{Summary: "Looks focused."}}
	svc := NewService(gh, llm, &fakeReporter{}, logger)

	if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc", DeliveryID: "d1"}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	logLine := buf.String()
	for _, want := range []string{"review context built", "patches=1", "full_files=1", "repo_docs=1"} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log line %q missing %q", logLine, want)
		}
	}
	if strings.Contains(logLine, "do-not-log") || strings.Contains(logLine, "package main") {
		t.Fatalf("log line leaked context content: %q", logLine)
	}
}

func TestServiceReportsVerifiedResultInsteadOfRawLLMResult(t *testing.T) {
	line := 2
	gh := &fakeGitHub{
		files: []FileChange{{
			Filename: "main.go",
			Status:   "modified",
			Patch:    "@@ -1,3 +1,3 @@\n package main\n func Name(user *User) string {\n+\treturn user.Name\n }\n",
		}},
		contents: map[string]string{
			"main.go": "package main\nfunc Name(user *User) string {\n\treturn user.Name\n}\n",
		},
	}
	llm := &fakeLLM{result: ReviewResult{
		Summary: "Found issues.",
		Findings: []Finding{
			findingFixture("warning", "main.go", &line, "return user.Name"),
			findingFixture("warning", "other.go", nil, "password is logged"),
		},
	}}
	reporter := &fakeReporter{}
	svc := NewService(gh, llm, reporter, nil)

	if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if reporter.completed != 1 || reporter.suppressed != 0 {
		t.Fatalf("reporter = %+v", reporter)
	}
	if len(reporter.result.Findings) != 1 {
		t.Fatalf("reported findings = %+v, want one verified finding", reporter.result.Findings)
	}
	if reporter.result.Findings[0].File != "main.go" || reporter.result.Findings[0].Evidence != "return user.Name" {
		t.Fatalf("reported result = %+v", reporter.result)
	}
}

func TestServiceUsesStaticCheckEvidenceBeforeVerification(t *testing.T) {
	gh := &fakeGitHub{
		files:    []FileChange{{Filename: "main.go", Status: "modified", Patch: "@@ -1,2 +1,2 @@\n package main\n+fmt.Println(\"%s\", name)\n"}},
		contents: map[string]string{"main.go": "package main\nfunc main() { fmt.Println(\"%s\", name) }\n"},
	}
	llm := &fakeLLM{result: ReviewResult{
		Summary: "Found issues.",
		Findings: []Finding{{
			Severity:        "warning",
			Category:        "bug",
			File:            "main.go",
			Title:           "Formatting directive is not applied",
			Evidence:        "fmt.Println call has possible formatting directive %s",
			FailureScenario: "The output contains the literal directive instead of the value.",
			Suggestion:      "Use fmt.Printf.",
		}},
	}}
	reporter := &fakeReporter{}
	analyzer := &fakeAnalyzer{result: GoAnalyzerResult{
		Evidence: []StaticCheckEvidence{{
			SourceType:   EvidenceSourceStaticCheck,
			Tool:         "go vet",
			ExitCategory: GoAnalyzerExitFailure,
			Path:         "main.go",
			Message:      "fmt.Println call has possible formatting directive %s",
		}},
		Stats: GoAnalyzerStats{ExitCategories: map[GoAnalyzerExitCategory]int{GoAnalyzerExitFailure: 1}},
	}}
	svc := NewService(gh, llm, reporter, nil)
	svc.SetGoAnalyzer(analyzer)

	if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if !analyzer.called {
		t.Fatal("analyzer was not called")
	}
	if reporter.completed != 1 || len(reporter.result.Findings) != 1 {
		t.Fatalf("reported result = %+v completed=%d", reporter.result, reporter.completed)
	}
}

func TestServiceAnalyzerFailureAndTimeoutAreNonBlocking(t *testing.T) {
	for _, category := range []GoAnalyzerExitCategory{GoAnalyzerExitInternalError, GoAnalyzerExitTimeout} {
		t.Run(string(category), func(t *testing.T) {
			gh := &fakeGitHub{files: []FileChange{{Filename: "main.go", Status: "modified", Patch: "@@ -1 +1 @@\n+return nil\n"}}}
			llm := &fakeLLM{result: ReviewResult{Summary: "Looks focused."}}
			reporter := &fakeReporter{}
			svc := NewService(gh, llm, reporter, nil)
			svc.SetGoAnalyzer(&fakeAnalyzer{result: GoAnalyzerResult{
				Limitations: []string{"Go analyzer recorded " + string(category)},
				Stats:       GoAnalyzerStats{ExitCategories: map[GoAnalyzerExitCategory]int{category: 1}},
			}})

			if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}); err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if llm.prompt == "" || reporter.completed != 1 || reporter.failed != 0 {
				t.Fatalf("pipeline blocked: llm=%+v reporter=%+v", llm, reporter)
			}
		})
	}
}

func TestServiceDefaultAnalyzerSkipsUnsafeWorkspaceWithoutBlocking(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	gh := &fakeGitHub{files: []FileChange{{Filename: "main.go", Status: "modified", Patch: "@@ -1 +1 @@\n+return nil\n"}}}
	llm := &fakeLLM{result: ReviewResult{Summary: "Looks focused."}}
	reporter := &fakeReporter{}
	svc := NewService(gh, llm, reporter, logger)

	if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc", DeliveryID: "d1"}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if reporter.completed != 1 || reporter.failed != 0 {
		t.Fatalf("reporter = %+v", reporter)
	}
	if !strings.Contains(buf.String(), "go analyzer completed") || !strings.Contains(buf.String(), "skipped=1") {
		t.Fatalf("log line = %q", buf.String())
	}
}

func TestServiceLogsSafeVerificationStats(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	gh := &fakeGitHub{
		files: []FileChange{{Filename: "main.go", Status: "modified", Patch: "@@ -1 +1 @@\n+const secret = \"do-not-log\"\n"}},
		contents: map[string]string{
			"main.go": "package main\nconst secret = \"do-not-log\"\n",
		},
	}
	llm := &fakeLLM{result: ReviewResult{
		Summary:  "Found issues.",
		Findings: []Finding{findingFixture("warning", "other.go", nil, "secret")},
	}}
	svc := NewService(gh, llm, &fakeReporter{}, logger)

	if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc", DeliveryID: "d1"}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	logLine := buf.String()
	for _, want := range []string{"finding verification completed", "total=1", "kept=0", "downgraded=0", "dropped=1", "unavailable_file=1"} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log line %q missing %q", logLine, want)
		}
	}
	if strings.Contains(logLine, "do-not-log") || strings.Contains(logLine, "const secret") {
		t.Fatalf("log line leaked private content: %q", logLine)
	}
}

func TestServiceSuppressesWhenVerifiedResultHasNoUsefulContent(t *testing.T) {
	gh := &fakeGitHub{
		files:    []FileChange{{Filename: "main.go", Status: "modified", Patch: "@@ -1 +1 @@\n+return nil\n"}},
		contents: map[string]string{"main.go": "package main\nfunc Run() error { return nil }\n"},
	}
	llm := &fakeLLM{result: ReviewResult{
		Findings: []Finding{findingFixture("warning", "other.go", nil, "password is logged")},
	}}
	reporter := &fakeReporter{}
	svc := NewService(gh, llm, reporter, nil)

	if err := svc.Process(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if reporter.suppressed != 1 || reporter.completed != 0 {
		t.Fatalf("reporter = %+v", reporter)
	}
	if len(reporter.result.Findings) != 0 {
		t.Fatalf("suppressed result = %+v", reporter.result)
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
	contents       map[string]string
	dirs           map[string][]RepositoryEntry
	fetched        []string
	ref            string
	err            error
}

func (f *fakeGitHub) FetchPullRequestFiles(ctx context.Context, installationID int64, owner, repo string, pullNumber int) ([]FileChange, error) {
	f.installationID, f.owner, f.repo, f.pullNumber = installationID, owner, repo, pullNumber
	return f.files, f.err
}

func (f *fakeGitHub) FetchFileContent(ctx context.Context, installationID int64, owner, repo, ref, path string) (string, error) {
	f.installationID, f.owner, f.repo, f.ref = installationID, owner, repo, ref
	f.fetched = append(f.fetched, path)
	content, ok := f.contents[path]
	if !ok {
		return "", ErrRepositoryContentNotFound
	}
	return content, nil
}

func (f *fakeGitHub) ListDirectory(ctx context.Context, installationID int64, owner, repo, ref, path string) ([]RepositoryEntry, error) {
	f.installationID, f.owner, f.repo, f.ref = installationID, owner, repo, ref
	entries, ok := f.dirs[path]
	if !ok {
		return nil, ErrRepositoryContentNotFound
	}
	return entries, nil
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

type fakeAnalyzer struct {
	result GoAnalyzerResult
	called bool
}

func (f *fakeAnalyzer) Analyze(context.Context, Job, RepoContext) GoAnalyzerResult {
	f.called = true
	return f.result
}
