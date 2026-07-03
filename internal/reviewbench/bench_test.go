package reviewbench

import (
	"context"
	"math"
	"reflect"
	"testing"

	"github-ai-reviewer/internal/review"
)

func TestRunReportsContextPrecisionRecall(t *testing.T) {
	fixture := Fixture{
		Name:  "cross-package-auth-change",
		Job:   review.Job{Owner: "octo", Repo: "repo", PullNumber: 9, HeadSHA: "abc"},
		Files: []review.FileChange{{Filename: "handler/user.go", Status: "modified", Patch: "@@ handler"}},
		RepoFiles: map[string]string{
			"go.mod":                "module example.com/repo\n",
			"handler/user.go":       "package handler\n\nimport \"example.com/repo/service\"\n\nfunc Handle() { service.RequireAuth() }\n",
			"handler/user_test.go":  "package handler\nfunc TestHandle() {}\n",
			"handler/route.go":      "package handler\nfunc Route() {}\n",
			"service/auth.go":       "package service\nfunc RequireAuth() {}\n",
			"service/profile.go":    "package service\nfunc Profile() {}\n",
			"README.md":             "# repo\n",
			"docs/security.md":      "security model\n",
			".github/ai-review.yml": "language: zh-CN\n",
		},
		GoldenRelevantFiles: []string{"handler/user.go", "handler/user_test.go", "service/auth.go"},
	}

	report, err := Run(context.Background(), fixture)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !reflect.DeepEqual(report.Context.FullFiles, []string{"handler/user.go"}) {
		t.Fatalf("full files = %#v", report.Context.FullFiles)
	}
	if !contains(report.Context.RelatedSources, "service/auth.go") {
		t.Fatalf("related sources missing service/auth.go: %#v", report.Context.RelatedSources)
	}
	if !contains(report.Context.RelatedTests, "handler/user_test.go") {
		t.Fatalf("related tests missing handler/user_test.go: %#v", report.Context.RelatedTests)
	}
	if report.Metrics.TruePositive != 3 || report.Metrics.FalseNegative != 0 {
		t.Fatalf("unexpected metrics: %+v", report.Metrics)
	}
	if report.Metrics.FalsePositive == 0 {
		t.Fatalf("expected docs and extra sources to count as precision noise: %+v", report.Metrics)
	}
	if report.SourceMetrics.Precision != 1 || report.SourceMetrics.Recall != 1 {
		t.Fatalf("source metrics = %+v", report.SourceMetrics)
	}
	if !reflect.DeepEqual(report.Context.PolicyFiles, []string{".github/ai-review.yml"}) {
		t.Fatalf("policy files = %#v", report.Context.PolicyFiles)
	}
	if math.Abs(report.Metrics.Recall-1) > 0.0001 {
		t.Fatalf("recall = %v", report.Metrics.Recall)
	}
}

func TestRunReportsPythonContextPrecisionRecall(t *testing.T) {
	fixture := Fixture{
		Name:  "python-fastapi-user-change",
		Job:   review.Job{Owner: "octo", Repo: "repo", PullNumber: 10, HeadSHA: "abc"},
		Files: []review.FileChange{{Filename: "app/api/user.py", Status: "modified", Patch: "@@ route"}},
		RepoFiles: map[string]string{
			"app/api/user.py":       "from app.services.user import create_user\nfrom .schemas import UserRequest\n\ndef route(): pass\n",
			"app/api/schemas.py":    "class UserRequest: pass\n",
			"app/api/test_user.py":  "def test_route(): pass\n",
			"app/services/user.py":  "def create_user(): pass\n",
			"app/services/audit.py": "def audit(): pass\n",
			"tests/test_user.py":    "def test_user_flow(): pass\n",
			"README.md":             "# repo\n",
		},
		GoldenRelevantFiles: []string{"app/api/user.py", "app/api/schemas.py", "app/api/test_user.py", "app/services/user.py", "tests/test_user.py"},
	}

	report, err := Run(context.Background(), fixture)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{"app/api/schemas.py", "app/services/user.py"} {
		if !contains(report.Context.RelatedSources, want) {
			t.Fatalf("related sources missing %s: %#v", want, report.Context.RelatedSources)
		}
	}
	for _, want := range []string{"app/api/test_user.py", "tests/test_user.py"} {
		if !contains(report.Context.RelatedTests, want) {
			t.Fatalf("related tests missing %s: %#v", want, report.Context.RelatedTests)
		}
	}
	if report.Metrics.TruePositive != 5 || report.Metrics.FalseNegative != 0 {
		t.Fatalf("unexpected metrics: %+v", report.Metrics)
	}
	if report.SourceMetrics.Precision != 1 || report.SourceMetrics.Recall != 1 {
		t.Fatalf("source metrics = %+v", report.SourceMetrics)
	}
	if math.Abs(report.Metrics.Recall-1) > 0.0001 {
		t.Fatalf("recall = %v", report.Metrics.Recall)
	}
}

func TestDecodeFixtureRejectsEmptyFiles(t *testing.T) {
	_, err := DecodeFixture([]byte(`{"name":"bad","repo_files":{}}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
