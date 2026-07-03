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

type PullRequestResolver interface {
	ResolvePullRequestHeadSHA(ctx context.Context, installationID int64, owner, repo string, pullNumber int) (string, error)
}

func New(webhookSecret string, sink JobSink) http.Handler {
	return NewWithResolver(webhookSecret, sink, nil)
}

func NewWithResolver(webhookSecret string, sink JobSink, resolver PullRequestResolver) http.Handler {
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
		job := result.Job
		if result.Command != nil {
			if resolver == nil {
				http.Error(w, "pull request resolver unavailable", http.StatusServiceUnavailable)
				return
			}
			headSHA, err := resolver.ResolvePullRequestHeadSHA(r.Context(), result.Command.InstallationID, result.Command.Owner, result.Command.Repo, result.Command.PullNumber)
			if err != nil || headSHA == "" {
				http.Error(w, "pull request metadata unavailable", http.StatusServiceUnavailable)
				return
			}
			job = &review.Job{
				InstallationID: result.Command.InstallationID,
				Owner:          result.Command.Owner,
				Repo:           result.Command.Repo,
				PullNumber:     result.Command.PullNumber,
				HeadSHA:        headSHA,
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
