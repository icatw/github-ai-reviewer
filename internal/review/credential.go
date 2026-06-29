package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrCheckoutCredentialAuth        = errors.New("checkout credential auth failed")
	ErrCheckoutCredentialRateLimited = errors.New("checkout credential rate limited")
	ErrCheckoutCredentialScope       = errors.New("checkout credential scope mismatch")
)

type CheckoutCredentialRequest struct {
	InstallationID int64
	Owner          string
	Repo           string
	HeadSHA        string
}

type CheckoutCredential struct {
	InstallationID int64
	Owner          string
	Repo           string
	HeadSHA        string
	token          string
}

func (c CheckoutCredential) String() string {
	if c.InstallationID == 0 && c.Owner == "" && c.Repo == "" && c.HeadSHA == "" {
		return "checkout_credential(empty)"
	}
	return fmt.Sprintf("checkout_credential(installation=%d repo=%s/%s head=%s)", c.InstallationID, c.Owner, c.Repo, c.HeadSHA)
}

func (c CheckoutCredential) TokenForCheckout(job Job) (string, bool) {
	if c.token == "" {
		return "", false
	}
	if c.InstallationID != job.InstallationID || c.Owner != job.Owner || c.Repo != job.Repo || c.HeadSHA != job.HeadSHA {
		return "", false
	}
	return c.token, true
}

type CheckoutCredentialProvider interface {
	CheckoutCredential(ctx context.Context, req CheckoutCredentialRequest) (CheckoutCredential, error)
}

type InstallationTokenSource interface {
	InstallationToken(ctx context.Context, installationID int64) (string, error)
}

type InstallationCheckoutCredentialProvider struct {
	source InstallationTokenSource
}

func NewInstallationCheckoutCredentialProvider(source InstallationTokenSource) *InstallationCheckoutCredentialProvider {
	return &InstallationCheckoutCredentialProvider{source: source}
}

func (p *InstallationCheckoutCredentialProvider) CheckoutCredential(ctx context.Context, req CheckoutCredentialRequest) (CheckoutCredential, error) {
	if !validCheckoutCredentialRequest(req) {
		return CheckoutCredential{}, WorkspaceProviderFailure{Category: GoAnalyzerCredentialScopeMismatch}
	}
	if p == nil || p.source == nil {
		return CheckoutCredential{}, WorkspaceProviderFailure{Category: GoAnalyzerCredentialUnavailable}
	}
	token, err := p.source.InstallationToken(ctx, req.InstallationID)
	if err != nil || strings.TrimSpace(token) == "" {
		return CheckoutCredential{}, WorkspaceProviderFailure{Category: checkoutCredentialFailureCategory(err)}
	}
	return CheckoutCredential{
		InstallationID: req.InstallationID,
		Owner:          req.Owner,
		Repo:           req.Repo,
		HeadSHA:        req.HeadSHA,
		token:          token,
	}, nil
}

func validCheckoutCredentialRequest(req CheckoutCredentialRequest) bool {
	return req.InstallationID > 0 &&
		safeRepoPartPattern.MatchString(req.Owner) &&
		safeRepoPartPattern.MatchString(req.Repo) &&
		isSafeRef(req.HeadSHA)
}

func checkoutCredentialFailureCategory(err error) GoAnalyzerExitCategory {
	var providerErr WorkspaceProviderFailure
	if errors.As(err, &providerErr) && providerErr.Category != "" {
		return providerErr.Category
	}
	if errors.Is(err, ErrCheckoutCredentialScope) {
		return GoAnalyzerCredentialScopeMismatch
	}
	return GoAnalyzerCredentialUnavailable
}
