package review

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrRepositoryContentNotFound = errors.New("repository content not found")

const (
	SectionPatch       ContextSection = "patch_context"
	SectionFullFile    ContextSection = "full_file_context"
	SectionRelatedTest ContextSection = "related_test_context"
	SectionRepoDocs    ContextSection = "repo_docs_context"
	SectionStaticCheck ContextSection = "static_check_context"
	SectionOmitted     ContextSection = "omitted_context"

	OmitDeleted         OmitReason = "deleted"
	OmitBinary          OmitReason = "binary"
	OmitGenerated       OmitReason = "generated"
	OmitLockFile        OmitReason = "lock_file"
	OmitVendorOrDist    OmitReason = "vendor_or_dist"
	OmitOversized       OmitReason = "oversized"
	OmitTruncated       OmitReason = "truncated"
	OmitBudgetExhausted OmitReason = "budget_exhausted"
	OmitMissing         OmitReason = "missing"
	OmitFetchError      OmitReason = "fetch_error"
)

var DefaultContextBudgets = ContextBudgets{
	MaxPatchBytes:       12000,
	MaxFileBytes:        24000,
	TotalBytes:          64000,
	MaxSamePackageTests: 4,
	MaxDocsFiles:        2,
}

type ContextSection string
type OmitReason string

type ContextBudgets struct {
	MaxPatchBytes       int
	MaxFileBytes        int
	TotalBytes          int
	MaxSamePackageTests int
	MaxDocsFiles        int
}

type RepoContext struct {
	Patches      []PatchContext
	FullFiles    []FileContext
	RelatedTests []FileContext
	RepoDocs     []FileContext
	StaticChecks []StaticCheckEvidence
	Omitted      []OmittedContext
}

type PatchContext struct {
	Path      string
	Status    string
	Additions int
	Deletions int
	Patch     string
}

type FileContext struct {
	Path    string
	Content string
}

type OmittedContext struct {
	Path    string
	Section ContextSection
	Reason  OmitReason
}

type RepositoryEntryType string

const RepositoryEntryFile RepositoryEntryType = "file"

type RepositoryEntry struct {
	Path string
	Type RepositoryEntryType
}

type RepositoryReader interface {
	FetchFileContent(ctx context.Context, installationID int64, owner, repo, ref, path string) (string, error)
	ListDirectory(ctx context.Context, installationID int64, owner, repo, ref, path string) ([]RepositoryEntry, error)
}

func BuildPatchContext(files []FileChange, maxPatchBytes int) RepoContext {
	if maxPatchBytes <= 0 {
		maxPatchBytes = DefaultContextBudgets.MaxPatchBytes
	}
	var out RepoContext
	remaining := maxPatchBytes
	for _, f := range sortedFileChanges(files) {
		patch := f.Patch
		if patch != "" {
			if remaining <= 0 {
				out.Omitted = append(out.Omitted, OmittedContext{Path: f.Filename, Section: SectionPatch, Reason: OmitBudgetExhausted})
				patch = ""
			} else if len(patch) > remaining {
				patch = patch[:remaining]
				remaining = 0
				out.Omitted = append(out.Omitted, OmittedContext{Path: f.Filename, Section: SectionPatch, Reason: OmitTruncated})
			} else {
				remaining -= len(patch)
			}
		}
		out.Patches = append(out.Patches, PatchContext{Path: f.Filename, Status: f.Status, Additions: f.Additions, Deletions: f.Deletions, Patch: patch})
	}
	return out
}

