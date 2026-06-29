package review

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCheckoutCredentialProviderScopesTokenToCurrentJob(t *testing.T) {
	source := &fakeInstallationTokenSource{token: "sentinel-checkout-token"}
	provider := NewInstallationCheckoutCredentialProvider(source)
	job := Job{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: strings.Repeat("a", 40)}

	credential, err := provider.CheckoutCredential(context.Background(), CheckoutCredentialRequest{
		InstallationID: job.InstallationID,
		Owner:          job.Owner,
		Repo:           job.Repo,
		HeadSHA:        job.HeadSHA,
	})
	if err != nil {
		t.Fatalf("CheckoutCredential() error = %v", err)
	}
	if credential.InstallationID != job.InstallationID || credential.Owner != job.Owner || credential.Repo != job.Repo || credential.HeadSHA != job.HeadSHA {
		t.Fatalf("credential scope = %+v, want current job scope", credential)
	}
	if source.calls != 1 || source.installationID != job.InstallationID {
		t.Fatalf("token source calls=%d installation=%d, want one current installation call", source.calls, source.installationID)
	}
	if strings.Contains(credential.String(), "sentinel-checkout-token") {
		t.Fatalf("credential string leaked token: %q", credential.String())
	}
	if got, ok := credential.TokenForCheckout(job); !ok || got != "sentinel-checkout-token" {
		t.Fatalf("TokenForCheckout(current job) = %q, %v", got, ok)
	}
	if got, ok := credential.TokenForCheckout(Job{InstallationID: 43, Owner: "octo", Repo: "repo", HeadSHA: job.HeadSHA}); ok || got != "" {
		t.Fatalf("TokenForCheckout(other job) = %q, %v, want rejected", got, ok)
	}
}

func TestCheckoutCredentialProviderMapsFailures(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want GoAnalyzerExitCategory
	}{
		{name: "auth", err: ErrCheckoutCredentialAuth, want: GoAnalyzerCredentialUnavailable},
		{name: "rate limit", err: ErrCheckoutCredentialRateLimited, want: GoAnalyzerCredentialUnavailable},
		{name: "unavailable", err: errors.New("provider unavailable"), want: GoAnalyzerCredentialUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewInstallationCheckoutCredentialProvider(&fakeInstallationTokenSource{err: tt.err})

			_, err := provider.CheckoutCredential(context.Background(), CheckoutCredentialRequest{
				InstallationID: 42,
				Owner:          "octo",
				Repo:           "repo",
				HeadSHA:        strings.Repeat("a", 40),
			})

			var providerErr WorkspaceProviderFailure
			if !errors.As(err, &providerErr) || providerErr.Category != tt.want {
				t.Fatalf("error = %v, want category %s", err, tt.want)
			}
		})
	}
}

func TestCheckoutCredentialProviderRejectsInvalidScope(t *testing.T) {
	provider := NewInstallationCheckoutCredentialProvider(&fakeInstallationTokenSource{token: "sentinel-checkout-token"})

	for _, req := range []CheckoutCredentialRequest{
		{InstallationID: 0, Owner: "octo", Repo: "repo", HeadSHA: strings.Repeat("a", 40)},
		{InstallationID: 42, Owner: "octo/evil", Repo: "repo", HeadSHA: strings.Repeat("a", 40)},
		{InstallationID: 42, Owner: "octo", Repo: "repo", HeadSHA: "main"},
	} {
		_, err := provider.CheckoutCredential(context.Background(), req)
		var providerErr WorkspaceProviderFailure
		if !errors.As(err, &providerErr) || providerErr.Category != GoAnalyzerCredentialScopeMismatch {
			t.Fatalf("CheckoutCredential(%+v) error = %v, want scope mismatch", req, err)
		}
	}
}

type fakeInstallationTokenSource struct {
	calls          int
	installationID int64
	token          string
	err            error
}

func (f *fakeInstallationTokenSource) InstallationToken(ctx context.Context, installationID int64) (string, error) {
	f.calls++
	f.installationID = installationID
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}
