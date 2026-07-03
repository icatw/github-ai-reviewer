package review

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestBuildRepoContextIncludesFullFilesRelatedTestsDocsAndOmissions(t *testing.T) {
	files := []FileChange{
		{Filename: "pkg/foo.go", Status: "modified", Patch: "@@ foo"},
		{Filename: "pkg/deleted.go", Status: "removed"},
		{Filename: "vendor/lib.go", Status: "modified"},
		{Filename: "web/dist/app.js", Status: "modified"},
		{Filename: "go.sum", Status: "modified"},
		{Filename: "pkg/generated.pb.go", Status: "modified"},
		{Filename: "pkg/image.png", Status: "modified"},
		{Filename: "pkg/huge.go", Status: "modified"},
		{Filename: "pkg/missing.go", Status: "modified"},
		{Filename: "pkg/fetcherr.go", Status: "modified"},
	}
	reader := &fakeRepoReader{
		contents: map[string]string{
			"pkg/foo.go":            "package pkg\nfunc Foo() {}\n",
			"pkg/foo_test.go":       "package pkg\nfunc TestFoo() {}\n",
			"pkg/bar_test.go":       "package pkg\nfunc TestBar() {}\n",
			"pkg/huge.go":           strings.Repeat("a", DefaultContextBudgets.MaxFileBytes+1),
			"README.md":             "# Repo\n",
			"docs/a.md":             "A doc\n",
			"docs/b.md":             "B doc\n",
			".github/ai-review.yml": "tone: concise\n",
		},
		dirs: map[string][]RepositoryEntry{
			"pkg": {
				{Path: "pkg/bar_test.go", Type: RepositoryEntryFile},
				{Path: "pkg/foo_test.go", Type: RepositoryEntryFile},
				{Path: "pkg/ignored.txt", Type: RepositoryEntryFile},
			},
			"docs": {
				{Path: "docs/b.md", Type: RepositoryEntryFile},
				{Path: "docs/a.md", Type: RepositoryEntryFile},
				{Path: "docs/c.md", Type: RepositoryEntryFile},
			},
		},
		errs: map[string]error{
			"pkg/fetcherr.go": errors.New("boom"),
		},
	}

	got := BuildRepoContext(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"}, files, reader, DefaultContextBudgets)

	if paths(got.FullFiles) != "pkg/foo.go" {
		t.Fatalf("full file paths = %s", paths(got.FullFiles))
	}
	if paths(got.RelatedTests) != "pkg/foo_test.go,pkg/bar_test.go" {
		t.Fatalf("related test paths = %s", paths(got.RelatedTests))
	}
	if paths(got.RepoDocs) != ".github/ai-review.yml" {
		t.Fatalf("repo docs paths = %s", paths(got.RepoDocs))
	}
	reasons := omissionReasons(got.Omitted)
	for _, want := range []string{
		"pkg/deleted.go:full_file_context:deleted",
		"vendor/lib.go:full_file_context:vendor_or_dist",
		"web/dist/app.js:full_file_context:vendor_or_dist",
		"go.sum:full_file_context:lock_file",
		"pkg/generated.pb.go:full_file_context:generated",
		"pkg/image.png:full_file_context:binary",
		"pkg/huge.go:full_file_context:oversized",
		"pkg/missing.go:full_file_context:missing",
		"pkg/fetcherr.go:full_file_context:fetch_error",
	} {
		if !contains(reasons, want) {
			t.Fatalf("omissions missing %q in %#v", want, reasons)
		}
	}
	if got.FullFiles[0].Content != "package pkg\nfunc Foo() {}\n" {
		t.Fatalf("full file content = %q", got.FullFiles[0].Content)
	}
}

func TestBuildRepoContextIncludesRelatedGoSources(t *testing.T) {
	reader := &fakeRepoReader{
		contents: map[string]string{
			"go.mod":                  "module example.com/app\n",
			"pkg/foo.go":              "package pkg\n\nimport \"example.com/app/internal/shared\"\n\nfunc Foo() { shared.Util() }\n",
			"pkg/helper.go":           "package pkg\nfunc Helper() {}\n",
			"pkg/foo_test.go":         "package pkg\nfunc TestFoo() {}\n",
			"internal/shared/util.go": "package shared\nfunc Util() {}\n",
			"internal/shared/more.go": "package shared\nfunc More() {}\n",
		},
		dirs: map[string][]RepositoryEntry{
			"pkg": {
				{Path: "pkg/foo.go", Type: RepositoryEntryFile},
				{Path: "pkg/foo_test.go", Type: RepositoryEntryFile},
				{Path: "pkg/helper.go", Type: RepositoryEntryFile},
			},
			"internal/shared": {
				{Path: "internal/shared/more.go", Type: RepositoryEntryFile},
				{Path: "internal/shared/util.go", Type: RepositoryEntryFile},
			},
		},
	}

	got := BuildRepoContext(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"}, []FileChange{{Filename: "pkg/foo.go", Status: "modified"}}, reader, DefaultContextBudgets)

	if paths(got.FullFiles) != "pkg/foo.go" {
		t.Fatalf("full file paths = %s", paths(got.FullFiles))
	}
	if paths(got.RelatedSources) != "internal/shared/util.go" {
		t.Fatalf("related source paths = %s", paths(got.RelatedSources))
	}
	if paths(got.RelatedTests) != "pkg/foo_test.go" {
		t.Fatalf("related test paths = %s", paths(got.RelatedTests))
	}
}

func TestBuildRepoContextIncludesRelatedPythonSourcesAndTests(t *testing.T) {
	reader := &fakeRepoReader{
		contents: map[string]string{
			"app/api/user.py":       "from app.services.user import create_user\nfrom .schemas import UserRequest\n\ndef route(): pass\n",
			"app/api/schemas.py":    "class UserRequest: pass\n",
			"app/api/test_user.py":  "def test_route(): pass\n",
			"app/services/user.py":  "def create_user(): pass\n",
			"app/services/audit.py": "def audit(): pass\n",
			"tests/test_user.py":    "def test_user_flow(): pass\n",
		},
		dirs: map[string][]RepositoryEntry{
			"": {
				{Path: "app/api/user.py", Type: RepositoryEntryFile},
			},
			"app/api": {
				{Path: "app/api/schemas.py", Type: RepositoryEntryFile},
				{Path: "app/api/test_user.py", Type: RepositoryEntryFile},
				{Path: "app/api/user.py", Type: RepositoryEntryFile},
			},
			"app/services": {
				{Path: "app/services/audit.py", Type: RepositoryEntryFile},
				{Path: "app/services/user.py", Type: RepositoryEntryFile},
			},
		},
	}

	got := BuildRepoContext(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"}, []FileChange{{Filename: "app/api/user.py", Status: "modified"}}, reader, DefaultContextBudgets)

	if paths(got.FullFiles) != "app/api/user.py" {
		t.Fatalf("full file paths = %s", paths(got.FullFiles))
	}
	if paths(got.RelatedSources) != "app/api/schemas.py,app/services/user.py" {
		t.Fatalf("related source paths = %s", paths(got.RelatedSources))
	}
	if paths(got.RelatedTests) != "app/api/test_user.py,tests/test_user.py" {
		t.Fatalf("related test paths = %s", paths(got.RelatedTests))
	}
}

func TestBuildRepoContextFiltersDocsByRelevance(t *testing.T) {
	reader := &fakeRepoReader{
		contents: map[string]string{
			"pkg/auth.go":           "package pkg\nfunc RequireAuth() {}\n",
			"README.md":             "# Generic repo introduction\n",
			"docs/security.md":      "All auth changes must preserve permission checks.\n",
			"docs/deployment.md":    "Deployment notes only.\n",
			".github/ai-review.yml": "language: zh-CN\n",
		},
		dirs: map[string][]RepositoryEntry{
			"pkg": {},
			"docs": {
				{Path: "docs/deployment.md", Type: RepositoryEntryFile},
				{Path: "docs/security.md", Type: RepositoryEntryFile},
			},
		},
	}

	got := BuildRepoContext(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"}, []FileChange{{Filename: "pkg/auth.go", Status: "modified", Patch: "@@ auth permission"}}, reader, DefaultContextBudgets)

	if paths(got.RepoDocs) != "docs/security.md,.github/ai-review.yml" {
		t.Fatalf("repo docs paths = %s", paths(got.RepoDocs))
	}
}

func TestBuildRepoContextDedupesRelatedTestsAndAppliesBudgetsDeterministically(t *testing.T) {
	budgets := DefaultContextBudgets
	budgets.MaxSamePackageTests = 1
	budgets.MaxDocsFiles = 1
	budgets.TotalBytes = 70
	budgets.MaxFileBytes = 40
	reader := &fakeRepoReader{
		contents: map[string]string{
			"pkg/a.go":           "package pkg\nfunc A() {}\n",
			"pkg/b.go":           "package pkg\nfunc B() {}\n",
			"pkg/a_test.go":      "package pkg\nfunc TestA() {}\n",
			"pkg/common_test.go": "package pkg\nfunc TestCommon() {}\n",
			"README.md":          "# readme\n",
			"docs/a.md":          "doc\n",
		},
		dirs: map[string][]RepositoryEntry{
			"pkg": {
				{Path: "pkg/common_test.go", Type: RepositoryEntryFile},
				{Path: "pkg/a_test.go", Type: RepositoryEntryFile},
			},
			"docs": {
				{Path: "docs/a.md", Type: RepositoryEntryFile},
				{Path: "docs/b.md", Type: RepositoryEntryFile},
			},
		},
	}
	files := []FileChange{
		{Filename: "pkg/b.go", Status: "modified"},
		{Filename: "pkg/a.go", Status: "modified"},
	}

	first := BuildRepoContext(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"}, files, reader, budgets)
	second := BuildRepoContext(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"}, files, reader, budgets)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("context is not deterministic:\n%#v\n%#v", first, second)
	}
	if paths(first.FullFiles) != "pkg/a.go,pkg/b.go" {
		t.Fatalf("full file paths = %s", paths(first.FullFiles))
	}
	if paths(first.RelatedTests) != "pkg/a_test.go" {
		t.Fatalf("related test paths = %s", paths(first.RelatedTests))
	}
	reasons := omissionReasons(first.Omitted)
	for _, want := range []string{
		"pkg/common_test.go:related_test_context:budget_exhausted",
	} {
		if !contains(reasons, want) {
			t.Fatalf("omissions missing %q in %#v", want, reasons)
		}
	}
}

func TestBuildRepoContextRecordsMissingDirectTestAndTruncatesWithinTotalBudget(t *testing.T) {
	budgets := DefaultContextBudgets
	budgets.MaxFileBytes = 100
	budgets.TotalBytes = len("package pkg\nfunc Foo() {}\n") + len("package pkg\nfunc Bar() {}\n") + len("package pkg\nfunc TestFoo()") + 2
	reader := &fakeRepoReader{
		contents: map[string]string{
			"pkg/foo.go":      "package pkg\nfunc Foo() {}\n",
			"pkg/foo_test.go": "package pkg\nfunc TestFoo() {}\n",
			"pkg/bar.go":      "package pkg\nfunc Bar() {}\n",
		},
		dirs: map[string][]RepositoryEntry{
			"pkg": {
				{Path: "pkg/foo_test.go", Type: RepositoryEntryFile},
			},
		},
	}

	got := BuildRepoContext(context.Background(), Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "abc"}, []FileChange{
		{Filename: "pkg/foo.go", Status: "modified"},
		{Filename: "pkg/bar.go", Status: "modified"},
	}, reader, budgets)

	if paths(got.RelatedTests) != "pkg/foo_test.go" {
		t.Fatalf("related test paths = %s", paths(got.RelatedTests))
	}
	reasons := omissionReasons(got.Omitted)
	for _, want := range []string{
		"pkg/foo_test.go:related_test_context:truncated",
		"pkg/bar_test.go:related_test_context:missing",
	} {
		if !contains(reasons, want) {
			t.Fatalf("omissions missing %q in %#v", want, reasons)
		}
	}
	if len(got.RelatedTests) != 1 || got.RelatedTests[0].Content == "package pkg\nfunc TestFoo() {}\n" {
		t.Fatalf("related test was not truncated: %+v", got.RelatedTests)
	}
}

func TestBuildPromptRendersStableRepoAwareSections(t *testing.T) {
	ctx := RepoContext{
		Patches:        []PatchContext{{Path: "main.go", Status: "modified", Additions: 1, Deletions: 0, Patch: "@@ patch"}},
		FullFiles:      []FileContext{{Path: "main.go", Content: "package main\n"}},
		RelatedSources: []FileContext{{Path: "helper.go", Content: "package main\n"}},
		RelatedTests:   []FileContext{{Path: "main_test.go", Content: "package main\n"}},
		RepoDocs:       []FileContext{{Path: "README.md", Content: "# repo\n"}},
		Omitted:        []OmittedContext{{Path: "big.go", Section: SectionFullFile, Reason: OmitOversized}},
	}

	prompt := BuildPromptWithContext(Job{Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc", Action: "opened"}, ctx)

	for _, want := range []string{"patch_context", "full_file_context", "related_source_context", "related_test_context", "repo_docs_context", "omitted_context", "Review pull request octo/repo#7", "JSON-only"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func paths(items []FileContext) string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Path)
	}
	return strings.Join(out, ",")
}

func omissionReasons(items []OmittedContext) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Path+":"+string(item.Section)+":"+string(item.Reason))
	}
	return out
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

type fakeRepoReader struct {
	contents map[string]string
	dirs     map[string][]RepositoryEntry
	errs     map[string]error
}

func (f *fakeRepoReader) FetchFileContent(ctx context.Context, installationID int64, owner, repo, ref, path string) (string, error) {
	if err := f.errs[path]; err != nil {
		return "", err
	}
	content, ok := f.contents[path]
	if !ok {
		return "", ErrRepositoryContentNotFound
	}
	return content, nil
}

func (f *fakeRepoReader) ListDirectory(ctx context.Context, installationID int64, owner, repo, ref, path string) ([]RepositoryEntry, error) {
	entries, ok := f.dirs[path]
	if !ok {
		return nil, ErrRepositoryContentNotFound
	}
	return entries, nil
}
