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

type CleanupProcessor interface {
	Cleanup(ctx context.Context, job review.CleanupJob) error
}

type Worker struct {
	processor Processor
	logger    *log.Logger
	jobs      chan review.Job
}

type CleanupWorker struct {
	processor CleanupProcessor
	logger    *log.Logger
	jobs      chan review.CleanupJob
}

func New(processor Processor, logger *log.Logger) *Worker {
	return &Worker{processor: processor, logger: logger, jobs: make(chan review.Job, 32)}
}

func NewCleanup(processor CleanupProcessor, logger *log.Logger) *CleanupWorker {
	return &CleanupWorker{processor: processor, logger: logger, jobs: make(chan review.CleanupJob, 32)}
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

func (w *CleanupWorker) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-w.jobs:
				if err := w.processor.Cleanup(ctx, job); err != nil && w.logger != nil {
					w.logger.Printf("cleanup job failed delivery=%s repo=%s/%s pull=%d state=%s category=processor_error error=%v", job.DeliveryID, job.Owner, job.Repo, job.PullNumber, job.State, err)
				}
			}
		}
	}()
}

func (w *CleanupWorker) SubmitCleanup(job review.CleanupJob) error {
	select {
	case w.jobs <- job:
		return nil
	default:
		return errors.New("cleanup job queue is full")
	}
}
