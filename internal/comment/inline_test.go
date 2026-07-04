package comment

import (
	"context"
	"strings"
	"testing"

	"github-ai-reviewer/internal/review"
)

func TestDiffRightLinesMapsAddedAndContextLines(t *testing.T) {
	patch := `@@ -10,4 +20,5 @@ func main() {
 context
-old
+new
 another
+added
\ No newline at end of file`
	lines := diffRightLines(patch)
	for _, want := range []int{20, 21, 22, 23} {
		if _, ok := lines[want]; !ok {
			t.Fatalf("line %d missing from %#v", want, lines)
		}
	}
	if _, ok := lines[24]; ok {
		t.Fatalf("unexpected line 24 in %#v", lines)
	}
}

func TestPublisherCreatesInlineCommentForDiffFinding(t *testing.T) {
	line := 21
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{
			Filename: "main.go",
			Patch:    "@@ -1,2 +20,3 @@\n context\n+badCall()\n more\n",
		}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{
		Summary: "summary",
		Findings: []review.Finding{{
			Severity:        "warning",
			File:            "main.go",
			Line:            &line,
			Title:           "Bad call",
			Evidence:        "badCall is introduced",
			FailureScenario: "runtime failure",
			Suggestion:      "guard it",
		}},
	})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if fake.createdReview.Path != "main.go" || fake.createdReview.Line != 21 || fake.createdReview.Side != "RIGHT" || fake.createdReview.CommitID != "abc123" {
		t.Fatalf("created review = %+v", fake.createdReview)
	}
	if !strings.Contains(fake.createdReview.Body, InlineMarker) || !strings.Contains(fake.createdReview.Body, "Bad call") {
		t.Fatalf("created body = %q", fake.createdReview.Body)
	}
	if fake.createdBody == "" {
		t.Fatal("summary issue comment was not created")
	}
}

func TestPublisherSkipsLowValueInlineFindings(t *testing.T) {
	line := 20
	lowConfidence := 0.69
	tests := []struct {
		name    string
		finding review.Finding
	}{
		{
			name: "question severity",
			finding: review.Finding{
				Severity:        "question",
				File:            "main.go",
				Line:            &line,
				Title:           "Question",
				Evidence:        "badCall()",
				FailureScenario: "unclear failure",
				Suggestion:      "check intent",
			},
		},
		{
			name: "low confidence",
			finding: review.Finding{
				Severity:        "warning",
				File:            "main.go",
				Line:            &line,
				Title:           "Low confidence",
				Evidence:        "badCall()",
				FailureScenario: "runtime failure",
				Suggestion:      "guard it",
				Confidence:      &lowConfidence,
			},
		},
		{
			name: "missing failure scenario",
			finding: review.Finding{
				Severity:   "warning",
				File:       "main.go",
				Line:       &line,
				Title:      "Incomplete",
				Evidence:   "badCall()",
				Suggestion: "guard it",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeInlineCommenter{files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+badCall()\n"}}}
			pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
			err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{Summary: "summary", Findings: []review.Finding{tt.finding}})
			if err != nil {
				t.Fatalf("PublishForHead() error = %v", err)
			}
			if fake.createdBody == "" {
				t.Fatal("summary issue comment was not created")
			}
			if fake.createdReview.Body != "" || fake.updatedReviewBody != "" {
				t.Fatalf("unexpected inline output: created=%+v updated=%q", fake.createdReview, fake.updatedReviewBody)
			}
		})
	}
}

func TestPublisherCreatesInlineCommentForHighConfidenceFinding(t *testing.T) {
	line := 20
	confidence := 0.70
	fake := &fakeInlineCommenter{files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+badCall()\n"}}}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{Summary: "summary", Findings: []review.Finding{{
		Severity:        "blocker",
		File:            "main.go",
		Line:            &line,
		Title:           "High confidence",
		Evidence:        "badCall()",
		FailureScenario: "runtime failure",
		Suggestion:      "guard it",
		Confidence:      &confidence,
	}}})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if fake.createdReview.Body == "" {
		t.Fatal("expected inline review comment")
	}
}

func TestPublisherSkipsInlineCommentOutsideDiff(t *testing.T) {
	line := 99
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+badCall()\n"}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{
		Summary: "summary",
		Findings: []review.Finding{{
			Severity:        "warning",
			File:            "main.go",
			Line:            &line,
			Title:           "Bad call",
			Evidence:        "evidence",
			FailureScenario: "failure",
			Suggestion:      "fix",
		}},
	})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if fake.createdReview.Body != "" || fake.updatedReviewBody != "" {
		t.Fatalf("unexpected inline output: created=%+v updated=%q", fake.createdReview, fake.updatedReviewBody)
	}
}

func TestPublisherUpdatesExistingInlineComment(t *testing.T) {
	line := 20
	finding := review.Finding{
		Severity:        "warning",
		File:            "main.go",
		Line:            &line,
		Title:           "Bad call",
		Evidence:        "evidence",
		FailureScenario: "failure",
		Suggestion:      "fix",
	}
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+badCall()\n"}},
		reviewComments: []ReviewComment{{
			ID:         55,
			AuthorType: "Bot",
			Body:       InlineMarker + " fingerprint=" + inlineFingerprint(finding) + " -->\nold",
		}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{Summary: "summary", Findings: []review.Finding{finding}})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if fake.updatedReviewID != 55 || !strings.Contains(fake.updatedReviewBody, "Bad call") {
		t.Fatalf("updated id/body = %d %q", fake.updatedReviewID, fake.updatedReviewBody)
	}
	if fake.createdReview.Body != "" {
		t.Fatalf("created duplicate inline review: %+v", fake.createdReview)
	}
}

type fakeInlineCommenter struct {
	fakeIssueCommenter
	files             []review.FileChange
	reviewComments    []ReviewComment
	createdReview     ReviewCommentRequest
	updatedReviewID   int64
	updatedReviewBody string
}

func (f *fakeInlineCommenter) FetchPullRequestFiles(context.Context, int64, string, string, int) ([]review.FileChange, error) {
	return f.files, nil
}

func (f *fakeInlineCommenter) ListPullRequestReviewComments(context.Context, int64, string, string, int) ([]ReviewComment, error) {
	return f.reviewComments, nil
}

func (f *fakeInlineCommenter) CreatePullRequestReviewComment(ctx context.Context, installationID int64, owner, repo string, pullNumber int, req ReviewCommentRequest) error {
	f.createdReview = req
	return nil
}

func (f *fakeInlineCommenter) UpdatePullRequestReviewComment(ctx context.Context, installationID int64, owner, repo string, commentID int64, body string) error {
	f.updatedReviewID = commentID
	f.updatedReviewBody = body
	return nil
}
