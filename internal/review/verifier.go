package review

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type VerificationOutcome string
type VerificationReason string
type EvidenceSourceType string

const (
	VerificationOutcomeKept       VerificationOutcome = "kept"
	VerificationOutcomeDowngraded VerificationOutcome = "downgraded"
	VerificationOutcomeDropped    VerificationOutcome = "dropped"

	VerificationReasonSupported                VerificationReason = "supported"
	VerificationReasonUnsupportedEvidence      VerificationReason = "unsupported_evidence"
	VerificationReasonUnavailableFile          VerificationReason = "unavailable_file"
	VerificationReasonLineUnavailable          VerificationReason = "line_unavailable"
	VerificationReasonLineMismatch             VerificationReason = "line_mismatch"
	VerificationReasonOmittedContextDependency VerificationReason = "omitted_context_dependency"
	VerificationReasonNoFindings               VerificationReason = "no_findings"

	EvidenceSourcePatch       EvidenceSourceType = "patch_context"
	EvidenceSourceFullFile    EvidenceSourceType = "full_file_context"
	EvidenceSourceRelatedTest EvidenceSourceType = "related_test_context"
	EvidenceSourceRepoDocs    EvidenceSourceType = "repo_docs_context"
	EvidenceSourceOmitted     EvidenceSourceType = "omitted_context"
	EvidenceSourceStaticCheck EvidenceSourceType = "static_check_context"
)

type VerificationStats struct {
	TotalFindings int
	Kept          int
	Downgraded    int
	Dropped       int
	Reasons       map[VerificationReason]int
}

type EvidenceSource struct {
	Type    EvidenceSourceType
	Path    string
	Content string
	Lines   lineSet
	Omitted bool
}

type EvidenceIndex struct {
	files   map[string]*indexedFile
	omitted map[string][]OmittedContext
}

type indexedFile struct {
	Path    string
	Sources []EvidenceSource
}

type lineSet map[int]struct{}

func VerifyReviewResult(raw ReviewResult, repoContext RepoContext) (ReviewResult, VerificationStats) {
	verified := cloneReviewResult(raw)
	stats := VerificationStats{
		TotalFindings: len(raw.Findings),
		Reasons:       map[VerificationReason]int{},
	}
	if len(raw.Findings) == 0 {
		stats.Reasons[VerificationReasonNoFindings] = 1
		return verified, stats
	}
	index := BuildEvidenceIndex(repoContext)
	verified.Findings = verified.Findings[:0]
	for _, finding := range raw.Findings {
		outcome, reason, outFinding := verifyFinding(finding, index)
		stats.Reasons[reason]++
		switch outcome {
		case VerificationOutcomeKept:
			stats.Kept++
			verified.Findings = append(verified.Findings, outFinding)
		case VerificationOutcomeDowngraded:
			stats.Downgraded++
			verified.Findings = append(verified.Findings, outFinding)
		case VerificationOutcomeDropped:
			stats.Dropped++
		}
	}
	return verified, stats
}

func BuildEvidenceIndex(ctx RepoContext) EvidenceIndex {
	index := EvidenceIndex{
		files:   map[string]*indexedFile{},
		omitted: map[string][]OmittedContext{},
	}
	for _, patch := range ctx.Patches {
		index.addSource(EvidenceSource{
			Type:    EvidenceSourcePatch,
			Path:    patch.Path,
			Content: patch.Patch,
			Lines:   patchLineSet(patch.Patch),
		})
	}
	for _, file := range ctx.FullFiles {
		index.addSource(EvidenceSource{
			Type:    EvidenceSourceFullFile,
			Path:    file.Path,
			Content: file.Content,
			Lines:   fullFileLineSet(file.Content),
		})
	}
	for _, file := range ctx.RelatedTests {
		index.addSource(EvidenceSource{
			Type:    EvidenceSourceRelatedTest,
			Path:    file.Path,
			Content: file.Content,
			Lines:   fullFileLineSet(file.Content),
		})
	}
	for _, file := range ctx.RepoDocs {
		index.addSource(EvidenceSource{
			Type:    EvidenceSourceRepoDocs,
			Path:    file.Path,
			Content: file.Content,
			Lines:   fullFileLineSet(file.Content),
		})
	}
	for _, omitted := range ctx.Omitted {
		normalized := NormalizeEvidencePath(omitted.Path)
		index.omitted[normalized] = append(index.omitted[normalized], omitted)
	}
	return index
}

