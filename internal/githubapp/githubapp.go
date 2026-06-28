package githubapp

import (
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
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
	return fmt.Errorf("github request failed: %s %s status %d", method, path, resp.StatusCode)
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

type githubIssueComment struct {
	ID   int64      `json:"id"`
	Body string     `json:"body"`
	User githubUser `json:"user"`
}

type githubUser struct {
	Type string `json:"type"`
}
