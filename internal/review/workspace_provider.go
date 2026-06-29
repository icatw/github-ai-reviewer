package review

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultWorkspaceCheckoutTimeout = 30 * time.Second
	defaultGitOutputLimitBytes      = 16 * 1024
)

var (
	ErrWorkspacePathInvalid          = errors.New("workspace path invalid")
	ErrWorkspaceCheckoutTimeout      = errors.New("workspace checkout timeout")
	ErrCheckoutCredentialUnavailable = errors.New("checkout credential unavailable")
)

var checkoutCredentialTokens sync.Map

type WorkspaceProviderFailure struct {
	Category GoAnalyzerExitCategory
}

func WorkspaceProviderError(category GoAnalyzerExitCategory, message string) error {
	return WorkspaceProviderFailure{Category: category}
}

func (e WorkspaceProviderFailure) Error() string {
	if e.Category == "" {
		return string(GoAnalyzerProviderUnavailable)
	}
	return string(e.Category)
}

type LocalGoWorkspaceProviderOptions struct {
	Enabled            bool
	Root               string
	Timeout            time.Duration
	OutputLimitBytes   int
	Executor           GitCommandExecutor
	CredentialProvider CheckoutCredentialProvider
	CheckoutCredential CheckoutCredential
}

type LocalGoWorkspaceProvider struct {
	options LocalGoWorkspaceProviderOptions
}

type GitCommandPlan struct {
	Argv             []string
	Env              []string
	Timeout          time.Duration
	OutputLimitBytes int
	Shell            bool
}

type GitCommandResult struct {
	Stdout string
	Stderr string
}

type GitCommandExecutor interface {
	ExecuteGit(ctx context.Context, plan GitCommandPlan) (GitCommandResult, error)
}

type GitCommandExecutorFunc func(context.Context, GitCommandPlan) (GitCommandResult, error)

func (f GitCommandExecutorFunc) ExecuteGit(ctx context.Context, plan GitCommandPlan) (GitCommandResult, error) {
	return f(ctx, plan)
}

type GitCheckoutPlanInput struct {
	RemoteURL string
	HeadSHA   string
	WorkDir   string
	Timeout   time.Duration
}

func NewLocalGoWorkspaceProvider(options LocalGoWorkspaceProviderOptions) *LocalGoWorkspaceProvider {
	if options.Timeout <= 0 {
		options.Timeout = defaultWorkspaceCheckoutTimeout
	}
	if options.OutputLimitBytes <= 0 {
		options.OutputLimitBytes = defaultGitOutputLimitBytes
	}
	if options.Executor == nil {
		options.Executor = GitCommandExecutorFunc(ExecuteGitCommand)
	}
	return &LocalGoWorkspaceProvider{options: options}
}

