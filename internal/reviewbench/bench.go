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
	Metadata            FixtureMetadata       `json:"metadata,omitempty"`
	Job                 review.Job            `json:"job"`
	Files               []review.FileChange   `json:"files"`
	RepoFiles           map[string]string     `json:"repo_files"`
	GoldenRelevantFiles []string              `json:"golden_relevant_files"`
	Budgets             review.ContextBudgets `json:"budgets,omitempty"`
	ExpectedFindings    []ExpectedFinding     `json:"expected_findings,omitempty"`
	ExpectedNoFindings  bool                  `json:"expected_no_findings,omitempty"`
	ActualFindings      []ActualFinding       `json:"actual_findings,omitempty"`
	QualityAnnotations  []QualityAnnotation   `json:"quality_annotations,omitempty"`
}

type FixtureMetadata struct {
	Source     string `json:"source,omitempty"`
	Provenance string `json:"provenance,omitempty"`
	Sanitized  bool   `json:"sanitized,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type ExpectedFinding struct {
	ID            string   `json:"id,omitempty"`
	File          string   `json:"file"`
	Line          int      `json:"line,omitempty"`
	LineEnd       int      `json:"line_end,omitempty"`
	Title         string   `json:"title"`
	Category      string   `json:"category,omitempty"`
	Severity      string   `json:"severity,omitempty"`
	Evidence      string   `json:"evidence,omitempty"`
	EvidenceHints []string `json:"evidence_hints,omitempty"`
	MatchingHints []string `json:"matching_hints,omitempty"`
	Notes         string   `json:"notes,omitempty"`
}

type ActualFinding struct {
	ID            string   `json:"id,omitempty"`
	File          string   `json:"file,omitempty"`
	Line          int      `json:"line,omitempty"`
	LineEnd       int      `json:"line_end,omitempty"`
	Title         string   `json:"title,omitempty"`
	Category      string   `json:"category,omitempty"`
	Severity      string   `json:"severity,omitempty"`
	EvidenceHints []string `json:"evidence_hints,omitempty"`
	MatchingHints []string `json:"matching_hints,omitempty"`
	QualityLabels []string `json:"quality_labels,omitempty"`
	DuplicateOf   string   `json:"duplicate_of,omitempty"`
}

type QualityAnnotation struct {
	ActualID    string   `json:"actual_id,omitempty"`
	Label       string   `json:"label,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	DuplicateOf string   `json:"duplicate_of,omitempty"`
}

type Report struct {
	Name             string           `json:"name"`
	Metrics          Metrics          `json:"metrics"`
	SourceMetrics    Metrics          `json:"source_metrics"`
	Context          ContextReport    `json:"context"`
	Budget           BudgetReport     `json:"budget"`
	Golden           GoldenReport     `json:"golden"`
	ExpectedFindings []FindingSummary `json:"expected_findings,omitempty"`
	FindingQuality   FindingQuality   `json:"finding_quality"`
}

