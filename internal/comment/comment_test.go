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

type fakeIssueCommenter struct {
	comments    []IssueComment
	listed      bool
	createdBody string
	updatedID   int64
	updatedBody string
}

func (f *fakeIssueCommenter) ListIssueComments(ctx context.Context, installationID int64, owner, repo string, number int) ([]IssueComment, error) {
	f.listed = true
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
