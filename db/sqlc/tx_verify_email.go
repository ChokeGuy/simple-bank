package sqlc

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// VerifyUserEmailTxParams contains the input parameters of the transfer transaction
type VerifyUserEmailTxParams struct {
	EmailId    int64
	SecretCode string
}

// VerifyUserEmailTxResult contains the result of the transfer transaction
type VerifyUserEmailTxResult struct {
	User        User
	VerifyEmail VerifyEmail
}

// VerifyUserEmailTxParams contains the input parameters of the transfer transaction
func (store *SQLStore) VerifyUserEmailTx(ctx context.Context, arg VerifyUserEmailTxParams) (VerifyUserEmailTxResult, error) {
	var result VerifyUserEmailTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		var err error

		result.VerifyEmail, err = q.UpdateVerifyEmail(ctx, UpdateVerifyEmailParams{
			ID:         arg.EmailId,
			SecretCode: arg.SecretCode,
		})

		if err != nil {
			return err
		}

		result.User, err = q.UpdateUser(ctx, UpdateUserParams{
			Username: result.VerifyEmail.Username,
			IsEmailVerified: pgtype.Bool{
				Bool:  true,
				Valid: true,
			},
		})

		return err
	})

	return result, err
}
