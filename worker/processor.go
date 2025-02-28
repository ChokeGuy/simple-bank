package worker

import (
	"context"

	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	"github.com/ChokeGuy/simple-bank/pkg/email"
	"github.com/ChokeGuy/simple-bank/pkg/logger"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
)

type TaskProcessor interface {
	start() error
	shutdown()
	ProcessTaskSendVerifyEmail(ctx context.Context, task *asynq.Task) error
}

type RedisTaskProcessor struct {
	server *asynq.Server
	store  db.Store
	mailer email.EmailSender
}

func NewRedisTaskProcessor(redisOpt asynq.RedisClientOpt, store db.Store, mailer email.EmailSender) TaskProcessor {
	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Queues: map[string]int{
				QueueCritical: 10,
				QueueDefault:  5,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				log.Error().
					Err(err).
					Str("task_type", task.Type()).
					Bytes("task_payload", task.Payload()).
					Msg("process task failed")
			}),
			Logger: logger.TaskLogger(),
		},
	)

	return &RedisTaskProcessor{
		server: server,
		store:  store,
		mailer: mailer,
	}
}

func (processor *RedisTaskProcessor) start() error {
	mux := asynq.NewServeMux()

	mux.HandleFunc(TaskSendVerifyEmail, processor.ProcessTaskSendVerifyEmail)

	return processor.server.Start(mux)
}

func (processor *RedisTaskProcessor) shutdown() {
	processor.server.Shutdown()
}

// RunTaskProcessor run redis task processor
func RunTaskProcessor(
	ctx context.Context,
	waitGroup *errgroup.Group,
	redisOpt asynq.RedisClientOpt,
	store db.Store,
) {
	mailer, err := email.NewSesEmailSender()

	if err != nil {
		log.Fatal().Msgf("cannot create email sender: %v", err)
	}
	taskProcessor := NewRedisTaskProcessor(redisOpt, store, mailer)

	log.Info().Msg("start task processor")
	if err := taskProcessor.start(); err != nil {
		log.Fatal().Err(err).Msg("fail to start task processor")
	}

	waitGroup.Go(func() error {
		<-ctx.Done()
		log.Info().Msg("gracefully stopping task processor")
		taskProcessor.shutdown()

		log.Info().Msg("task processor shutdown complete")
		return nil
	})
}
