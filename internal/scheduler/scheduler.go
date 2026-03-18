package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/chairswithlegs/monstera/internal/natsutil"
)

var retryBackoff = []time.Duration{30 * time.Second, 2 * time.Minute}

// Scheduler manages periodic background jobs with distributed deduplication.
// Each registered job publishes a deduplicated tick message at its interval;
// exactly one server instance processes each tick.
type Scheduler interface {
	Register(job Job)
	Start(ctx context.Context) error
}

// Job defines a periodic background task.
type Job struct {
	// Name is a unique identifier (e.g. "scheduled-statuses") used as the
	// NATS subject suffix and consumer durable name.
	Name string

	// Interval is how often the job fires. Must be >= 1 minute.
	Interval time.Duration

	// Handler is called once per tick on exactly one server instance.
	Handler func(ctx context.Context) error
}

// New returns a NATS JetStream-backed Scheduler.
func New(js jetstream.JetStream) Scheduler {
	return &natsScheduler{js: js}
}

type natsScheduler struct {
	js   jetstream.JetStream
	jobs []Job
}

func (s *natsScheduler) Register(job Job) {
	s.jobs = append(s.jobs, job)
}

func (s *natsScheduler) Start(ctx context.Context) error {
	if len(s.jobs) == 0 {
		<-ctx.Done()
		return nil
	}

	errc := make(chan error, len(s.jobs)*2)

	for _, job := range s.jobs {
		_, err := s.js.CreateOrUpdateConsumer(ctx, streamName, consumerConfigForJob(job))
		if err != nil {
			return fmt.Errorf("scheduler: create consumer for %s: %w", job.Name, err)
		}
		go s.runPublisher(ctx, job, errc)
		go s.runConsumer(ctx, job, errc)
	}

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		return nil
	}
}

func consumerConfigForJob(job Job) jetstream.ConsumerConfig {
	return jetstream.ConsumerConfig{
		Durable:       "scheduler-" + job.Name,
		FilterSubject: subjectPrefix + job.Name,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxAckPending: 1,
		AckWait:       2 * job.Interval,
		MaxDeliver:    3,
		BackOff:       retryBackoff,
	}
}

func (s *natsScheduler) runPublisher(ctx context.Context, job Job, _ chan<- error) {
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			slot := t.Truncate(job.Interval).Unix()
			msgID := fmt.Sprintf("%s.%d", job.Name, slot)
			if err := natsutil.Publish(ctx, s.js, subjectPrefix+job.Name, nil, jetstream.WithMsgID(msgID)); err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.WarnContext(ctx, "scheduler: publish tick failed",
					slog.String("job", job.Name),
					slog.Any("error", err),
				)
			}
		}
	}
}

func (s *natsScheduler) runConsumer(ctx context.Context, job Job, errc chan<- error) {
	consumerName := "scheduler-" + job.Name
	if err := natsutil.RunConsumer(ctx, s.js, streamName, consumerName,
		func(msg jetstream.Msg) { go s.processMessage(ctx, job, msg) },
		natsutil.WithMaxMessages(1),
		natsutil.WithLabel("scheduler: "+job.Name),
	); err != nil {
		errc <- err
	}
}

func (s *natsScheduler) processMessage(ctx context.Context, job Job, msg jetstream.Msg) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "scheduler: job panicked",
				slog.String("job", job.Name),
				slog.Any("panic", r),
			)
			_ = msg.Nak()
		}
	}()

	if err := job.Handler(ctx); err != nil {
		slog.WarnContext(ctx, "scheduler: job handler failed",
			slog.String("job", job.Name),
			slog.Any("error", err),
		)
		meta, _ := msg.Metadata()
		natsutil.NAKWithBackoff(msg, meta, retryBackoff)
		return
	}

	_ = msg.Ack()
}
