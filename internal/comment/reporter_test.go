package comment

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github-ai-reviewer/internal/review"
)

func TestReporterPreservesMarkerCommentUpdate(t *testing.T) {
	fake := &fakeIssueCommenter{
		comments: []IssueComment{
			{ID: 10, Body: "human note", AuthorType: "User"},
			{ID: 11, Body: "old\n" + Marker, AuthorType: "Bot"},
			{ID: 12, Body: "unrelated bot", AuthorType: "Bot"},
		},
	}
	reporter := NewReporter(NewPublisher(fake))

	err := reporter.ReviewCompleted(context.Background(), review.Job{
		InstallationID: 42,
		Owner:          "octo",
		Repo:           "repo",
		PullNumber:     7,
	}, review.ReviewResult{Summary: "new body"})
	if err != nil {
		t.Fatalf("ReviewCompleted() error = %v", err)
	}
	if fake.updatedID != 11 || !strings.Contains(fake.updatedBody, "new body") || fake.createdBody != "" {
		t.Fatalf("fake = %+v", fake)
	}
}

func TestReporterSuppressesEmptyOutputWithoutListingComments(t *testing.T) {
	fake := &fakeIssueCommenter{}
	reporter := NewReporter(NewPublisher(fake))

	if err := reporter.OutputSuppressed(context.Background(), review.Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7}, review.ReviewResult{}); err != nil {
		t.Fatalf("OutputSuppressed() error = %v", err)
	}
	if fake.listed || fake.createdBody != "" || fake.updatedBody != "" {
		t.Fatalf("fake = %+v", fake)
	}
}

func TestReporterPublishFailureDoesNotCreateFallbackComment(t *testing.T) {
	fake := &fakeIssueCommenter{listErr: errors.New("github token should not be copied")}
	reporter := NewReporter(NewPublisher(fake))

	err := reporter.ReviewCompleted(context.Background(), review.Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7}, review.ReviewResult{Summary: "body"})
	if err == nil {
		t.Fatal("ReviewCompleted() error = nil")
	}
	if fake.createdBody != "" || fake.updatedBody != "" {
		t.Fatalf("reporter created fallback output after failure: %+v", fake)
	}
}
