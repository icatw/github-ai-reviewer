package githubapp

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github-ai-reviewer/internal/comment"
	"github-ai-reviewer/internal/review"
)

type Auth struct {
	appID int64
	key   *rsa.PrivateKey
	now   func() time.Time
}

func NewAuth(appID int64, privateKeyPEM string) (*Auth, error) {
	key, err := parsePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	return &Auth{appID: appID, key: key, now: time.Now}, nil
}

func (a *Auth) JWT() (string, error) {
	now := a.now()
	claims := jwt.RegisteredClaims{
		Issuer:    fmt.Sprintf("%d", a.appID),
		IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(a.key)
}

type Client struct {
	auth      *Auth
	baseURL   string
	http      *http.Client
	tokenLock sync.Mutex
	tokens    map[int64]string
}

type Installation struct {
	ID                  int64
	AccountLogin        string
	RepositorySelection string
}

type HTTPError struct {
	Method string
	Path   string
	Status int
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("github request failed: %s %s status %d category=%s", e.Method, e.Path, e.Status, e.Category())
}

func (e HTTPError) GitHubStatusCode() int {
	return e.Status
}

func (e HTTPError) Category() string {
	switch e.Status {
	case http.StatusUnauthorized:
		return "github_unauthorized"
	case http.StatusForbidden:
		return "github_forbidden"
	case http.StatusNotFound:
		return "github_not_found"
	default:
		if e.Status >= 500 {
			return "github_server_error"
		}
		return "github_http_error"
	}
}

func IsHTTPStatus(err error, statuses ...int) bool {
	var httpErr HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	for _, status := range statuses {
		if httpErr.Status == status {
			return true
		}
	}
	return false
}

func NewClient(appID int64, privateKeyPEM, baseURL string, httpClient *http.Client) (*Client, error) {
	auth, err := NewAuth(appID, privateKeyPEM)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{auth: auth, baseURL: strings.TrimRight(baseURL, "/"), http: httpClient, tokens: map[int64]string{}}, nil
}

func (c *Client) ResolveRepositoryInstallation(ctx context.Context, owner, repo string) (Installation, error) {
	appJWT, err := c.auth.JWT()
	if err != nil {
		return Installation{}, err
	}
	path := fmt.Sprintf("/repos/%s/%s/installation", owner, repo)
	var out githubInstallation
	if err := c.doJSON(ctx, http.MethodGet, path, appJWT, nil, &out, http.StatusOK); err != nil {
		return Installation{}, err
	}
	if out.ID == 0 {
		return Installation{}, errors.New("repository installation response missing id")
	}
	return Installation{ID: out.ID, AccountLogin: out.Account.Login, RepositorySelection: out.RepositorySelection}, nil
}

func (c *Client) FetchPullRequestFiles(ctx context.Context, installationID int64, owner, repo string, pullNumber int) ([]review.FileChange, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files", owner, repo, pullNumber)
	var files []githubFile
	if err := c.doJSON(ctx, http.MethodGet, path, token, nil, &files, http.StatusOK); err != nil {
		return nil, err
	}
	out := make([]review.FileChange, 0, len(files))
	for _, f := range files {
		out = append(out, review.FileChange{
			Filename:  f.Filename,
			Status:    f.Status,
			Additions: f.Additions,
			Deletions: f.Deletions,
			Patch:     f.Patch,
		})
	}
	return out, nil
}

func (c *Client) ResolvePullRequestHeadSHA(ctx context.Context, installationID int64, owner, repo string, pullNumber int) (string, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, pullNumber)
	var out githubPullRequest
	if err := c.doJSON(ctx, http.MethodGet, path, token, nil, &out, http.StatusOK); err != nil {
		return "", err
	}
	if out.Head.SHA == "" {
		return "", errors.New("pull request response missing head sha")
	}
	return out.Head.SHA, nil
}

func (c *Client) FetchFileContent(ctx context.Context, installationID int64, owner, repo, ref, filePath string) (string, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return "", err
	}
	apiPath := contentPath(owner, repo, filePath, ref)
	var out githubContent
	if err := c.doJSON(ctx, http.MethodGet, apiPath, token, nil, &out, http.StatusOK); err != nil {
		if isNotFound(err) {
			return "", review.ErrRepositoryContentNotFound
		}
		return "", err
	}
	if out.Type != "file" {
		return "", review.ErrRepositoryContentNotFound
	}
	if out.Encoding != "base64" {
		return "", fmt.Errorf("github content encoding %q is unsupported", out.Encoding)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(out.Content, "\n", ""))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func (c *Client) ListDirectory(ctx context.Context, installationID int64, owner, repo, ref, dirPath string) ([]review.RepositoryEntry, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	apiPath := contentPath(owner, repo, dirPath, ref)
	var contents []githubContent
	if err := c.doJSON(ctx, http.MethodGet, apiPath, token, nil, &contents, http.StatusOK); err != nil {
		if isNotFound(err) {
			return nil, review.ErrRepositoryContentNotFound
		}
		return nil, err
	}
	out := make([]review.RepositoryEntry, 0, len(contents))
	for _, item := range contents {
		entryType := review.RepositoryEntryType(item.Type)
		out = append(out, review.RepositoryEntry{Path: item.Path, Type: entryType})
	}
	return out, nil
}

