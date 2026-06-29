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
	llmClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey, cfg.LLM.Model, nil)
	publisher := comment.NewPublisher(gh)
	reporters := review.MultiReporter{
		comment.NewReporter(publisher),
		review.NewCheckRunReporter(gh),
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

	handler := server.New(cfg.GitHub.WebhookSecret, w)
	logger.Printf("listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, handler); err != nil {
		logger.Fatalf("server error: %v", err)
	}
}
