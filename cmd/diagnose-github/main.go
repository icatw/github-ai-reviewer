package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github-ai-reviewer/internal/config"
	"github-ai-reviewer/internal/githubapp"
	"github-ai-reviewer/internal/review"
)

type checkResult struct {
	Name     string
	Status   string
	Detail   string
	Advisory bool
}

func main() {
	envFile := flag.String("env-file", ".env.production", "environment file to load before reading process env")
	owner := flag.String("owner", "", "repository owner")
	repo := flag.String("repo", "", "repository name")
	pull := flag.Int("pull", 0, "pull request number")
	flag.Parse()

	if *owner == "" || *repo == "" || *pull <= 0 {
		fmt.Fprintln(os.Stderr, "usage: diagnose-github -env-file .env.production -owner OWNER -repo REPO -pull NUMBER")
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

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	fmt.Printf("GitHub App diagnostics for %s/%s#%d\n", *owner, *repo, *pull)
	fmt.Printf("app_id: %d\n", cfg.GitHub.AppID)

	installation, ok := diagnoseInstallation(ctx, client, *owner, *repo)
	if !ok {
		os.Exit(1)
	}
	failed := false
	headSHA, prResult := diagnosePullMetadata(ctx, client, installation.ID, *owner, *repo, *pull)
	for _, result := range []checkResult{
		diagnoseToken(ctx, client, installation.ID),
		prResult,
		diagnoseFiles(ctx, client, installation.ID, *owner, *repo, *pull),
		diagnoseIssueComments(ctx, client, installation.ID, *owner, *repo, *pull),
		diagnoseCheckRuns(ctx, client, installation.ID, *owner, *repo, headSHA),
	} {
		printResult(result)
		if result.Status != "ok" && !result.Advisory {
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func diagnoseInstallation(ctx context.Context, client *githubapp.Client, owner, repo string) (githubapp.Installation, bool) {
	installation, err := client.ResolveRepositoryInstallation(ctx, owner, repo)
	if err != nil {
		printResult(checkResult{Name: "repository installation", Status: "fail", Detail: explainGitHubError(err, "App is not installed on this repository, the repository is private and unavailable to the App, or the App ID/private key is wrong.")})
		return githubapp.Installation{}, false
	}
	detail := fmt.Sprintf("installation_id=%d account=%s repository_selection=%s", installation.ID, fallback(installation.AccountLogin, "unknown"), fallback(installation.RepositorySelection, "unknown"))
	printResult(checkResult{Name: "repository installation", Status: "ok", Detail: detail})
	return installation, true
}

func diagnoseToken(ctx context.Context, client *githubapp.Client, installationID int64) checkResult {
	_, err := client.InstallationToken(ctx, installationID)
	if err != nil {
		return checkResult{Name: "installation token", Status: "fail", Detail: explainGitHubError(err, "Cannot exchange installation token. Check installation_id and App private key.")}
	}
	return checkResult{Name: "installation token", Status: "ok", Detail: "token exchange succeeded"}
}

func diagnosePullMetadata(ctx context.Context, client *githubapp.Client, installationID int64, owner, repo string, pull int) (string, checkResult) {
	headSHA, err := client.ResolvePullRequestHeadSHA(ctx, installationID, owner, repo, pull)
	if err != nil {
		return "", checkResult{Name: "pull request metadata", Status: "fail", Detail: explainGitHubError(err, "Requires Pull requests: read and repository installation access.")}
	}
	return headSHA, checkResult{Name: "pull request metadata", Status: "ok", Detail: "head_sha=" + shortSHA(headSHA)}
}

func diagnoseFiles(ctx context.Context, client *githubapp.Client, installationID int64, owner, repo string, pull int) checkResult {
	files, err := client.FetchPullRequestFiles(ctx, installationID, owner, repo, pull)
	if err != nil {
		return checkResult{Name: "pull request files", Status: "fail", Detail: explainGitHubError(err, "Requires Pull requests: read and Contents: read for the installed repository.")}
	}
	return checkResult{Name: "pull request files", Status: "ok", Detail: fmt.Sprintf("changed_files=%d", len(files))}
}

func diagnoseIssueComments(ctx context.Context, client *githubapp.Client, installationID int64, owner, repo string, pull int) checkResult {
	_, err := client.ListIssueComments(ctx, installationID, owner, repo, pull)
	if err != nil {
		return checkResult{Name: "issue comments", Status: "fail", Detail: explainGitHubError(err, "Requires Issues: read/write because PR conversation comments use the Issues API.")}
	}
	return checkResult{Name: "issue comments", Status: "ok", Detail: "comments API reachable"}
}

func diagnoseCheckRuns(ctx context.Context, client *githubapp.Client, installationID int64, owner, repo, headSHA string) checkResult {
	if strings.TrimSpace(headSHA) == "" {
		return checkResult{Name: "check runs", Status: "warn", Detail: "skipped because pull request metadata failed", Advisory: true}
	}
	_, err := client.ListCheckRuns(ctx, installationID, owner, repo, headSHA)
	if err != nil {
		return checkResult{Name: "check runs", Status: "warn", Detail: explainGitHubError(err, "Checks: read/write may be missing. Review comments can still work; set CHECK_RUN_ENABLED=false to skip Check Runs."), Advisory: true}
	}
	return checkResult{Name: "check runs", Status: "ok", Detail: "check runs API reachable"}
}

func explainGitHubError(err error, hint string) string {
	var httpErr githubapp.HTTPError
	if errors.As(err, &httpErr) {
		return fmt.Sprintf("status=%d category=%s hint=%s", httpErr.Status, httpErr.Category(), hint)
	}
	if errors.Is(err, review.ErrRepositoryContentNotFound) {
		return "category=repository_content_not_found hint=" + hint
	}
	return "category=unknown hint=" + hint
}

func printResult(result checkResult) {
	fmt.Printf("[%s] %s: %s\n", result.Status, result.Name, result.Detail)
}

func loadEnvFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

func fallback(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}

func shortSHA(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

func init() {
	flag.CommandLine.SetOutput(os.Stderr)
}
