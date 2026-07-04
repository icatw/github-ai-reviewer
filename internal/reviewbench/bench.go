package reviewbench

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github-ai-reviewer/internal/review"
)

type Fixture struct {
	Name                string                `json:"name"`
	Job                 review.Job            `json:"job"`
	Files               []review.FileChange   `json:"files"`
	RepoFiles           map[string]string     `json:"repo_files"`
	GoldenRelevantFiles []string              `json:"golden_relevant_files"`
	Budgets             review.ContextBudgets `json:"budgets,omitempty"`
	ExpectedFindings    []ExpectedFinding     `json:"expected_findings,omitempty"`
}

type ExpectedFinding struct {
	File     string `json:"file"`
	Line     int    `json:"line,omitempty"`
	Title    string `json:"title"`
	Evidence string `json:"evidence,omitempty"`
}

type Report struct {
	Name             string            `json:"name"`
	Metrics          Metrics           `json:"metrics"`
	SourceMetrics    Metrics           `json:"source_metrics"`
	Context          ContextReport     `json:"context"`
	Budget           BudgetReport      `json:"budget"`
	Golden           GoldenReport      `json:"golden"`
	ExpectedFindings []ExpectedFinding `json:"expected_findings,omitempty"`
}

type SuiteReport struct {
	FixtureCount  int      `json:"fixture_count"`
	Metrics       Metrics  `json:"metrics"`
	SourceMetrics Metrics  `json:"source_metrics"`
	Cases         []Report `json:"cases"`
}

type Metrics struct {
	RelevantTotal  int     `json:"relevant_total"`
	RetrievedTotal int     `json:"retrieved_total"`
	TruePositive   int     `json:"true_positive"`
	FalsePositive  int     `json:"false_positive"`
	FalseNegative  int     `json:"false_negative"`
	Precision      float64 `json:"precision"`
	Recall         float64 `json:"recall"`
	F1             float64 `json:"f1"`
}

type ContextReport struct {
	PatchFiles     []string         `json:"patch_files"`
	FullFiles      []string         `json:"full_files"`
	RelatedSources []string         `json:"related_sources"`
	RelatedTests   []string         `json:"related_tests"`
	RepoDocs       []string         `json:"repo_docs"`
	PolicyFiles    []string         `json:"policy_files"`
	StaticChecks   []string         `json:"static_checks,omitempty"`
	RetrievedFiles []string         `json:"retrieved_files"`
	SourceFiles    []string         `json:"source_files"`
	Omissions      []OmissionReport `json:"omissions"`
}

type OmissionReport struct {
	Path    string `json:"path"`
	Section string `json:"section"`
	Reason  string `json:"reason"`
}

type BudgetReport struct {
	MaxPatchBytes      int `json:"max_patch_bytes"`
	MaxFileBytes       int `json:"max_file_bytes"`
	TotalBytes         int `json:"total_bytes"`
	RetrievedBytes     int `json:"retrieved_bytes"`
	OmittedCount       int `json:"omitted_count"`
	BudgetOmittedCount int `json:"budget_omitted_count"`
}

type GoldenReport struct {
	RelevantFiles []string `json:"relevant_files"`
	MatchedFiles  []string `json:"matched_files"`
	MissedFiles   []string `json:"missed_files"`
	ExtraFiles    []string `json:"extra_files"`
}

func Run(ctx context.Context, fixture Fixture) (Report, error) {
	if fixture.Name == "" {
		fixture.Name = "unnamed"
	}
	if fixture.Job.Owner == "" {
		fixture.Job.Owner = "fixture"
	}
	if fixture.Job.Repo == "" {
		fixture.Job.Repo = fixture.Name
	}
	if fixture.Job.HeadSHA == "" {
		fixture.Job.HeadSHA = "fixture-head"
	}
	reader := newFixtureReader(fixture.RepoFiles)
	repoContext := review.BuildRepoContext(ctx, fixture.Job, fixture.Files, reader, fixture.Budgets)
	return buildReport(fixture, repoContext), nil
}

func RunSuite(ctx context.Context, fixtures []Fixture) (SuiteReport, error) {
	reports := make([]Report, 0, len(fixtures))
	for _, fixture := range fixtures {
		report, err := Run(ctx, fixture)
		if err != nil {
			return SuiteReport{}, err
		}
		reports = append(reports, report)
	}
	return buildSuiteReport(reports), nil
}

func DecodeFixture(data []byte) (Fixture, error) {
	var fixture Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return Fixture{}, err
	}
	if len(fixture.Files) == 0 {
		return Fixture{}, fmt.Errorf("fixture files must not be empty")
	}
	if fixture.RepoFiles == nil {
		return Fixture{}, fmt.Errorf("fixture repo_files must not be nil")
	}
	return fixture, nil
}

