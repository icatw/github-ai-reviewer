package review

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrRepositoryContentNotFound = errors.New("repository content not found")

const (
	SectionPatch       ContextSection = "patch_context"
	SectionFullFile    ContextSection = "full_file_context"
	SectionRelatedSrc  ContextSection = "related_source_context"
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
	MaxRelatedSources:   8,
	MaxSamePackageTests: 4,
	MaxDocsFiles:        2,
}

type ContextSection string
type OmitReason string

type ContextBudgets struct {
	MaxPatchBytes       int
	MaxFileBytes        int
	TotalBytes          int
	MaxRelatedSources   int
	MaxSamePackageTests int
	MaxDocsFiles        int
}

type RepoContext struct {
	Patches        []PatchContext
	FullFiles      []FileContext
	RelatedSources []FileContext
	RelatedTests   []FileContext
	RepoDocs       []FileContext
	StaticChecks   []StaticCheckEvidence
	Omitted        []OmittedContext
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
	sourceCandidates, sourceOmissions := relatedSourceCandidates(ctx, reader, job, files, budgets)
	out.Omitted = append(out.Omitted, sourceOmissions...)
	for _, candidate := range sourceCandidates {
		item, omission, ok := fetchContextFile(ctx, reader, job, candidate, SectionRelatedSrc, budgets, &used)
		if ok {
			out.RelatedSources = append(out.RelatedSources, item)
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
	docCandidates, docOmissions := docsCandidates(ctx, reader, job, files, budgets)
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

func relatedSourceCandidates(ctx context.Context, reader RepositoryReader, job Job, files []FileChange, budgets ContextBudgets) ([]string, []OmittedContext) {
	changed := map[string]struct{}{}
	dirs := map[string]struct{}{}
	preferredFiles := map[string]struct{}{}
	for _, f := range sortedFileChanges(files) {
		if omitForChangedFile(f) != "" || !isReviewableSourceFile(f.Filename) || isTestSourceFile(f.Filename) {
			continue
		}
		changed[f.Filename] = struct{}{}
		dir, _ := path.Split(f.Filename)
		dir = strings.TrimSuffix(dir, "/")
		for _, samePackageFile := range relatedSamePackageFiles(ctx, reader, job, f.Filename, dir) {
			preferredFiles[samePackageFile] = struct{}{}
		}
		targets := localImportTargets(ctx, reader, job, f.Filename)
		for _, importFile := range targets.Files {
			preferredFiles[importFile] = struct{}{}
		}
		for _, importDir := range targets.Dirs {
			dirs[importDir] = struct{}{}
		}
	}
	orderedPreferredFiles := make([]string, 0, len(preferredFiles))
	for filePath := range preferredFiles {
		orderedPreferredFiles = append(orderedPreferredFiles, filePath)
	}
	sort.Strings(orderedPreferredFiles)
	orderedDirs := make([]string, 0, len(dirs))
	for dir := range dirs {
		orderedDirs = append(orderedDirs, dir)
	}
	sort.Strings(orderedDirs)
	included := map[string]struct{}{}
	var out []string
	var omitted []OmittedContext
	addCandidate := func(filePath string) {
		if _, ok := changed[filePath]; ok {
			return
		}
		if _, ok := included[filePath]; ok {
			return
		}
		if omitForPath(filePath) != "" {
			return
		}
		if len(out) >= budgets.MaxRelatedSources {
			omitted = append(omitted, OmittedContext{Path: filePath, Section: SectionRelatedSrc, Reason: OmitBudgetExhausted})
			return
		}
		out = append(out, filePath)
		included[filePath] = struct{}{}
	}
	for _, filePath := range orderedPreferredFiles {
		addCandidate(filePath)
	}
	for _, dir := range orderedDirs {
		entries, err := reader.ListDirectory(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, dir)
		if err != nil {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
		for _, entry := range entries {
			if entry.Type != RepositoryEntryFile || !isReviewableSourceFile(entry.Path) || isTestSourceFile(entry.Path) {
				continue
			}
			addCandidate(entry.Path)
		}
	}
	return out, omitted
}

type localImportTarget struct {
	Dirs  []string
	Files []string
}

func relatedSamePackageFiles(ctx context.Context, reader RepositoryReader, job Job, filePath, dir string) []string {
	content, err := reader.FetchFileContent(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, filePath)
	if err != nil {
		return nil
	}
	symbols := changedFileSymbols(filePath, content)
	if len(symbols) == 0 {
		return nil
	}
	symbolSet := map[string]struct{}{}
	for _, symbol := range symbols {
		symbolSet[symbol] = struct{}{}
	}
	entries, err := reader.ListDirectory(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, dir)
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var out []string
	for _, entry := range entries {
		if entry.Type != RepositoryEntryFile || !isReviewableSourceFile(entry.Path) || isTestSourceFile(entry.Path) || entry.Path == filePath || omitForPath(entry.Path) != "" {
			continue
		}
		candidate, err := reader.FetchFileContent(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, entry.Path)
		if err != nil {
			continue
		}
		if contentReferencesAnySymbol(entry.Path, candidate, symbolSet) {
			out = append(out, entry.Path)
		}
	}
	return out
}

func changedFileSymbols(filePath, content string) []string {
	switch {
	case strings.HasSuffix(filePath, ".go"):
		return goDefinedSymbols(content)
	case strings.HasSuffix(filePath, ".py"):
		return pythonDefinedSymbols(content)
	default:
		return nil
	}
}

func contentReferencesAnySymbol(filePath, content string, symbols map[string]struct{}) bool {
	switch {
	case strings.HasSuffix(filePath, ".go"):
		return goContentReferencesAnySymbol(content, symbols)
	case strings.HasSuffix(filePath, ".py"):
		return pythonContentReferencesAnySymbol(content, symbols)
	default:
		return false
	}
}

func localImportTargets(ctx context.Context, reader RepositoryReader, job Job, filePath string) localImportTarget {
	switch {
	case strings.HasSuffix(filePath, ".go"):
		return localGoImportTargets(ctx, reader, job, filePath)
	case strings.HasSuffix(filePath, ".py"):
		return localPythonImportTargets(ctx, reader, job, filePath)
	default:
		return localImportTarget{}
	}
}

func localGoImportTargets(ctx context.Context, reader RepositoryReader, job Job, filePath string) localImportTarget {
	content, err := reader.FetchFileContent(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, filePath)
	if err != nil {
		return localImportTarget{}
	}
	modulePath := goModulePath(ctx, reader, job, filePath)
	imports := parseGoImportAliases(content)
	selectors := parseGoSelectorUses(content)
	targets := localImportTarget{}
	for alias, importPath := range imports {
		if modulePath == "" || !strings.HasPrefix(importPath, modulePath+"/") {
			continue
		}
		dir := strings.TrimPrefix(importPath, modulePath+"/")
		if !safeRepoDir(dir) {
			continue
		}
		symbols := selectors[alias]
		matched := goFilesDefiningSymbols(ctx, reader, job, dir, symbols)
		if len(matched) > 0 {
			targets.Files = append(targets.Files, matched...)
			continue
		}
		targets.Dirs = append(targets.Dirs, dir)
	}
	sort.Strings(targets.Files)
	targets.Files = dedupeStrings(targets.Files)
	sort.Strings(targets.Dirs)
	targets.Dirs = dedupeStrings(targets.Dirs)
	return targets
}

func localGoImportDirs(ctx context.Context, reader RepositoryReader, job Job, filePath string) []string {
	content, err := reader.FetchFileContent(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, filePath)
	if err != nil {
		return nil
	}
	modulePath := goModulePath(ctx, reader, job, filePath)
	var dirs []string
	for _, imp := range parseGoImports(content) {
		if modulePath == "" || !strings.HasPrefix(imp, modulePath+"/") {
			continue
		}
		dir := strings.TrimPrefix(imp, modulePath+"/")
		if !safeRepoDir(dir) {
			continue
		}
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dedupeStrings(dirs)
}

func goModulePath(ctx context.Context, reader RepositoryReader, job Job, filePath string) string {
	modulePath := strings.TrimSuffix(path.Base(filePath), ".go")
	if mod, err := reader.FetchFileContent(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, "go.mod"); err == nil {
		for _, line := range strings.Split(mod, "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "module" {
				modulePath = fields[1]
				break
			}
		}
	}
	return modulePath
}

func safeRepoDir(dir string) bool {
	return dir != "" && !strings.Contains(dir, "..") && !strings.HasPrefix(dir, "/")
}

func parseGoImportAliases(content string) map[string]string {
	file, err := parser.ParseFile(token.NewFileSet(), "context.go", content, parser.ImportsOnly)
	if err != nil {
		return fallbackGoImportAliases(content)
	}
	out := map[string]string{}
	for _, spec := range file.Imports {
		importPath := strings.Trim(spec.Path.Value, "\"")
		if importPath == "" {
			continue
		}
		alias := path.Base(importPath)
		if spec.Name != nil {
			if spec.Name.Name == "." || spec.Name.Name == "_" {
				continue
			}
			alias = spec.Name.Name
		}
		out[alias] = importPath
	}
	return out
}

func fallbackGoImportAliases(content string) map[string]string {
	out := map[string]string{}
	for _, importPath := range parseGoImports(content) {
		out[path.Base(importPath)] = importPath
	}
	return out
}

func parseGoSelectorUses(content string) map[string][]string {
	file, err := parser.ParseFile(token.NewFileSet(), "context.go", content, 0)
	if err != nil {
		return map[string][]string{}
	}
	out := map[string][]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || ident.Name == "" || selector.Sel == nil || selector.Sel.Name == "" {
			return true
		}
		out[ident.Name] = append(out[ident.Name], selector.Sel.Name)
		return true
	})
	for alias, symbols := range out {
		sort.Strings(symbols)
		out[alias] = dedupeStrings(symbols)
	}
	return out
}

func goFilesDefiningSymbols(ctx context.Context, reader RepositoryReader, job Job, dir string, symbols []string) []string {
	if len(symbols) == 0 {
		return nil
	}
	symbolSet := map[string]struct{}{}
	for _, symbol := range symbols {
		symbolSet[symbol] = struct{}{}
	}
	entries, err := reader.ListDirectory(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, dir)
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var out []string
	for _, entry := range entries {
		if entry.Type != RepositoryEntryFile || !strings.HasSuffix(entry.Path, ".go") || strings.HasSuffix(entry.Path, "_test.go") || omitForPath(entry.Path) != "" {
			continue
		}
		content, err := reader.FetchFileContent(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, entry.Path)
		if err != nil {
			continue
		}
		if goContentDefinesAnySymbol(content, symbolSet) {
			out = append(out, entry.Path)
		}
	}
	return out
}

func goContentDefinesAnySymbol(content string, symbols map[string]struct{}) bool {
	file, err := parser.ParseFile(token.NewFileSet(), "candidate.go", content, 0)
	if err != nil {
		return false
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name != nil {
				if _, ok := symbols[d.Name.Name]; ok {
					return true
				}
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if _, ok := symbols[s.Name.Name]; ok {
						return true
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if _, ok := symbols[name.Name]; ok {
							return true
						}
					}
				}
			}
		}
	}
	return false
}