func BuildRepoContext(ctx context.Context, job Job, files []FileChange, reader RepositoryReader, budgets ContextBudgets) RepoContext {
	budgets = normalizeBudgets(budgets)
	out := BuildPatchContext(files, budgets.MaxPatchBytes)
	if reader == nil {
		return out
	}
	used := 0
	for _, f := range sortedFileChanges(files) {
		if omitted := omitForChangedFile(f); omitted != "" {
			out.Omitted = append(out.Omitted, OmittedContext{Path: f.Filename, Section: SectionFullFile, Reason: omitted})
			continue
		}
		item, omission, ok := fetchContextFile(ctx, reader, job, f.Filename, SectionFullFile, budgets, &used)
		if ok {
			out.FullFiles = append(out.FullFiles, item)
		}
		if omission != nil {
			out.Omitted = append(out.Omitted, *omission)
		}
	}
	testCandidates, testOmissions := relatedTestCandidates(ctx, reader, job, files, budgets)
	out.Omitted = append(out.Omitted, testOmissions...)
	for _, candidate := range testCandidates {
		item, omission, ok := fetchContextFile(ctx, reader, job, candidate, SectionRelatedTest, budgets, &used)
		if ok {
			out.RelatedTests = append(out.RelatedTests, item)
		}
		if omission != nil {
			out.Omitted = append(out.Omitted, *omission)
		}
	}
	docCandidates, docOmissions := docsCandidates(ctx, reader, job, budgets)
	out.Omitted = append(out.Omitted, docOmissions...)
	for _, candidate := range docCandidates {
		item, omission, ok := fetchContextFile(ctx, reader, job, candidate, SectionRepoDocs, budgets, &used)
		if ok {
			out.RepoDocs = append(out.RepoDocs, item)
		}
		if omission != nil {
			out.Omitted = append(out.Omitted, *omission)
		}
	}
	return out
}

func fetchContextFile(ctx context.Context, reader RepositoryReader, job Job, filePath string, section ContextSection, budgets ContextBudgets, used *int) (FileContext, *OmittedContext, bool) {
	if reason := omitForPath(filePath); reason != "" {
		return FileContext{}, &OmittedContext{Path: filePath, Section: section, Reason: reason}, false
	}
	content, err := reader.FetchFileContent(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, filePath)
	if errors.Is(err, ErrRepositoryContentNotFound) {
		return FileContext{}, &OmittedContext{Path: filePath, Section: section, Reason: OmitMissing}, false
	}
	if err != nil {
		return FileContext{}, &OmittedContext{Path: filePath, Section: section, Reason: OmitFetchError}, false
	}
	if content == "" {
		return FileContext{Path: filePath, Content: content}, nil, true
	}
	if !isText(content) {
		return FileContext{}, &OmittedContext{Path: filePath, Section: section, Reason: OmitBinary}, false
	}
	if len(content) > budgets.MaxFileBytes {
		return FileContext{}, &OmittedContext{Path: filePath, Section: section, Reason: OmitOversized}, false
	}
	remaining := budgets.TotalBytes - *used
	if remaining <= 0 {
		return FileContext{}, &OmittedContext{Path: filePath, Section: section, Reason: OmitBudgetExhausted}, false
	}
	if len(content) > remaining {
		item := FileContext{Path: filePath, Content: content[:remaining]}
		*used += remaining
		return item, &OmittedContext{Path: filePath, Section: section, Reason: OmitTruncated}, true
	}
	*used += len(content)
	return FileContext{Path: filePath, Content: content}, nil, true
}

func omitForChangedFile(f FileChange) OmitReason {
	if strings.EqualFold(f.Status, "removed") || strings.EqualFold(f.Status, "deleted") {
		return OmitDeleted
	}
	return omitForPath(f.Filename)
}

func omitForPath(filePath string) OmitReason {
	clean := path.Clean(filePath)
	lower := strings.ToLower(clean)
	parts := strings.Split(lower, "/")
	for _, part := range parts {
		switch part {
		case "vendor", "dist", "build", "node_modules":
			return OmitVendorOrDist
		}
	}
	base := path.Base(lower)
	switch {
	case base == "go.sum", base == "package-lock.json", base == "yarn.lock", base == "pnpm-lock.yaml", base == "composer.lock", base == "gemfile.lock", strings.HasSuffix(base, ".lock"):
		return OmitLockFile
	case strings.HasSuffix(base, ".pb.go"), strings.HasSuffix(base, ".gen.go"), strings.Contains(base, "generated"):
		return OmitGenerated
	case isBinaryPath(lower):
		return OmitBinary
	default:
		return ""
	}
}

