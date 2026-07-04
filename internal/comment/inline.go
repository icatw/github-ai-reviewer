package comment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github-ai-reviewer/internal/review"
)

const (
	InlineMarker        = "<!-- github-ai-reviewer:inline-comment:v1"
	InlineStaleMarker   = "<!-- github-ai-reviewer:inline-comment:stale:v1 -->"
	maxInlineComments   = 10
	minInlineConfidence = 0.70
)

type inlinePolicy struct {
	enabled             bool
	maxComments         int
	severityThreshold   review.SeverityThreshold
	confidenceThreshold float64
	pathIgnore          review.PathIgnorePatterns
	language            review.Language
}

type PullRequestFileLister interface {
	FetchPullRequestFiles(ctx context.Context, installationID int64, owner, repo string, pullNumber int) ([]review.FileChange, error)
}

type PullRequestReviewCommenter interface {
	ListPullRequestReviewComments(ctx context.Context, installationID int64, owner, repo string, pullNumber int) ([]ReviewComment, error)
	CreatePullRequestReviewComment(ctx context.Context, installationID int64, owner, repo string, pullNumber int, req ReviewCommentRequest) error
	UpdatePullRequestReviewComment(ctx context.Context, installationID int64, owner, repo string, commentID int64, body string) error
}

type PullRequestReviewer interface {
	CreatePullRequestReview(ctx context.Context, installationID int64, owner, repo string, pullNumber int, req PullRequestReviewRequest) (PullRequestReview, error)
}

type ReviewComment struct {
	ID         int64
	Body       string
	AuthorType string
}

type PullRequestReview struct {
	ID int64
}

type PullRequestReviewRequest struct {
	CommitID string
	Body     string
	Event    string
	Comments []ReviewCommentRequest
}

type ReviewCommentRequest struct {
	CommitID string
	Path     string
	Line     int
	Side     string
	Body     string
}

type inlineClient interface {
	PullRequestFileLister
	PullRequestReviewCommenter
	PullRequestReviewer
}

type InlineStats struct {
	Findings                 int
	Eligible                 int
	Mapped                   int
	Created                  int
	Updated                  int
	Stale                    int
	SkippedDisabled          int
	SkippedUnsupportedClient int
	SkippedQuality           int
	SkippedUnmapped          int
	SkippedLimit             int
}

func (p *Publisher) publishInlineComments(ctx context.Context, installationID int64, owner, repo string, number int, headSHA string, effective *review.EffectiveReviewConfig, result review.ReviewResult) error {
	policy := p.inlinePolicy(effective)
	stats := InlineStats{Findings: len(result.Findings)}
	defer func() { p.logInlineStats(owner, repo, number, stats) }()
	if !policy.enabled {
		stats.SkippedDisabled = len(result.Findings)
		return nil
	}
	if headSHA == "" || len(result.Findings) == 0 {
		return nil
	}
	client, ok := p.client.(inlineClient)
	if !ok {
		stats.SkippedUnsupportedClient = len(result.Findings)
		return nil
	}
	files, err := client.FetchPullRequestFiles(ctx, installationID, owner, repo, number)
	if err != nil {
		return err
	}
	files, ignoredFiles := filterInlineIgnoredFiles(files, policy.pathIgnore)
	comments, err := client.ListPullRequestReviewComments(ctx, installationID, owner, repo, number)
	if err != nil {
		return err
	}
	existing := existingInlineComments(comments)
	patches := patchLineIndex(files)
	published := 0
	current := map[string]struct{}{}
	var creates []ReviewCommentRequest
	for _, finding := range result.Findings {
		if policy.pathIgnore.Matches(finding.File) {
			stats.SkippedQuality++
			continue
		}
		if published >= policy.maxComments {
			stats.SkippedLimit++
			continue
		}
		if !shouldPublishInlineFindingWithPolicy(finding, policy) {
			stats.SkippedQuality++
			continue
		}
		stats.Eligible++
		line, ok := findingLine(finding)
		if !ok || !patches.contains(finding.File, line) {
			stats.SkippedUnmapped++
			continue
		}
		stats.Mapped++
		fingerprint := inlineFingerprint(finding)
		body := renderInlineFinding(finding, policy.language)
		body = fmt.Sprintf("%s\n\n%s fingerprint=%s -->", body, InlineMarker, fingerprint)
		current[fingerprint] = struct{}{}
		if existingComment, ok := existing[fingerprint]; ok {
			if err := client.UpdatePullRequestReviewComment(ctx, installationID, owner, repo, existingComment.ID, body); err != nil {
				return err
			}
			stats.Updated++
			published++
			continue
		}
		creates = append(creates, ReviewCommentRequest{
			CommitID: headSHA,
			Path:     finding.File,
			Line:     line,
			Side:     "RIGHT",
			Body:     body,
		})
		stats.Created++
		published++
	}
	if len(creates) > 0 {
		if _, err := client.CreatePullRequestReview(ctx, installationID, owner, repo, number, PullRequestReviewRequest{
			CommitID: headSHA,
			Body:     renderPullRequestReviewBody(len(creates), policy.language),
			Event:    "COMMENT",
			Comments: creates,
		}); err != nil {
			p.logInlineBatchFailure(owner, repo, number)
			if err := p.fallbackCreateReviewComments(ctx, client, installationID, owner, repo, number, creates); err != nil {
				return err
			}
		}
	}
	for fingerprint, existingComment := range existing {
		if _, ok := current[fingerprint]; ok {
			continue
		}
		if strings.Contains(existingComment.Body, InlineStaleMarker) {
			continue
		}
		body := renderStaleInlineComment(existingComment.Body, fingerprint, headSHA)
		if err := client.UpdatePullRequestReviewComment(ctx, installationID, owner, repo, existingComment.ID, body); err != nil {
			p.logInlineStaleFailure(owner, repo, number)
			continue
		}
		stats.Stale++
	}
	stats.SkippedQuality += ignoredFiles
	return nil
}