func goDefinedSymbols(content string) []string {
	file, err := parser.ParseFile(token.NewFileSet(), "changed.go", content, 0)
	if err != nil {
		return nil
	}
	var out []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name != nil {
				out = append(out, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					out = append(out, s.Name.Name)
				case *ast.ValueSpec:
					for _, name := range s.Names {
						out = append(out, name.Name)
					}
				}
			}
		}
	}
	sort.Strings(out)
	return dedupeStrings(out)
}

func goContentReferencesAnySymbol(content string, symbols map[string]struct{}) bool {
	file, err := parser.ParseFile(token.NewFileSet(), "candidate.go", content, 0)
	if err != nil {
		return false
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		if found {
			return false
		}
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		_, found = symbols[ident.Name]
		return !found
	})
	return found
}

func localPythonImportTargets(ctx context.Context, reader RepositoryReader, job Job, filePath string) localImportTarget {
	content, err := reader.FetchFileContent(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, filePath)
	if err != nil {
		return localImportTarget{}
	}
	changedDir := path.Dir(filePath)
	if changedDir == "." {
		changedDir = ""
	}
	rootDirs := topLevelPythonPackages(ctx, reader, job)
	targets := localImportTarget{}
	for _, imp := range parsePythonImports(content) {
		candidate := pythonImportCandidate(imp, changedDir, rootDirs)
		targets.Files = append(targets.Files, candidate.Files...)
		targets.Dirs = append(targets.Dirs, candidate.Dirs...)
	}
	sort.Strings(targets.Files)
	targets.Files = dedupeStrings(targets.Files)
	sort.Strings(targets.Dirs)
	targets.Dirs = dedupeStrings(targets.Dirs)
	return targets
}