func (p *LocalGoWorkspaceProvider) WorkspaceForReview(ctx context.Context, job Job, repoContext RepoContext) (SafeGoWorkspace, error) {
	if p == nil || !p.options.Enabled {
		return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: GoAnalyzerProviderDisabled}
	}
	root := strings.TrimSpace(p.options.Root)
	if root == "" {
		root = filepath.Join(os.TempDir(), "github-ai-reviewer-workspaces")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: GoAnalyzerPathInvalid}
	}
	root, err := ValidateContainedPath(filepath.Dir(root), root)
	if err != nil {
		return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: GoAnalyzerPathInvalid}
	}
	jobRoot := filepath.Join(root, workspaceJobDir(job))
	if err := os.MkdirAll(jobRoot, 0o700); err != nil {
		return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: GoAnalyzerPathInvalid}
	}
	jobRoot, err = ValidateContainedPath(root, jobRoot)
	if err != nil {
		return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: GoAnalyzerPathInvalid}
	}
	workDir := filepath.Join(jobRoot, "repo")
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: GoAnalyzerPathInvalid}
	}
	workDir, err = ValidateContainedPath(jobRoot, workDir)
	if err != nil {
		return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: GoAnalyzerPathInvalid}
	}

	credential, err := p.checkoutCredential(ctx, job)
	if err != nil {
		return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: checkoutCredentialFailureCategory(err)}
	}
	plans := PlanGitCheckoutCommands(GitCheckoutPlanInput{
		RemoteURL: githubHTTPSRemote(job.Owner, job.Repo),
		HeadSHA:   job.HeadSHA,
		WorkDir:   workDir,
		Timeout:   p.options.Timeout,
	})
	cleanupCredential := func() {}
	if token, ok := credential.TokenForCheckout(job); ok {
		plans, cleanupCredential, err = InjectCheckoutCredential(plans, token)
		if err != nil {
			return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: GoAnalyzerCredentialInjectionFailed}
		}
	}
	defer cleanupCredential()
	var head string
	for _, plan := range plans {
		plan.OutputLimitBytes = p.options.OutputLimitBytes
		result, err := p.options.Executor.ExecuteGit(ctx, plan)
		if err != nil {
			return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: checkoutFailureCategory(err)}
		}
		if len(plan.Argv) >= 2 && plan.Argv[len(plan.Argv)-2] == "rev-parse" && plan.Argv[len(plan.Argv)-1] == "HEAD" {
			head = strings.TrimSpace(result.Stdout)
		}
	}
	if head == "" || head != job.HeadSHA {
		return SafeGoWorkspace{}, WorkspaceProviderFailure{Category: GoAnalyzerHeadMismatch}
	}
	return SafeGoWorkspace{
		Root:               jobRoot,
		WorkDir:            workDir,
		HeadSHA:            head,
		WorkspaceValidated: true,
		Cleanup: func(ctx context.Context) error {
			return CleanupSafeWorkspace(ctx, root, jobRoot)
		},
	}, nil
}

func (p *LocalGoWorkspaceProvider) checkoutCredential(ctx context.Context, job Job) (CheckoutCredential, error) {
	if p.options.CredentialProvider != nil {
		return p.options.CredentialProvider.CheckoutCredential(ctx, CheckoutCredentialRequest{
			InstallationID: job.InstallationID,
			Owner:          job.Owner,
			Repo:           job.Repo,
			HeadSHA:        job.HeadSHA,
		})
	}
	if p.options.CheckoutCredential.token == "" {
		return CheckoutCredential{}, nil
	}
	if _, ok := p.options.CheckoutCredential.TokenForCheckout(job); !ok {
		return CheckoutCredential{}, WorkspaceProviderFailure{Category: GoAnalyzerCredentialScopeMismatch}
	}
	return p.options.CheckoutCredential, nil
}

func PlanGitCheckoutCommands(input GitCheckoutPlanInput) []GitCommandPlan {
	timeout := input.Timeout
	if timeout <= 0 {
		timeout = defaultWorkspaceCheckoutTimeout
	}
	env := []string{"GIT_TERMINAL_PROMPT=0"}
	return []GitCommandPlan{
		{Argv: []string{"git", "init", input.WorkDir}, Env: env, Timeout: timeout},
		{Argv: []string{"git", "-C", input.WorkDir, "remote", "add", "origin", input.RemoteURL}, Env: env, Timeout: timeout},
		{Argv: []string{"git", "-C", input.WorkDir, "fetch", "--depth=1", "--filter=blob:none", "origin", input.HeadSHA}, Env: env, Timeout: timeout},
		{Argv: []string{"git", "-C", input.WorkDir, "checkout", "--detach", input.HeadSHA}, Env: env, Timeout: timeout},
		{Argv: []string{"git", "-C", input.WorkDir, "rev-parse", "HEAD"}, Env: env, Timeout: timeout},
	}
}