func (p *Publisher) inlinePolicy(effective *review.EffectiveReviewConfig) inlinePolicy {
	policy := inlinePolicy{
		enabled:             p.inlineEnabled,
		maxComments:         maxInlineComments,
		severityThreshold:   review.SeverityWarning,
		confidenceThreshold: minInlineConfidence,
		language:            p.language,
	}
	if policy.language == "" {
		policy.language = review.LanguageEnglish
	}
	if effective == nil {
		return policy
	}
	policy.enabled = policy.enabled && effective.InlineCommentsEnabled
	policy.maxComments = effective.InlineMaxComments
	if policy.maxComments < 0 {
		policy.maxComments = 0
	}
	if policy.maxComments > maxInlineComments {
		policy.maxComments = maxInlineComments
	}
	if effective.InlineSeverityThreshold != "" {
		policy.severityThreshold = effective.InlineSeverityThreshold
	}
	if effective.InlineConfidenceThreshold >= 0 && effective.InlineConfidenceThreshold <= 1 {
		policy.confidenceThreshold = effective.InlineConfidenceThreshold
	}
	if effective.Language != "" {
		policy.language = effective.Language
	}
	policy.pathIgnore = effective.PathIgnore
	return policy
}

func filterInlineIgnoredFiles(files []review.FileChange, patterns review.PathIgnorePatterns) ([]review.FileChange, int) {
	if len(patterns) == 0 {
		return files, 0
	}
	out := make([]review.FileChange, 0, len(files))
	ignored := 0
	for _, file := range files {
		if patterns.Matches(file.Filename) {
			ignored++
			continue
		}
		out = append(out, file)
	}
	return out, ignored
}

func (p *Publisher) cleanupInlineComments(ctx context.Context, job review.CleanupJob) error {
	if !p.inlineEnabled {
		return nil
	}
	client, ok := p.client.(PullRequestReviewCommenter)
	if !ok {
		return nil
	}
	comments, err := client.ListPullRequestReviewComments(ctx, job.InstallationID, job.Owner, job.Repo, job.PullNumber)
	if err != nil {
		p.logCleanup(job, "inline_list_failed")
		return err
	}
	updated := 0
	for _, existingComment := range existingInlineComments(comments) {
		if strings.Contains(existingComment.Body, InlineStaleMarker) {
			continue
		}
		fingerprint := extractInlineFingerprint(existingComment.Body)
		body := renderInactiveInlineComment(existingComment.Body, fingerprint, job.State)
		if err := client.UpdatePullRequestReviewComment(ctx, job.InstallationID, job.Owner, job.Repo, existingComment.ID, body); err != nil {
			p.logCleanup(job, "inline_update_failed")
			continue
		}
		updated++
	}
	if updated > 0 {
		p.logCleanup(job, "inline_inactive")
	}
	return nil
}

func (p *Publisher) logInlineStats(owner, repo string, number int, stats InlineStats) {
	if p.logger == nil {
		return
	}
	p.logger.Printf("inline comments completed repo=%s/%s pull=%d findings=%d eligible=%d mapped=%d created=%d updated=%d stale=%d skipped_disabled=%d skipped_unsupported_client=%d skipped_quality=%d skipped_unmapped=%d skipped_limit=%d", owner, repo, number, stats.Findings, stats.Eligible, stats.Mapped, stats.Created, stats.Updated, stats.Stale, stats.SkippedDisabled, stats.SkippedUnsupportedClient, stats.SkippedQuality, stats.SkippedUnmapped, stats.SkippedLimit)
}

