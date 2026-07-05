package reviewbench

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"strings"
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

func TestRunSuiteAggregatesMetrics(t *testing.T) {
	fixtures := []Fixture{
		{
			Name:  "go-change",
			Job:   review.Job{Owner: "octo", Repo: "repo", PullNumber: 1, HeadSHA: "abc"},
			Files: []review.FileChange{{Filename: "handler/user.go", Status: "modified", Patch: "@@ handler"}},
			RepoFiles: map[string]string{
				"handler/user.go":      "package handler\nfunc Handle() {}\n",
				"handler/user_test.go": "package handler\nfunc TestHandle() {}\n",
			},
			GoldenRelevantFiles: []string{"handler/user.go", "handler/user_test.go"},
		},
		{
			Name:  "python-change",
			Job:   review.Job{Owner: "octo", Repo: "repo", PullNumber: 2, HeadSHA: "def"},
			Files: []review.FileChange{{Filename: "app/api/user.py", Status: "modified", Patch: "@@ route"}},
			RepoFiles: map[string]string{
				"app/api/user.py":      "def route(): pass\n",
				"app/api/test_user.py": "def test_route(): pass\n",
			},
			GoldenRelevantFiles: []string{"app/api/user.py", "app/api/test_user.py"},
		},
	}

	report, err := RunSuite(context.Background(), fixtures)
	if err != nil {
		t.Fatalf("RunSuite() error = %v", err)
	}
	if report.FixtureCount != 2 || len(report.Cases) != 2 {
		t.Fatalf("unexpected suite shape: %+v", report)
	}
	if report.Metrics.RelevantTotal != report.Cases[0].Metrics.RelevantTotal+report.Cases[1].Metrics.RelevantTotal {
		t.Fatalf("relevant total was not aggregated: %+v", report.Metrics)
	}
	if report.Metrics.TruePositive != report.Cases[0].Metrics.TruePositive+report.Cases[1].Metrics.TruePositive {
		t.Fatalf("true positives were not aggregated: %+v", report.Metrics)
	}
	if report.SourceMetrics.FalseNegative != report.Cases[0].SourceMetrics.FalseNegative+report.Cases[1].SourceMetrics.FalseNegative {
		t.Fatalf("source false negatives were not aggregated: %+v", report.SourceMetrics)
	}
}

