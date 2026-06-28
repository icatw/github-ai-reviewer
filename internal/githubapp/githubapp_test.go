package githubapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github-ai-reviewer/internal/review"
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

func TestClientFetchesFileContentAndListsDirectoryAtRef(t *testing.T) {
	key := testPrivateKey(t)
	var sawFileRef, sawDirRef bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "installation-token"})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo/contents/pkg/foo.go":
			sawFileRef = r.Header.Get("Authorization") == "Bearer installation-token" && r.URL.Query().Get("ref") == "abc"
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type":     "file",
				"path":     "pkg/foo.go",
				"encoding": "base64",
				"content":  base64.StdEncoding.EncodeToString([]byte("package pkg\n")),
			})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo/contents/docs":
			sawDirRef = r.Header.Get("Authorization") == "Bearer installation-token" && r.URL.Query().Get("ref") == "abc"
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"type": "file", "path": "docs/b.md"},
				{"type": "dir", "path": "docs/nested"},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	client, err := NewClient(123, key, srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	content, err := client.FetchFileContent(context.Background(), 42, "octo", "repo", "abc", "pkg/foo.go")
	if err != nil {
		t.Fatalf("FetchFileContent() error = %v", err)
	}
	entries, err := client.ListDirectory(context.Background(), 42, "octo", "repo", "abc", "docs")
	if err != nil {
		t.Fatalf("ListDirectory() error = %v", err)
	}
	if content != "package pkg\n" {
		t.Fatalf("content = %q", content)
	}
	if len(entries) != 2 || entries[0].Path != "docs/b.md" || entries[0].Type != review.RepositoryEntryFile {
		t.Fatalf("entries = %+v", entries)
	}
	if !sawFileRef || !sawDirRef {
		t.Fatalf("sawFileRef=%v sawDirRef=%v", sawFileRef, sawDirRef)
	}
}

func TestClientListsRootDirectoryAtRef(t *testing.T) {
	key := testPrivateKey(t)
	var sawRoot bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "installation-token"})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo/contents":
			sawRoot = r.URL.Query().Get("ref") == "abc"
			_ = json.NewEncoder(w).Encode([]map[string]any{{"type": "file", "path": "main_test.go"}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	client, err := NewClient(123, key, srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	entries, err := client.ListDirectory(context.Background(), 42, "octo", "repo", "abc", ".")
	if err != nil {
		t.Fatalf("ListDirectory() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Path != "main_test.go" || !sawRoot {
		t.Fatalf("entries=%+v sawRoot=%v", entries, sawRoot)
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

func TestClientCreatesListsAndUpdatesCheckRuns(t *testing.T) {
	key := testPrivateKey(t)
	var sawList, sawCreate, sawUpdate bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "installation-token"})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/repo/commits/abc/check-runs":
			sawList = r.Header.Get("Authorization") == "Bearer installation-token" && r.URL.Query().Get("check_name") == "AI Review"
			_ = json.NewEncoder(w).Encode(map[string]any{"check_runs": []map[string]any{{
				"id":       22,
				"name":     "AI Review",
				"head_sha": "abc",
			}}})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/octo/repo/check-runs":
			sawCreate = r.Header.Get("Authorization") == "Bearer installation-token"
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req["name"] != "AI Review" || req["head_sha"] != "abc" || req["status"] != "in_progress" {
				t.Fatalf("create request = %+v", req)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 23, "name": "AI Review", "head_sha": "abc"})
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/octo/repo/check-runs/23":
			sawUpdate = r.Header.Get("Authorization") == "Bearer installation-token"
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req["status"] != "completed" || req["conclusion"] != "neutral" {
				t.Fatalf("update request = %+v", req)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 23})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	client, err := NewClient(123, key, srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	runs, err := client.ListCheckRuns(context.Background(), 42, "octo", "repo", "abc")
	if err != nil {
		t.Fatalf("ListCheckRuns() error = %v", err)
	}
	if len(runs) != 1 || runs[0].ID != 22 || runs[0].Name != "AI Review" || runs[0].HeadSHA != "abc" {
		t.Fatalf("runs = %+v", runs)
	}
	run, err := client.CreateCheckRun(context.Background(), 42, "octo", "repo", review.CheckRunCreateRequest{
		Name:    "AI Review",
		HeadSHA: "abc",
		Status:  "in_progress",
		Output:  review.CheckRunOutput{Title: "title", Summary: "summary"},
	})
	if err != nil {
		t.Fatalf("CreateCheckRun() error = %v", err)
	}
	if run.ID != 23 || run.Name != "AI Review" || run.HeadSHA != "abc" {
		t.Fatalf("run = %+v", run)
	}
	if err := client.UpdateCheckRun(context.Background(), 42, "octo", "repo", 23, review.CheckRunUpdateRequest{
		Status:     "completed",
		Conclusion: "neutral",
		Output:     review.CheckRunOutput{Title: "title", Summary: "summary"},
	}); err != nil {
		t.Fatalf("UpdateCheckRun() error = %v", err)
	}
	if !sawList || !sawCreate || !sawUpdate {
		t.Fatalf("sawList=%v sawCreate=%v sawUpdate=%v", sawList, sawCreate, sawUpdate)
	}
}

func TestClientCreatesCompletedCheckRunWithConclusion(t *testing.T) {
	key := testPrivateKey(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/app/installations/42/access_tokens":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "installation-token"})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/octo/repo/check-runs":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req["status"] != "completed" || req["conclusion"] != "neutral" {
				t.Fatalf("create request = %+v", req)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 23, "name": "AI Review", "head_sha": "abc"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	client, err := NewClient(123, key, srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.CreateCheckRun(context.Background(), 42, "octo", "repo", review.CheckRunCreateRequest{
		Name:       "AI Review",
		HeadSHA:    "abc",
		Status:     "completed",
		Conclusion: "neutral",
		Output:     review.CheckRunOutput{Title: "title", Summary: "summary"},
	})
	if err != nil {
		t.Fatalf("CreateCheckRun() error = %v", err)
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
