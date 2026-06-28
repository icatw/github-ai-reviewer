package review

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
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
	TotalFindings            int
	Kept                     int
	Downgraded               int
	Dropped                  int
	NoFindingCount           int
	KeptRate                 float64
	DowngradedRate           float64
	DroppedRate              float64
	StaticCheckEvidenceCount int
	StaticCheckSupported     int
	StaticCheckSkipped       map[GoAnalyzerExitCategory]int
	Reasons                  map[VerificationReason]int
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
	stats.collectStaticCheckStats(repoContext)
	if len(raw.Findings) == 0 {
		stats.NoFindingCount = 1
		stats.Reasons[VerificationReasonNoFindings] = 1
		return verified, stats
	}
	index := BuildEvidenceIndex(repoContext)
	verified.Findings = verified.Findings[:0]
	for _, finding := range raw.Findings {
		outcome, reason, outFinding := verifyFinding(finding, index)
		stats.Reasons[reason]++
		if outcome == VerificationOutcomeKept && reason == VerificationReasonSupported && findingSupportedByStaticCheck(finding, index) {
			stats.StaticCheckSupported++
		}
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
	stats.computeRates()
	return verified, stats
}

func (s *VerificationStats) computeRates() {
	if s.TotalFindings <= 0 {
		return
	}
	total := float64(s.TotalFindings)
	s.KeptRate = float64(s.Kept) / total
	s.DowngradedRate = float64(s.Downgraded) / total
	s.DroppedRate = float64(s.Dropped) / total
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
	for _, item := range ctx.StaticChecks {
		if item.Path == "" || item.Message == "" {
			continue
		}
		index.addSource(EvidenceSource{
			Type:    EvidenceSourceStaticCheck,
			Path:    item.Path,
			Content: staticCheckEvidenceContent(item),
			Lines:   staticCheckLineSet(item),
		})
	}
	for _, omitted := range ctx.Omitted {
		normalized := NormalizeEvidencePath(omitted.Path)
		index.omitted[normalized] = append(index.omitted[normalized], omitted)
	}
	return index
}

func (s *VerificationStats) collectStaticCheckStats(ctx RepoContext) {
	if len(ctx.StaticChecks) == 0 {
		return
	}
	s.StaticCheckSkipped = map[GoAnalyzerExitCategory]int{}
	for _, item := range ctx.StaticChecks {
		s.StaticCheckEvidenceCount++
		if item.ExitCategory != "" {
			s.StaticCheckSkipped[item.ExitCategory]++
		}
	}
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
	matcher := newEvidenceMatcher(finding, file.Path)
	if matcher.normalizedEvidence == "" {
		return false
	}
	for _, source := range file.Sources {
		if !sourceCompatible(finding, file.Path, source) {
			continue
		}
		if matcher.matches(source) {
			return true
		}
	}
	return false
}

func findingSupportedByStaticCheck(finding Finding, index EvidenceIndex) bool {
	normalizedFile := NormalizeEvidencePath(finding.File)
	if normalizedFile == "" {
		for _, file := range index.files {
			if findingSupportedByStaticCheckInFile(finding, file) {
				return true
			}
		}
		return false
	}
	file := index.files[normalizedFile]
	return findingSupportedByStaticCheckInFile(finding, file)
}

func findingSupportedByStaticCheckInFile(finding Finding, file *indexedFile) bool {
	if file == nil {
		return false
	}
	matcher := newEvidenceMatcher(finding, file.Path)
	if matcher.normalizedEvidence == "" {
		return false
	}
	for _, source := range file.Sources {
		if source.Type == EvidenceSourceStaticCheck && matcher.matches(source) {
			return true
		}
	}
	return false
}

type evidenceMatcher struct {
	finding            Finding
	filePath           string
	normalizedEvidence string
	claimText          string
	evidenceTokens     map[string]struct{}
	claimTokens        map[string]struct{}
	meaningfulTokens   map[string]struct{}
	strongSignals      map[string]struct{}
	shortEvidence      bool
}

