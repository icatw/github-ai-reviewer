package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strconv"
	"testing"

	"github-ai-reviewer/internal/review"
)

func TestVerifySignatureAcceptsValidSignature(t *testing.T) {
	body := []byte(`{"ok":true}`)
	if err := VerifySignature("secret", body, signature("secret", body)); err != nil {
		t.Fatalf("VerifySignature() error = %v", err)
	}
}

func TestVerifySignatureRejectsInvalidSignature(t *testing.T) {
	body := []byte(`{"ok":true}`)
	if err := VerifySignature("secret", body, signature("wrong", body)); err == nil {
		t.Fatal("VerifySignature() error = nil, want mismatch")
	}
}

func TestVerifySignatureRejectsMissingOrMalformedSignature(t *testing.T) {
	body := []byte(`{"ok":true}`)
	for _, sig := range []string{"", "sha1=abc", "sha256=nothex"} {
		if err := VerifySignature("secret", body, sig); err == nil {
			t.Fatalf("VerifySignature(%q) error = nil", sig)
		}
	}
}

func TestParseDeliveryIgnoresUnsupportedEvent(t *testing.T) {
	result, err := ParseDelivery("push", "delivery-1", []byte(`{"ignored":true}`))
	if err != nil {
		t.Fatalf("ParseDelivery() error = %v", err)
	}
	if !result.Ignored || result.Job != nil {
		t.Fatalf("result = %+v, want ignored without job", result)
	}
}

func TestParseDeliveryIgnoresUnsupportedPullRequestAction(t *testing.T) {
	payload := []byte(`{"action":"labeled","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"pull_request":{"number":7,"head":{"sha":"abc123"},"merged":false}}`)
	result, err := ParseDelivery("pull_request", "delivery-2", payload)
	if err != nil {
		t.Fatalf("ParseDelivery() error = %v", err)
	}
	if !result.Ignored || result.Job != nil || result.Cleanup != nil {
		t.Fatalf("result = %+v, want ignored without job", result)
	}
}

func TestParseDeliveryExtractsExactIssueCommentCommand(t *testing.T) {
	payload := []byte(`{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-review"}}`)
	result, err := ParseDelivery("issue_comment", "delivery-command", payload)
	if err != nil {
		t.Fatalf("ParseDelivery() error = %v", err)
	}
	if result.Ignored || result.Command == nil || result.Job != nil {
		t.Fatalf("result = %+v, want command without job", result)
	}
	command := *result.Command
	if command.InstallationID != 42 || command.Owner != "octo" || command.Repo != "repo" || command.PullNumber != 7 || command.Action != "created" || command.DeliveryID != "delivery-command" {
		t.Fatalf("unexpected command: %+v", command)
	}
}

func TestParseDeliveryExtractsIssueCommentCommandWithWhitespace(t *testing.T) {
	payload := []byte(`{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-review please"}}`)
	result, err := ParseDelivery("issue_comment", "delivery-command", payload)
	if err != nil {
		t.Fatalf("ParseDelivery() error = %v", err)
	}
	if result.Ignored || result.Command == nil {
		t.Fatalf("result = %+v, want command", result)
	}
}

func TestParseDeliveryIgnoresIssueCommentNonCommandsPlainIssuesAndUnsupportedActions(t *testing.T) {
	tests := []struct {
		name    string
		payload string
	}{
		{
			name:    "non-command",
			payload: `{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"please review"}}`,
		},
		{
			name:    "lookalike",
			payload: `{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-reviewer"}}`,
		},
		{
			name:    "plain-issue",
			payload: `{"action":"created","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7},"comment":{"body":"/ai-review"}}`,
		},
		{
			name:    "unsupported-action",
			payload: `{"action":"edited","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-review"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseDelivery("issue_comment", "delivery-ignore", []byte(tt.payload))
			if err != nil {
				t.Fatalf("ParseDelivery() error = %v", err)
			}
			if !result.Ignored || result.Job != nil || result.Command != nil {
				t.Fatalf("result = %+v, want ignored without job or command", result)
			}
		})
	}
}

func TestParseDeliveryRejectsIssueCommentMissingFields(t *testing.T) {
	payload := []byte(`{"action":"created","installation":{"id":42},"repository":{"name":"repo"},"issue":{"number":7,"pull_request":{"url":"https://api.github.com/repos/octo/repo/pulls/7"}},"comment":{"body":"/ai-review"}}`)
	if _, err := ParseDelivery("issue_comment", "delivery-missing", payload); err == nil {
		t.Fatal("ParseDelivery() error = nil, want missing field error")
	}
}

func TestParseDeliveryRejectsMissingFields(t *testing.T) {
	payload := []byte(`{"action":"opened","installation":{"id":42},"repository":{"name":"repo"},"pull_request":{"number":7,"head":{"sha":"abc123"}}}`)
	if _, err := ParseDelivery("pull_request", "delivery-3", payload); err == nil {
		t.Fatal("ParseDelivery() error = nil, want missing field error")
	}
}

func TestParseDeliveryExtractsSupportedJob(t *testing.T) {
	payload, err := os.ReadFile("testdata/pull_request_opened.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := ParseDelivery("pull_request", "delivery-4", payload)
	if err != nil {
		t.Fatalf("ParseDelivery() error = %v", err)
	}
	if result.Ignored || result.Job == nil {
		t.Fatalf("result = %+v, want job", result)
	}
	job := *result.Job
	if job.InstallationID != 42 || job.Owner != "octo" || job.Repo != "repo" || job.PullNumber != 7 || job.HeadSHA != "abc123" || job.Action != "opened" || job.DeliveryID != "delivery-4" {
		t.Fatalf("unexpected job: %+v", job)
	}
}

func TestParseDeliveryExtractsClosedPullRequestCleanup(t *testing.T) {
	tests := []struct {
		name   string
		merged bool
		state  review.CleanupState
	}{
		{name: "closed unmerged", merged: false, state: review.CleanupStateClosed},
		{name: "merged", merged: true, state: review.CleanupStateMerged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := []byte(`{"action":"closed","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"pull_request":{"number":7,"head":{"sha":"abc123"},"merged":` + strconv.FormatBool(tt.merged) + `}}`)
			result, err := ParseDelivery("pull_request", "delivery-close", payload)
			if err != nil {
				t.Fatalf("ParseDelivery() error = %v", err)
			}
			if result.Ignored || result.Job != nil || result.Cleanup == nil {
				t.Fatalf("result = %+v, want cleanup without review job", result)
			}
			cleanup := *result.Cleanup
			if cleanup.InstallationID != 42 || cleanup.Owner != "octo" || cleanup.Repo != "repo" || cleanup.PullNumber != 7 || cleanup.HeadSHA != "abc123" || cleanup.Action != "closed" || cleanup.DeliveryID != "delivery-close" || cleanup.State != tt.state {
				t.Fatalf("unexpected cleanup: %+v", cleanup)
			}
		})
	}
}

func TestParseDeliveryRejectsClosedPullRequestMissingMergedState(t *testing.T) {
	payload := []byte(`{"action":"closed","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"pull_request":{"number":7,"head":{"sha":"abc123"}}}`)
	if _, err := ParseDelivery("pull_request", "delivery-close", payload); err == nil {
		t.Fatal("ParseDelivery() error = nil, want missing merged field error")
	}
}

func signature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
