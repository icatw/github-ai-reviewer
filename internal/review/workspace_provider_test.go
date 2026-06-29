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

func TestLocalGoWorkspaceProviderCreatesContainedWorkspace(t *testing.T) {
	root := t.TempDir()
	executor := &recordingGitExecutor{heads: []string{strings.Repeat("a", 40)}}
	provider := NewLocalGoWorkspaceProvider(LocalGoWorkspaceProviderOptions{
		Enabled:  true,
		Root:     root,
		Executor: executor,
		Timeout:  time.Second,
	})
	headSHA := strings.Repeat("a", 40)

	workspace, err := provider.WorkspaceForReview(context.Background(), Job{
		InstallationID: 42,
		Owner:          "octo",
		Repo:           "repo",
		PullNumber:     7,
		HeadSHA:        headSHA,
		DeliveryID:     "../delivery",
	}, RepoContext{Patches: []PatchContext{{Path: "main.go"}}})
	if err != nil {
		t.Fatalf("WorkspaceForReview() error = %v", err)
	}
	if workspace.Root == "" || workspace.WorkDir == "" || workspace.HeadSHA != headSHA || !workspace.WorkspaceValidated {
		t.Fatalf("workspace = %+v, want validated workspace", workspace)
	}
	if !pathContained(root, workspace.Root) || !pathContained(root, workspace.WorkDir) {
		t.Fatalf("workspace paths escaped root: root=%q workspace=%+v", root, workspace)
	}
	if strings.Contains(workspace.Root, "..") || strings.Contains(workspace.Root, string(filepath.Separator)+".."+string(filepath.Separator)) {
		t.Fatalf("workspace root was not sanitized: %q", workspace.Root)
	}
	assertNoSecretInGitCalls(t, executor.calls, "token-value")
}

func TestLocalGoWorkspaceProviderUsesCredentialOnlyForFetchWithoutLeakingToken(t *testing.T) {
	root := t.TempDir()
	headSHA := strings.Repeat("a", 40)
	token := "sentinel-checkout-token-1234567890"
	executor := &recordingGitExecutor{heads: []string{headSHA}}
	provider := NewLocalGoWorkspaceProvider(LocalGoWorkspaceProviderOptions{
		Enabled:            true,
		Root:               root,
		Executor:           executor,
		CheckoutCredential: CheckoutCredential{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: headSHA, token: token},
	})

	_, err := provider.WorkspaceForReview(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 1, HeadSHA: headSHA}, RepoContext{Patches: []PatchContext{{Path: "main.go"}}})
	if err != nil {
		t.Fatalf("WorkspaceForReview() error = %v", err)
	}

	fetchCredentialEnv := 0
	for _, call := range executor.calls {
		joinedArgv := strings.Join(call.Argv, " ")
		if strings.Contains(joinedArgv, token) {
			t.Fatalf("git argv leaked token: %+v", call)
		}
		if strings.Contains(joinedArgv, "@github.com") {
			t.Fatalf("git argv used tokenized/userinfo remote: %+v", call)
		}
		hasCredential := strings.Contains(strings.Join(call.Env, "\n"), "GITHUB_AI_REVIEWER_CHECKOUT_CREDENTIAL_ID=")
		isFetch := len(call.Argv) >= 4 && call.Argv[3] == "fetch"
		if hasCredential && !isFetch {
			t.Fatalf("credential env used outside fetch: %+v", call)
		}
		if hasCredential {
			fetchCredentialEnv++
		}
	}
	if fetchCredentialEnv != 1 {
		t.Fatalf("credential-bearing fetch env count = %d, want 1", fetchCredentialEnv)
	}
}

func TestLocalGoWorkspaceProviderRejectsCredentialScopeMismatchBeforeGit(t *testing.T) {
	root := t.TempDir()
	executor := &recordingGitExecutor{}
	provider := NewLocalGoWorkspaceProvider(LocalGoWorkspaceProviderOptions{
		Enabled:            true,
		Root:               root,
		Executor:           executor,
		CheckoutCredential: CheckoutCredential{InstallationID: 43, Owner: "octo", Repo: "repo", HeadSHA: strings.Repeat("a", 40), token: "sentinel-checkout-token"},
	})

	_, err := provider.WorkspaceForReview(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 1, HeadSHA: strings.Repeat("a", 40)}, RepoContext{Patches: []PatchContext{{Path: "main.go"}}})

	var providerErr WorkspaceProviderFailure
	if !errors.As(err, &providerErr) || providerErr.Category != GoAnalyzerCredentialScopeMismatch {
		t.Fatalf("error = %v, want credential scope mismatch", err)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("git calls = %d, want no checkout with mismatched credential", len(executor.calls))
	}
}