func topLevelPythonPackages(ctx context.Context, reader RepositoryReader, job Job) map[string]struct{} {
	out := map[string]struct{}{}
	entries, err := reader.ListDirectory(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, "")
	if err != nil {
		return out
	}
	for _, entry := range entries {
		parts := strings.Split(strings.Trim(entry.Path, "/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			out[parts[0]] = struct{}{}
		}
	}
	return out
}

func parsePythonImports(content string) []string {
	imports := []string{}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if hash := strings.Index(line, "#"); hash >= 0 {
			line = strings.TrimSpace(line[:hash])
		}
		switch {
		case strings.HasPrefix(line, "import "):
			for _, item := range strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "import ")), ",") {
				fields := strings.Fields(strings.TrimSpace(item))
				if len(fields) > 0 {
					imports = append(imports, fields[0])
				}
			}
		case strings.HasPrefix(line, "from "):
			rest := strings.TrimSpace(strings.TrimPrefix(line, "from "))
			parts := strings.SplitN(rest, " import ", 2)
			if len(parts) == 2 {
				imports = append(imports, strings.TrimSpace(parts[0]))
			}
		}
	}
	return dedupeStrings(imports)
}

func pythonImportCandidate(importPath, changedDir string, rootDirs map[string]struct{}) localImportTarget {
	importPath = strings.TrimSpace(importPath)
	if importPath == "" {
		return localImportTarget{}
	}
	if strings.HasPrefix(importPath, ".") {
		for strings.HasPrefix(importPath, ".") {
			importPath = strings.TrimPrefix(importPath, ".")
		}
		if importPath == "" {
			return localImportTarget{Dirs: []string{changedDir}}
		}
		parts := strings.Split(importPath, ".")
		if len(parts) == 1 {
			return localImportTarget{Files: []string{path.Join(changedDir, parts[0]+".py")}}
		}
		dir := path.Join(append([]string{changedDir}, parts[:len(parts)-1]...)...)
		return localImportTarget{Files: []string{path.Join(dir, parts[len(parts)-1]+".py")}}
	}
	parts := strings.Split(importPath, ".")
	if len(parts) == 0 || parts[0] == "" {
		return localImportTarget{}
	}
	changedTop := strings.Split(strings.Trim(changedDir, "/"), "/")[0]
	_, knownRoot := rootDirs[parts[0]]
	if !knownRoot && parts[0] != changedTop {
		return localImportTarget{}
	}
	if len(parts) == 1 {
		return localImportTarget{Dirs: []string{parts[0]}}
	}
	dir := path.Join(parts[:len(parts)-1]...)
	return localImportTarget{Files: []string{path.Join(dir, parts[len(parts)-1]+".py")}}
}

