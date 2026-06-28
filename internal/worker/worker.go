package worker

import (
	"context"
	"errors"
	"log"

	"github-ai-reviewer/internal/review"
)

type Processor interface {
	Process(ctx context.Context, job review.Job) error
}

type Worker struct {
	processor Processor
	logger    *log.Logger
	jobs      chan review.Job
}

func New(processor Processor, logger *log.Logger) *Worker {
	return &Worker{processor: processor, logger: logger, jobs: make(chan review.Job, 32)}
}

func (w *Worker) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-w.jobs:
				if err := w.processor.Process(ctx, job); err != nil && w.logger != nil {
					w.logger.Printf("review job failed delivery=%s repo=%s/%s pull=%d error=%v", job.DeliveryID, job.Owner, job.Repo, job.PullNumber, err)
				}
			}
		}
	}()
}

func (w *Worker) Submit(job review.Job) error {
	select {
	case w.jobs <- job:
		return nil
	default:
		return errors.New("review job queue is full")
	}
}