func (c *Client) CreateIssueComment(ctx context.Context, installationID int64, owner, repo string, number int, body string) error {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	return c.doJSON(ctx, http.MethodPost, path, token, map[string]string{"body": body}, nil, http.StatusCreated)
}

func (c *Client) ListIssueComments(ctx context.Context, installationID int64, owner, repo string, number int) ([]comment.IssueComment, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	var comments []githubIssueComment
	if err := c.doJSON(ctx, http.MethodGet, path, token, nil, &comments, http.StatusOK); err != nil {
		return nil, err
	}
	out := make([]comment.IssueComment, 0, len(comments))
	for _, issueComment := range comments {
		out = append(out, comment.IssueComment{ID: issueComment.ID, Body: issueComment.Body, AuthorType: issueComment.User.Type})
	}
	return out, nil
}

func (c *Client) UpdateIssueComment(ctx context.Context, installationID int64, owner, repo string, commentID int64, body string) error {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/comments/%d", owner, repo, commentID)
	return c.doJSON(ctx, http.MethodPatch, path, token, map[string]string{"body": body}, nil, http.StatusOK)
}

func (c *Client) ListPullRequestReviewComments(ctx context.Context, installationID int64, owner, repo string, pullNumber int) ([]comment.ReviewComment, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, pullNumber)
	var comments []githubReviewComment
	if err := c.doJSON(ctx, http.MethodGet, path, token, nil, &comments, http.StatusOK); err != nil {
		return nil, err
	}
	out := make([]comment.ReviewComment, 0, len(comments))
	for _, reviewComment := range comments {
		out = append(out, comment.ReviewComment{ID: reviewComment.ID, Body: reviewComment.Body, AuthorType: reviewComment.User.Type})
	}
	return out, nil
}

func (c *Client) CreatePullRequestReviewComment(ctx context.Context, installationID int64, owner, repo string, pullNumber int, req comment.ReviewCommentRequest) error {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/comments", owner, repo, pullNumber)
	in := githubReviewCommentRequest{
		Body:     req.Body,
		CommitID: req.CommitID,
		Path:     req.Path,
		Line:     req.Line,
		Side:     req.Side,
	}
	return c.doJSON(ctx, http.MethodPost, path, token, in, nil, http.StatusCreated)
}

func (c *Client) CreatePullRequestReview(ctx context.Context, installationID int64, owner, repo string, pullNumber int, req comment.PullRequestReviewRequest) (comment.PullRequestReview, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return comment.PullRequestReview{}, err
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo, pullNumber)
	comments := make([]githubPullRequestReviewCommentRequest, 0, len(req.Comments))
	for _, reviewComment := range req.Comments {
		comments = append(comments, githubPullRequestReviewCommentRequest{
			Path: reviewComment.Path,
			Line: reviewComment.Line,
			Side: reviewComment.Side,
			Body: reviewComment.Body,
		})
	}
	in := githubPullRequestReviewRequest{
		CommitID: req.CommitID,
		Body:     req.Body,
		Event:    req.Event,
		Comments: comments,
	}
	var out githubPullRequestReview
	if err := c.doJSON(ctx, http.MethodPost, path, token, in, &out, http.StatusCreated, http.StatusOK); err != nil {
		return comment.PullRequestReview{}, err
	}
	return comment.PullRequestReview{ID: out.ID}, nil
}

func (c *Client) UpdatePullRequestReviewComment(ctx context.Context, installationID int64, owner, repo string, commentID int64, body string) error {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/comments/%d", owner, repo, commentID)
	return c.doJSON(ctx, http.MethodPatch, path, token, map[string]string{"body": body}, nil, http.StatusOK)
}

