package main

import (
	"log"

	"github-ai-reviewer/internal/config"
	"github-ai-reviewer/internal/review"
)

type reviewServiceDeps struct {
	github             review.GitHubClient
	installationTokens review.InstallationTokenSource
	llm                review.LLMClient
	reporter           review.Reporter
	logger             *log.Logger
}

func buildReviewService(cfg config.Config, deps reviewServiceDeps) *review.Service {
	opts := review.ServiceOptions{Language: review.NormalizeLanguage(cfg.LLM.Language)}
	if !cfg.GoWorkspace.Enabled {
		return review.NewServiceWithWorkspaceProviderAndOptions(deps.github, deps.llm, deps.reporter, deps.logger, nil, opts)
	}
	provider := review.NewLocalGoWorkspaceProvider(review.LocalGoWorkspaceProviderOptions{
		Enabled:            true,
		Root:               cfg.GoWorkspace.Root,
		Timeout:            cfg.GoWorkspace.CheckoutTimeout,
		OutputLimitBytes:   cfg.GoWorkspace.OutputLimitBytes,
		CredentialProvider: review.NewInstallationCheckoutCredentialProvider(deps.installationTokens),
	})
	return review.NewServiceWithWorkspaceProviderAndOptions(deps.github, deps.llm, deps.reporter, deps.logger, provider, opts)
}
