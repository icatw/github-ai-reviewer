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
	if event != "pull_request" {
		return ParseResult{Ignored: true}, nil
	}
	var payload pullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ParseResult{}, fmt.Errorf("parse pull_request payload: %w", err)
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

func isSupportedAction(action string) bool {
	switch action {
	case "opened", "synchronize", "reopened":
		return true
	default:
		return false
	}
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
	} `json:"pull_request"`
}
