package server

import (
	"context"
	"io"
	"net/http"

	"github-ai-reviewer/internal/review"
	"github-ai-reviewer/internal/webhook"
)

type JobSink interface {
	Submit(job review.Job) error
}

type CleanupSink interface {
	SubmitCleanup(job review.CleanupJob) error
}

type PullRequestResolver interface {
	ResolvePullRequestHeadSHA(ctx context.Context, installationID int64, owner, repo string, pullNumber int) (string, error)
}

type PullRequestMetadata = review.PullRequestMetadata

type PullRequestMetadataResolver interface {
	ResolvePullRequestMetadata(ctx context.Context, installationID int64, owner, repo string, pullNumber int) (PullRequestMetadata, error)
}

func New(webhookSecret string, sink JobSink) http.Handler {
	return NewWithResolver(webhookSecret, sink, nil)
}

func NewWithResolver(webhookSecret string, sink JobSink, resolver PullRequestResolver) http.Handler {
	return NewWithCleanup(webhookSecret, sink, resolver, nil)
}

func NewWithCleanup(webhookSecret string, sink JobSink, resolver PullRequestResolver, cleanupSink CleanupSink) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("POST /github/webhook", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
		if err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := webhook.VerifySignature(webhookSecret, body, r.Header.Get("X-Hub-Signature-256")); err != nil {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		result, err := webhook.ParseDelivery(r.Header.Get("X-GitHub-Event"), r.Header.Get("X-GitHub-Delivery"), body)
		if err != nil {
			http.Error(w, "invalid webhook payload", http.StatusBadRequest)
			return
		}
		if result.Ignored {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if result.Cleanup != nil {
			if cleanupSink == nil {
				w.WriteHeader(http.StatusAccepted)
				return
			}
			if err := cleanupSink.SubmitCleanup(*result.Cleanup); err != nil {
				http.Error(w, "cleanup submission failed", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			return
		}
		job := result.Job
		if result.Command != nil {
			if resolver == nil {
				http.Error(w, "pull request resolver unavailable", http.StatusServiceUnavailable)
				return
			}
			metadata, err := resolvePullRequestMetadata(r.Context(), resolver, result.Command)
			if err != nil || metadata.HeadSHA == "" {
				http.Error(w, "pull request metadata unavailable", http.StatusServiceUnavailable)
				return
			}
			if metadata.State == "closed" {
				if cleanupSink != nil {
					if err := cleanupSink.SubmitCleanup(review.CleanupJob{
						InstallationID: result.Command.InstallationID,
						Owner:          result.Command.Owner,
						Repo:           result.Command.Repo,
						PullNumber:     result.Command.PullNumber,
						HeadSHA:        metadata.HeadSHA,
						Action:         result.Command.Action,
						DeliveryID:     result.Command.DeliveryID,
						State:          cleanupState(metadata.Merged),
					}); err != nil {
						http.Error(w, "cleanup submission failed", http.StatusServiceUnavailable)
						return
					}
				}
				w.WriteHeader(http.StatusAccepted)
				return
			}
			job = &review.Job{
				InstallationID: result.Command.InstallationID,
				Owner:          result.Command.Owner,
				Repo:           result.Command.Repo,
				PullNumber:     result.Command.PullNumber,
				HeadSHA:        metadata.HeadSHA,
				Action:         result.Command.Action,
				DeliveryID:     result.Command.DeliveryID,
			}
		}
		if err := sink.Submit(*job); err != nil {
			http.Error(w, "job submission failed", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	return mux
}

func resolvePullRequestMetadata(ctx context.Context, resolver PullRequestResolver, command *webhook.ReviewCommand) (PullRequestMetadata, error) {
	if metadataResolver, ok := resolver.(PullRequestMetadataResolver); ok {
		return metadataResolver.ResolvePullRequestMetadata(ctx, command.InstallationID, command.Owner, command.Repo, command.PullNumber)
	}
	headSHA, err := resolver.ResolvePullRequestHeadSHA(ctx, command.InstallationID, command.Owner, command.Repo, command.PullNumber)
	if err != nil {
		return PullRequestMetadata{}, err
	}
	return PullRequestMetadata{HeadSHA: headSHA, State: "open"}, nil
}

func cleanupState(merged bool) review.CleanupState {
	if merged {
		return review.CleanupStateMerged
	}
	return review.CleanupStateClosed
}