func TestLocalGoWorkspaceProviderCleansCredentialIDWhenCheckoutFailsBeforeFetch(t *testing.T) {
	root := t.TempDir()
	headSHA := strings.Repeat("a", 40)
	executor := &recordingGitExecutor{err: ErrWorkspacePathInvalid}
	provider := NewLocalGoWorkspaceProvider(LocalGoWorkspaceProviderOptions{
		Enabled:            true,
		Root:               root,
		Executor:           executor,
		CheckoutCredential: CheckoutCredential{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: headSHA, token: "sentinel-checkout-token"},
	})

	_, err := provider.WorkspaceForReview(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 1, HeadSHA: headSHA}, RepoContext{Patches: []PatchContext{{Path: "main.go"}}})

	var providerErr WorkspaceProviderFailure
	if !errors.As(err, &providerErr) || providerErr.Category != GoAnalyzerPathInvalid {
		t.Fatalf("error = %v, want path invalid", err)
	}
	for _, call := range executor.calls {
		id := envValue(call.Env, "GITHUB_AI_REVIEWER_CHECKOUT_CREDENTIAL_ID")
		if id == "" {
			continue
		}
		if _, ok := checkoutCredentialTokens.Load(id); ok {
			t.Fatalf("credential id %q was not cleaned up", id)
		}
	}
}

func TestLocalGoWorkspaceProviderRejectsUnsafePaths(t *testing.T) {
	root := t.TempDir()
	if _, err := ValidateContainedPath(root, filepath.Join(root, "repo")); err != nil {
		t.Fatalf("valid contained path error = %v", err)
	}
	for _, candidate := range []string{
		filepath.Join(root, "..", "escape"),
		filepath.Dir(root),
		string([]byte{0}),
		"",
	} {
		if _, err := ValidateContainedPath(root, candidate); !errors.Is(err, ErrWorkspacePathInvalid) {
			t.Fatalf("ValidateContainedPath(%q) error = %v, want ErrWorkspacePathInvalid", candidate, err)
		}
	}

	target := t.TempDir()
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := ValidateContainedPath(root, link); !errors.Is(err, ErrWorkspacePathInvalid) {
		t.Fatalf("ValidateContainedPath(symlink escape) error = %v, want ErrWorkspacePathInvalid", err)
	}
}

func TestLocalGoWorkspaceProviderPlansFixedGitArgv(t *testing.T) {
	plans := PlanGitCheckoutCommands(GitCheckoutPlanInput{
		RemoteURL: "https://github.com/octo/repo.git",
		HeadSHA:   strings.Repeat("a", 40),
		WorkDir:   "/tmp/work/repo",
		Timeout:   2 * time.Second,
	})
	got := make([][]string, 0, len(plans))
	for _, plan := range plans {
		got = append(got, plan.Argv)
		if plan.Shell || strings.Contains(strings.Join(plan.Argv, " "), "token-value") {
			t.Fatalf("unsafe git plan = %+v", plan)
		}
	}
	want := [][]string{
		{"git", "init", "/tmp/work/repo"},
		{"git", "-C", "/tmp/work/repo", "remote", "add", "origin", "https://github.com/octo/repo.git"},
		{"git", "-C", "/tmp/work/repo", "fetch", "--depth=1", "--filter=blob:none", "origin", strings.Repeat("a", 40)},
		{"git", "-C", "/tmp/work/repo", "checkout", "--detach", strings.Repeat("a", 40)},
		{"git", "-C", "/tmp/work/repo", "rev-parse", "HEAD"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("argv = %#v, want %#v", got, want)
	}
}

func TestPlanGitCheckoutCommandsRejectsTokenizedRemote(t *testing.T) {
	plans := PlanGitCheckoutCommands(GitCheckoutPlanInput{
		RemoteURL: "https://x-access-token:sentinel-checkout-token@github.com/octo/repo.git",
		HeadSHA:   strings.Repeat("a", 40),
		WorkDir:   "/tmp/work/repo",
	})
	for _, plan := range plans {
		if len(plan.Argv) >= 4 && plan.Argv[3] == "remote" && isFixedGitArgv(plan.Argv) {
			t.Fatalf("tokenized git argv accepted: %+v", plan)
		}
	}
}

func TestLocalGoWorkspaceProviderMapsCheckoutFailures(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		head string
		want GoAnalyzerExitCategory
	}{
		{name: "command failure", err: errors.New("git failed"), want: GoAnalyzerCheckoutFailed},
		{name: "timeout", err: ErrWorkspaceCheckoutTimeout, want: GoAnalyzerCheckoutTimeout},
		{name: "head mismatch", head: strings.Repeat("b", 40), want: GoAnalyzerHeadMismatch},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			headSHA := strings.Repeat("a", 40)
			provider := NewLocalGoWorkspaceProvider(LocalGoWorkspaceProviderOptions{
				Enabled:  true,
				Root:     root,
				Executor: &recordingGitExecutor{err: tt.err, heads: []string{tt.head}},
			})
			_, err := provider.WorkspaceForReview(context.Background(), Job{Owner: "octo", Repo: "repo", PullNumber: 1, HeadSHA: headSHA}, RepoContext{})
			var providerErr WorkspaceProviderFailure
			if !errors.As(err, &providerErr) || providerErr.Category != tt.want {
				t.Fatalf("error = %v, want category %s", err, tt.want)
			}
		})
	}
}

