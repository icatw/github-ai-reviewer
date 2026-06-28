package server

import (
	"io"
	"net/http"

	"github-ai-reviewer/internal/review"
	"github-ai-reviewer/internal/webhook"
)

type JobSink interface {
	Submit(job review.Job) error
}

func New(webhookSecret string, sink JobSink) http.Handler {
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
		if err := sink.Submit(*result.Job); err != nil {
			http.Error(w, "job submission failed", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	return mux
}
