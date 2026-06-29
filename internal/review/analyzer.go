package review

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	defaultGoAnalyzerTimeout          = 30 * time.Second
	defaultGoAnalyzerOutputLimitBytes = 64 * 1024
	maxStaticCheckMessageBytes        = 240
)

var (
	ErrUnsafeAnalyzerWorkspace = errors.New("unsafe analyzer workspace")
	ErrAnalyzerTimeout         = errors.New("analyzer timeout")
	ErrAnalyzerToolUnavailable = errors.New("analyzer tool unavailable")
)

type GoAnalyzerExitCategory string

const (
	GoAnalyzerExitSkipped           GoAnalyzerExitCategory = "skipped"
	GoAnalyzerExitUnavailable       GoAnalyzerExitCategory = "unavailable"
	GoAnalyzerExitSuccess           GoAnalyzerExitCategory = "success"
	GoAnalyzerExitFailure           GoAnalyzerExitCategory = "failure"
	GoAnalyzerExitTimeout           GoAnalyzerExitCategory = "timeout"
	GoAnalyzerExitInternalError     GoAnalyzerExitCategory = "internal_error"
	GoAnalyzerProviderDisabled      GoAnalyzerExitCategory = "provider_disabled"
	GoAnalyzerProviderUnavailable   GoAnalyzerExitCategory = "provider_unavailable"
	GoAnalyzerCheckoutFailed        GoAnalyzerExitCategory = "checkout_failed"
	GoAnalyzerCheckoutTimeout       GoAnalyzerExitCategory = "checkout_timeout"
	GoAnalyzerHeadMismatch          GoAnalyzerExitCategory = "head_mismatch"
	GoAnalyzerPathInvalid           GoAnalyzerExitCategory = "path_invalid"
	GoAnalyzerCredentialUnavailable GoAnalyzerExitCategory = "credential_unavailable"
	GoAnalyzerWorkspaceReady        GoAnalyzerExitCategory = "workspace_ready"
	GoAnalyzerCleanupFailed         GoAnalyzerExitCategory = "cleanup_failed"
)

type SafeGoWorkspace struct {
	Root               string
	WorkDir            string
	HeadSHA            string
	WorkspaceValidated bool
	Cleanup            func(context.Context) error
}

type GoAnalyzerCommandPlan struct {
	Tool             string
	Argv             []string
	WorkDir          string
	Env              []string
	Timeout          time.Duration
	OutputLimitBytes int
}

type GoCommandExecution struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

type StaticCheckEvidence struct {
	SourceType         EvidenceSourceType
	Tool               string
	ExitCategory       GoAnalyzerExitCategory
	Package            string
	Path               string
	Line               *int
	Message            string
	Limitations        []string
	WorkspaceValidated bool
}

type GoAnalyzerResult struct {
	Evidence    []StaticCheckEvidence
	Limitations []string
	Stats       GoAnalyzerStats
}

type GoAnalyzerStats struct {
	PlannedCommands  int
	ExecutedCommands int
	ParsedEvidence   int
	OutputTruncated  bool
	ExitCategories   map[GoAnalyzerExitCategory]int
}

type GoAnalyzerOptions struct {
	Timeout          time.Duration
	OutputLimitBytes int
}

type GoWorkspaceProvider interface {
	WorkspaceForReview(ctx context.Context, job Job, repoContext RepoContext) (SafeGoWorkspace, error)
}

type GoCommandExecutor interface {
	Execute(ctx context.Context, plan GoAnalyzerCommandPlan) (GoCommandExecution, error)
}

type GoAnalyzer struct {
	workspaceProvider GoWorkspaceProvider
	executor          GoCommandExecutor
	options           GoAnalyzerOptions
}

func NewGoAnalyzer(provider GoWorkspaceProvider, executor GoCommandExecutor, options GoAnalyzerOptions) *GoAnalyzer {
	if executor == nil {
		executor = GoCommandExecutorFunc(ExecuteGoAnalyzerCommand)
	}
	options = normalizeGoAnalyzerOptions(options)
	return &GoAnalyzer{workspaceProvider: provider, executor: executor, options: options}
}