func TestLocalGoWorkspaceProviderMapsCredentialFailures(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want GoAnalyzerExitCategory
	}{
		{name: "provider failure", err: WorkspaceProviderFailure{Category: GoAnalyzerCredentialUnavailable}, want: GoAnalyzerCredentialUnavailable},
		{name: "plain failure", err: ErrCheckoutCredentialUnavailable, want: GoAnalyzerCredentialUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			provider := NewLocalGoWorkspaceProvider(LocalGoWorkspaceProviderOptions{
				Enabled:            true,
				Root:               root,
				Executor:           &recordingGitExecutor{},
				CredentialProvider: staticCheckoutCredentialProvider{err: tt.err},
			})
			_, err := provider.WorkspaceForReview(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 1, HeadSHA: strings.Repeat("a", 40)}, RepoContext{Patches: []PatchContext{{Path: "main.go"}}})
			var providerErr WorkspaceProviderFailure
			if !errors.As(err, &providerErr) || providerErr.Category != tt.want {
				t.Fatalf("error = %v, want category %s", err, tt.want)
			}
		})
	}
}

func TestCleanupSafeWorkspaceValidatesTarget(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "job")
	if err := os.Mkdir(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CleanupSafeWorkspace(context.Background(), root, inside); err != nil {
		t.Fatalf("CleanupSafeWorkspace(contained) error = %v", err)
	}
	if _, err := os.Stat(inside); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("workspace still exists or unexpected stat error: %v", err)
	}
	if err := CleanupSafeWorkspace(context.Background(), root, filepath.Dir(root)); !errors.Is(err, ErrWorkspacePathInvalid) {
		t.Fatalf("CleanupSafeWorkspace(escape) error = %v, want path invalid", err)
	}
}

type recordingGitExecutor struct {
	calls []GitCommandPlan
	heads []string
	err   error
}

type staticCheckoutCredentialProvider struct {
	credential CheckoutCredential
	err        error
}

func (p staticCheckoutCredentialProvider) CheckoutCredential(context.Context, CheckoutCredentialRequest) (CheckoutCredential, error) {
	return p.credential, p.err
}

func (e *recordingGitExecutor) ExecuteGit(ctx context.Context, plan GitCommandPlan) (GitCommandResult, error) {
	e.calls = append(e.calls, plan)
	if e.err != nil {
		return GitCommandResult{}, e.err
	}
	if len(plan.Argv) >= 2 && plan.Argv[len(plan.Argv)-2] == "rev-parse" {
		head := ""
		if len(e.heads) > 0 {
			head = e.heads[0]
			e.heads = e.heads[1:]
		}
		return GitCommandResult{Stdout: head + "\n"}, nil
	}
	return GitCommandResult{}, nil
}

func pathContained(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func assertNoSecretInGitCalls(t *testing.T, calls []GitCommandPlan, secret string) {
	t.Helper()
	for _, call := range calls {
		if strings.Contains(strings.Join(call.Argv, " "), secret) || strings.Contains(strings.Join(call.Env, " "), secret) {
			t.Fatalf("git command leaked secret: %+v", call)
		}
	}
}
