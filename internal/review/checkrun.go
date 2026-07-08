package review

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

const (
	CheckRunName                = "AI Review"
	CheckRunStatusInProgress    = "in_progress"
	CheckRunStatusCompleted     = "completed"
	CheckRunConclusionNeutral   = "neutral"
	CheckRunConclusionFailure   = "failure"
	checkRunReporterName        = "github_check_run"
	checkRunStartedTitle        = "AI review in progress"
	checkRunCompletedTitle      = "AI review completed"
	checkRunSuppressedTitle     = "AI review completed"
	checkRunInfrastructureTitle = "AI review failed"
)

type CheckRun struct {
	ID      int64
	Name    string
	HeadSHA string
	Status  string
}

type CheckRunOutput struct {
	Title   string
	Summary string
}

type CheckRunCreateRequest struct {
	Name       string
	HeadSHA    string
	Status     string
	Conclusion string
	Output     CheckRunOutput
}

type CheckRunUpdateRequest struct {
	Status     string
	Conclusion string
	Output     CheckRunOutput
}

type CheckRunClient interface {
	ListCheckRuns(ctx context.Context, installationID int64, owner, repo, ref string) ([]CheckRun, error)
	CreateCheckRun(ctx context.Context, installationID int64, owner, repo string, req CheckRunCreateRequest) (CheckRun, error)
	UpdateCheckRun(ctx context.Context, installationID int64, owner, repo string, id int64, req CheckRunUpdateRequest) error
}

type CheckRunReporter struct {
	client CheckRunClient
}

func NewCheckRunReporter(client CheckRunClient) *CheckRunReporter {
	return &CheckRunReporter{client: client}
}

func (r *CheckRunReporter) Name() string { return checkRunReporterName }

func (r *CheckRunReporter) JobStarted(ctx context.Context, job Job) error {
	req := CheckRunCreateRequest{
		Name:    CheckRunName,
		HeadSHA: job.HeadSHA,
		Status:  CheckRunStatusInProgress,
		Output: CheckRunOutput{
			Title:   checkRunStartedTitle,
			Summary: "AI review is processing this pull request. Findings are advisory and non-blocking.",
		},
	}
	if _, err := r.client.CreateCheckRun(ctx, job.InstallationID, job.Owner, job.Repo, req); err != nil {
		if shouldDegradeCheckRun(err) {
			return nil
		}
		return checkRunFailure(err)
	}
	return nil
}

func (r *CheckRunReporter) ReviewCompleted(ctx context.Context, job Job, result ReviewResult) error {
	req := CheckRunUpdateRequest{
		Status:     CheckRunStatusCompleted,
		Conclusion: CheckRunConclusionNeutral,
		Output: CheckRunOutput{
			Title:   checkRunCompletedTitle,
			Summary: completedSummary(result),
		},
	}
	return r.updateOrCreate(ctx, job, req)
}

func (r *CheckRunReporter) OutputSuppressed(ctx context.Context, job Job, result ReviewResult) error {
	req := CheckRunUpdateRequest{
		Status:     CheckRunStatusCompleted,
		Conclusion: CheckRunConclusionNeutral,
		Output: CheckRunOutput{
			Title:   checkRunSuppressedTitle,
			Summary: "AI review completed without enough useful output for a PR summary comment. The check is advisory and non-blocking.",
		},
	}
	return r.updateOrCreate(ctx, job, req)
}

func (r *CheckRunReporter) JobFailed(ctx context.Context, job Job, failure Failure) error {
	category := safeFailureCategory(failure.Category)
	req := CheckRunUpdateRequest{
		Status:     CheckRunStatusCompleted,
		Conclusion: CheckRunConclusionFailure,
		Output: CheckRunOutput{
			Title:   checkRunInfrastructureTitle,
			Summary: fmt.Sprintf("AI review could not complete due to a service execution failure. Category: `%s`. This failure is infrastructure-related and not based on AI findings.", category),
		},
	}
	return r.updateOrCreate(ctx, job, req)
}

func (r *CheckRunReporter) upsert(ctx context.Context, job Job, create CheckRunCreateRequest, update CheckRunUpdateRequest) error {
	run, ok, err := r.match(ctx, job)
	if err != nil {
		if shouldDegradeCheckRun(err) {
			return nil
		}
		return checkRunFailure(err)
	}
	if ok {
		if err := r.client.UpdateCheckRun(ctx, job.InstallationID, job.Owner, job.Repo, run.ID, update); err != nil {
			if shouldDegradeCheckRun(err) {
				return nil
			}
			return checkRunFailure(err)
		}
		return nil
	}
	if _, err := r.client.CreateCheckRun(ctx, job.InstallationID, job.Owner, job.Repo, create); err != nil {
		if shouldDegradeCheckRun(err) {
			return nil
		}
		return checkRunFailure(err)
	}
	return nil
}

func (r *CheckRunReporter) updateOrCreate(ctx context.Context, job Job, update CheckRunUpdateRequest) error {
	create := CheckRunCreateRequest{
		Name:       CheckRunName,
		HeadSHA:    job.HeadSHA,
		Status:     update.Status,
		Conclusion: update.Conclusion,
		Output:     update.Output,
	}
	return r.upsert(ctx, job, create, update)
}

func (r *CheckRunReporter) match(ctx context.Context, job Job) (CheckRun, bool, error) {
	runs, err := r.client.ListCheckRuns(ctx, job.InstallationID, job.Owner, job.Repo, job.HeadSHA)
	if err != nil {
		return CheckRun{}, false, err
	}
	for i := len(runs) - 1; i >= 0; i-- {
		if runs[i].Name == CheckRunName && runs[i].HeadSHA == job.HeadSHA && runs[i].ID != 0 && runs[i].Status == CheckRunStatusInProgress {
			return runs[i], true, nil
		}
	}
	return CheckRun{}, false, nil
}

func checkRunFailure(err error) ReporterFailure {
	return ReporterFailure{Reporter: checkRunReporterName, Category: FailureCategoryGitHub}
}

func shouldDegradeCheckRun(err error) bool {
	var statusErr GitHubStatusClassifier
	if !errors.As(err, &statusErr) {
		return false
	}
	switch statusErr.GitHubStatusCode() {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return true
	default:
		return false
	}
}

func completedSummary(result ReviewResult) string {
	summary := "AI review completed. Findings are advisory and non-blocking."
	if len(result.Findings) > 0 {
		summary += fmt.Sprintf(" The review found %d advisory item(s); see the PR summary comment for details.", len(result.Findings))
	}
	if len(result.MissingTests) > 0 {
		summary += fmt.Sprintf(" It also noted %d missing test item(s).", len(result.MissingTests))
	}
	if len(result.Limitations) > 0 {
		summary += fmt.Sprintf(" It recorded %d limitation(s).", len(result.Limitations))
	}
	return summary
}

func safeFailureCategory(category string) string {
	switch category {
	case FailureCategoryGitHub, FailureCategoryLLM, FailureCategoryReporter:
		return category
	default:
		return "job_execution_error"
	}
}
