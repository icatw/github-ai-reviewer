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

type Logger interface {
	Printf(format string, args ...any)
}

type Publisher struct {
	client        IssueCommenter
	language      review.Language
	inlineEnabled bool
	logger        Logger
}

type PublisherOptions struct {
	Language              review.Language
	InlineCommentsEnabled bool
	Logger                Logger
}

func NewPublisher(client IssueCommenter) *Publisher {
	return NewPublisherWithOptions(client, PublisherOptions{})
}

func NewPublisherWithOptions(client IssueCommenter, opts PublisherOptions) *Publisher {
	language := opts.Language
	if language == "" {
		language = review.LanguageEnglish
	}
	return &Publisher{client: client, language: language, inlineEnabled: opts.InlineCommentsEnabled, logger: opts.Logger}
}

func Render(result review.ReviewResult) (string, bool) {
	return RenderWithLanguage(result, review.LanguageEnglish)
}

func RenderWithLanguage(result review.ReviewResult, language review.Language) (string, bool) {
	if !result.HasUsefulContent() {
		return "", false
	}
	labels := labelsForLanguage(language)
	var b strings.Builder
	b.WriteString(Marker)
	fmt.Fprintf(&b, "\n## %s\n\n", labels.SummaryTitle)
	if result.Summary != "" {
		b.WriteString(result.Summary)
		b.WriteString("\n\n")
	}
	if result.RiskScore != nil {
		fmt.Fprintf(&b, "**%s:** %d/100\n\n", labels.Risk, *result.RiskScore)
	}
	if len(result.Findings) > 0 {
		fmt.Fprintf(&b, "### %s\n\n", labels.Findings)
		fmt.Fprintf(&b, "%s\n\n", labels.Advisory)
		for i, finding := range result.Findings {
			fmt.Fprintf(&b, "%d. **%s: %s**\n", i+1, titleCase(finding.Severity), finding.Title)
			if finding.Category != "" {
				fmt.Fprintf(&b, "   - %s: %s\n", labels.Category, finding.Category)
			}
			if finding.File != "" {
				location := finding.File
				if finding.Line != nil {
					location = fmt.Sprintf("%s:%d", location, *finding.Line)
				}
				fmt.Fprintf(&b, "   - %s: %s\n", labels.Location, location)
			}
			fmt.Fprintf(&b, "   - %s: %s\n", labels.Evidence, finding.Evidence)
			fmt.Fprintf(&b, "   - %s: %s\n", labels.FailureScenario, finding.FailureScenario)
			fmt.Fprintf(&b, "   - %s: %s\n", labels.Suggestion, finding.Suggestion)
			if finding.Confidence != nil {
				fmt.Fprintf(&b, "   - %s: %.2f\n", labels.Confidence, *finding.Confidence)
			}
			b.WriteString("\n")
		}
	}
	writeListSection(&b, labels.MissingTests, result.MissingTests)
	writeListSection(&b, labels.Limitations, result.Limitations)
	body := strings.TrimRight(b.String(), "\n")
	body += "\n\n---\n" + labels.Footer
	return body, true
}

func (p *Publisher) Publish(ctx context.Context, installationID int64, owner, repo string, number int, result review.ReviewResult) error {
	return p.PublishForHead(ctx, installationID, owner, repo, number, "", result)
}

func (p *Publisher) PublishForHead(ctx context.Context, installationID int64, owner, repo string, number int, headSHA string, result review.ReviewResult) error {
	body, ok := RenderWithLanguage(result, p.language)
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
			if err := p.client.UpdateIssueComment(ctx, installationID, owner, repo, issueComment.ID, body); err != nil {
				return err
			}
			return p.publishInlineComments(ctx, installationID, owner, repo, number, headSHA, result)
		}
	}
	if err := p.client.CreateIssueComment(ctx, installationID, owner, repo, number, body); err != nil {
		return err
	}
	return p.publishInlineComments(ctx, installationID, owner, repo, number, headSHA, result)
}

