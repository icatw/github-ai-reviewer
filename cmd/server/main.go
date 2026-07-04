package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github-ai-reviewer/internal/comment"
	"github-ai-reviewer/internal/config"
	"github-ai-reviewer/internal/githubapp"
	"github-ai-reviewer/internal/llm"
	"github-ai-reviewer/internal/review"
	"github-ai-reviewer/internal/server"
	"github-ai-reviewer/internal/worker"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}
	privateKey, err := cfg.GitHub.PrivateKeyPEM()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}
	gh, err := githubapp.NewClient(cfg.GitHub.AppID, privateKey, "", nil)
	if err != nil {
		logger.Fatalf("github app setup error: %v", err)
	}
	language := review.NormalizeLanguage(cfg.LLM.Language)
	llmClient := llm.NewClientWithOptions(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, nil, llm.ClientOptions{Language: language})
	publisher := comment.NewPublisherWithOptions(gh, comment.PublisherOptions{Language: language, InlineCommentsEnabled: cfg.InlineComments.Enabled, Logger: logger})
	reporters := review.MultiReporter{
		comment.NewReporter(publisher),
	}
	if cfg.CheckRun.Enabled {
		reporters = append(reporters, review.NewCheckRunReporter(gh))
	}
	reviewSvc := buildReviewService(cfg, reviewServiceDeps{
		github:             gh,
		installationTokens: gh,
		llm:                llmClient,
		reporter:           reporters,
		logger:             logger,
	})
	w := worker.New(reviewSvc, logger)
	w.Start(context.Background())

	handler := server.NewWithResolver(cfg.GitHub.WebhookSecret, w, gh)
	logger.Printf("listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, handler); err != nil {
		logger.Fatalf("server error: %v", err)
	}
}
