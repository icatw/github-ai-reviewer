package review

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestDetectGoProjectFromChangedFilesAndContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  RepoContext
		want bool
	}{
		{name: "changed go file", ctx: RepoContext{Patches: []PatchContext{{Path: "internal/app.go"}}}, want: true},
		{name: "go module context", ctx: RepoContext{FullFiles: []FileContext{{Path: "go.mod", Content: "module example.com/app\n"}}}, want: true},
		{name: "related go test", ctx: RepoContext{RelatedTests: []FileContext{{Path: "internal/app_test.go"}}}, want: true},
		{name: "non go", ctx: RepoContext{Patches: []PatchContext{{Path: "README.md"}}}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectGoProject(tt.ctx); got != tt.want {
				t.Fatalf("DetectGoProject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlanGoAnalyzerCommandsUsesOnlyFixedArgvUnderSafeWorkspace(t *testing.T) {
	root := t.TempDir()
	moduleDir := filepath.Join(root, "service")
	if err := os.Mkdir(moduleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plans, err := PlanGoAnalyzerCommands(SafeGoWorkspace{Root: root, WorkDir: moduleDir, HeadSHA: "abc"}, "abc")
	if err != nil {
		t.Fatalf("PlanGoAnalyzerCommands() error = %v", err)
	}
	wantArgv := [][]string{{"go", "test", "./..."}, {"go", "vet", "./..."}}
	if len(plans) != len(wantArgv) {
		t.Fatalf("plans = %#v, want %d", plans, len(wantArgv))
	}
	for i, plan := range plans {
		if !reflect.DeepEqual(plan.Argv, wantArgv[i]) {
			t.Fatalf("plan[%d].Argv = %#v, want %#v", i, plan.Argv, wantArgv[i])
		}
		if plan.WorkDir != moduleDir {
			t.Fatalf("plan[%d].WorkDir = %q, want %q", i, plan.WorkDir, moduleDir)
		}
	}
}

func TestPlanGoAnalyzerCommandsRejectsUnsafeWorkspace(t *testing.T) {
	root := t.TempDir()
	tests := []SafeGoWorkspace{
		{Root: root, WorkDir: filepath.Dir(root), HeadSHA: "abc"},
		{Root: root, WorkDir: filepath.Join(root, ".."), HeadSHA: "abc"},
		{Root: root, WorkDir: root, HeadSHA: "other"},
		{Root: "", WorkDir: root, HeadSHA: "abc"},
	}
	for _, workspace := range tests {
		if _, err := PlanGoAnalyzerCommands(workspace, "abc"); !errors.Is(err, ErrUnsafeAnalyzerWorkspace) {
			t.Fatalf("PlanGoAnalyzerCommands(%+v) error = %v, want ErrUnsafeAnalyzerWorkspace", workspace, err)
		}
	}
}

func TestGoAnalyzerSkipsNonGoAndUnsafeWorkspace(t *testing.T) {
	analyzer := NewGoAnalyzer(nil, nil, GoAnalyzerOptions{})
	nonGo := analyzer.Analyze(context.Background(), Job{HeadSHA: "abc"}, RepoContext{Patches: []PatchContext{{Path: "README.md"}}})
	if nonGo.Stats.ExitCategories[GoAnalyzerExitSkipped] != 1 || len(nonGo.Evidence) != 1 {
		t.Fatalf("non-Go result = %+v", nonGo)
	}
	if !strings.Contains(strings.Join(nonGo.Limitations, " "), "not a Go project") {
		t.Fatalf("non-Go limitations = %#v", nonGo.Limitations)
	}

	goProject := analyzer.Analyze(context.Background(), Job{HeadSHA: "abc"}, RepoContext{Patches: []PatchContext{{Path: "main.go"}}})
	if goProject.Stats.ExitCategories[GoAnalyzerExitSkipped] != 1 || len(goProject.Evidence) != 1 {
		t.Fatalf("unsafe workspace result = %+v", goProject)
	}
	if !strings.Contains(strings.Join(goProject.Limitations, " "), "safe workspace unavailable") {
		t.Fatalf("unsafe workspace limitations = %#v", goProject.Limitations)
	}
}

func TestGoAnalyzerExecutionCategoriesOutputLimitAndSecretFreeEnvironment(t *testing.T) {
	root := t.TempDir()
	provider := staticWorkspaceProvider{workspace: SafeGoWorkspace{Root: root, WorkDir: root, HeadSHA: "abc"}}
	executor := &recordingExecutor{results: []GoCommandExecution{
		{ExitCode: 0, Stdout: "ok"},
		{ExitCode: 1, Stderr: strings.Repeat("x", 40) + "\nmain.go:12:6: undefined: missingSecret\n"},
	}}
	analyzer := NewGoAnalyzer(provider, executor, GoAnalyzerOptions{Timeout: time.Second, OutputLimitBytes: 32})
	t.Setenv("OPENAI_API_KEY", "must-not-leak")
	t.Setenv("GITHUB_TOKEN", "must-not-leak")

	result := analyzer.Analyze(context.Background(), Job{HeadSHA: "abc"}, RepoContext{Patches: []PatchContext{{Path: "main.go"}}})

	if result.Stats.ExitCategories[GoAnalyzerExitSuccess] != 1 || result.Stats.ExitCategories[GoAnalyzerExitFailure] != 1 {
		t.Fatalf("exit categories = %#v", result.Stats.ExitCategories)
	}
	if !result.Stats.OutputTruncated {
		t.Fatalf("OutputTruncated = false, want true")
	}
	if len(executor.calls) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(executor.calls))
	}
	for _, call := range executor.calls {
		for _, env := range call.Env {
			if strings.Contains(env, "OPENAI_API_KEY") || strings.Contains(env, "GITHUB_TOKEN") || strings.Contains(env, "must-not-leak") {
				t.Fatalf("secret env propagated: %#v", call.Env)
			}
		}
	}
}

func TestGoAnalyzerMapsTimeoutUnavailableAndInternalErrorsNonBlocking(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		err  error
		want GoAnalyzerExitCategory
	}{
		{name: "timeout", err: ErrAnalyzerTimeout, want: GoAnalyzerExitTimeout},
		{name: "unavailable", err: ErrAnalyzerToolUnavailable, want: GoAnalyzerExitUnavailable},
		{name: "internal", err: errors.New("boom"), want: GoAnalyzerExitInternalError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewGoAnalyzer(
				staticWorkspaceProvider{workspace: SafeGoWorkspace{Root: root, WorkDir: root, HeadSHA: "abc"}},
				&recordingExecutor{err: tt.err},
				GoAnalyzerOptions{Timeout: time.Second, OutputLimitBytes: 128},
			)
			result := analyzer.Analyze(context.Background(), Job{HeadSHA: "abc"}, RepoContext{Patches: []PatchContext{{Path: "main.go"}}})
			if result.Stats.ExitCategories[tt.want] != 2 {
				t.Fatalf("exit categories = %#v, want two %s", result.Stats.ExitCategories, tt.want)
			}
			if len(result.Limitations) == 0 {
				t.Fatalf("limitations = nil, want safe limitation")
			}
		})
	}
}