func newEvidenceMatcher(finding Finding, filePath string) evidenceMatcher {
	normalizedEvidence := normalizeEvidenceText(finding.Evidence)
	claimText := normalizeEvidenceText(strings.Join([]string{
		finding.Title,
		finding.Evidence,
		finding.FailureScenario,
		finding.Suggestion,
		path.Base(filePath),
	}, " "))
	evidenceTokens := tokenSet(finding.Evidence)
	claimTokens := tokenSet(claimText)
	meaningful := meaningfulTokenSet(evidenceTokens)
	strong := strongSignalSet(finding.Evidence, filePath)
	for token := range meaningfulTokenSet(tokenSet(finding.Title)) {
		if _, inEvidence := meaningful[token]; inEvidence {
			strong[token] = struct{}{}
		}
	}
	return evidenceMatcher{
		finding:            finding,
		filePath:           filePath,
		normalizedEvidence: normalizedEvidence,
		claimText:          claimText,
		evidenceTokens:     evidenceTokens,
		claimTokens:        claimTokens,
		meaningfulTokens:   meaningful,
		strongSignals:      strong,
		shortEvidence:      len(evidenceTokens) <= 3 || len(normalizedEvidence) < 24,
	}
}

func (m evidenceMatcher) matches(source EvidenceSource) bool {
	sourceNorm := normalizeEvidenceText(source.Content)
	if sourceNorm == "" {
		return false
	}
	if strings.Contains(sourceNorm, m.normalizedEvidence) {
		if m.shortEvidence {
			return m.hasStrongSignalIn(source.Content)
		}
		return true
	}
	if m.normalizedSnippetMatches(sourceNorm) {
		return true
	}
	sourceTokens := tokenSet(source.Content)
	if m.identifierAwareMatches(sourceTokens, source.Content) {
		return true
	}
	return m.tokenOverlapMatches(sourceTokens)
}

func (m evidenceMatcher) normalizedSnippetMatches(sourceNorm string) bool {
	if m.normalizedEvidence == "" || m.shortEvidence && !m.hasStrongSignalIn(sourceNorm) {
		return false
	}
	parts := splitNormalizedSnippet(m.normalizedEvidence)
	for _, part := range parts {
		if len(part) >= 12 && strings.Contains(sourceNorm, part) {
			return true
		}
	}
	return false
}

func (m evidenceMatcher) identifierAwareMatches(sourceTokens map[string]struct{}, sourceContent string) bool {
	strongMatches := 0
	for signal := range m.strongSignals {
		if _, ok := sourceTokens[signal]; ok {
			strongMatches++
		}
	}
	if m.shortEvidence {
		return strongMatches > 0
	}
	if strongMatches >= 2 {
		return true
	}
	if strongMatches == 1 && m.tokenOverlapRatio(sourceTokens, m.meaningfulTokens) >= 0.50 {
		return true
	}
	return m.hasCodePhraseIn(sourceContent)
}

func (m evidenceMatcher) tokenOverlapMatches(sourceTokens map[string]struct{}) bool {
	if m.shortEvidence {
		return false
	}
	return m.tokenOverlapRatio(sourceTokens, m.meaningfulTokens) >= 0.68 && overlapCount(sourceTokens, m.strongSignals) > 0
}

func (m evidenceMatcher) tokenOverlapRatio(sourceTokens map[string]struct{}, needles map[string]struct{}) float64 {
	if len(needles) == 0 {
		return 0
	}
	return float64(overlapCount(sourceTokens, needles)) / float64(len(needles))
}

func (m evidenceMatcher) hasStrongSignalIn(content string) bool {
	sourceTokens := tokenSet(content)
	if overlapCount(sourceTokens, m.strongSignals) > 0 {
		return true
	}
	return m.hasCodePhraseIn(content)
}

func (m evidenceMatcher) hasCodePhraseIn(content string) bool {
	contentNorm := normalizeEvidenceText(content)
	for _, phrase := range codePhrases(m.finding.Evidence) {
		if strings.Contains(contentNorm, normalizeEvidenceText(phrase)) {
			return true
		}
	}
	return false
}