type SuiteReport struct {
	FixtureCount   int                 `json:"fixture_count"`
	Metrics        Metrics             `json:"metrics"`
	SourceMetrics  Metrics             `json:"source_metrics"`
	FindingQuality SuiteFindingQuality `json:"finding_quality"`
	Cases          []Report            `json:"cases"`
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

type FindingQualityStatus string

const (
	FindingQualityNotAnnotated FindingQualityStatus = "not_annotated"
	FindingQualityAnnotated    FindingQualityStatus = "annotated"
)

type FindingQuality struct {
	Status             FindingQualityStatus `json:"status"`
	ExpectedNoFindings bool                 `json:"expected_no_findings,omitempty"`
	ExpectedCount      int                  `json:"expected_count"`
	ActualCount        int                  `json:"actual_count,omitempty"`
	CoveredCount       int                  `json:"covered_count"`
	MissedCount        int                  `json:"missed_count"`
	UnexpectedCount    int                  `json:"unexpected_count"`
	DuplicateCount     int                  `json:"duplicate_count"`
	LowValueCount      int                  `json:"low_value_count"`
	Covered            []CoveredFinding     `json:"covered,omitempty"`
	Missed             []FindingSummary     `json:"missed,omitempty"`
	Unexpected         []FindingSummary     `json:"unexpected,omitempty"`
	Duplicates         []FindingSummary     `json:"duplicates,omitempty"`
	LowValue           []FindingSummary     `json:"low_value,omitempty"`
}

type SuiteFindingQuality struct {
	AnnotatedFixtureCount    int `json:"annotated_fixture_count"`
	NotAnnotatedFixtureCount int `json:"not_annotated_fixture_count"`
	ExpectedCount            int `json:"expected_count"`
	ActualCount              int `json:"actual_count"`
	CoveredCount             int `json:"covered_count"`
	MissedCount              int `json:"missed_count"`
	UnexpectedCount          int `json:"unexpected_count"`
	DuplicateCount           int `json:"duplicate_count"`
	LowValueCount            int `json:"low_value_count"`
}

type CoveredFinding struct {
	ExpectedID string         `json:"expected_id,omitempty"`
	Actual     FindingSummary `json:"actual"`
}

type FindingSummary struct {
	ID            string   `json:"id,omitempty"`
	ExpectedID    string   `json:"expected_id,omitempty"`
	File          string   `json:"file,omitempty"`
	Line          int      `json:"line,omitempty"`
	LineEnd       int      `json:"line_end,omitempty"`
	Title         string   `json:"title,omitempty"`
	Category      string   `json:"category,omitempty"`
	Severity      string   `json:"severity,omitempty"`
	QualityLabels []string `json:"quality_labels,omitempty"`
	DuplicateOf   string   `json:"duplicate_of,omitempty"`
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
	if fixture.ExpectedNoFindings && len(fixture.ExpectedFindings) > 0 {
		return Fixture{}, fmt.Errorf("fixture expected_no_findings cannot be combined with expected_findings")
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
		ExpectedFindings: summarizeExpectedFindings(fixture.ExpectedFindings),
		FindingQuality:   evaluateFindingQuality(fixture),
	}
}

func buildSuiteReport(reports []Report) SuiteReport {
	metrics := Metrics{}
	sourceMetrics := Metrics{}
	findingQuality := SuiteFindingQuality{}
	for _, report := range reports {
		metrics = addMetrics(metrics, report.Metrics)
		sourceMetrics = addMetrics(sourceMetrics, report.SourceMetrics)
		findingQuality = addFindingQuality(findingQuality, report.FindingQuality)
	}
	metrics = finalizeMetrics(metrics)
	sourceMetrics = finalizeMetrics(sourceMetrics)
	return SuiteReport{
		FixtureCount:   len(reports),
		Metrics:        metrics,
		SourceMetrics:  sourceMetrics,
		FindingQuality: findingQuality,
		Cases:          reports,
	}
}

func addFindingQuality(total SuiteFindingQuality, item FindingQuality) SuiteFindingQuality {
	if item.Status == FindingQualityAnnotated {
		total.AnnotatedFixtureCount++
	} else {
		total.NotAnnotatedFixtureCount++
		return total
	}
	total.ExpectedCount += item.ExpectedCount
	total.ActualCount += item.ActualCount
	total.CoveredCount += item.CoveredCount
	total.MissedCount += item.MissedCount
	total.UnexpectedCount += item.UnexpectedCount
	total.DuplicateCount += item.DuplicateCount
	total.LowValueCount += item.LowValueCount
	return total
}

func addMetrics(total, item Metrics) Metrics {
	total.RelevantTotal += item.RelevantTotal
	total.RetrievedTotal += item.RetrievedTotal
	total.TruePositive += item.TruePositive
	total.FalsePositive += item.FalsePositive
	total.FalseNegative += item.FalseNegative
	return total
}

func evaluateFindingQuality(fixture Fixture) FindingQuality {
	quality := FindingQuality{
		Status:             FindingQualityNotAnnotated,
		ExpectedNoFindings: fixture.ExpectedNoFindings,
		ExpectedCount:      len(fixture.ExpectedFindings),
		ActualCount:        len(fixture.ActualFindings),
	}
	if len(fixture.ActualFindings) == 0 && !fixture.ExpectedNoFindings {
		return quality
	}
	quality.Status = FindingQualityAnnotated
	if fixture.ExpectedNoFindings {
		for _, actual := range fixture.ActualFindings {
			actual = applyQualityAnnotation(actual, fixture.QualityAnnotations)
			summary := summarizeActualFinding(actual)
			switch {
			case isDuplicate(actual):
				quality.Duplicates = append(quality.Duplicates, summary)
			case isLowValue(actual):
				quality.LowValue = append(quality.LowValue, summary)
			default:
				quality.Unexpected = append(quality.Unexpected, summary)
			}
		}
		finalizeFindingQuality(&quality)
		return quality
	}

	matchedExpected := map[int]bool{}
	for _, actual := range fixture.ActualFindings {
		actual = applyQualityAnnotation(actual, fixture.QualityAnnotations)
		summary := summarizeActualFinding(actual)
		switch {
		case isDuplicate(actual):
			quality.Duplicates = append(quality.Duplicates, summary)
			continue
		case isLowValue(actual):
			quality.LowValue = append(quality.LowValue, summary)
			continue
		}
		match := -1
		for i, expected := range fixture.ExpectedFindings {
			if matchedExpected[i] {
				continue
			}
			if findingMatches(expected, actual) {
				match = i
				break
			}
		}
		if match >= 0 {
			matchedExpected[match] = true
			quality.Covered = append(quality.Covered, CoveredFinding{
				ExpectedID: expectedFindingID(fixture.ExpectedFindings[match]),
				Actual:     summary,
			})
			continue
		}
		quality.Unexpected = append(quality.Unexpected, summary)
	}
	for i, expected := range fixture.ExpectedFindings {
		if !matchedExpected[i] {
			quality.Missed = append(quality.Missed, summarizeExpectedFinding(expected))
		}
	}
	finalizeFindingQuality(&quality)
	return quality
}

func finalizeFindingQuality(quality *FindingQuality) {
	sort.Slice(quality.Covered, func(i, j int) bool {
		if quality.Covered[i].ExpectedID != quality.Covered[j].ExpectedID {
			return quality.Covered[i].ExpectedID < quality.Covered[j].ExpectedID
		}
		return lessFindingSummary(quality.Covered[i].Actual, quality.Covered[j].Actual)
	})
	sortFindingSummaries(quality.Missed)
	sortFindingSummaries(quality.Unexpected)
	sortFindingSummaries(quality.Duplicates)
	sortFindingSummaries(quality.LowValue)
	quality.CoveredCount = len(quality.Covered)
	quality.MissedCount = len(quality.Missed)
	quality.UnexpectedCount = len(quality.Unexpected)
	quality.DuplicateCount = len(quality.Duplicates)
	quality.LowValueCount = len(quality.LowValue)
}

func applyQualityAnnotation(actual ActualFinding, annotations []QualityAnnotation) ActualFinding {
	for _, annotation := range annotations {
		if strings.TrimSpace(annotation.ActualID) == "" || annotation.ActualID != actual.ID {
			continue
		}
		if annotation.Label != "" {
			actual.QualityLabels = append(actual.QualityLabels, annotation.Label)
		}
		actual.QualityLabels = append(actual.QualityLabels, annotation.Labels...)
		if annotation.DuplicateOf != "" {
			actual.DuplicateOf = annotation.DuplicateOf
		}
	}
	actual.QualityLabels = uniqueStrings(actual.QualityLabels)
	return actual
}

func findingMatches(expected ExpectedFinding, actual ActualFinding) bool {
	if expected.ID != "" && actual.DuplicateOf == expected.ID {
		return true
	}
	if expected.File != "" && actual.File != "" && cleanReportPath(expected.File) != cleanReportPath(actual.File) {
		return false
	}
	if expected.Line > 0 && actual.Line > 0 && !lineMatches(expected.Line, expected.LineEnd, actual.Line, actual.LineEnd) {
		return false
	}
	if expected.Category != "" && actual.Category != "" && !strings.EqualFold(expected.Category, actual.Category) {
		return false
	}
	if expected.Severity != "" && actual.Severity != "" && !strings.EqualFold(expected.Severity, actual.Severity) {
		return false
	}
	if hintsMatch(expected.EvidenceHints, actual.EvidenceHints) || hintsMatch(expected.MatchingHints, actual.MatchingHints) {
		return true
	}
	return titleTokensMatch(expected.Title, actual.Title)
}

func lineMatches(expectedStart, expectedEnd, actualStart, actualEnd int) bool {
	if expectedEnd <= 0 {
		expectedEnd = expectedStart
	}
	if actualEnd <= 0 {
		actualEnd = actualStart
	}
	return expectedStart <= actualEnd && actualStart <= expectedEnd
}

func hintsMatch(expected, actual []string) bool {
	expectedSet := normalizedSet(expected)
	actualSet := normalizedSet(actual)
	for hint := range expectedSet {
		if actualSet[hint] {
			return true
		}
	}
	return false
}

func titleTokensMatch(expected, actual string) bool {
	expectedTokens := titleTokenSet(expected)
	actualTokens := titleTokenSet(actual)
	if len(expectedTokens) == 0 || len(actualTokens) == 0 {
		return false
	}
	matches := 0
	for token := range expectedTokens {
		if actualTokens[token] {
			matches++
		}
	}
	return matches >= 2 || matches == len(expectedTokens)
}

func titleTokenSet(value string) map[string]bool {
	words := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	out := map[string]bool{}
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		out[word] = true
	}
	return out
}

func normalizedSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			out[value] = true
		}
	}
	return out
}

func isDuplicate(actual ActualFinding) bool {
	if strings.TrimSpace(actual.DuplicateOf) != "" {
		return true
	}
	for _, label := range actual.QualityLabels {
		if normalizeLabel(label) == "duplicate" || strings.HasPrefix(normalizeLabel(label), "duplicate-") {
			return true
		}
	}
	return false
}

func isLowValue(actual ActualFinding) bool {
	for _, label := range actual.QualityLabels {
		switch normalizeLabel(label) {
		case "low-value", "style-only", "too-generic", "unsupported":
			return true
		}
	}
	return false
}

func normalizeLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

func summarizeExpectedFinding(finding ExpectedFinding) FindingSummary {
	return FindingSummary{
		ID:         finding.ID,
		ExpectedID: expectedFindingID(finding),
		File:       cleanReportPath(finding.File),
		Line:       finding.Line,
		LineEnd:    finding.LineEnd,
		Title:      finding.Title,
		Category:   finding.Category,
		Severity:   finding.Severity,
	}
}

func summarizeExpectedFindings(findings []ExpectedFinding) []FindingSummary {
	out := make([]FindingSummary, 0, len(findings))
	for _, finding := range findings {
		out = append(out, summarizeExpectedFinding(finding))
	}
	sortFindingSummaries(out)
	return out
}

