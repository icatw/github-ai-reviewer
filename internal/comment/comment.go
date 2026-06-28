package comment

import (
	"context"
	"fmt"
	"strings"

	"github-ai-reviewer/internal/review"
)

const Marker = "<!-- github-ai-reviewer:review-comment:v1 -->"

type IssueComment struct {
	ID         int64
	Body       string
	AuthorType string
}

type IssueCommenter interface {
	ListIssueComments(ctx context.Context, installationID int64, owner, repo string, number int) ([]IssueComment, error)
	CreateIssueComment(ctx context.Context, installationID int64, owner, repo string, number int, body string) error
	UpdateIssueComment(ctx context.Context, installationID int64, owner, repo string, commentID int64, body string) error
}

type Publisher struct {
	client IssueCommenter
}

func NewPublisher(client IssueCommenter) *Publisher {
	return &Publisher{client: client}
}

func Render(result review.ReviewResult) (string, bool) {
	if !result.HasUsefulContent() {
		return "", false
	}
	var b strings.Builder
	b.WriteString(Marker)
	b.WriteString("\n## AI Review Summary\n\n")
	if result.Summary != "" {
		b.WriteString(result.Summary)
		b.WriteString("\n\n")
	}
	if result.RiskScore != nil {
		fmt.Fprintf(&b, "**Risk:** %d/100\n\n", *result.RiskScore)
	}
	if len(result.Findings) > 0 {
		b.WriteString("### Findings\n\n")
		b.WriteString("Findings are advisory and non-blocking in this M2 review.\n\n")
		for i, finding := range result.Findings {
			fmt.Fprintf(&b, "%d. **%s: %s**\n", i+1, titleCase(finding.Severity), finding.Title)
			if finding.Category != "" {
				fmt.Fprintf(&b, "   - Category: %s\n", finding.Category)
			}
			if finding.File != "" {
				location := finding.File
				if finding.Line != nil {
					location = fmt.Sprintf("%s:%d", location, *finding.Line)
				}
				fmt.Fprintf(&b, "   - Location: %s\n", location)
			}
			fmt.Fprintf(&b, "   - Evidence: %s\n", finding.Evidence)
			fmt.Fprintf(&b, "   - Failure scenario: %s\n", finding.FailureScenario)
			fmt.Fprintf(&b, "   - Suggestion: %s\n", finding.Suggestion)
			if finding.Confidence != nil {
				fmt.Fprintf(&b, "   - Confidence: %.2f\n", *finding.Confidence)
			}
			b.WriteString("\n")
		}
	}
	writeListSection(&b, "Missing Tests", result.MissingTests)
	writeListSection(&b, "Limitations", result.Limitations)
	body := strings.TrimRight(b.String(), "\n")
	body += "\n\n---\nThis is a non-blocking AI-generated review based on the available PR diff context."
	return body, true
}

func (p *Publisher) Publish(ctx context.Context, installationID int64, owner, repo string, number int, result review.ReviewResult) error {
	body, ok := Render(result)
	if !ok {
		return nil
	}
	if strings.TrimSpace(body) == "" {
		return nil
	}
	comments, err := p.client.ListIssueComments(ctx, installationID, owner, repo, number)
	if err != nil {
		return err
	}
	for _, issueComment := range comments {
		if strings.Contains(issueComment.Body, Marker) && issueComment.AuthorType == "Bot" {
			return p.client.UpdateIssueComment(ctx, installationID, owner, repo, issueComment.ID, body)
		}
	}
	return p.client.CreateIssueComment(ctx, installationID, owner, repo, number, body)
}

func writeListSection(b *strings.Builder, title string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "### %s\n\n", title)
	for _, value := range values {
		fmt.Fprintf(b, "- %s\n", value)
	}
	b.WriteString("\n")
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