func TestDecodeFixtureRejectsEmptyFiles(t *testing.T) {
	_, err := DecodeFixture([]byte(`{"name":"bad","repo_files":{}}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeFixtureSupportsFindingAnnotationsAndMetadata(t *testing.T) {
	fixture, err := DecodeFixture([]byte(`{
		"name": "annotated",
		"metadata": {
			"source": "sanitized-real-pr",
			"provenance": "octo/repo#12",
			"sanitized": true,
			"notes": "public fixture"
		},
		"files": [{"Filename":"app/auth.go","Status":"modified","Patch":"@@"}],
		"repo_files": {"app/auth.go":"package app\n"},
		"golden_relevant_files": ["app/auth.go"],
		"expected_findings": [{
			"id": "auth-required",
			"file": "app/auth.go",
			"line": 42,
			"line_end": 45,
			"title": "auth check is skipped",
			"category": "security",
			"severity": "warning",
			"evidence_hints": ["RequireAuth", "docs/security.md"],
			"matching_hints": ["missing auth"]
		}],
		"actual_findings": [{
			"id": "actual-auth",
			"file": "app/auth.go",
			"line": 43,
			"title": "handler skips auth check",
			"category": "security",
			"severity": "warning",
			"evidence_hints": ["RequireAuth"]
		}]
	}`))
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if !fixture.Metadata.Sanitized || fixture.Metadata.Provenance != "octo/repo#12" {
		t.Fatalf("metadata = %+v", fixture.Metadata)
	}
	if got := fixture.ExpectedFindings[0].ID; got != "auth-required" {
		t.Fatalf("expected finding id = %q", got)
	}
	if got := fixture.ActualFindings[0].EvidenceHints; !reflect.DeepEqual(got, []string{"RequireAuth"}) {
		t.Fatalf("actual evidence hints = %#v", got)
	}
}

func TestDecodeFixtureSupportsExpectedNoFindings(t *testing.T) {
	fixture, err := DecodeFixture([]byte(`{
		"name": "clean",
		"files": [{"Filename":"app/user.go","Status":"modified","Patch":"@@"}],
		"repo_files": {"app/user.go":"package app\n"},
		"expected_no_findings": true,
		"actual_findings": [{
			"id": "style-note",
			"file": "app/user.go",
			"line": 7,
			"title": "rename this helper",
			"category": "style",
			"severity": "note"
		}]
	}`))
	if err != nil {
		t.Fatalf("DecodeFixture() error = %v", err)
	}
	if !fixture.ExpectedNoFindings {
		t.Fatal("expected no-finding flag to decode")
	}
}

func TestDecodeFixtureRejectsExpectedNoFindingsWithExpectedFindings(t *testing.T) {
	_, err := DecodeFixture([]byte(`{
		"name": "conflict",
		"files": [{"Filename":"app/user.go","Status":"modified","Patch":"@@"}],
		"repo_files": {"app/user.go":"package app\n"},
		"expected_no_findings": true,
		"expected_findings": [{"id":"bug","file":"app/user.go","title":"bug"}]
	}`))
	if err == nil {
		t.Fatal("expected conflicting annotations error")
	}
}

func TestRunReportsFindingQualityCategories(t *testing.T) {
	fixture := Fixture{
		Name: "quality",
		Files: []review.FileChange{{
			Filename: "app/auth.go",
			Status:   "modified",
			Patch:    "@@ auth",
		}},
		RepoFiles:           map[string]string{"app/auth.go": "package app\n"},
		GoldenRelevantFiles: []string{"app/auth.go"},
		ExpectedFindings: []ExpectedFinding{
			{ID: "auth-required", File: "app/auth.go", Line: 40, LineEnd: 45, Title: "missing auth check", Category: "security", Severity: "warning", EvidenceHints: []string{"RequireAuth"}},
			{ID: "audit-log", File: "app/audit.go", Line: 8, Title: "missing audit log", Category: "audit", Severity: "note"},
		},
		ActualFindings: []ActualFinding{
			{ID: "actual-auth", File: "app/auth.go", Line: 42, Title: "auth check is missing", Category: "security", Severity: "warning", EvidenceHints: []string{"RequireAuth"}},
			{ID: "actual-dup", File: "app/auth.go", Line: 43, Title: "missing auth check again", Category: "security", Severity: "warning", QualityLabels: []string{"duplicate"}, DuplicateOf: "auth-required"},
			{ID: "actual-low", File: "app/auth.go", Line: 50, Title: "minor naming nit", Category: "style", Severity: "note", QualityLabels: []string{"style-only"}},
			{ID: "actual-unexpected", File: "app/other.go", Line: 3, Title: "other issue", Category: "correctness", Severity: "warning"},
		},
	}

	report, err := Run(context.Background(), fixture)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	quality := report.FindingQuality
	if quality.Status != FindingQualityAnnotated {
		t.Fatalf("quality status = %q", quality.Status)
	}
	if quality.ExpectedCount != 2 || quality.CoveredCount != 1 || quality.MissedCount != 1 || quality.UnexpectedCount != 1 || quality.DuplicateCount != 1 || quality.LowValueCount != 1 {
		t.Fatalf("quality counts = %+v", quality)
	}
	if got := quality.Covered[0].ExpectedID; got != "auth-required" {
		t.Fatalf("covered expected id = %q", got)
	}
	if got := quality.Missed[0].ExpectedID; got != "audit-log" {
		t.Fatalf("missed expected id = %q", got)
	}
	if got := quality.Unexpected[0].Title; got != "other issue" {
		t.Fatalf("unexpected title = %q", got)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	if strings.Contains(string(encoded), "EvidenceHints") || strings.Contains(string(encoded), "RequireAuth") {
		t.Fatalf("report leaked evidence hints: %s", string(encoded))
	}
}

func TestRunReportsExpectedNoFindingUnexpected(t *testing.T) {
	report, err := Run(context.Background(), Fixture{
		Name: "clean",
		Files: []review.FileChange{{
			Filename: "app/user.go",
			Status:   "modified",
			Patch:    "@@",
		}},
		RepoFiles:          map[string]string{"app/user.go": "package app\n"},
		ExpectedNoFindings: true,
		ActualFindings: []ActualFinding{{
			ID:       "actual-noise",
			File:     "app/user.go",
			Line:     9,
			Title:    "unnecessary helper rename",
			Category: "style",
			Severity: "note",
		}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.FindingQuality.Status != FindingQualityAnnotated {
		t.Fatalf("quality status = %q", report.FindingQuality.Status)
	}
	if report.FindingQuality.ExpectedNoFindings != true || report.FindingQuality.UnexpectedCount != 1 {
		t.Fatalf("quality = %+v", report.FindingQuality)
	}
}

func TestRunOmitsFindingQualityForContextOnlyFixture(t *testing.T) {
	report, err := Run(context.Background(), Fixture{
		Name: "context-only",
		Files: []review.FileChange{{
			Filename: "app/user.go",
			Status:   "modified",
			Patch:    "@@",
		}},
		RepoFiles:           map[string]string{"app/user.go": "package app\n"},
		ExpectedFindings:    []ExpectedFinding{{ID: "annotated-later", File: "app/user.go", Title: "future expected finding"}},
		GoldenRelevantFiles: []string{"app/user.go"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.FindingQuality.Status != FindingQualityNotAnnotated {
		t.Fatalf("quality status = %q", report.FindingQuality.Status)
	}
	if report.FindingQuality.ExpectedCount != 1 || report.FindingQuality.CoveredCount != 0 {
		t.Fatalf("quality = %+v", report.FindingQuality)
	}
}

func TestRunSuiteAggregatesFindingQuality(t *testing.T) {
	fixtures := []Fixture{
		{
			Name:      "annotated",
			Files:     []review.FileChange{{Filename: "app/a.go", Status: "modified", Patch: "@@"}},
			RepoFiles: map[string]string{"app/a.go": "package app\n"},
			ExpectedFindings: []ExpectedFinding{{
				ID: "a", File: "app/a.go", Line: 10, Title: "missing validation", Category: "correctness", Severity: "warning",
			}},
			ActualFindings: []ActualFinding{{
				ID: "actual-a", File: "app/a.go", Line: 10, Title: "missing validation", Category: "correctness", Severity: "warning",
			}},
		},
		{
			Name:      "context-only",
			Files:     []review.FileChange{{Filename: "app/b.go", Status: "modified", Patch: "@@"}},
			RepoFiles: map[string]string{"app/b.go": "package app\n"},
			ExpectedFindings: []ExpectedFinding{{
				ID: "legacy-b", File: "app/b.go", Line: 5, Title: "legacy expected finding", Category: "correctness", Severity: "warning",
			}},
		},
		{
			Name:               "clean",
			Files:              []review.FileChange{{Filename: "app/c.go", Status: "modified", Patch: "@@"}},
			RepoFiles:          map[string]string{"app/c.go": "package app\n"},
			ExpectedNoFindings: true,
			ActualFindings: []ActualFinding{{
				ID: "actual-c", File: "app/c.go", Line: 1, Title: "generic note", Category: "style", Severity: "note", QualityLabels: []string{"too-generic"},
			}},
		},
	}

	report, err := RunSuite(context.Background(), fixtures)
	if err != nil {
		t.Fatalf("RunSuite() error = %v", err)
	}
	if report.FindingQuality.AnnotatedFixtureCount != 2 || report.FindingQuality.NotAnnotatedFixtureCount != 1 {
		t.Fatalf("suite quality fixture counts = %+v", report.FindingQuality)
	}
	if report.FindingQuality.ExpectedCount != 1 || report.FindingQuality.CoveredCount != 1 || report.FindingQuality.LowValueCount != 1 {
		t.Fatalf("suite quality counts = %+v", report.FindingQuality)
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