func sourceCompatible(finding Finding, filePath string, source EvidenceSource) bool {
	claim := normalizeEvidenceText(strings.Join([]string{
		finding.Category,
		finding.Title,
		finding.Evidence,
		finding.FailureScenario,
		finding.Suggestion,
		filePath,
	}, " "))
	switch source.Type {
	case EvidenceSourcePatch, EvidenceSourceFullFile, EvidenceSourceStaticCheck:
		return true
	case EvidenceSourceRelatedTest:
		return isTestPath(filePath) || strings.Contains(claim, "test") || strings.Contains(claim, "coverage")
	case EvidenceSourceRepoDocs:
		return isDocsOrConfigPath(filePath) || isDocsOrConfigFinding(claim)
	case EvidenceSourceOmitted:
		return isLimitationFinding(finding)
	default:
		return false
	}
}

func staticCheckEvidenceContent(item StaticCheckEvidence) string {
	parts := []string{item.Tool, string(item.ExitCategory), item.Package, item.Message}
	if item.Line != nil {
		parts = append(parts, "line "+strconv.Itoa(*item.Line))
	}
	return strings.Join(parts, " ")
}

func staticCheckLineSet(item StaticCheckEvidence) lineSet {
	lines := lineSet{}
	if item.Line != nil && *item.Line > 0 {
		lines[*item.Line] = struct{}{}
	}
	return lines
}

func normalizeEvidenceText(text string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(text)))
	return strings.Join(fields, " ")
}

func splitNormalizedSnippet(text string) []string {
	var parts []string
	for _, sep := range []string{" before ", " after ", " because ", " when ", " while ", " and ", " only "} {
		if strings.Contains(text, sep) {
			for _, part := range strings.Split(text, sep) {
				part = strings.TrimSpace(part)
				if part != "" {
					parts = append(parts, part)
				}
			}
		}
	}
	if len(parts) == 0 {
		parts = append(parts, text)
	}
	return parts
}

func tokenSet(text string) map[string]struct{} {
	tokens := map[string]struct{}{}
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		token := strings.ToLower(b.String())
		tokens[token] = struct{}{}
		b.Reset()
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	for _, token := range splitIdentifiers(tokens) {
		tokens[token] = struct{}{}
	}
	return tokens
}

func splitIdentifiers(tokens map[string]struct{}) []string {
	var out []string
	for token := range tokens {
		for _, part := range strings.FieldsFunc(token, func(r rune) bool {
			return r == '_' || r == '-' || r == '.'
		}) {
			part = strings.ToLower(strings.TrimSpace(part))
			if part != "" && part != token {
				out = append(out, part)
			}
		}
	}
	return out
}

func meaningfulTokenSet(tokens map[string]struct{}) map[string]struct{} {
	out := map[string]struct{}{}
	for token := range tokens {
		if isMeaningfulToken(token) {
			out[token] = struct{}{}
		}
	}
	return out
}

func strongSignalSet(text, filePath string) map[string]struct{} {
	signals := map[string]struct{}{}
	for token := range tokenSet(text) {
		if isStrongSignalToken(token) {
			signals[token] = struct{}{}
		}
	}
	for token := range tokenSet(path.Base(filePath)) {
		if isStrongSignalToken(token) {
			signals[token] = struct{}{}
		}
	}
	for _, literal := range literalsIn(text) {
		signals[literal] = struct{}{}
	}
	for _, key := range configKeysIn(text) {
		signals[key] = struct{}{}
	}
	return signals
}

func isMeaningfulToken(token string) bool {
	if token == "" || genericEvidenceTokens[token] {
		return false
	}
	if len(token) < 3 && !hasDigit(token) {
		return false
	}
	return true
}

func isStrongSignalToken(token string) bool {
	if !isMeaningfulToken(token) {
		return false
	}
	return hasDigit(token) ||
		strings.ContainsAny(token, "._-") ||
		strings.HasSuffix(token, "()") ||
		len(token) >= 4
}

