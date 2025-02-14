package worker

import (
	"context"

	"github.com/hibiken/asynq"
)

type TaskDistributior interface {
	DistributeTaskSendVerifyEmail(
		ctx context.Context,
		payload *PayloadSendVerifyEmail,
		opts ...asynq.Option,
	) error
}

type RedisTaskDistributior struct {
	client *asynq.Client
}

func NewRedisTaskDistributior(redisOpt asynq.RedisClientOpt) TaskDistributior {
	client := asynq.NewClient(redisOpt)

	return &RedisTaskDistributior{
		client: client,
	}
}