func (e *EvidenceIndex) addSource(source EvidenceSource) {
	normalized := NormalizeEvidencePath(source.Path)
	if normalized == "" {
		return
	}
	source.Path = normalized
	file := e.files[normalized]
	if file == nil {
		file = &indexedFile{Path: normalized}
		e.files[normalized] = file
	}
	file.Sources = append(file.Sources, source)
}

func NormalizeEvidencePath(filePath string) string {
	filePath = strings.TrimSpace(strings.ReplaceAll(filePath, "\\", "/"))
	if filePath == "" {
		return ""
	}
	for strings.HasPrefix(filePath, "./") {
		filePath = strings.TrimPrefix(filePath, "./")
	}
	clean := path.Clean(filePath)
	if clean == "." {
		return ""
	}
	return strings.TrimPrefix(clean, "/")
}

func verifyFinding(finding Finding, index EvidenceIndex) (VerificationOutcome, VerificationReason, Finding) {
	normalizedFile := NormalizeEvidencePath(finding.File)
	if normalizedFile == "" {
		if evidenceSupportedAnyPath(finding, index) {
			return VerificationOutcomeKept, VerificationReasonSupported, finding
		}
		return VerificationOutcomeDropped, VerificationReasonUnsupportedEvidence, Finding{}
	}
	if hasOmittedDependency(finding, normalizedFile, index) {
		if isLimitationFinding(finding) {
			return VerificationOutcomeDowngraded, VerificationReasonOmittedContextDependency, downgradeFinding(finding, VerificationReasonOmittedContextDependency)
		}
		return VerificationOutcomeDropped, VerificationReasonOmittedContextDependency, Finding{}
	}
	file := index.files[normalizedFile]
	if file == nil {
		return VerificationOutcomeDropped, VerificationReasonUnavailableFile, Finding{}
	}
	lineOK, lineReason := lineSupported(finding, file)
	if !lineOK {
		if evidenceSupportedForFile(finding, file) {
			return VerificationOutcomeDowngraded, lineReason, downgradeFinding(finding, lineReason)
		}
		return VerificationOutcomeDropped, lineReason, Finding{}
	}
	if evidenceSupportedForFile(finding, file) {
		return VerificationOutcomeKept, VerificationReasonSupported, finding
	}
	if isLimitationFinding(finding) {
		return VerificationOutcomeDowngraded, VerificationReasonUnsupportedEvidence, downgradeFinding(finding, VerificationReasonUnsupportedEvidence)
	}
	return VerificationOutcomeDropped, VerificationReasonUnsupportedEvidence, Finding{}
}

func lineSupported(finding Finding, file *indexedFile) (bool, VerificationReason) {
	if finding.Line == nil {
		return true, VerificationReasonSupported
	}
	line := *finding.Line
	if line <= 0 {
		return false, VerificationReasonLineUnavailable
	}
	hasLineMetadata := false
	for _, source := range file.Sources {
		if len(source.Lines) == 0 {
			continue
		}
		hasLineMetadata = true
		if _, ok := source.Lines[line]; ok {
			return true, VerificationReasonSupported
		}
	}
	if !hasLineMetadata {
		return false, VerificationReasonLineUnavailable
	}
	return false, VerificationReasonLineMismatch
}

func evidenceSupportedAnyPath(finding Finding, index EvidenceIndex) bool {
	for _, file := range index.files {
		if evidenceSupportedForFile(finding, file) {
			return true
		}
	}
	return false
}