func (p *Publisher) logInlineBatchFailure(owner, repo string, number int) {
	if p.logger == nil {
		return
	}
	p.logger.Printf("inline batch create failed repo=%s/%s pull=%d category=github_error fallback=individual", owner, repo, number)
}

func (p *Publisher) logInlineStaleFailure(owner, repo string, number int) {
	if p.logger == nil {
		return
	}
	p.logger.Printf("inline stale mark failed repo=%s/%s pull=%d category=github_error", owner, repo, number)
}

func (p *Publisher) fallbackCreateReviewComments(ctx context.Context, client inlineClient, installationID int64, owner, repo string, number int, creates []ReviewCommentRequest) error {
	for _, req := range creates {
		if err := client.CreatePullRequestReviewComment(ctx, installationID, owner, repo, number, req); err != nil {
			return err
		}
	}
	return nil
}

func findingLine(finding review.Finding) (int, bool) {
	if strings.TrimSpace(finding.File) == "" || finding.Line == nil || *finding.Line <= 0 {
		return 0, false
	}
	return *finding.Line, true
}

func shouldPublishInlineFinding(finding review.Finding) bool {
	return shouldPublishInlineFindingWithPolicy(finding, inlinePolicy{
		enabled:             true,
		maxComments:         maxInlineComments,
		severityThreshold:   review.SeverityWarning,
		confidenceThreshold: minInlineConfidence,
	})
}

func shouldPublishInlineFindingWithPolicy(finding review.Finding, policy inlinePolicy) bool {
	switch strings.ToLower(strings.TrimSpace(finding.Severity)) {
	case "blocker", "warning":
	default:
		return false
	}
	if inlineSeverityRank(review.SeverityThreshold(strings.ToLower(strings.TrimSpace(finding.Severity)))) > inlineSeverityRank(policy.severityThreshold) {
		return false
	}
	if strings.TrimSpace(finding.Title) == "" || strings.TrimSpace(finding.Evidence) == "" || strings.TrimSpace(finding.FailureScenario) == "" || strings.TrimSpace(finding.Suggestion) == "" {
		return false
	}
	if finding.Confidence != nil && *finding.Confidence < policy.confidenceThreshold {
		return false
	}
	return true
}

func inlineSeverityRank(severity review.SeverityThreshold) int {
	switch severity {
	case review.SeverityBlocker:
		return 0
	case review.SeverityWarning:
		return 1
	case review.SeveritySuggestion:
		return 2
	case review.SeverityQuestion:
		return 3
	default:
		return 1
	}
}

func renderInlineFinding(finding review.Finding, language review.Language) string {
	labels := inlineLabelsForLanguage(language)
	var b strings.Builder
	fmt.Fprintf(&b, "%s **%s:** %s", labels.severityIcon(finding.Severity), labels.severityText(finding.Severity), strings.TrimSpace(finding.Title))
	if strings.TrimSpace(finding.Suggestion) != "" {
		fmt.Fprintf(&b, "\n\n**%s:** %s", labels.Suggestion, strings.TrimSpace(finding.Suggestion))
	}
	var details strings.Builder
	if finding.Evidence != "" {
		fmt.Fprintf(&details, "\n\n**%s:** %s", labels.Evidence, strings.TrimSpace(finding.Evidence))
	}
	if finding.FailureScenario != "" {
		fmt.Fprintf(&details, "\n\n**%s:** %s", labels.FailureScenario, strings.TrimSpace(finding.FailureScenario))
	}
	if finding.Confidence != nil {
		fmt.Fprintf(&details, "\n\n**%s:** %.2f", labels.Confidence, *finding.Confidence)
	}
	if details.Len() > 0 {
		fmt.Fprintf(&b, "\n\n<details>\n<summary>%s</summary>%s\n\n</details>", labels.Details, details.String())
	}
	return strings.TrimSpace(b.String())
}

func renderPullRequestReviewBody(commentCount int, language review.Language) string {
	if language == review.LanguageSimplifiedChinese {
		return fmt.Sprintf("Review Cat 留下了 %d 条行级评论。所有发现均为建议，不会阻塞合并。", commentCount)
	}
	return fmt.Sprintf("Review Cat left %d inline comment(s). Findings are advisory and non-blocking.", commentCount)
}