func hasDigit(token string) bool {
	for _, r := range token {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func overlapCount(sourceTokens, needles map[string]struct{}) int {
	count := 0
	for token := range needles {
		if _, ok := sourceTokens[token]; ok {
			count++
		}
	}
	return count
}

func literalsIn(text string) []string {
	var out []string
	for token := range tokenSet(text) {
		if hasDigit(token) || strings.EqualFold(token, "true") || strings.EqualFold(token, "false") {
			out = append(out, token)
		}
	}
	return out
}

func configKeysIn(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			if key != "" {
				for token := range tokenSet(key) {
					if isMeaningfulToken(token) {
						out = append(out, token)
					}
				}
			}
		}
	}
	return out
}

func codePhrases(text string) []string {
	var phrases []string
	if strings.ContainsAny(text, "=!<>:&|+-*/{}()[]") {
		phrases = append(phrases, text)
	}
	return phrases
}

func isTestPath(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.Contains(lower, "_test.") || strings.Contains(lower, "/test/") || strings.Contains(lower, "/tests/")
}

func isDocsOrConfigPath(filePath string) bool {
	lower := strings.ToLower(filePath)
	base := path.Base(lower)
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".yml") ||
		strings.HasSuffix(lower, ".yaml") ||
		strings.HasSuffix(lower, ".json") ||
		strings.HasSuffix(lower, ".toml") ||
		strings.HasSuffix(lower, ".ini") ||
		strings.HasPrefix(lower, ".github/") ||
		base == "dockerfile"
}

func isDocsOrConfigFinding(text string) bool {
	for _, token := range []string{"doc", "docs", "documentation", "readme", "workflow", "configuration", "config", "setting", "settings", "advisory"} {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

var genericEvidenceTokens = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "be": true, "before": true, "by": true,
	"can": true, "code": true, "config": true, "could": true, "do": true, "does": true, "error": true,
	"file": true, "for": true, "from": true, "handler": true, "if": true, "in": true, "is": true, "it": true,
	"may": true, "nil": true, "not": true, "of": true, "on": true, "only": true, "or": true, "request": true,
	"response": true, "return": true, "set": true, "test": true, "that": true, "the": true, "this": true,
	"timeout": true, "to": true, "value": true, "when": true, "with": true,
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

func (s VerificationStats) String() string {
	reasonParts := make([]string, 0, len(s.Reasons))
	for _, reason := range s.SortedReasons() {
		reasonParts = append(reasonParts, string(reason)+"="+strconv.Itoa(s.Reasons[reason]))
	}
	staticSkipParts := make([]string, 0, len(s.StaticCheckSkipped))
	for _, category := range s.SortedStaticCheckSkipped() {
		staticSkipParts = append(staticSkipParts, string(category)+"="+strconv.Itoa(s.StaticCheckSkipped[category]))
	}
	return fmt.Sprintf(
		"total=%d kept=%d downgraded=%d dropped=%d no_findings=%d kept_rate=%.4f downgraded_rate=%.4f dropped_rate=%.4f static_check_evidence=%d static_check_supported=%d static_check_categories=%s reasons=%s",
		s.TotalFindings,
		s.Kept,
		s.Downgraded,
		s.Dropped,
		s.NoFindingCount,
		s.KeptRate,
		s.DowngradedRate,
		s.DroppedRate,
		s.StaticCheckEvidenceCount,
		s.StaticCheckSupported,
		strings.Join(staticSkipParts, ","),
		strings.Join(reasonParts, ","),
	)
}

func (s VerificationStats) SortedStaticCheckSkipped() []GoAnalyzerExitCategory {
	categories := make([]GoAnalyzerExitCategory, 0, len(s.StaticCheckSkipped))
	for category := range s.StaticCheckSkipped {
		categories = append(categories, category)
	}
	sort.Slice(categories, func(i, j int) bool { return categories[i] < categories[j] })
	return categories
}
