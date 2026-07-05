package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github-ai-reviewer/internal/config"
	"github-ai-reviewer/internal/githubapp"
	"github-ai-reviewer/internal/review"
	"github-ai-reviewer/internal/reviewbench"
)

type recordingReader struct {
	reader review.RepositoryReader
	files  map[string]string
	dirs   map[string][]review.RepositoryEntry
}

func main() {
	envFile := flag.String("env-file", ".env.production", "environment file to load before reading process env")
	owner := flag.String("owner", "", "repository owner")
	repo := flag.String("repo", "", "repository name")
	pull := flag.Int("pull", 0, "pull request number")
	outPath := flag.String("out", "", "output fixture path; stdout when empty")
	flag.Parse()

	if *owner == "" || *repo == "" || *pull <= 0 {
		fmt.Fprintln(os.Stderr, "usage: review-bench-from-pr -env-file .env.production -owner OWNER -repo REPO -pull NUMBER [-out fixture.json]")
		os.Exit(2)
	}
	if err := loadEnvFile(*envFile); err != nil {
		fmt.Fprintf(os.Stderr, "load env file: %v\n", err)
		os.Exit(2)
	}
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(2)
	}
	privateKey, err := cfg.GitHub.PrivateKeyPEM()
	if err != nil {
		fmt.Fprintf(os.Stderr, "private key: %v\n", err)
		os.Exit(2)
	}
	client, err := githubapp.NewClient(cfg.GitHub.AppID, privateKey, "", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "github client: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	installation, err := client.ResolveRepositoryInstallation(ctx, *owner, *repo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve installation: %v\n", err)
		os.Exit(1)
	}
	headSHA, err := client.ResolvePullRequestHeadSHA(ctx, installation.ID, *owner, *repo, *pull)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve pull head: %v\n", err)
		os.Exit(1)
	}
	files, err := client.FetchPullRequestFiles(ctx, installation.ID, *owner, *repo, *pull)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetch pull files: %v\n", err)
		os.Exit(1)
	}

	recorder := &recordingReader{reader: client, files: map[string]string{}, dirs: map[string][]review.RepositoryEntry{}}
	job := review.Job{InstallationID: installation.ID, Owner: *owner, Repo: *repo, PullNumber: *pull, HeadSHA: headSHA, Action: "bench-from-pr"}
	review.BuildRepoContext(ctx, job, files, recorder, review.DefaultContextBudgets)

	fixture := reviewbench.Fixture{
		Name: fmt.Sprintf("%s-%s-pr-%d", sanitizeName(*owner), sanitizeName(*repo), *pull),
		Metadata: reviewbench.FixtureMetadata{
			Source:     "generated-real-pr",
			Provenance: fmt.Sprintf("%s/%s#%d", *owner, *repo, *pull),
			Sanitized:  false,
			Notes:      "Generated fixture; keep in a gitignored or temporary path until reviewed and sanitized.",
		},
		Job:       job,
		Files:     files,
		RepoFiles: recorder.files,
	}
	encoded, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode fixture: %v\n", err)
		os.Exit(1)
	}
	if *outPath == "" {
		fmt.Println(string(encoded))
		return
	}
	if err := os.MkdirAll(path.Dir(*outPath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(*outPath, append(encoded, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write fixture: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote unsanitized fixture %s changed_files=%d repo_files=%d directories=%d\n", *outPath, len(files), len(recorder.files), len(recorder.dirs))
	fmt.Fprintln(os.Stderr, "keep generated private fixtures in /tmp, data/, or another gitignored quarantine path until sanitized; set metadata.sanitized=true only after review")
}

func (r *recordingReader) FetchFileContent(ctx context.Context, installationID int64, owner, repo, ref, filePath string) (string, error) {
	content, err := r.reader.FetchFileContent(ctx, installationID, owner, repo, ref, filePath)
	if err == nil {
		r.files[cleanPath(filePath)] = content
	}
	return content, err
}

func (r *recordingReader) ListDirectory(ctx context.Context, installationID int64, owner, repo, ref, dirPath string) ([]review.RepositoryEntry, error) {
	entries, err := r.reader.ListDirectory(ctx, installationID, owner, repo, ref, dirPath)
	if err == nil {
		clean := cleanDir(dirPath)
		copyEntries := append([]review.RepositoryEntry(nil), entries...)
		sort.Slice(copyEntries, func(i, j int) bool { return copyEntries[i].Path < copyEntries[j].Path })
		r.dirs[clean] = copyEntries
	}
	return entries, err
}

func loadEnvFile(filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
	return nil
}

func cleanPath(filePath string) string {
	clean := path.Clean(strings.TrimPrefix(filePath, "/"))
	if clean == "." {
		return ""
	}
	return clean
}

func cleanDir(dir string) string {
	clean := cleanPath(dir)
	if clean == "." {
		return ""
	}
	return clean
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return strings.Trim(b.String(), "-")
}