func (a *GoAnalyzer) Analyze(ctx context.Context, job Job, repoContext RepoContext) GoAnalyzerResult {
	result := GoAnalyzerResult{Stats: GoAnalyzerStats{ExitCategories: map[GoAnalyzerExitCategory]int{}}}
	if !DetectGoProject(repoContext) {
		result.addLimitation(GoAnalyzerExitSkipped, "Go analyzer skipped: not a Go project.")
		return result
	}
	if a == nil || a.workspaceProvider == nil {
		result.addLimitation(GoAnalyzerProviderDisabled, "Go analyzer skipped: safe workspace provider disabled.")
		return result
	}
	workspace, err := a.workspaceProvider.WorkspaceForReview(ctx, job, repoContext)
	if err != nil {
		result.addProviderLimitation(providerFailureCategory(err), "Go analyzer skipped: workspace provider recorded "+string(providerFailureCategory(err))+".")
		return result
	}
	result.Stats.ExitCategories[GoAnalyzerWorkspaceReady]++
	cleanup := func() {
		if workspace.Cleanup != nil {
			if err := workspace.Cleanup(ctx); err != nil {
				result.addProviderLimitation(GoAnalyzerCleanupFailed, "Go analyzer workspace cleanup recorded cleanup_failed.")
			}
		}
	}
	plans, err := PlanGoAnalyzerCommands(workspace, job.HeadSHA)
	if err != nil {
		result.addProviderLimitation(GoAnalyzerPathInvalid, "Go analyzer skipped: unsafe workspace.")
		cleanup()
		result.Stats.ParsedEvidence = len(result.Evidence)
		return result
	}
	for i := range plans {
		plans[i].Timeout = a.options.Timeout
		plans[i].OutputLimitBytes = a.options.OutputLimitBytes
		plans[i].Env = MinimalGoAnalyzerEnv()
	}
	result.Stats.PlannedCommands = len(plans)
	for _, plan := range plans {
		execution, err := a.executor.Execute(ctx, plan)
		category := classifyGoAnalyzerExecution(execution, err)
		result.Stats.ExecutedCommands++
		result.Stats.ExitCategories[category]++
		if err != nil {
			result.addLimitationEvidence(plan.Tool, category, safeAnalyzerLimitation(plan.Tool, category))
			continue
		}
		output, truncated := boundAnalyzerOutput(execution.Stdout+"\n"+execution.Stderr, a.options.OutputLimitBytes)
		result.Stats.OutputTruncated = result.Stats.OutputTruncated || truncated
		evidence, limitations := ParseGoAnalyzerOutput(plan, category, output, truncated)
		for i := range evidence {
			evidence[i].WorkspaceValidated = workspace.WorkspaceValidated
		}
		result.Evidence = append(result.Evidence, evidence...)
		for _, limitation := range limitations {
			result.addLimitationEvidence(plan.Tool, category, limitation)
		}
	}
	cleanup()
	result.Stats.ParsedEvidence = len(result.Evidence)
	return result
}

func (r *GoAnalyzerResult) addLimitation(category GoAnalyzerExitCategory, message string) {
	if r.Stats.ExitCategories == nil {
		r.Stats.ExitCategories = map[GoAnalyzerExitCategory]int{}
	}
	r.Stats.ExitCategories[category]++
	r.Limitations = append(r.Limitations, message)
	r.Evidence = append(r.Evidence, StaticCheckEvidence{
		SourceType:   EvidenceSourceStaticCheck,
		Tool:         "go analyzer",
		ExitCategory: category,
		Message:      sanitizeAnalyzerMessage(message),
		Limitations:  []string{sanitizeAnalyzerMessage(message)},
	})
}

func (r *GoAnalyzerResult) addLimitationEvidence(tool string, category GoAnalyzerExitCategory, message string) {
	message = sanitizeAnalyzerMessage(message)
	r.Limitations = append(r.Limitations, message)
	r.Evidence = append(r.Evidence, StaticCheckEvidence{
		SourceType:   EvidenceSourceStaticCheck,
		Tool:         tool,
		ExitCategory: category,
		Message:      message,
		Limitations:  []string{message},
	})
}

func (r *GoAnalyzerResult) addProviderLimitation(category GoAnalyzerExitCategory, message string) {
	r.addLimitation(category, message)
}

type GoCommandExecutorFunc func(ctx context.Context, plan GoAnalyzerCommandPlan) (GoCommandExecution, error)

func (f GoCommandExecutorFunc) Execute(ctx context.Context, plan GoAnalyzerCommandPlan) (GoCommandExecution, error) {
	return f(ctx, plan)
}

func DetectGoProject(ctx RepoContext) bool {
	for _, path := range repoContextPaths(ctx) {
		base := filepath.Base(path)
		if base == "go.mod" || strings.HasSuffix(path, ".go") {
			return true
		}
	}
	return false
}

func repoContextPaths(ctx RepoContext) []string {
	var paths []string
	for _, item := range ctx.Patches {
		paths = append(paths, item.Path)
	}
	for _, item := range ctx.FullFiles {
		paths = append(paths, item.Path)
	}
	for _, item := range ctx.RelatedTests {
		paths = append(paths, item.Path)
	}
	for _, item := range ctx.RepoDocs {
		paths = append(paths, item.Path)
	}
	for _, item := range ctx.Omitted {
		paths = append(paths, item.Path)
	}
	return paths
}

