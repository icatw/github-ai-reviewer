package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github-ai-reviewer/internal/review"
)

func TestBuildPromptBoundsPatchContext(t *testing.T) {
	files := []review.FileChange{{
		Filename:  "main.go",
		Status:    "modified",
		Additions: 1,
		Deletions: 1,
		Patch:     strings.Repeat("x", 80),
	}}
	prompt := BuildPrompt(review.Job{Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc"}, files, 40)
	if !strings.Contains(prompt, "octo/repo#7") || !strings.Contains(prompt, "Some patch context was omitted") {
		t.Fatalf("prompt did not include expected metadata/omission: %s", prompt)
	}
	for _, want := range []string{"JSON-only", `"summary"`, `"risk_score"`, `"findings"`, "advisory and non-blocking"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, strings.Repeat("x", 80)) {
		t.Fatal("prompt contains unbounded patch")
	}
}

func TestClientReviewSendsOpenAICompatibleRequestAndParsesStructuredResult(t *testing.T) {
	var gotAuth, gotModel, gotSystem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		gotModel = req.Model
		gotSystem = req.Messages[0].Content
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary\":\"Looks focused.\",\"risk_score\":10}"}}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL+"/v1", "key", "model-a", srv.Client())
	got, err := client.Review(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if got.Summary != "Looks focused." || got.RiskScore == nil || *got.RiskScore != 10 || gotAuth != "Bearer key" || gotModel != "model-a" {
		t.Fatalf("got result=%+v auth=%q model=%q", got, gotAuth, gotModel)
	}
	if !strings.Contains(gotSystem, "JSON") || !strings.Contains(gotSystem, "non-blocking") {
		t.Fatalf("system prompt = %q", gotSystem)
	}
}

func TestClientReviewChineseOptionAddsLanguageInstruction(t *testing.T) {
	var gotSystem string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		gotSystem = req.Messages[0].Content
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary\":\"整体风险较低。\"}"}}]}`))
	}))
	defer srv.Close()

	client := NewClientWithOptions(srv.URL, "key", "model-a", srv.Client(), ClientOptions{Language: review.LanguageSimplifiedChinese})
	got, err := client.Review(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if got.Summary != "整体风险较低。" || !strings.Contains(gotSystem, "Simplified Chinese") {
		t.Fatalf("result=%+v system prompt=%q", got, gotSystem)
	}
}

func TestClientReviewRejectsMalformedOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not json"}}]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key", "model-a", srv.Client())
	_, err := client.Review(context.Background(), "prompt")
	if !errors.Is(err, review.ErrMalformedResult) {
		t.Fatalf("Review() error = %v, want ErrMalformedResult", err)
	}
}

func TestClientReviewRejectsEmptyChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key", "model-a", srv.Client())
	_, err := client.Review(context.Background(), "prompt")
	if err == nil {
		t.Fatal("Review() error = nil")
	}
}