func evidenceSupportedForFile(finding Finding, file *indexedFile) bool {
	needle := normalizeEvidenceText(finding.Evidence)
	if needle == "" {
		return false
	}
	for _, source := range file.Sources {
		if normalizeEvidenceContains(source.Content, needle) {
			return true
		}
	}
	return false
}

func normalizeEvidenceContains(content, normalizedNeedle string) bool {
	haystack := normalizeEvidenceText(content)
	return haystack != "" && strings.Contains(haystack, normalizedNeedle)
}

func normalizeEvidenceText(text string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(text)))
	return strings.Join(fields, " ")
}

func hasOmittedDependency(finding Finding, normalizedFile string, index EvidenceIndex) bool {
	if len(index.omitted[normalizedFile]) > 0 {
		return true
	}
	text := normalizeEvidenceText(strings.Join([]string{finding.Title, finding.Evidence, finding.FailureScenario, finding.Suggestion}, " "))
	for omittedPath := range index.omitted {
		if omittedPath != "" && strings.Contains(text, strings.ToLower(omittedPath)) {
			return true
		}
	}
	for _, phrase := range []string{"omitted", "skipped", "truncated", "not fetched", "unavailable", "could not be verified", "insufficient context"} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func isLimitationFinding(finding Finding) bool {
	text := normalizeEvidenceText(strings.Join([]string{finding.Title, finding.Evidence, finding.FailureScenario, finding.Suggestion}, " "))
	for _, phrase := range []string{"omitted", "skipped", "truncated", "not fetched", "unavailable", "could not be verified", "insufficient context", "limitation", "cannot verify", "could not verify"} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return strings.EqualFold(finding.Severity, "question")
}

func downgradeFinding(finding Finding, reason VerificationReason) Finding {
	out := finding
	out.Severity = "question"
	out.Line = nil
	limitation := fmt.Sprintf("Verification limitation: %s.", reason)
	if out.Evidence == "" {
		out.Evidence = limitation
	} else if !strings.Contains(out.Evidence, limitation) {
		out.Evidence = strings.TrimSpace(out.Evidence) + " " + limitation
	}
	return out
}

func patchLineSet(patchText string) lineSet {
	lines := lineSet{}
	newLine := 0
	for _, line := range strings.Split(patchText, "\n") {
		if strings.HasPrefix(line, "@@") {
			parsed, ok := parseNewHunkStart(line)
			if ok {
				newLine = parsed
			}
			continue
		}
		if newLine <= 0 {
			continue
		}
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			lines[newLine] = struct{}{}
			newLine++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
		default:
			lines[newLine] = struct{}{}
			newLine++
		}
	}
	return lines
}

func parseNewHunkStart(header string) (int, bool) {
	start := strings.Index(header, "+")
	if start < 0 {
		return 0, false
	}
	value := header[start+1:]
	end := strings.IndexAny(value, " ,")
	if end >= 0 {
		value = value[:end]
	}
	var line int
	if _, err := fmt.Sscanf(value, "%d", &line); err != nil || line <= 0 {
		return 0, false
	}
	return line, true
}

func fullFileLineSet(content string) lineSet {
	lines := lineSet{}
	if content == "" {
		return lines
	}
	count := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") {
		count++
	}
	for i := 1; i <= count; i++ {
		lines[i] = struct{}{}
	}
	return lines
}

func cloneReviewResult(in ReviewResult) ReviewResult {
	out := in
	out.Findings = append([]Finding(nil), in.Findings...)
	out.MissingTests = append([]string(nil), in.MissingTests...)
	out.Limitations = append([]string(nil), in.Limitations...)
	return out
}

func (s VerificationStats) SortedReasons() []VerificationReason {
	reasons := make([]VerificationReason, 0, len(s.Reasons))
	for reason := range s.Reasons {
		reasons = append(reasons, reason)
	}
	sort.Slice(reasons, func(i, j int) bool { return reasons[i] < reasons[j] })
	return reasons
}