func InjectCheckoutCredential(plans []GitCommandPlan, token string) ([]GitCommandPlan, func(), error) {
	if strings.TrimSpace(token) == "" {
		return nil, func() {}, ErrCheckoutCredentialUnavailable
	}
	id, err := newCredentialID()
	if err != nil {
		return nil, func() {}, err
	}
	checkoutCredentialTokens.Store(id, token)
	cleanup := func() { checkoutCredentialTokens.Delete(id) }
	out := make([]GitCommandPlan, len(plans))
	for i, plan := range plans {
		out[i] = plan
		out[i].Argv = append([]string(nil), plan.Argv...)
		out[i].Env = append([]string(nil), plan.Env...)
		if len(plan.Argv) >= 4 && plan.Argv[3] == "fetch" {
			out[i].Env = append(out[i].Env, "GITHUB_AI_REVIEWER_CHECKOUT_CREDENTIAL_ID="+id)
		}
	}
	return out, cleanup, nil
}

func ExecuteGitCommand(ctx context.Context, plan GitCommandPlan) (GitCommandResult, error) {
	if !isFixedGitArgv(plan.Argv) || plan.Shell {
		return GitCommandResult{}, ErrWorkspacePathInvalid
	}
	timeout := plan.Timeout
	if timeout <= 0 {
		timeout = defaultWorkspaceCheckoutTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, plan.Argv[0], plan.Argv[1:]...)
	env, cleanup, err := materializeCheckoutCredentialEnv(plan.Env)
	if err != nil {
		return GitCommandResult{}, err
	}
	defer cleanup()
	cmd.Env = env
	var stdout, stderr limitedBuffer
	stdout.limit = plan.OutputLimitBytes
	stderr.limit = plan.OutputLimitBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	result := GitCommandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if runCtx.Err() == context.DeadlineExceeded {
		return result, ErrWorkspaceCheckoutTimeout
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

func newCredentialID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", ErrCheckoutCredentialUnavailable
	}
	return hex.EncodeToString(b[:]), nil
}

func materializeCheckoutCredentialEnv(env []string) ([]string, func(), error) {
	out := append([]string(nil), env...)
	id := envValue(out, "GITHUB_AI_REVIEWER_CHECKOUT_CREDENTIAL_ID")
	if id == "" {
		return out, func() {}, nil
	}
	value, ok := checkoutCredentialTokens.LoadAndDelete(id)
	if !ok {
		return nil, func() {}, ErrCheckoutCredentialUnavailable
	}
	token, ok := value.(string)
	if !ok || token == "" {
		return nil, func() {}, ErrCheckoutCredentialUnavailable
	}
	path, cleanup, err := writeAskPassHelper(token)
	if err != nil {
		return nil, func() {}, err
	}
	out = removeEnvKey(out, "GITHUB_AI_REVIEWER_CHECKOUT_CREDENTIAL_ID")
	out = append(out, "GIT_ASKPASS="+path)
	return out, cleanup, nil
}

func writeAskPassHelper(token string) (string, func(), error) {
	file, err := os.CreateTemp("", "github-ai-reviewer-askpass-*")
	if err != nil {
		return "", func() {}, ErrCheckoutCredentialUnavailable
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	script := "#!/bin/sh\ncase \"$1\" in\n*Username*) printf '%s\\n' x-access-token ;;\n*) printf '%s\\n' " + shellSingleQuote(token) + " ;;\nesac\n"
	if _, err := file.WriteString(script); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, ErrCheckoutCredentialUnavailable
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, ErrCheckoutCredentialUnavailable
	}
	if err := os.Chmod(path, 0o700); err != nil {
		cleanup()
		return "", func() {}, ErrCheckoutCredentialUnavailable
	}
	return path, cleanup, nil
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix)
		}
	}
	return ""
}

func removeEnvKey(env []string, key string) []string {
	prefix := key + "="
	out := env[:0]
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return out
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func ValidateContainedPath(root, candidate string) (string, error) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(candidate) == "" || strings.ContainsRune(root, 0) || strings.ContainsRune(candidate, 0) {
		return "", ErrWorkspacePathInvalid
	}
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", ErrWorkspacePathInvalid
	}
	rootEval, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", ErrWorkspacePathInvalid
	}
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", ErrWorkspacePathInvalid
	}
	candidateEval, err := evalExistingOrParent(candidateAbs)
	if err != nil {
		return "", ErrWorkspacePathInvalid
	}
	rel, err := filepath.Rel(rootEval, candidateEval)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", ErrWorkspacePathInvalid
	}
	return candidateEval, nil
}

