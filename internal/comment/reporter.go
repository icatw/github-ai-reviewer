package comment

import (
	"context"

	"github-ai-reviewer/internal/review"
)

type Reporter struct {
	publisher *Publisher
}

func NewReporter(publisher *Publisher) *Reporter {
	return &Reporter{publisher: publisher}
}

func (r *Reporter) Name() string { return "pr_summary_comment" }

func (r *Reporter) JobStarted(ctx context.Context, job review.Job) error {
	return nil
}

func (r *Reporter) ReviewCompleted(ctx context.Context, job review.Job, result review.ReviewResult) error {
	if r.publisher == nil {
		return nil
	}
	return r.publisher.PublishForHead(ctx, job.InstallationID, job.Owner, job.Repo, job.PullNumber, job.HeadSHA, result)
}

func (r *Reporter) OutputSuppressed(ctx context.Context, job review.Job, result review.ReviewResult) error {
	return nil
}

func (r *Reporter) JobFailed(ctx context.Context, job review.Job, failure review.Failure) error {
	return nil
}