func buildReport(fixture Fixture, ctx review.RepoContext) Report {
	patchFiles := patchPaths(ctx.Patches)
	fullFiles := filePaths(ctx.FullFiles)
	relatedSources := filePaths(ctx.RelatedSources)
	relatedTests := filePaths(ctx.RelatedTests)
	repoDocs := filePaths(ctx.RepoDocs)
	policyFiles, scoredRepoDocs := splitPolicyFiles(repoDocs)
	staticChecks := staticCheckPaths(ctx.StaticChecks)
	sourceFiles := uniqueSorted(append(append([]string{}, fullFiles...), append(relatedSources, relatedTests...)...))
	retrieved := uniqueSorted(append(append([]string{}, sourceFiles...), scoredRepoDocs...))
	relevant := uniqueSorted(fixture.GoldenRelevantFiles)
	metrics, matched, missed, extra := calculateMetrics(relevant, retrieved)
	sourceMetrics, _, _, _ := calculateMetrics(sourceRelevantGoldenFiles(relevant), sourceFiles)
	omissions := omissionReports(ctx.Omitted)
	budgets := normalizedBudgetForReport(fixture.Budgets)
	return Report{
		Name:          fixture.Name,
		Metrics:       metrics,
		SourceMetrics: sourceMetrics,
		Context: ContextReport{
			PatchFiles:     patchFiles,
			FullFiles:      fullFiles,
			RelatedSources: relatedSources,
			RelatedTests:   relatedTests,
			RepoDocs:       repoDocs,
			PolicyFiles:    policyFiles,
			StaticChecks:   staticChecks,
			RetrievedFiles: retrieved,
			SourceFiles:    sourceFiles,
			Omissions:      omissions,
		},
		Budget: BudgetReport{
			MaxPatchBytes:      budgets.MaxPatchBytes,
			MaxFileBytes:       budgets.MaxFileBytes,
			TotalBytes:         budgets.TotalBytes,
			RetrievedBytes:     retrievedBytes(ctx),
			OmittedCount:       len(ctx.Omitted),
			BudgetOmittedCount: countBudgetOmissions(ctx.Omitted),
		},
		Golden: GoldenReport{
			RelevantFiles: relevant,
			MatchedFiles:  matched,
			MissedFiles:   missed,
			ExtraFiles:    extra,
		},
		ExpectedFindings: fixture.ExpectedFindings,
	}
}

func buildSuiteReport(reports []Report) SuiteReport {
	metrics := Metrics{}
	sourceMetrics := Metrics{}
	for _, report := range reports {
		metrics = addMetrics(metrics, report.Metrics)
		sourceMetrics = addMetrics(sourceMetrics, report.SourceMetrics)
	}
	metrics = finalizeMetrics(metrics)
	sourceMetrics = finalizeMetrics(sourceMetrics)
	return SuiteReport{
		FixtureCount:  len(reports),
		Metrics:       metrics,
		SourceMetrics: sourceMetrics,
		Cases:         reports,
	}
}

func addMetrics(total, item Metrics) Metrics {
	total.RelevantTotal += item.RelevantTotal
	total.RetrievedTotal += item.RetrievedTotal
	total.TruePositive += item.TruePositive
	total.FalsePositive += item.FalsePositive
	total.FalseNegative += item.FalseNegative
	return total
}

func finalizeMetrics(metrics Metrics) Metrics {
	metrics.Precision = ratio(metrics.TruePositive, metrics.TruePositive+metrics.FalsePositive)
	metrics.Recall = ratio(metrics.TruePositive, metrics.TruePositive+metrics.FalseNegative)
	if metrics.Precision+metrics.Recall > 0 {
		metrics.F1 = 2 * metrics.Precision * metrics.Recall / (metrics.Precision + metrics.Recall)
	}
	return metrics
}

type fixtureReader struct {
	files map[string]string
	dirs  map[string][]review.RepositoryEntry
}

func newFixtureReader(files map[string]string) *fixtureReader {
	dirs := map[string][]review.RepositoryEntry{}
	rootEntries := map[string]review.RepositoryEntry{}
	for filePath := range files {
		clean := path.Clean(strings.TrimPrefix(filePath, "/"))
		if clean == "." || clean == "" {
			continue
		}
		dir := path.Dir(clean)
		if dir == "." {
			dir = ""
		}
		dirs[dir] = append(dirs[dir], review.RepositoryEntry{Path: clean, Type: review.RepositoryEntryFile})
		root := strings.Split(clean, "/")[0]
		if root != "" {
			rootEntries[root] = review.RepositoryEntry{Path: root, Type: review.RepositoryEntryFile}
		}
	}
	if len(rootEntries) > 0 {
		dirs[""] = dirs[""][:0]
		for _, entry := range rootEntries {
			dirs[""] = append(dirs[""], entry)
		}
	}
	for dir := range dirs {
		sort.Slice(dirs[dir], func(i, j int) bool { return dirs[dir][i].Path < dirs[dir][j].Path })
	}
	return &fixtureReader{files: files, dirs: dirs}
}

func (r *fixtureReader) FetchFileContent(_ context.Context, _ int64, _, _, _, filePath string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(filePath, "/"))
	content, ok := r.files[clean]
	if !ok {
		return "", review.ErrRepositoryContentNotFound
	}
	return content, nil
}

