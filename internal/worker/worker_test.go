package worker

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github-ai-reviewer/internal/review"
)

func TestSubmitReturnsErrorWhenQueueIsFull(t *testing.T) {
	w := New(&blockingProcessor{}, nil)
	for i := 0; i < cap(w.jobs); i++ {
		if err := w.Submit(review.Job{PullNumber: i + 1}); err != nil {
			t.Fatalf("Submit() before full error = %v", err)
		}
	}
	if err := w.Submit(review.Job{PullNumber: cap(w.jobs) + 1}); err == nil {
		t.Fatal("Submit() error = nil, want queue full error")
	}
}

func TestWorkerLogsProcessorErrorDetails(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	w := New(&failingProcessor{err: errors.New("boom")}, logger)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.Start(ctx)

	if err := w.Submit(review.Job{DeliveryID: "d1", Owner: "octo", Repo: "repo", PullNumber: 7}); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		logLine := buf.String()
		if strings.Contains(logLine, "error=boom") && strings.Contains(logLine, "repo=octo/repo") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("worker log did not include error details: %q", buf.String())
}

type blockingProcessor struct{}

func (blockingProcessor) Process(context.Context, review.Job) error { return nil }

type failingProcessor struct{ err error }

func (p failingProcessor) Process(context.Context, review.Job) error { return p.err }