func TestParseGoAnalyzerOutputSanitizesAndExtractsEvidence(t *testing.T) {
	output := "panic: leaked token abc123\n./internal/app.go:17:6: undefined: missingThing\nFAIL\texample.com/app/internal\t0.01s\n"
	items, limitations := ParseGoAnalyzerOutput(GoAnalyzerCommandPlan{Tool: "go test"}, GoAnalyzerExitFailure, output, false)
	if len(limitations) != 0 {
		t.Fatalf("limitations = %#v, want none", limitations)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v, want one", items)
	}
	got := items[0]
	if got.SourceType != EvidenceSourceStaticCheck || got.Tool != "go test" || got.Path != "internal/app.go" || got.Line == nil || *got.Line != 17 {
		t.Fatalf("item = %+v", got)
	}
	if strings.Contains(got.Message, "token abc123") || len(got.Message) > maxStaticCheckMessageBytes {
		t.Fatalf("message not sanitized/bounded: %q", got.Message)
	}
}

func TestParseGoAnalyzerOutputHandlesMalformedAndTruncatedOutput(t *testing.T) {
	items, limitations := ParseGoAnalyzerOutput(GoAnalyzerCommandPlan{Tool: "go vet"}, GoAnalyzerExitFailure, "not parseable", true)
	if len(items) != 0 {
		t.Fatalf("items = %#v, want none", items)
	}
	if len(limitations) == 0 || !strings.Contains(strings.Join(limitations, " "), "truncated") {
		t.Fatalf("limitations = %#v, want truncation limitation", limitations)
	}
}

type staticWorkspaceProvider struct {
	workspace SafeGoWorkspace
	err       error
}

func (p staticWorkspaceProvider) WorkspaceForReview(context.Context, Job, RepoContext) (SafeGoWorkspace, error) {
	return p.workspace, p.err
}

type recordingExecutor struct {
	calls   []GoAnalyzerCommandPlan
	results []GoCommandExecution
	err     error
}

func (e *recordingExecutor) Execute(ctx context.Context, plan GoAnalyzerCommandPlan) (GoCommandExecution, error) {
	e.calls = append(e.calls, plan)
	if e.err != nil {
		return GoCommandExecution{}, e.err
	}
	if len(e.results) == 0 {
		return GoCommandExecution{}, nil
	}
	result := e.results[0]
	e.results = e.results[1:]
	return result, nil
}