func pythonDefinedSymbols(content string) []string {
	var out []string
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "def "):
			if name := pythonNameAfterKeyword(line, "def "); name != "" {
				out = append(out, name)
			}
		case strings.HasPrefix(line, "async def "):
			if name := pythonNameAfterKeyword(line, "async def "); name != "" {
				out = append(out, name)
			}
		case strings.HasPrefix(line, "class "):
			if name := pythonNameAfterKeyword(line, "class "); name != "" {
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return dedupeStrings(out)
}

func pythonNameAfterKeyword(line, keyword string) string {
	name := strings.TrimSpace(strings.TrimPrefix(line, keyword))
	for i, r := range name {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9') {
			return name[:i]
		}
	}
	return name
}

func pythonContentReferencesAnySymbol(content string, symbols map[string]struct{}) bool {
	for symbol := range symbols {
		if containsIdentifier(content, symbol) {
			return true
		}
	}
	return false
}

func containsIdentifier(content, symbol string) bool {
	if symbol == "" {
		return false
	}
	for start := 0; ; {
		idx := strings.Index(content[start:], symbol)
		if idx < 0 {
			return false
		}
		idx += start
		beforeOK := idx == 0 || !isIdentifierRune(rune(content[idx-1]))
		after := idx + len(symbol)
		afterOK := after >= len(content) || !isIdentifierRune(rune(content[after]))
		if beforeOK && afterOK {
			return true
		}
		start = idx + len(symbol)
	}
}

