package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github-ai-reviewer/internal/review"
)

func TestHealthz(t *testing.T) {
	srv := New("secret", &recordingSink{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestWebhookAcceptedSubmitsJobAndReturns202(t *testing.T) {
	sink := &recordingSink{}
	srv := New("secret", sink)
	body := []byte(`{"action":"opened","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"pull_request":{"number":7,"head":{"sha":"abc123"}}}`)
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "delivery-1")
	req.Header.Set("X-Hub-Signature-256", testSignature("secret", body))
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
	if len(sink.jobs) != 1 || sink.jobs[0].Owner != "octo" {
		t.Fatalf("jobs = %+v", sink.jobs)
	}
}

func TestWebhookRejectsInvalidSignatureBeforeSubmittingJob(t *testing.T) {
	sink := &recordingSink{}
	srv := New("secret", sink)
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(`{"action":"opened"}`))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", "sha256=bad")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || len(sink.jobs) != 0 {
		t.Fatalf("code=%d jobs=%+v", rec.Code, sink.jobs)
	}
}

func TestWebhookIgnoresUnsupportedEvent(t *testing.T) {
	sink := &recordingSink{}
	srv := New("secret", sink)
	body := []byte(`{"zen":"keep it logically awesome"}`)
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-GitHub-Event", "ping")
	req.Header.Set("X-Hub-Signature-256", testSignature("secret", body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || len(sink.jobs) != 0 {
		t.Fatalf("code=%d jobs=%+v", rec.Code, sink.jobs)
	}
}

type recordingSink struct {
	jobs []review.Job
}

func (r *recordingSink) Submit(job review.Job) error {
	r.jobs = append(r.jobs, job)
	return nil
}

func testSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