func CleanupSafeWorkspace(ctx context.Context, root, target string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	cleanTarget, err := ValidateContainedPath(root, target)
	if err != nil {
		return err
	}
	return os.RemoveAll(cleanTarget)
}

func checkoutFailureCategory(err error) GoAnalyzerExitCategory {
	var providerErr WorkspaceProviderFailure
	if errors.As(err, &providerErr) && providerErr.Category != "" {
		return providerErr.Category
	}
	if errors.Is(err, ErrCheckoutCredentialUnavailable) {
		return GoAnalyzerCredentialUnavailable
	}
	if errors.Is(err, ErrWorkspaceCheckoutTimeout) {
		return GoAnalyzerCheckoutTimeout
	}
	if errors.Is(err, ErrWorkspacePathInvalid) {
		return GoAnalyzerPathInvalid
	}
	return GoAnalyzerCheckoutFailed
}

func evalExistingOrParent(candidate string) (string, error) {
	if _, err := os.Lstat(candidate); err == nil {
		return filepath.EvalSymlinks(candidate)
	}
	parent := filepath.Dir(candidate)
	parentEval, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(parentEval, filepath.Base(candidate)), nil
}

func isFixedGitArgv(argv []string) bool {
	if len(argv) == 0 || argv[0] != "git" {
		return false
	}
	if reflectGitArgv(argv, []string{"git", "init", ""}) {
		return true
	}
	if len(argv) < 4 || argv[1] != "-C" || strings.TrimSpace(argv[2]) == "" {
		return false
	}
	switch {
	case reflectGitArgv(argv[:4], []string{"git", "-C", "", "remote"}) && len(argv) == 7 && argv[4] == "add" && argv[5] == "origin":
		return isSafeHTTPSRemote(argv[6])
	case reflectGitArgv(argv[:4], []string{"git", "-C", "", "fetch"}) && len(argv) == 8 && argv[4] == "--depth=1" && argv[5] == "--filter=blob:none" && argv[6] == "origin":
		return isSafeRef(argv[7])
	case reflectGitArgv(argv[:4], []string{"git", "-C", "", "checkout"}) && len(argv) == 6 && argv[4] == "--detach":
		return isSafeRef(argv[5])
	case reflectGitArgv(argv[:4], []string{"git", "-C", "", "rev-parse"}) && len(argv) == 5 && argv[4] == "HEAD":
		return true
	default:
		return false
	}
}

func reflectGitArgv(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if want[i] != "" && got[i] != want[i] {
			return false
		}
	}
	return true
}

var (
	safeRepoPartPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	safeRefPattern      = regexp.MustCompile(`^[A-Fa-f0-9]{40}$`)
)

func githubHTTPSRemote(owner, repo string) string {
	if !safeRepoPartPattern.MatchString(owner) || !safeRepoPartPattern.MatchString(repo) {
		return "https://github.com/invalid/invalid.git"
	}
	return fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
}

func isSafeHTTPSRemote(remote string) bool {
	if strings.Contains(remote, "@") {
		return false
	}
	return strings.HasPrefix(remote, "https://github.com/") && strings.HasSuffix(remote, ".git")
}

func isSafeRef(ref string) bool {
	return safeRefPattern.MatchString(ref)
}

func workspaceJobDir(job Job) string {
	parts := []string{
		sanitizeWorkspacePart(job.DeliveryID),
		sanitizeWorkspacePart(job.Owner),
		sanitizeWorkspacePart(job.Repo),
		fmt.Sprintf("pr-%d", job.PullNumber),
		sanitizeWorkspacePart(job.HeadSHA),
	}
	return strings.Join(parts, "-")
}

func sanitizeWorkspacePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), ".-")
	if out == "" {
		return "unknown"
	}
	if len(out) > 64 {
		return out[:64]
	}
	return out
}
