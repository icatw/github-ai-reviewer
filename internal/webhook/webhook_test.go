package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
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
	payload := []byte(`{"action":"closed","installation":{"id":42},"repository":{"name":"repo","owner":{"login":"octo"}},"pull_request":{"number":7,"head":{"sha":"abc123"}}}`)
	result, err := ParseDelivery("pull_request", "delivery-2", payload)
	if err != nil {
		t.Fatalf("ParseDelivery() error = %v", err)
	}
	if !result.Ignored || result.Job != nil {
		t.Fatalf("result = %+v, want ignored without job", result)
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

func signature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
