package comment

import (
	"context"
	"strings"
	"testing"

	"github-ai-reviewer/internal/review"
)

func TestRenderProducesStableMarkdown(t *testing.T) {
	risk := 42
	line := 12
	confidence := 0.75
	body, ok := Render(review.ReviewResult{
		Summary:   "Looks focused.",
		RiskScore: &risk,
		Findings: []review.Finding{{
			Severity:        "warning",
			Category:        "correctness",
			File:            "main.go",
			Line:            &line,
			Title:           "Nil response can panic",
			Evidence:        "resp is used before err is checked",
			FailureScenario: "provider returns a transport error",
			Suggestion:      "check err before using resp",
			Confidence:      &confidence,
		}},
		MissingTests: []string{"transport error path"},
		Limitations:  []string{"Only diff context was available."},
	})
	if !ok {
		t.Fatal("Render() ok = false, want true")
	}
	want := `<!-- github-ai-reviewer:review-comment:v1 -->
## AI Review Summary

Looks focused.

**Risk:** 42/100

### Findings

Findings are advisory and non-blocking in this M2 review.

1. **Warning: Nil response can panic**
   - Category: correctness
   - Location: main.go:12
   - Evidence: resp is used before err is checked
   - Failure scenario: provider returns a transport error
   - Suggestion: check err before using resp
   - Confidence: 0.75

### Missing Tests

- transport error path

### Limitations

- Only diff context was available.

---
This is a non-blocking AI-generated review based on the available PR diff context.`
	if body != want {
		t.Fatalf("body mismatch\nwant:\n%s\n\ngot:\n%s", want, body)
	}
}

func TestRenderSuppressesEmptyOutput(t *testing.T) {
	if body, ok := Render(review.ReviewResult{}); ok || body != "" {
		t.Fatalf("Render() = %q, %v; want empty false", body, ok)
	}
}