func (p *Publisher) Cleanup(ctx context.Context, job review.CleanupJob) error {
	body := RenderInactive(job.State, p.language)
	comments, err := p.client.ListIssueComments(ctx, job.InstallationID, job.Owner, job.Repo, job.PullNumber)
	if err != nil {
		p.logCleanup(job, "summary_list_failed")
		return err
	}
	updatedSummary := false
	for _, issueComment := range comments {
		if strings.Contains(issueComment.Body, Marker) && issueComment.AuthorType == "Bot" {
			if err := p.client.UpdateIssueComment(ctx, job.InstallationID, job.Owner, job.Repo, issueComment.ID, body); err != nil {
				p.logCleanup(job, "summary_update_failed")
				return err
			}
			updatedSummary = true
			break
		}
	}
	category := "summary_marker_missing"
	if updatedSummary {
		category = "summary_inactive"
	}
	p.logCleanup(job, category)
	return p.cleanupInlineComments(ctx, job)
}

func RenderInactive(state review.CleanupState, language review.Language) string {
	labels := inactiveLabelsForLanguage(state, language)
	var b strings.Builder
	b.WriteString(Marker)
	fmt.Fprintf(&b, "\n## %s\n\n", labels.Title)
	b.WriteString(labels.Body)
	b.WriteString("\n\n---\n")
	b.WriteString(labels.Footer)
	return b.String()
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

type renderLabels struct {
	SummaryTitle    string
	Risk            string
	Findings        string
	Advisory        string
	Category        string
	Location        string
	Evidence        string
	FailureScenario string
	Suggestion      string
	Confidence      string
	MissingTests    string
	Limitations     string
	Footer          string
}

func labelsForLanguage(language review.Language) renderLabels {
	if language == review.LanguageSimplifiedChinese {
		return renderLabels{
			SummaryTitle:    "AI Review 总结",
			Risk:            "风险",
			Findings:        "发现的问题",
			Advisory:        "以下发现仅作为建议，不会阻塞本次 M2 Review。",
			Category:        "类别",
			Location:        "位置",
			Evidence:        "证据",
			FailureScenario: "失败场景",
			Suggestion:      "建议",
			Confidence:      "置信度",
			MissingTests:    "缺失的测试",
			Limitations:     "限制",
			Footer:          "这是基于当前 PR diff 上下文生成的非阻塞 AI Review。",
		}
	}
	return renderLabels{
		SummaryTitle:    "AI Review Summary",
		Risk:            "Risk",
		Findings:        "Findings",
		Advisory:        "Findings are advisory and non-blocking in this M2 review.",
		Category:        "Category",
		Location:        "Location",
		Evidence:        "Evidence",
		FailureScenario: "Failure scenario",
		Suggestion:      "Suggestion",
		Confidence:      "Confidence",
		MissingTests:    "Missing Tests",
		Limitations:     "Limitations",
		Footer:          "This is a non-blocking AI-generated review based on the available PR diff context.",
	}
}

type inactiveLabels struct {
	Title  string
	Body   string
	Footer string
}

func inactiveLabelsForLanguage(state review.CleanupState, language review.Language) inactiveLabels {
	if language == review.LanguageSimplifiedChinese {
		if state == review.CleanupStateMerged {
			return inactiveLabels{
				Title:  "AI Review 已归档",
				Body:   "此 AI Review 输出已停用，因为该 Pull Request 已合并。不会阻塞合并，也不会触发新的 LLM Review。",
				Footer: "这是一个非阻塞的 AI Review 生命周期状态。",
			}
		}
		return inactiveLabels{
			Title:  "AI Review 已归档",
			Body:   "此 AI Review 输出已停用，因为该 Pull Request 已关闭。不会阻塞合并，也不会触发新的 LLM Review。",
			Footer: "这是一个非阻塞的 AI Review 生命周期状态。",
		}
	}
	if state == review.CleanupStateMerged {
		return inactiveLabels{
			Title:  "AI Review Archived",
			Body:   "This AI Review output is inactive because this pull request was merged. No new LLM review was started, and this status is advisory and non-blocking.",
			Footer: "This is a non-blocking AI Review lifecycle status.",
		}
	}
	return inactiveLabels{
		Title:  "AI Review Archived",
		Body:   "This AI Review output is inactive because this pull request was closed. No new LLM review was started, and this status is advisory and non-blocking.",
		Footer: "This is a non-blocking AI Review lifecycle status.",
	}
}

func (p *Publisher) logCleanup(job review.CleanupJob, category string) {
	if p.logger == nil {
		return
	}
	p.logger.Printf("cleanup completed delivery=%s repo=%s/%s pull=%d state=%s category=%s", job.DeliveryID, job.Owner, job.Repo, job.PullNumber, job.State, category)
}