func PlanGoAnalyzerCommands(workspace SafeGoWorkspace, expectedHeadSHA string) ([]GoAnalyzerCommandPlan, error) {
	workDir, err := validateSafeGoWorkspace(workspace, expectedHeadSHA)
	if err != nil {
		return nil, err
	}
	return []GoAnalyzerCommandPlan{
		{Tool: "go test", Argv: []string{"go", "test", "./..."}, WorkDir: workDir},
		{Tool: "go vet", Argv: []string{"go", "vet", "./..."}, WorkDir: workDir},
	}, nil
}

func validateSafeGoWorkspace(workspace SafeGoWorkspace, expectedHeadSHA string) (string, error) {
	if strings.TrimSpace(workspace.Root) == "" || strings.TrimSpace(workspace.WorkDir) == "" || strings.TrimSpace(workspace.HeadSHA) == "" || workspace.HeadSHA != expectedHeadSHA || !workspace.WorkspaceValidated {
		return "", ErrUnsafeAnalyzerWorkspace
	}
	root, err := filepath.Abs(workspace.Root)
	if err != nil {
		return "", ErrUnsafeAnalyzerWorkspace
	}
	workDir, err := filepath.Abs(workspace.WorkDir)
	if err != nil {
		return "", ErrUnsafeAnalyzerWorkspace
	}
	rel, err := filepath.Rel(root, workDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", ErrUnsafeAnalyzerWorkspace
	}
	return workDir, nil
}

func providerFailureCategory(err error) GoAnalyzerExitCategory {
	var providerErr WorkspaceProviderFailure
	if errors.As(err, &providerErr) && providerErr.Category != "" {
		return providerErr.Category
	}
	return GoAnalyzerProviderUnavailable
}

func ExecuteGoAnalyzerCommand(ctx context.Context, plan GoAnalyzerCommandPlan) (GoCommandExecution, error) {
	if !isFixedGoAnalyzerArgv(plan.Argv) {
		return GoCommandExecution{}, ErrUnsafeAnalyzerWorkspace
	}
	timeout := plan.Timeout
	if timeout <= 0 {
		timeout = defaultGoAnalyzerTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(runCtx, plan.Argv[0], plan.Argv[1:]...)
	cmd.Dir = plan.WorkDir
	cmd.Env = append([]string(nil), plan.Env...)
	var stdout, stderr limitedBuffer
	stdout.limit = plan.OutputLimitBytes
	stderr.limit = plan.OutputLimitBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	execution := GoCommandExecution{Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(start)}
	if runCtx.Err() == context.DeadlineExceeded {
		return execution, ErrAnalyzerTimeout
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			execution.ExitCode = exitErr.ExitCode()
			return execution, nil
		}
		if errors.Is(err, exec.ErrNotFound) {
			return execution, ErrAnalyzerToolUnavailable
		}
		return execution, err
	}
	return execution, nil
}

type limitedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit <= 0 {
		b.limit = defaultGoAnalyzerOutputLimitBytes
	}
	accepted := len(p)
	remaining := b.limit - b.Buffer.Len()
	if remaining <= 0 {
		return accepted, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	_, _ = b.Buffer.Write(p)
	return accepted, nil
}

func isFixedGoAnalyzerArgv(argv []string) bool {
	return reflectArgv(argv, []string{"go", "test", "./..."}) || reflectArgv(argv, []string{"go", "vet", "./..."})
}