func summarizeActualFinding(finding ActualFinding) FindingSummary {
	return FindingSummary{
		ID:            finding.ID,
		File:          cleanReportPath(finding.File),
		Line:          finding.Line,
		LineEnd:       finding.LineEnd,
		Title:         finding.Title,
		Category:      finding.Category,
		Severity:      finding.Severity,
		QualityLabels: uniqueStrings(finding.QualityLabels),
		DuplicateOf:   finding.DuplicateOf,
	}
}

func expectedFindingID(finding ExpectedFinding) string {
	if strings.TrimSpace(finding.ID) != "" {
		return finding.ID
	}
	if finding.File != "" && finding.Line > 0 {
		return fmt.Sprintf("%s:%d", cleanReportPath(finding.File), finding.Line)
	}
	return strings.TrimSpace(finding.Title)
}

func sortFindingSummaries(items []FindingSummary) {
	sort.Slice(items, func(i, j int) bool {
		return lessFindingSummary(items[i], items[j])
	})
}

func lessFindingSummary(a, b FindingSummary) bool {
	if a.File != b.File {
		return a.File < b.File
	}
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	if a.ExpectedID != b.ExpectedID {
		return a.ExpectedID < b.ExpectedID
	}
	if a.ID != b.ID {
		return a.ID < b.ID
	}
	return a.Title < b.Title
}

func cleanReportPath(filePath string) string {
	clean := path.Clean(strings.TrimPrefix(strings.TrimSpace(filePath), "/"))
	if clean == "." {
		return ""
	}
	return clean
}

func uniqueStrings(values []string) []string {
	set := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
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
