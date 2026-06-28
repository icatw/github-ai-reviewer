package githubapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateJWT(t *testing.T) {
	key := testPrivateKey(t)
	auth, err := NewAuth(123, key)
	if err != nil {
		t.Fatalf("NewAuth() error = %v", err)
	}
	token, err := auth.JWT()
	if err != nil {
		t.Fatalf("JWT() error = %v", err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("JWT() = %q, want compact token", token)
	}
}

func TestClientExchangesTokenFetchesFilesAndPublishesComment(t *testing.T) {
	key := testPrivateKey(t)
	var sawJWT, sawToken bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
			sawJWT = strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "installation-token"})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo/pulls/7/files":
			sawToken = r.Header.Get("Authorization") == "Bearer installation-token"
			_ = json.NewEncoder(w).Encode([]map[string]any{{"filename": "main.go", "status": "modified", "additions": 2, "deletions": 1, "patch": "@@ patch"}})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/octo/repo/issues/7/comments":
			sawToken = sawToken && r.Header.Get("Authorization") == "Bearer installation-token"
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":1}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := NewClient(123, key, srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	files, err := client.FetchPullRequestFiles(context.Background(), 42, "octo", "repo", 7)
	if err != nil {
		t.Fatalf("FetchPullRequestFiles() error = %v", err)
	}
	if len(files) != 1 || files[0].Filename != "main.go" || files[0].Patch != "@@ patch" {
		t.Fatalf("files = %+v", files)
	}
	if err := client.CreateIssueComment(context.Background(), 42, "octo", "repo", 7, "body"); err != nil {
		t.Fatalf("CreateIssueComment() error = %v", err)
	}
	if !sawJWT || !sawToken {
		t.Fatalf("sawJWT=%v sawToken=%v", sawJWT, sawToken)
	}
}

func TestClientListsAndUpdatesIssueComments(t *testing.T) {
	key := testPrivateKey(t)
	var sawList, sawPatch bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "installation-token"})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo/issues/7/comments":
			sawList = r.Header.Get("Authorization") == "Bearer installation-token"
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id":   11,
				"body": "<!-- github-ai-reviewer:review-comment:v1 -->\nold",
				"user": map[string]any{"type": "Bot"},
			}})
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/octo/repo/issues/comments/11":
			sawPatch = r.Header.Get("Authorization") == "Bearer installation-token"
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req["body"] != "new body" {
				t.Fatalf("body = %q", req["body"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 11})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	client, err := NewClient(123, key, srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	comments, err := client.ListIssueComments(context.Background(), 42, "octo", "repo", 7)
	if err != nil {
		t.Fatalf("ListIssueComments() error = %v", err)
	}
	if len(comments) != 1 || comments[0].ID != 11 || !strings.Contains(comments[0].Body, "github-ai-reviewer") || comments[0].AuthorType != "Bot" {
		t.Fatalf("comments = %+v", comments)
	}
	if err := client.UpdateIssueComment(context.Background(), 42, "octo", "repo", 11, "new body"); err != nil {
		t.Fatalf("UpdateIssueComment() error = %v", err)
	}
	if !sawList || !sawPatch {
		t.Fatalf("sawList=%v sawPatch=%v", sawList, sawPatch)
	}
}

func testPrivateKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
}