func relatedTestCandidates(ctx context.Context, reader RepositoryReader, job Job, files []FileChange, budgets ContextBudgets) ([]string, []OmittedContext) {
	directSet := map[string]struct{}{}
	packages := map[string]struct{}{}
	for _, f := range sortedFileChanges(files) {
		if omitForChangedFile(f) != "" || !strings.HasSuffix(f.Filename, ".go") || strings.HasSuffix(f.Filename, "_test.go") {
			continue
		}
		dir, base := path.Split(f.Filename)
		directSet[strings.TrimSuffix(dir+strings.TrimSuffix(base, ".go")+"_test.go", "/")] = struct{}{}
		packages[strings.TrimSuffix(dir, "/")] = struct{}{}
	}
	direct := make([]string, 0, len(directSet))
	for candidate := range directSet {
		direct = append(direct, candidate)
	}
	sort.Strings(direct)
	out := append([]string(nil), direct...)
	included := map[string]struct{}{}
	for _, candidate := range out {
		included[candidate] = struct{}{}
	}
	var omitted []OmittedContext
	packageDirs := make([]string, 0, len(packages))
	for dir := range packages {
		packageDirs = append(packageDirs, dir)
	}
	sort.Strings(packageDirs)
	for _, dir := range packageDirs {
		entries, err := reader.ListDirectory(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, dir)
		if err != nil {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
		count := 0
		for _, entry := range entries {
			if entry.Type != RepositoryEntryFile || !strings.HasSuffix(entry.Path, "_test.go") {
				continue
			}
			if _, ok := included[entry.Path]; ok {
				continue
			}
			if count >= budgets.MaxSamePackageTests {
				omitted = append(omitted, OmittedContext{Path: entry.Path, Section: SectionRelatedTest, Reason: OmitBudgetExhausted})
				continue
			}
			out = append(out, entry.Path)
			included[entry.Path] = struct{}{}
			count++
		}
	}
	return out, omitted
}

func docsCandidates(ctx context.Context, reader RepositoryReader, job Job, budgets ContextBudgets) ([]string, []OmittedContext) {
	out := []string{"README.md"}
	var omitted []OmittedContext
	entries, err := reader.ListDirectory(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, "docs")
	if err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
		includedDocs := 0
		for _, entry := range entries {
			if entry.Type != RepositoryEntryFile || !strings.HasSuffix(strings.ToLower(entry.Path), ".md") {
				continue
			}
			if includedDocs >= budgets.MaxDocsFiles {
				omitted = append(omitted, OmittedContext{Path: entry.Path, Section: SectionRepoDocs, Reason: OmitBudgetExhausted})
				continue
			}
			out = append(out, entry.Path)
			includedDocs++
		}
	}
	out = append(out, ".github/ai-review.yml")
	return out, omitted
}

func normalizeBudgets(b ContextBudgets) ContextBudgets {
	if b.MaxPatchBytes <= 0 {
		b.MaxPatchBytes = DefaultContextBudgets.MaxPatchBytes
	}
	if b.MaxFileBytes <= 0 {
		b.MaxFileBytes = DefaultContextBudgets.MaxFileBytes
	}
	if b.TotalBytes <= 0 {
		b.TotalBytes = DefaultContextBudgets.TotalBytes
	}
	if b.MaxSamePackageTests <= 0 {
		b.MaxSamePackageTests = DefaultContextBudgets.MaxSamePackageTests
	}
	if b.MaxDocsFiles <= 0 {
		b.MaxDocsFiles = DefaultContextBudgets.MaxDocsFiles
	}
	return b
}

func sortedFileChanges(files []FileChange) []FileChange {
	out := append([]FileChange(nil), files...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Filename < out[j].Filename })
	return out
}

func isText(content string) bool {
	return utf8.ValidString(content) && !strings.ContainsRune(content, '\x00')
}

func isBinaryPath(filePath string) bool {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".pdf", ".zip", ".gz", ".tar", ".wasm", ".bin"} {
		if strings.HasSuffix(filePath, ext) {
			return true
		}
	}
	return false
}

