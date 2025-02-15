package worker

import (
	"context"
	"encoding/json"
	"fmt"

	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	"github.com/ChokeGuy/simple-bank/pkg/email"
	"github.com/ChokeGuy/simple-bank/util"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

const (
	TaskSendVerifyEmail = "task:send_verify_email"
)

type PayloadSendVerifyEmail struct {
	UserName string `json:"username"`
}

func (distributor *RedisTaskDistributor) DistributeTaskSendVerifyEmail(
	ctx context.Context,
	payload *PayloadSendVerifyEmail,
	opts ...asynq.Option,
) error {

	jsonPayload, err := json.Marshal(payload)

	if err != nil {
		return fmt.Errorf("fail to marshal payload: %v", err)
	}
	task := asynq.NewTask(TaskSendVerifyEmail, jsonPayload, opts...)
	info, err := distributor.client.EnqueueContext(ctx, task)

	if err != nil {
		return fmt.Errorf("fail to enqueue task: %v", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int("max_retry", info.MaxRetry).
		Msg("enqueued task")

	return nil
}

func (processor *RedisTaskProcessor) ProcessTaskSendVerifyEmail(ctx context.Context, task *asynq.Task) error {
	var payload PayloadSendVerifyEmail

	err := json.Unmarshal(task.Payload(), &payload)
	if err != nil {
		return fmt.Errorf("fail to unmarshal payload: %w", asynq.SkipRetry)
	}

	user, err := processor.store.GetUserByUserName(ctx, payload.UserName)

	if err != nil {
		// if err == sql.ErrNoRows {
		// 	return fmt.Errorf("user not found: %w", asynq.SkipRetry)
		// }
		return fmt.Errorf("fail to get user: %w", err)
	}

	arg := db.CreateVerifyEmailParams{
		Username:   user.Username,
		Email:      user.Email,
		SecretCode: util.RandomString(32),
	}
	verifyEmail, err := processor.store.CreateVerifyEmail(ctx, arg)

	if err != nil {
		return fmt.Errorf("fail to create verify email: %w", err)
	}

	verifyUrl := fmt.Sprintf("http://localhost:8080/user/verify-email?emailId=%d&secretCode=%s", verifyEmail.ID, verifyEmail.SecretCode)
	receivers := []string{user.Email}

	emailPayload := email.EmailPayload{
		Subject: "Welcome to Simple Bank",
		Content: fmt.Sprintf(`Hello %s, <br/>
		Thank you for registering an account with us!<br/>
		Please <a href="%s">click here</a> to verify your email address.<br/>
		`, user.Username, verifyUrl),
		To: receivers,
	}

	if err := processor.mailer.SendEmail(emailPayload); err != nil {
		return fmt.Errorf("fail to send email: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Bytes("payload", task.Payload()).
		Str("email", user.Email).
		Msg("processed task")

	return nil
}