func reflectArgv(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func MinimalGoAnalyzerEnv() []string {
	return []string{
		"HOME=",
		"GOCACHE=",
		"GOMODCACHE=",
		"GONOSUMDB=",
		"GOPRIVATE=",
		"GOSUMDB=sum.golang.org",
		"PATH=/usr/local/go/bin:/usr/bin:/bin",
	}
}

func classifyGoAnalyzerExecution(execution GoCommandExecution, err error) GoAnalyzerExitCategory {
	switch {
	case errors.Is(err, ErrAnalyzerTimeout):
		return GoAnalyzerExitTimeout
	case errors.Is(err, ErrAnalyzerToolUnavailable):
		return GoAnalyzerExitUnavailable
	case err != nil:
		return GoAnalyzerExitInternalError
	case execution.ExitCode != 0:
		return GoAnalyzerExitFailure
	default:
		return GoAnalyzerExitSuccess
	}
}

func safeAnalyzerLimitation(tool string, category GoAnalyzerExitCategory) string {
	return fmt.Sprintf("Go analyzer %s recorded %s; raw output omitted.", tool, category)
}

func boundAnalyzerOutput(output string, limit int) (string, bool) {
	if limit <= 0 {
		limit = defaultGoAnalyzerOutputLimitBytes
	}
	if len(output) <= limit {
		return output, false
	}
	return output[:limit], true
}

var (
	goFileLinePattern = regexp.MustCompile(`^(?:\./)?([^:\s]+\.go):([0-9]+)(?::[0-9]+)?:\s*(.+)$`)
	goFailPattern     = regexp.MustCompile(`^FAIL\s+([^\s]+)`)
)

func ParseGoAnalyzerOutput(plan GoAnalyzerCommandPlan, category GoAnalyzerExitCategory, output string, truncated bool) ([]StaticCheckEvidence, []string) {
	var items []StaticCheckEvidence
	var limitations []string
	seen := map[string]struct{}{}
	currentPackage := ""
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if m := goFailPattern.FindStringSubmatch(line); len(m) == 2 {
			currentPackage = m[1]
			continue
		}
		m := goFileLinePattern.FindStringSubmatch(line)
		if len(m) != 4 {
			continue
		}
		lineNumber, err := strconv.Atoi(m[2])
		if err != nil || lineNumber <= 0 {
			continue
		}
		message := sanitizeAnalyzerMessage(m[3])
		if message == "" {
			continue
		}
		path := NormalizeEvidencePath(m[1])
		key := fmt.Sprintf("%s:%d:%s:%s", path, lineNumber, plan.Tool, message)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		lineCopy := lineNumber
		items = append(items, StaticCheckEvidence{
			SourceType:   EvidenceSourceStaticCheck,
			Tool:         plan.Tool,
			ExitCategory: category,
			Package:      currentPackage,
			Path:         path,
			Line:         &lineCopy,
			Message:      message,
		})
	}
	if truncated {
		limitations = append(limitations, fmt.Sprintf("Go analyzer %s output was truncated before parsing completed.", plan.Tool))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path != items[j].Path {
			return items[i].Path < items[j].Path
		}
		if lineValue(items[i].Line) != lineValue(items[j].Line) {
			return lineValue(items[i].Line) < lineValue(items[j].Line)
		}
		return items[i].Message < items[j].Message
	})
	return items, limitations
}

func sanitizeAnalyzerMessage(message string) string {
	message = strings.Join(strings.Fields(message), " ")
	message = redactSecretishText(message)
	if len(message) > maxStaticCheckMessageBytes {
		message = strings.TrimSpace(message[:maxStaticCheckMessageBytes])
	}
	return message
}

func redactSecretishText(text string) string {
	tokens := strings.Fields(text)
	for i := range tokens {
		lower := strings.ToLower(strings.Trim(tokens[i], ":=,;\"'`"))
		if lower == "token" || lower == "secret" || lower == "password" || lower == "api_key" || lower == "apikey" || lower == "private_key" {
			if i+1 < len(tokens) {
				tokens[i+1] = "[redacted]"
			}
		}
		if looksSecretish(tokens[i]) {
			tokens[i] = "[redacted]"
		}
	}
	return strings.Join(tokens, " ")
}

func looksSecretish(token string) bool {
	trimmed := strings.Trim(token, ":=,;\"'`")
	if len(trimmed) < 24 {
		return false
	}
	hasLetter, hasDigit := false, false
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		case r == '_' || r == '-' || r == '.':
		default:
			return false
		}
	}
	return hasLetter && hasDigit
}

func lineValue(line *int) int {
	if line == nil {
		return 0
	}
	return *line
}

func normalizeGoAnalyzerOptions(options GoAnalyzerOptions) GoAnalyzerOptions {
	if options.Timeout <= 0 {
		options.Timeout = defaultGoAnalyzerTimeout
	}
	if options.OutputLimitBytes <= 0 {
		options.OutputLimitBytes = defaultGoAnalyzerOutputLimitBytes
	}
	return options
}

func (s GoAnalyzerStats) SortedExitCategories() []GoAnalyzerExitCategory {
	categories := make([]GoAnalyzerExitCategory, 0, len(s.ExitCategories))
	for category := range s.ExitCategories {
		categories = append(categories, category)
	}
	sort.Slice(categories, func(i, j int) bool { return categories[i] < categories[j] })
	return categories
}

func (s GoAnalyzerStats) String() string {
	parts := make([]string, 0, len(s.ExitCategories))
	for _, category := range s.SortedExitCategories() {
		parts = append(parts, string(category)+"="+strconv.Itoa(s.ExitCategories[category]))
	}
	return fmt.Sprintf(
		"planned=%d executed=%d parsed_evidence=%d output_truncated=%t exit_categories=%s",
		s.PlannedCommands,
		s.ExecutedCommands,
		s.ParsedEvidence,
		s.OutputTruncated,
		strings.Join(parts, ","),
	)
}
