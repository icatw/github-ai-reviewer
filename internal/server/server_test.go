package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

func TestWebhookIssueCommentCommandResolvesHeadSHABeforeSubmittingJob(t *testing.T) {
	sink := &recordingSink{}
	resolver := &recordingResolver{headSHA: "abc123"}
	srv := NewWithResolver("secret", sink, resolver)
	body := []byte(`{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-review"}}`)
	req := signedWebhookRequest("issue_comment", "delivery-command", body)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("code=%d body=%q", rec.Code, rec.Body.String())
	}
	if len(sink.jobs) != 1 {
		t.Fatalf("jobs = %+v, want one job", sink.jobs)
	}
	job := sink.jobs[0]
	if job.InstallationID != 42 || job.Owner != "octo" || job.Repo != "repo" || job.PullNumber != 7 || job.HeadSHA != "abc123" || job.Action != "created" || job.DeliveryID != "delivery-command" {
		t.Fatalf("unexpected job: %+v", job)
	}
	if resolver.calls != 1 || resolver.installationID != 42 || resolver.owner != "octo" || resolver.repo != "repo" || resolver.pullNumber != 7 {
		t.Fatalf("resolver = %+v", resolver)
	}
}

func TestWebhookIssueCommentCommandWithWhitespaceSubmitsJob(t *testing.T) {
	sink := &recordingSink{}
	srv := NewWithResolver("secret", sink, &recordingResolver{headSHA: "abc123"})
	body := []byte(`{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-review now"}}`)
	req := signedWebhookRequest("issue_comment", "delivery-command", body)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted || len(sink.jobs) != 1 {
		t.Fatalf("code=%d jobs=%+v", rec.Code, sink.jobs)
	}
}

func TestWebhookIgnoresIssueCommentNonCommandsPlainIssuesUnsupportedActionsAndUnsupportedEvents(t *testing.T) {
	tests := []struct {
		name  string
		event string
		body  []byte
	}{
		{
			name:  "non-command",
			event: "issue_comment",
			body:  []byte(`{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"please review"}}`),
		},
		{
			name:  "lookalike",
			event: "issue_comment",
			body:  []byte(`{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-reviewer"}}`),
		},
		{
			name:  "plain-issue",
			event: "issue_comment",
			body:  []byte(`{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7},"comment":{"body":"/ai-review"}}`),
		},
		{
			name:  "unsupported-action",
			event: "issue_comment",
			body:  []byte(`{"action":"edited","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-review"}}`),
		},
		{
			name:  "unsupported-event",
			event: "push",
			body:  []byte(`{"ref":"refs/heads/main"}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink := &recordingSink{}
			resolver := &recordingResolver{headSHA: "abc123"}
			srv := NewWithResolver("secret", sink, resolver)
			req := signedWebhookRequest(tt.event, "delivery-ignore", tt.body)
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent || len(sink.jobs) != 0 || resolver.calls != 0 {
				t.Fatalf("code=%d jobs=%+v resolverCalls=%d", rec.Code, sink.jobs, resolver.calls)
			}
		})
	}
}

func TestWebhookIssueCommentCommandMissingFieldsDoesNotSubmitJob(t *testing.T) {
	sink := &recordingSink{}
	resolver := &recordingResolver{headSHA: "abc123"}
	srv := NewWithResolver("secret", sink, resolver)
	body := []byte(`{"action":"created","installation":{"id":42},"repository":{"name":"repo"},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-review"}}`)
	req := signedWebhookRequest("issue_comment", "delivery-command", body)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || len(sink.jobs) != 0 || resolver.calls != 0 {
		t.Fatalf("code=%d jobs=%+v resolverCalls=%d", rec.Code, sink.jobs, resolver.calls)
	}
}

func TestWebhookIssueCommentCommandResolverFailureDoesNotSubmitJob(t *testing.T) {
	sink := &recordingSink{}
	resolver := &recordingResolver{err: errors.New("resolver failed")}
	srv := NewWithResolver("secret", sink, resolver)
	body := []byte(`{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-review"}}`)
	req := signedWebhookRequest("issue_comment", "delivery-command", body)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || len(sink.jobs) != 0 || resolver.calls != 1 {
		t.Fatalf("code=%d jobs=%+v resolverCalls=%d", rec.Code, sink.jobs, resolver.calls)
	}
}

func TestWebhookRejectsInvalidSignatureBeforeSubmittingJob(t *testing.T) {
	sink := &recordingSink{}
	resolver := &recordingResolver{headSHA: "abc123"}
	srv := NewWithResolver("secret", sink, resolver)
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(`{"action":"opened"}`))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", "sha256=bad")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || len(sink.jobs) != 0 || resolver.calls != 0 {
		t.Fatalf("code=%d jobs=%+v resolverCalls=%d", rec.Code, sink.jobs, resolver.calls)
	}
}

func TestWebhookRejectsMissingSignatureBeforeParsingIssueCommentPayload(t *testing.T) {
	sink := &recordingSink{}
	resolver := &recordingResolver{headSHA: "abc123"}
	srv := NewWithResolver("secret", sink, resolver)
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(`{`))
	req.Header.Set("X-GitHub-Event", "issue_comment")
	req.Header.Set("X-GitHub-Delivery", "delivery-bad")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || len(sink.jobs) != 0 || resolver.calls != 0 {
		t.Fatalf("code=%d jobs=%+v resolverCalls=%d", rec.Code, sink.jobs, resolver.calls)
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

func signedWebhookRequest(event, deliveryID string, body []byte) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/github/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-GitHub-Delivery", deliveryID)
	req.Header.Set("X-Hub-Signature-256", testSignature("secret", body))
	return req
}

type recordingSink struct {
	jobs []review.Job
}

func (r *recordingSink) Submit(job review.Job) error {
	r.jobs = append(r.jobs, job)
	return nil
}

type recordingResolver struct {
	headSHA        string
	err            error
	calls          int
	installationID int64
	owner          string
	repo           string
	pullNumber     int
}

func (r *recordingResolver) ResolvePullRequestHeadSHA(ctx context.Context, installationID int64, owner, repo string, pullNumber int) (string, error) {
	r.calls++
	r.installationID = installationID
	r.owner = owner
	r.repo = repo
	r.pullNumber = pullNumber
	return r.headSHA, r.err
}

func testSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
