package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github-ai-reviewer/internal/review"
)

type Client struct {
	baseURL  string
	apiKey   string
	model    string
	language review.Language
	http     *http.Client
}

type ClientOptions struct {
	Language review.Language
}

func NewClient(baseURL, apiKey, model string, httpClient *http.Client) *Client {
	return NewClientWithOptions(baseURL, apiKey, model, httpClient, ClientOptions{})
}

func NewClientWithOptions(baseURL, apiKey, model string, httpClient *http.Client, opts ClientOptions) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}
	language := opts.Language
	if language == "" {
		language = review.LanguageEnglish
	}
	return &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		model:    model,
		language: language,
		http:     httpClient,
	}
}

func BuildPrompt(job review.Job, files []review.FileChange, maxPatchChars int) string {
	return review.BuildPrompt(job, files, maxPatchChars)
}

func (c *Client) Review(ctx context.Context, prompt string) (review.ReviewResult, error) {
	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt(c.language)},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqBody); err != nil {
		return review.ReviewResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", &buf)
	if err != nil {
		return review.ReviewResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return review.ReviewResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return review.ReviewResult{}, fmt.Errorf("llm request failed: status %d", resp.StatusCode)
	}
	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return review.ReviewResult{}, err
	}
	if len(out.Choices) == 0 {
		return review.ReviewResult{}, errorsUnusableResponse()
	}
	return review.ParseReviewResult(strings.TrimSpace(out.Choices[0].Message.Content))
}

func errorsUnusableResponse() error {
	return fmt.Errorf("llm response did not include choices")
}

func systemPrompt(language review.Language) string {
	base := "You are a conservative code reviewer. Return one JSON object only, with advisory and non-blocking findings based only on provided diff context."
	if language == review.LanguageSimplifiedChinese {
		return base + " Write all human-readable JSON string values in Simplified Chinese. Keep JSON keys and enum values in English exactly as requested."
	}
	return base
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}