func (c *Client) ListCheckRuns(ctx context.Context, installationID int64, owner, repo, ref string) ([]review.CheckRun, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?check_name=%s", owner, repo, ref, urlQueryEscape(review.CheckRunName))
	var out struct {
		CheckRuns []githubCheckRun `json:"check_runs"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, token, nil, &out, http.StatusOK); err != nil {
		return nil, err
	}
	runs := make([]review.CheckRun, 0, len(out.CheckRuns))
	for _, run := range out.CheckRuns {
		runs = append(runs, review.CheckRun{ID: run.ID, Name: run.Name, HeadSHA: run.HeadSHA})
	}
	return runs, nil
}

func (c *Client) CreateCheckRun(ctx context.Context, installationID int64, owner, repo string, req review.CheckRunCreateRequest) (review.CheckRun, error) {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return review.CheckRun{}, err
	}
	path := fmt.Sprintf("/repos/%s/%s/check-runs", owner, repo)
	var out githubCheckRun
	in := githubCheckRunRequest{
		Name:       req.Name,
		HeadSHA:    req.HeadSHA,
		Status:     req.Status,
		Conclusion: req.Conclusion,
		Output:     githubCheckRunOutput(req.Output),
	}
	if err := c.doJSON(ctx, http.MethodPost, path, token, in, &out, http.StatusCreated); err != nil {
		return review.CheckRun{}, err
	}
	return review.CheckRun{ID: out.ID, Name: out.Name, HeadSHA: out.HeadSHA}, nil
}

func (c *Client) UpdateCheckRun(ctx context.Context, installationID int64, owner, repo string, id int64, req review.CheckRunUpdateRequest) error {
	token, err := c.installationToken(ctx, installationID)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/repos/%s/%s/check-runs/%d", owner, repo, id)
	in := githubCheckRunRequest{
		Status:     req.Status,
		Conclusion: req.Conclusion,
		Output:     githubCheckRunOutput(req.Output),
	}
	return c.doJSON(ctx, http.MethodPatch, path, token, in, nil, http.StatusOK)
}

func (c *Client) installationToken(ctx context.Context, installationID int64) (string, error) {
	c.tokenLock.Lock()
	if token := c.tokens[installationID]; token != "" {
		c.tokenLock.Unlock()
		return token, nil
	}
	c.tokenLock.Unlock()

	appJWT, err := c.auth.JWT()
	if err != nil {
		return "", err
	}
	var out struct {
		Token string `json:"token"`
	}
	path := fmt.Sprintf("/app/installations/%d/access_tokens", installationID)
	if err := c.doJSON(ctx, http.MethodPost, path, appJWT, nil, &out, http.StatusCreated, http.StatusOK); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("installation token response missing token")
	}
	c.tokenLock.Lock()
	c.tokens[installationID] = out.Token
	c.tokenLock.Unlock()
	return out.Token, nil
}

func (c *Client) InstallationToken(ctx context.Context, installationID int64) (string, error) {
	return c.installationToken(ctx, installationID)
}

func (c *Client) doJSON(ctx context.Context, method, path, bearer string, in any, out any, want ...int) error {
	var body *bytes.Reader
	if in == nil {
		body = bytes.NewReader(nil)
	} else {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(in); err != nil {
			return err
		}
		body = bytes.NewReader(buf.Bytes())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Authorization", "Bearer "+bearer)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	for _, status := range want {
		if resp.StatusCode == status {
			if out != nil {
				return json.NewDecoder(resp.Body).Decode(out)
			}
			return nil
		}
	}
	return HTTPError{Method: method, Path: path, Status: resp.StatusCode}
}

func parsePrivateKey(privateKeyPEM string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, errors.New("private key PEM is invalid")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not RSA")
	}
	return key, nil
}

type githubFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Patch     string `json:"patch"`
}

type githubInstallation struct {
	ID                  int64  `json:"id"`
	RepositorySelection string `json:"repository_selection"`
	Account             struct {
		Login string `json:"login"`
	} `json:"account"`
}

type githubPullRequest struct {
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type githubIssueComment struct {
	ID   int64      `json:"id"`
	Body string     `json:"body"`
	User githubUser `json:"user"`
}

type githubReviewComment struct {
	ID   int64      `json:"id"`
	Body string     `json:"body"`
	User githubUser `json:"user"`
}

type githubReviewCommentRequest struct {
	Body     string `json:"body"`
	CommitID string `json:"commit_id"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Side     string `json:"side"`
}

type githubPullRequestReview struct {
	ID int64 `json:"id"`
}

type githubPullRequestReviewRequest struct {
	CommitID string                                  `json:"commit_id"`
	Body     string                                  `json:"body"`
	Event    string                                  `json:"event"`
	Comments []githubPullRequestReviewCommentRequest `json:"comments"`
}

type githubPullRequestReviewCommentRequest struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Side string `json:"side"`
	Body string `json:"body"`
}

type githubUser struct {
	Type string `json:"type"`
}

type githubCheckRun struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	HeadSHA string `json:"head_sha"`
}

type githubCheckRunRequest struct {
	Name       string               `json:"name,omitempty"`
	HeadSHA    string               `json:"head_sha,omitempty"`
	Status     string               `json:"status,omitempty"`
	Conclusion string               `json:"conclusion,omitempty"`
	Output     githubCheckRunOutput `json:"output,omitempty"`
}

type githubCheckRunOutput struct {
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
}

func urlQueryEscape(value string) string {
	return url.QueryEscape(value)
}

func urlPathEscape(value string) string {
	escaped := url.PathEscape(value)
	return strings.ReplaceAll(escaped, "%2F", "/")
}

func contentPath(owner, repo, repoPath, ref string) string {
	base := fmt.Sprintf("/repos/%s/%s/contents", owner, repo)
	clean := strings.Trim(strings.TrimSpace(repoPath), "/")
	if clean == "" || clean == "." {
		return base + "?ref=" + urlQueryEscape(ref)
	}
	return base + "/" + urlPathEscape(clean) + "?ref=" + urlQueryEscape(ref)
}

func isNotFound(err error) bool {
	return IsHTTPStatus(err, http.StatusNotFound)
}

type githubContent struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}