func renderStaleInlineComment(existingBody, fingerprint, headSHA string) string {
	stale := InlineStaleMarker + "\n_Stale: this finding was not produced by the latest AI review run"
	if strings.TrimSpace(headSHA) != "" {
		stale += fmt.Sprintf(" for `%s`", headSHA)
	}
	stale += "._"
	body := strings.TrimSpace(existingBody)
	if body == "" {
		return fmt.Sprintf("%s fingerprint=%s -->\n%s", InlineMarker, fingerprint, stale)
	}
	return body + "\n\n" + stale
}

func renderInactiveInlineComment(existingBody, fingerprint string, state review.CleanupState) string {
	reason := "closed"
	if state == review.CleanupStateMerged {
		reason = "merged"
	}
	stale := InlineStaleMarker + "\n_Inactive: this finding is inactive because this pull request was " + reason + "._"
	body := strings.TrimSpace(existingBody)
	if body == "" {
		return fmt.Sprintf("%s fingerprint=%s -->\n%s", InlineMarker, fingerprint, stale)
	}
	return body + "\n\n" + stale
}

func inlineFingerprint(finding review.Finding) string {
	line := 0
	if finding.Line != nil {
		line = *finding.Line
	}
	input := fmt.Sprintf("%s\x00%d\x00%s\x00%s\x00%s", finding.File, line, finding.Severity, finding.Title, finding.Evidence)
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:8])
}

func existingInlineComments(comments []ReviewComment) map[string]ReviewComment {
	out := map[string]ReviewComment{}
	for _, comment := range comments {
		if comment.AuthorType != "Bot" || !strings.Contains(comment.Body, InlineMarker) {
			continue
		}
		fingerprint := extractInlineFingerprint(comment.Body)
		if fingerprint != "" {
			out[fingerprint] = comment
		}
	}
	return out
}

func extractInlineFingerprint(body string) string {
	idx := strings.Index(body, "fingerprint=")
	if idx < 0 {
		return ""
	}
	start := idx + len("fingerprint=")
	end := start
	for end < len(body) {
		c := body[end]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			break
		}
		end++
	}
	fingerprint := body[start:end]
	if len(fingerprint) != 16 {
		return ""
	}
	return fingerprint
}

type inlineRenderLabels struct {
	BlockingRisk    string
	PotentialIssue  string
	Suggestion      string
	Details         string
	Evidence        string
	FailureScenario string
	Confidence      string
}

func inlineLabelsForLanguage(language review.Language) inlineRenderLabels {
	if language == review.LanguageSimplifiedChinese {
		return inlineRenderLabels{
			BlockingRisk:    "阻塞风险",
			PotentialIssue:  "潜在问题",
			Suggestion:      "建议",
			Details:         "详情",
			Evidence:        "证据",
			FailureScenario: "失败场景",
			Confidence:      "置信度",
		}
	}
	return inlineRenderLabels{
		BlockingRisk:    "Blocking risk",
		PotentialIssue:  "Potential issue",
		Suggestion:      "Suggestion",
		Details:         "Details",
		Evidence:        "Evidence",
		FailureScenario: "Failure scenario",
		Confidence:      "Confidence",
	}
}

func (l inlineRenderLabels) severityIcon(severity string) string {
	if strings.EqualFold(strings.TrimSpace(severity), "blocker") {
		return "🚨"
	}
	return "⚠️"
}

func (l inlineRenderLabels) severityText(severity string) string {
	if strings.EqualFold(strings.TrimSpace(severity), "blocker") {
		return l.BlockingRisk
	}
	return l.PotentialIssue
}

type lineIndex map[string]map[int]struct{}

func (i lineIndex) contains(filePath string, line int) bool {
	lines := i[strings.TrimSpace(filePath)]
	if lines == nil {
		return false
	}
	_, ok := lines[line]
	return ok
}

func patchLineIndex(files []review.FileChange) lineIndex {
	out := lineIndex{}
	for _, file := range files {
		lines := diffRightLines(file.Patch)
		if len(lines) == 0 {
			continue
		}
		out[file.Filename] = lines
	}
	return out
}

var hunkHeaderRE = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

func diffRightLines(patch string) map[int]struct{} {
	out := map[int]struct{}{}
	newLine := 0
	inHunk := false
	for _, raw := range strings.Split(patch, "\n") {
		if matches := hunkHeaderRE.FindStringSubmatch(raw); len(matches) == 2 {
			line, err := strconv.Atoi(matches[1])
			if err != nil {
				inHunk = false
				continue
			}
			newLine = line
			inHunk = true
			continue
		}
		if !inHunk || raw == "" {
			continue
		}
		switch raw[0] {
		case '+', ' ':
			out[newLine] = struct{}{}
			newLine++
		case '-':
			continue
		case '\\':
			continue
		default:
			inHunk = false
		}
	}
	return out
}
