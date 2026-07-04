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
	InlineMarker      = "<!-- github-ai-reviewer:inline-comment:v1"
	maxInlineComments = 10
)

type PullRequestFileLister interface {
	FetchPullRequestFiles(ctx context.Context, installationID int64, owner, repo string, pullNumber int) ([]review.FileChange, error)
}

type PullRequestReviewCommenter interface {
	ListPullRequestReviewComments(ctx context.Context, installationID int64, owner, repo string, pullNumber int) ([]ReviewComment, error)
	CreatePullRequestReviewComment(ctx context.Context, installationID int64, owner, repo string, pullNumber int, req ReviewCommentRequest) error
	UpdatePullRequestReviewComment(ctx context.Context, installationID int64, owner, repo string, commentID int64, body string) error
}

type ReviewComment struct {
	ID         int64
	Body       string
	AuthorType string
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
}

func (p *Publisher) publishInlineComments(ctx context.Context, installationID int64, owner, repo string, number int, headSHA string, result review.ReviewResult) error {
	if !p.inlineEnabled || headSHA == "" || len(result.Findings) == 0 {
		return nil
	}
	client, ok := p.client.(inlineClient)
	if !ok {
		return nil
	}
	files, err := client.FetchPullRequestFiles(ctx, installationID, owner, repo, number)
	if err != nil {
		return err
	}
	comments, err := client.ListPullRequestReviewComments(ctx, installationID, owner, repo, number)
	if err != nil {
		return err
	}
	existing := existingInlineComments(comments)
	patches := patchLineIndex(files)
	created := 0
	for _, finding := range result.Findings {
		if created >= maxInlineComments {
			break
		}
		line, ok := findingLine(finding)
		if !ok || !patches.contains(finding.File, line) {
			continue
		}
		body := renderInlineFinding(finding, p.language)
		fingerprint := inlineFingerprint(finding)
		body = fmt.Sprintf("%s fingerprint=%s -->\n%s", InlineMarker, fingerprint, body)
		if commentID, ok := existing[fingerprint]; ok {
			if err := client.UpdatePullRequestReviewComment(ctx, installationID, owner, repo, commentID, body); err != nil {
				return err
			}
			created++
			continue
		}
		if err := client.CreatePullRequestReviewComment(ctx, installationID, owner, repo, number, ReviewCommentRequest{
			CommitID: headSHA,
			Path:     finding.File,
			Line:     line,
			Side:     "RIGHT",
			Body:     body,
		}); err != nil {
			return err
		}
		created++
	}
	return nil
}

func findingLine(finding review.Finding) (int, bool) {
	if strings.TrimSpace(finding.File) == "" || finding.Line == nil || *finding.Line <= 0 {
		return 0, false
	}
	return *finding.Line, true
}

func renderInlineFinding(finding review.Finding, language review.Language) string {
	labels := labelsForLanguage(language)
	var b strings.Builder
	fmt.Fprintf(&b, "**%s: %s**\n\n", titleCase(finding.Severity), finding.Title)
	if finding.Evidence != "" {
		fmt.Fprintf(&b, "- %s: %s\n", labels.Evidence, finding.Evidence)
	}
	if finding.FailureScenario != "" {
		fmt.Fprintf(&b, "- %s: %s\n", labels.FailureScenario, finding.FailureScenario)
	}
	if finding.Suggestion != "" {
		fmt.Fprintf(&b, "- %s: %s\n", labels.Suggestion, finding.Suggestion)
	}
	return strings.TrimSpace(b.String())
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

func existingInlineComments(comments []ReviewComment) map[string]int64 {
	out := map[string]int64{}
	for _, comment := range comments {
		if comment.AuthorType != "Bot" || !strings.Contains(comment.Body, InlineMarker) {
			continue
		}
		fingerprint := extractInlineFingerprint(comment.Body)
		if fingerprint != "" {
			out[fingerprint] = comment.ID
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
	return body[start:end]
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