func isIdentifierRune(r rune) bool {
	return r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

func parseGoImports(content string) []string {
	var imports []string
	inBlock := false
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "import (") {
			inBlock = true
			continue
		}
		if inBlock && line == ")" {
			inBlock = false
			continue
		}
		if strings.HasPrefix(line, "import ") {
			if imp := quotedImport(strings.TrimSpace(strings.TrimPrefix(line, "import "))); imp != "" {
				imports = append(imports, imp)
			}
			continue
		}
		if inBlock {
			if imp := quotedImport(line); imp != "" {
				imports = append(imports, imp)
			}
		}
	}
	return dedupeStrings(imports)
}

func quotedImport(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "//") || line == "" {
		return ""
	}
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ""
	}
	candidate := parts[len(parts)-1]
	candidate = strings.Trim(candidate, "\"")
	if candidate == "" || strings.Contains(candidate, "`") {
		return ""
	}
	return candidate
}

func dedupeStrings(values []string) []string {
	out := values[:0]
	seen := map[string]struct{}{}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func relatedTestCandidates(ctx context.Context, reader RepositoryReader, job Job, files []FileChange, budgets ContextBudgets) ([]string, []OmittedContext) {
	directSet := map[string]struct{}{}
	packages := map[string]struct{}{}
	for _, f := range sortedFileChanges(files) {
		if omitForChangedFile(f) != "" || !isReviewableSourceFile(f.Filename) || isTestSourceFile(f.Filename) {
			continue
		}
		dir, base := path.Split(f.Filename)
		switch {
		case strings.HasSuffix(base, ".go"):
			directSet[strings.TrimSuffix(dir+strings.TrimSuffix(base, ".go")+"_test.go", "/")] = struct{}{}
		case strings.HasSuffix(base, ".py"):
			name := strings.TrimSuffix(base, ".py")
			directSet[strings.TrimSuffix(dir+"test_"+name+".py", "/")] = struct{}{}
			directSet[strings.TrimSuffix(dir+name+"_test.py", "/")] = struct{}{}
			directSet["tests/test_"+name+".py"] = struct{}{}
		}
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
			if entry.Type != RepositoryEntryFile || !isTestSourceFile(entry.Path) {
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

func docsCandidates(ctx context.Context, reader RepositoryReader, job Job, files []FileChange, budgets ContextBudgets) ([]string, []OmittedContext) {
	out := []string{}
	var omitted []OmittedContext
	keywords := docRelevanceKeywords(files)
	includedDocs := 0
	if relevantDoc(ctx, reader, job, "README.md", keywords) {
		out = append(out, "README.md")
		includedDocs++
	}
	entries, err := reader.ListDirectory(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, "docs")
	if err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
		for _, entry := range entries {
			if entry.Type != RepositoryEntryFile || !strings.HasSuffix(strings.ToLower(entry.Path), ".md") {
				continue
			}
			if !relevantDoc(ctx, reader, job, entry.Path, keywords) {
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

func docRelevanceKeywords(files []FileChange) []string {
	keywords := []string{"auth", "security", "validate", "validation", "permission", "token", "secret", "password", "payment", "migration", "config", "api", "schema", "user"}
	for _, f := range files {
		pathParts := strings.FieldsFunc(strings.ToLower(f.Filename), func(r rune) bool {
			return r == '/' || r == '.' || r == '-' || r == '_' || r == ' '
		})
		keywords = append(keywords, pathParts...)
		for _, token := range strings.FieldsFunc(strings.ToLower(f.Patch), func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
		}) {
			if len(token) >= 4 {
				keywords = append(keywords, token)
			}
		}
	}
	sort.Strings(keywords)
	return dedupeStrings(keywords)
}

func relevantDoc(ctx context.Context, reader RepositoryReader, job Job, filePath string, keywords []string) bool {
	content, err := reader.FetchFileContent(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA, filePath)
	if err != nil || content == "" {
		return false
	}
	lowerPath := strings.ToLower(filePath)
	lowerContent := strings.ToLower(content)
	for _, keyword := range keywords {
		if len(keyword) < 3 {
			continue
		}
		if strings.Contains(lowerPath, keyword) || strings.Contains(lowerContent, keyword) {
			return true
		}
	}
	return false
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
	if b.MaxRelatedSources <= 0 {
		b.MaxRelatedSources = DefaultContextBudgets.MaxRelatedSources
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

func isReviewableSourceFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".py")
}

func isTestSourceFile(filePath string) bool {
	lower := strings.ToLower(path.Base(filePath))
	return strings.HasSuffix(lower, "_test.go") || (strings.HasPrefix(lower, "test_") && strings.HasSuffix(lower, ".py")) || strings.HasSuffix(lower, "_test.py")
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
	return RenderPromptWithLanguage(job, ctx, LanguageEnglish)
}

func RenderPromptWithLanguage(job Job, ctx RepoContext, language Language) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Review pull request %s/%s#%d at head %s.\n", job.Owner, job.Repo, job.PullNumber, job.HeadSHA)
	fmt.Fprintf(&b, "Webhook action: %s.\n", job.Action)
	b.WriteString("Return JSON-only output matching this schema: {\"summary\": string, \"risk_score\": integer 0-100, \"findings\": [{\"severity\": \"blocker|warning|suggestion|question\", \"category\": string, \"file\": string, \"line\": integer, \"title\": string, \"evidence\": string, \"failure_scenario\": string, \"suggestion\": string, \"confidence\": number 0.0-1.0}], \"missing_tests\": [string], \"limitations\": [string]}.\n")
	b.WriteString("Findings are advisory and non-blocking. Be concise and evidence-based. Use only the context below; if context is insufficient, record the limitation instead of fabricating unavailable context.\n\n")
	if language == LanguageSimplifiedChinese {
		b.WriteString("Write all human-readable JSON string values in Simplified Chinese, including summary, finding title, evidence, failure_scenario, suggestion, missing_tests, and limitations. Keep JSON keys and severity enum values exactly as specified in English.\n\n")
	}
	renderPatchSection(&b, ctx.Patches)
	renderFileSection(&b, SectionFullFile, ctx.FullFiles)
	renderFileSection(&b, SectionRelatedSrc, ctx.RelatedSources)
	renderFileSection(&b, SectionRelatedTest, ctx.RelatedTests)
	renderFileSection(&b, SectionRepoDocs, ctx.RepoDocs)
	renderStaticCheckSection(&b, ctx.StaticChecks)
	renderOmittedSection(&b, ctx.Omitted)
	return b.String()
}

func BuildPromptWithContext(job Job, ctx RepoContext) string {
	return RenderPrompt(job, ctx)
}

func BuildPromptWithContextAndLanguage(job Job, ctx RepoContext, language Language) string {
	return RenderPromptWithLanguage(job, ctx, language)
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