func (r *fixtureReader) ListDirectory(_ context.Context, _ int64, _, _, _, dir string) ([]review.RepositoryEntry, error) {
	clean := path.Clean(strings.TrimPrefix(dir, "/"))
	if clean == "." {
		clean = ""
	}
	return append([]review.RepositoryEntry(nil), r.dirs[clean]...), nil
}

func patchPaths(items []review.PatchContext) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Path)
	}
	return uniqueSorted(out)
}

func filePaths(items []review.FileContext) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Path)
	}
	return uniqueSorted(out)
}

func staticCheckPaths(items []review.StaticCheckEvidence) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Path != "" {
			out = append(out, item.Path)
		}
	}
	return uniqueSorted(out)
}

func omissionReports(items []review.OmittedContext) []OmissionReport {
	out := make([]OmissionReport, 0, len(items))
	for _, item := range items {
		out = append(out, OmissionReport{Path: item.Path, Section: string(item.Section), Reason: string(item.Reason)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Section != out[j].Section {
			return out[i].Section < out[j].Section
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

func splitPolicyFiles(files []string) ([]string, []string) {
	policy := []string{}
	scored := []string{}
	for _, filePath := range files {
		if isPolicyFile(filePath) {
			policy = append(policy, filePath)
			continue
		}
		scored = append(scored, filePath)
	}
	return uniqueSorted(policy), uniqueSorted(scored)
}

func isPolicyFile(filePath string) bool {
	return filePath == ".github/ai-review.yml" || filePath == ".github/ai-review.yaml"
}

func calculateMetrics(relevant, retrieved []string) (Metrics, []string, []string, []string) {
	matched, missed, extra := compareSets(relevant, retrieved)
	metrics := Metrics{
		RelevantTotal:  len(relevant),
		RetrievedTotal: len(retrieved),
		TruePositive:   len(matched),
		FalsePositive:  len(extra),
		FalseNegative:  len(missed),
	}
	metrics.Precision = ratio(metrics.TruePositive, metrics.TruePositive+metrics.FalsePositive)
	metrics.Recall = ratio(metrics.TruePositive, metrics.TruePositive+metrics.FalseNegative)
	if metrics.Precision+metrics.Recall > 0 {
		metrics.F1 = 2 * metrics.Precision * metrics.Recall / (metrics.Precision + metrics.Recall)
	}
	return metrics, matched, missed, extra
}

func sourceRelevantGoldenFiles(relevant []string) []string {
	out := []string{}
	for _, filePath := range relevant {
		if isPolicyFile(filePath) || strings.HasSuffix(strings.ToLower(filePath), ".md") {
			continue
		}
		out = append(out, filePath)
	}
	return uniqueSorted(out)
}

func compareSets(relevant, retrieved []string) ([]string, []string, []string) {
	relevantSet := stringSet(relevant)
	retrievedSet := stringSet(retrieved)
	matched := []string{}
	missed := []string{}
	extra := []string{}
	for _, item := range relevant {
		if retrievedSet[item] {
			matched = append(matched, item)
		} else {
			missed = append(missed, item)
		}
	}
	for _, item := range retrieved {
		if !relevantSet[item] {
			extra = append(extra, item)
		}
	}
	return matched, missed, extra
}

func stringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}

func uniqueSorted(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		value = path.Clean(strings.TrimPrefix(strings.TrimSpace(value), "/"))
		if value == "." || value == "" {
			continue
		}
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func ratio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func retrievedBytes(ctx review.RepoContext) int {
	total := 0
	for _, item := range ctx.FullFiles {
		total += len(item.Content)
	}
	for _, item := range ctx.RelatedSources {
		total += len(item.Content)
	}
	for _, item := range ctx.RelatedTests {
		total += len(item.Content)
	}
	for _, item := range ctx.RepoDocs {
		total += len(item.Content)
	}
	return total
}

func countBudgetOmissions(items []review.OmittedContext) int {
	count := 0
	for _, item := range items {
		if item.Reason == review.OmitBudgetExhausted || item.Reason == review.OmitTruncated || item.Reason == review.OmitOversized {
			count++
		}
	}
	return count
}

func normalizedBudgetForReport(b review.ContextBudgets) review.ContextBudgets {
	if b.MaxPatchBytes <= 0 {
		b.MaxPatchBytes = review.DefaultContextBudgets.MaxPatchBytes
	}
	if b.MaxFileBytes <= 0 {
		b.MaxFileBytes = review.DefaultContextBudgets.MaxFileBytes
	}
	if b.TotalBytes <= 0 {
		b.TotalBytes = review.DefaultContextBudgets.TotalBytes
	}
	if b.MaxRelatedSources <= 0 {
		b.MaxRelatedSources = review.DefaultContextBudgets.MaxRelatedSources
	}
	if b.MaxSamePackageTests <= 0 {
		b.MaxSamePackageTests = review.DefaultContextBudgets.MaxSamePackageTests
	}
	if b.MaxDocsFiles <= 0 {
		b.MaxDocsFiles = review.DefaultContextBudgets.MaxDocsFiles
	}
	return b
}