func RenderPrompt(job Job, ctx RepoContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Review pull request %s/%s#%d at head %s.\n", job.Owner, job.Repo, job.PullNumber, job.HeadSHA)
	fmt.Fprintf(&b, "Webhook action: %s.\n", job.Action)
	b.WriteString("Return JSON-only output matching this schema: {\"summary\": string, \"risk_score\": integer 0-100, \"findings\": [{\"severity\": \"blocker|warning|suggestion|question\", \"category\": string, \"file\": string, \"line\": integer, \"title\": string, \"evidence\": string, \"failure_scenario\": string, \"suggestion\": string, \"confidence\": number 0.0-1.0}], \"missing_tests\": [string], \"limitations\": [string]}.\n")
	b.WriteString("Findings are advisory and non-blocking. Be concise and evidence-based. Use only the context below; if context is insufficient, record the limitation instead of fabricating unavailable context.\n\n")
	renderPatchSection(&b, ctx.Patches)
	renderFileSection(&b, SectionFullFile, ctx.FullFiles)
	renderFileSection(&b, SectionRelatedTest, ctx.RelatedTests)
	renderFileSection(&b, SectionRepoDocs, ctx.RepoDocs)
	renderStaticCheckSection(&b, ctx.StaticChecks)
	renderOmittedSection(&b, ctx.Omitted)
	return b.String()
}

func BuildPromptWithContext(job Job, ctx RepoContext) string {
	return RenderPrompt(job, ctx)
}

func renderPatchSection(b *strings.Builder, patches []PatchContext) {
	b.WriteString("## patch_context\n")
	if len(patches) == 0 {
		b.WriteString("(none)\n\n")
		return
	}
	for _, f := range patches {
		fmt.Fprintf(b, "File: %s\nStatus: %s\nAdditions: %d Deletions: %d\n", f.Path, f.Status, f.Additions, f.Deletions)
		if f.Patch != "" {
			b.WriteString("Patch:\n")
			b.WriteString(f.Patch)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
}

func renderFileSection(b *strings.Builder, section ContextSection, files []FileContext) {
	fmt.Fprintf(b, "## %s\n", section)
	if len(files) == 0 {
		b.WriteString("(none)\n\n")
		return
	}
	for _, f := range files {
		fmt.Fprintf(b, "File: %s\nContent:\n%s\n\n", f.Path, f.Content)
	}
}

func renderStaticCheckSection(b *strings.Builder, items []StaticCheckEvidence) {
	b.WriteString("## static_check_context\n")
	if len(items) == 0 {
		b.WriteString("(none)\n\n")
		return
	}
	for _, item := range items {
		fmt.Fprintf(b, "- tool=%s category=%s", item.Tool, item.ExitCategory)
		if item.Path != "" {
			fmt.Fprintf(b, " path=%s", item.Path)
		}
		if item.Line != nil {
			fmt.Fprintf(b, " line=%d", *item.Line)
		}
		if item.Message != "" {
			fmt.Fprintf(b, " message=%s", sanitizeAnalyzerMessage(item.Message))
		}
		if len(item.Limitations) > 0 {
			fmt.Fprintf(b, " limitations=%s", strings.Join(sanitizeStringList(item.Limitations), "; "))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func renderOmittedSection(b *strings.Builder, items []OmittedContext) {
	b.WriteString("## omitted_context\n")
	if len(items) == 0 {
		b.WriteString("(none)\n")
		return
	}
	if hasPatchOmission(items) {
		b.WriteString("Some patch context was omitted due to the prompt budget. Mention this limitation if it affects confidence.\n")
	}
	for _, item := range items {
		fmt.Fprintf(b, "- path=%s section=%s reason=%s\n", item.Path, item.Section, item.Reason)
	}
}

func sanitizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if sanitized := sanitizeAnalyzerMessage(value); sanitized != "" {
			out = append(out, sanitized)
		}
	}
	return out
}

func hasPatchOmission(items []OmittedContext) bool {
	for _, item := range items {
		if item.Section == SectionPatch {
			return true
		}
	}
	return false
}