func TestRenderPartialResult(t *testing.T) {
	body, ok := Render(review.ReviewResult{Limitations: []string{"Patch context was omitted."}})
	if !ok {
		t.Fatal("Render() ok = false, want true")
	}
	if !strings.Contains(body, Marker) || !strings.Contains(body, "### Limitations") || strings.Contains(body, "### Findings") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestRenderWithSimplifiedChineseLabels(t *testing.T) {
	risk := 20
	body, ok := RenderWithLanguage(review.ReviewResult{
		Summary:      "整体风险较低。",
		RiskScore:    &risk,
		MissingTests: []string{"补充异常路径测试"},
		Limitations:  []string{"仅分析了 diff 上下文。"},
	}, review.LanguageSimplifiedChinese)
	if !ok {
		t.Fatal("RenderWithLanguage() ok = false, want true")
	}
	for _, want := range []string{"## AI Review 总结", "**风险:** 20/100", "### 缺失的测试", "### 限制", "非阻塞 AI Review"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestPublisherUsesConfiguredLanguage(t *testing.T) {
	fake := &fakeIssueCommenter{}
	pub := NewPublisherWithOptions(fake, PublisherOptions{Language: review.LanguageSimplifiedChinese})
	if err := pub.Publish(context.Background(), 42, "octo", "repo", 7, review.ReviewResult{Summary: "中文总结"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !strings.Contains(fake.createdBody, "## AI Review 总结") || !strings.Contains(fake.createdBody, "中文总结") {
		t.Fatalf("created body did not use Chinese labels: %s", fake.createdBody)
	}
}

func TestPublisherCreatesWhenNoMarkerCommentExists(t *testing.T) {
	fake := &fakeIssueCommenter{}
	pub := NewPublisher(fake)
	if err := pub.Publish(context.Background(), 42, "octo", "repo", 7, review.ReviewResult{Summary: "body"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !strings.Contains(fake.createdBody, "body") || fake.updatedBody != "" || fake.updatedID != 0 {
		t.Fatalf("fake = %+v", fake)
	}
}

func TestPublisherUpdatesExistingMarkerComment(t *testing.T) {
	fake := &fakeIssueCommenter{
		comments: []IssueComment{
			{ID: 10, Body: "human note"},
			{ID: 11, Body: "old\n" + Marker, AuthorType: "Bot"},
		},
	}
	pub := NewPublisher(fake)
	if err := pub.Publish(context.Background(), 42, "octo", "repo", 7, review.ReviewResult{Summary: "new body"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if fake.updatedID != 11 || !strings.Contains(fake.updatedBody, "new body") || fake.createdBody != "" {
		t.Fatalf("fake = %+v", fake)
	}
}

func TestPublisherIgnoresHumanMarkerComment(t *testing.T) {
	fake := &fakeIssueCommenter{
		comments: []IssueComment{
			{ID: 10, Body: "human note\n" + Marker, AuthorType: "User"},
		},
	}
	pub := NewPublisher(fake)
	if err := pub.Publish(context.Background(), 42, "octo", "repo", 7, review.ReviewResult{Summary: "body"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !strings.Contains(fake.createdBody, "body") || fake.updatedBody != "" || fake.updatedID != 0 {
		t.Fatalf("fake = %+v", fake)
	}
}

func TestPublisherIgnoresUnrelatedBotComment(t *testing.T) {
	fake := &fakeIssueCommenter{
		comments: []IssueComment{
			{ID: 10, Body: "human note", AuthorType: "User"},
			{ID: 11, Body: "other bot comment", AuthorType: "Bot"},
		},
	}
	pub := NewPublisher(fake)
	if err := pub.Publish(context.Background(), 42, "octo", "repo", 7, review.ReviewResult{Summary: "body"}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if !strings.Contains(fake.createdBody, "body") || fake.updatedBody != "" || fake.updatedID != 0 {
		t.Fatalf("fake = %+v", fake)
	}
}

func TestPublisherNoOpsEmptyBody(t *testing.T) {
	fake := &fakeIssueCommenter{}
	pub := NewPublisher(fake)
	if err := pub.Publish(context.Background(), 42, "octo", "repo", 7, review.ReviewResult{}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if fake.listed || fake.createdBody != "" || fake.updatedBody != "" {
		t.Fatalf("fake = %+v", fake)
	}
}

func TestPublisherMarksExistingSummaryInactiveForClosedPullRequest(t *testing.T) {
	fake := &fakeIssueCommenter{
		comments: []IssueComment{{ID: 11, Body: "old\n" + Marker, AuthorType: "Bot"}},
	}
	pub := NewPublisher(fake)
	err := pub.Cleanup(context.Background(), review.CleanupJob{
		InstallationID: 42,
		Owner:          "octo",
		Repo:           "repo",
		PullNumber:     7,
		HeadSHA:        "abc123",
		State:          review.CleanupStateClosed,
	})
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if fake.updatedID != 11 || fake.createdBody != "" {
		t.Fatalf("fake = %+v", fake)
	}
	if !strings.Contains(fake.updatedBody, Marker) || !strings.Contains(fake.updatedBody, "inactive because this pull request was closed") || strings.Contains(fake.updatedBody, "old") {
		t.Fatalf("updated body = %q", fake.updatedBody)
	}
}

func TestPublisherMarksExistingSummaryInactiveForMergedPullRequest(t *testing.T) {
	fake := &fakeIssueCommenter{
		comments: []IssueComment{{ID: 11, Body: "old\n" + Marker, AuthorType: "Bot"}},
	}
	pub := NewPublisher(fake)
	err := pub.Cleanup(context.Background(), review.CleanupJob{
		InstallationID: 42,
		Owner:          "octo",
		Repo:           "repo",
		PullNumber:     7,
		HeadSHA:        "abc123",
		State:          review.CleanupStateMerged,
	})
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if fake.updatedID != 11 || fake.createdBody != "" {
		t.Fatalf("fake = %+v", fake)
	}
	if !strings.Contains(fake.updatedBody, "inactive because this pull request was merged") {
		t.Fatalf("updated body = %q", fake.updatedBody)
	}
}

func TestPublisherCleanupMissingMarkerDoesNotCreateComment(t *testing.T) {
	fake := &fakeIssueCommenter{
		comments: []IssueComment{{ID: 11, Body: "other bot comment", AuthorType: "Bot"}},
	}
	pub := NewPublisher(fake)
	err := pub.Cleanup(context.Background(), review.CleanupJob{
		InstallationID: 42,
		Owner:          "octo",
		Repo:           "repo",
		PullNumber:     7,
		HeadSHA:        "abc123",
		State:          review.CleanupStateClosed,
	})
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if fake.createdBody != "" || fake.updatedBody != "" {
		t.Fatalf("fake = %+v, want no create or update", fake)
	}
}

func TestPublisherCleanupIgnoresHumanMarkerComment(t *testing.T) {
	fake := &fakeIssueCommenter{
		comments: []IssueComment{{ID: 11, Body: "human\n" + Marker, AuthorType: "User"}},
	}
	pub := NewPublisher(fake)
	err := pub.Cleanup(context.Background(), review.CleanupJob{
		InstallationID: 42,
		Owner:          "octo",
		Repo:           "repo",
		PullNumber:     7,
		HeadSHA:        "abc123",
		State:          review.CleanupStateClosed,
	})
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if fake.createdBody != "" || fake.updatedBody != "" {
		t.Fatalf("fake = %+v, want no create or update", fake)
	}
}

type fakeIssueCommenter struct {
	comments    []IssueComment
	listed      bool
	createdBody string
	updatedID   int64
	updatedBody string
	listErr     error
}

func (f *fakeIssueCommenter) ListIssueComments(ctx context.Context, installationID int64, owner, repo string, number int) ([]IssueComment, error) {
	f.listed = true
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.comments, nil
}

func (f *fakeIssueCommenter) CreateIssueComment(ctx context.Context, installationID int64, owner, repo string, number int, body string) error {
	f.createdBody = body
	return nil
}

func (f *fakeIssueCommenter) UpdateIssueComment(ctx context.Context, installationID int64, owner, repo string, commentID int64, body string) error {
	f.updatedID = commentID
	f.updatedBody = body
	return nil
}
