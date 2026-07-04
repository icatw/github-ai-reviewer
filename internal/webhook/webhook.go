package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github-ai-reviewer/internal/review"
)

type ParseResult struct {
	Ignored bool
	Job     *review.Job
	Cleanup *review.CleanupJob
	Command *ReviewCommand
}

type ReviewCommand struct {
	InstallationID int64
	Owner          string
	Repo           string
	PullNumber     int
	Action         string
	DeliveryID     string
}

func VerifySignature(secret string, body []byte, signature string) error {
	const prefix = "sha256="
	if secret == "" {
		return errors.New("webhook secret is required")
	}
	if !strings.HasPrefix(signature, prefix) {
		return errors.New("missing or malformed signature")
	}
	got, err := hex.DecodeString(strings.TrimPrefix(signature, prefix))
	if err != nil {
		return errors.New("malformed signature")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := mac.Sum(nil)
	if !hmac.Equal(got, want) {
		return errors.New("signature mismatch")
	}
	return nil
}

func ParseDelivery(event, deliveryID string, body []byte) (ParseResult, error) {
	switch event {
	case "pull_request":
		return parsePullRequestDelivery(deliveryID, body)
	case "issue_comment":
		return parseIssueCommentDelivery(deliveryID, body)
	default:
		return ParseResult{Ignored: true}, nil
	}
}

func parsePullRequestDelivery(deliveryID string, body []byte) (ParseResult, error) {
	var payload pullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ParseResult{}, fmt.Errorf("parse pull_request payload: %w", err)
	}
	if payload.Action == "closed" {
		cleanup := review.CleanupJob{
			InstallationID: payload.Installation.ID,
			Owner:          payload.Repository.Owner.Login,
			Repo:           payload.Repository.Name,
			PullNumber:     payload.PullRequest.Number,
			HeadSHA:        payload.PullRequest.Head.SHA,
			Action:         payload.Action,
			DeliveryID:     deliveryID,
			State:          cleanupState(payload.PullRequest.Merged.Value),
		}
		if err := validateCleanupJob(cleanup, payload.PullRequest.Merged.Set); err != nil {
			return ParseResult{}, err
		}
		return ParseResult{Cleanup: &cleanup}, nil
	}
	if !isSupportedAction(payload.Action) {
		return ParseResult{Ignored: true}, nil
	}
	job := review.Job{
		InstallationID: payload.Installation.ID,
		Owner:          payload.Repository.Owner.Login,
		Repo:           payload.Repository.Name,
		PullNumber:     payload.PullRequest.Number,
		HeadSHA:        payload.PullRequest.Head.SHA,
		Action:         payload.Action,
		DeliveryID:     deliveryID,
	}
	if err := validateJob(job); err != nil {
		return ParseResult{}, err
	}
	return ParseResult{Job: &job}, nil
}

func parseIssueCommentDelivery(deliveryID string, body []byte) (ParseResult, error) {
	var payload issueCommentPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ParseResult{}, fmt.Errorf("parse issue_comment payload: %w", err)
	}
	if payload.Action != "created" {
		return ParseResult{Ignored: true}, nil
	}
	if payload.Issue.PullRequest.URL == "" {
		return ParseResult{Ignored: true}, nil
	}
	if !isReviewCommand(payload.Comment.Body) {
		return ParseResult{Ignored: true}, nil
	}
	command := ReviewCommand{
		InstallationID: payload.Installation.ID,
		Owner:          payload.Repository.Owner.Login,
		Repo:           payload.Repository.Name,
		PullNumber:     payload.Issue.Number,
		Action:         payload.Action,
		DeliveryID:     deliveryID,
	}
	if err := validateCommand(command); err != nil {
		return ParseResult{}, err
	}
	return ParseResult{Command: &command}, nil
}

func isSupportedAction(action string) bool {
	switch action {
	case "opened", "synchronize", "reopened":
		return true
	default:
		return false
	}
}

func cleanupState(merged bool) review.CleanupState {
	if merged {
		return review.CleanupStateMerged
	}
	return review.CleanupStateClosed
}

func isReviewCommand(body string) bool {
	if body == "/ai-review" {
		return true
	}
	if strings.HasPrefix(body, "/ai-review") && len(body) > len("/ai-review") {
		return strings.ContainsAny(body[len("/ai-review"):len("/ai-review")+1], " \t\r\n")
	}
	return false
}

func validateJob(job review.Job) error {
	var missing []string
	if job.InstallationID == 0 {
		missing = append(missing, "installation.id")
	}
	if job.Owner == "" {
		missing = append(missing, "repository.owner.login")
	}
	if job.Repo == "" {
		missing = append(missing, "repository.name")
	}
	if job.PullNumber == 0 {
		missing = append(missing, "pull_request.number")
	}
	if job.HeadSHA == "" {
		missing = append(missing, "pull_request.head.sha")
	}
	if job.DeliveryID == "" {
		missing = append(missing, "X-GitHub-Delivery")
	}
	if len(missing) > 0 {
		return errors.New("missing required webhook fields: " + strings.Join(missing, ", "))
	}
	return nil
}

func validateCleanupJob(job review.CleanupJob, mergedSet bool) error {
	var missing []string
	if job.InstallationID == 0 {
		missing = append(missing, "installation.id")
	}
	if job.Owner == "" {
		missing = append(missing, "repository.owner.login")
	}
	if job.Repo == "" {
		missing = append(missing, "repository.name")
	}
	if job.PullNumber == 0 {
		missing = append(missing, "pull_request.number")
	}
	if job.HeadSHA == "" {
		missing = append(missing, "pull_request.head.sha")
	}
	if !mergedSet {
		missing = append(missing, "pull_request.merged")
	}
	if job.DeliveryID == "" {
		missing = append(missing, "X-GitHub-Delivery")
	}
	if len(missing) > 0 {
		return errors.New("missing required webhook fields: " + strings.Join(missing, ", "))
	}
	return nil
}

func validateCommand(command ReviewCommand) error {
	var missing []string
	if command.InstallationID == 0 {
		missing = append(missing, "installation.id")
	}
	if command.Owner == "" {
		missing = append(missing, "repository.owner.login")
	}
	if command.Repo == "" {
		missing = append(missing, "repository.name")
	}
	if command.PullNumber == 0 {
		missing = append(missing, "issue.number")
	}
	if command.DeliveryID == "" {
		missing = append(missing, "X-GitHub-Delivery")
	}
	if len(missing) > 0 {
		return errors.New("missing required webhook fields: " + strings.Join(missing, ", "))
	}
	return nil
}

type pullRequestPayload struct {
	Action       string `json:"action"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	PullRequest struct {
		Number int `json:"number"`
		Head   struct {
			SHA string `json:"sha"`
		} `json:"head"`
		Merged boolField `json:"merged"`
	} `json:"pull_request"`
}

type boolField struct {
	Value bool
	Set   bool
}

func (b *boolField) UnmarshalJSON(data []byte) error {
	var value bool
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	b.Value = value
	b.Set = true
	return nil
}

type issueCommentPayload struct {
	Action       string `json:"action"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Issue struct {
		Number      int `json:"number"`
		PullRequest struct {
			URL string `json:"url"`
		} `json:"pull_request"`
	} `json:"issue"`
	Comment struct {
		Body string `json:"body"`
	} `json:"comment"`
}
