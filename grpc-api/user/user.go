package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ChokeGuy/simple-bank/consts"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	"github.com/ChokeGuy/simple-bank/pb"
	myErr "github.com/ChokeGuy/simple-bank/pkg/errors"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	sv "github.com/ChokeGuy/simple-bank/server/grpc"
	"github.com/ChokeGuy/simple-bank/util"
	pw "github.com/ChokeGuy/simple-bank/util/password"
	"github.com/ChokeGuy/simple-bank/validations"
	"github.com/ChokeGuy/simple-bank/worker"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type UserHandler struct {
	*sv.Server
}

func NewUserHandler(server *sv.Server) *UserHandler {
	return &UserHandler{Server: server}
}

// Create radom user
func RandomUser(t *testing.T) (db.User, string) {
	password := util.RandomPassword()
	hashedPassword, err := pw.HashPassword(password)
	require.NoError(t, err)

	return db.User{
		Username:       util.RandomOwner(),
		FullName:       util.RandomOwner(),
		Email:          util.RandomEmail(),
		HashedPassword: hashedPassword,
	}, password
}

// Create random verify email
func RandomVerifyEmail(t *testing.T, user db.User) db.VerifyEmail {
	return db.VerifyEmail{
		ID:         util.RandomInt(1, 10000),
		Username:   user.Username,
		Email:      user.Email,
		IsUsed:     false,
		SecretCode: util.RandomString(32),
		ExpiredAt:  time.Now().Add(time.Hour),
		CreatedAt:  time.Now(),
	}
}

func (h *UserHandler) authorizeUser(ctx context.Context) (*token.Payload, error) {
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		return nil, fmt.Errorf("metadata not provided")
	}

	values := md.Get(consts.AuthorizationHeader)
	if len(values) == 0 {
		return nil, fmt.Errorf("missing authorization header")
	}
	authHeader := values[0]
	fields := strings.Fields(authHeader)
	if len(fields) != 2 {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	authType := strings.ToLower(fields[0])

	if authType != consts.AuthorizationType {
		return nil, fmt.Errorf("unsupported authorization type")
	}

	accessToken := fields[1]
	payload, err := h.Server.TokenMaker.VerifyToken(accessToken)
	if err != nil {
		return nil, fmt.Errorf("invalid access token")
	}

	return payload, nil
}

func (h *UserHandler) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	violations := validateCreateUserRequest(req)

	if violations != nil {
		return nil, myErr.InvalidAgrumentError(violations)
	}

	hashedPassword, err := pw.HashPassword(req.GetPassword())

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to hash password: %v", err)
	}

	arg := db.CreateUserTxParams{
		CreateUserParams: db.CreateUserParams{
			Username:       req.UserName,
			FullName:       req.FullName,
			Email:          req.Email,
			HashedPassword: hashedPassword,
		},
		AfterCreate: func(user db.User) error {
			//Send verification email to user
			taskPayload := &worker.PayloadSendVerifyEmail{
				UserName: user.Username,
			}

			opts := []asynq.Option{
				asynq.MaxRetry(10),
				asynq.ProcessIn(10 * time.Second),
				asynq.Queue(worker.QueueDefault),
			}

			return h.TaskDistributor.DistributeTaskSendVerifyEmail(ctx, taskPayload, opts...)
		},
	}

	user, err := h.Store.CreateUserTx(ctx, arg)

	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			return nil, status.Errorf(codes.AlreadyExists, "%s", err.Error())
		}

		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	response := &pb.CreateUserResponse{
		User: convertUser(user.User),
	}
	return response, nil
}

func (h *UserHandler) LoginUser(ctx context.Context, req *pb.LoginUserRequest) (*pb.LoginUserResponse, error) {
	violations := validateLoginUserRequest(req)

	if violations != nil {
		return nil, myErr.InvalidAgrumentError(violations)
	}

	user, err := h.Store.GetUserByUserName(ctx, req.GetUserName())

	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}

		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	if err := pw.CheckPassword(req.Password, user.HashedPassword); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid password")
	}

	if session, _ := h.Store.GetSessionByUserName(ctx, req.GetUserName()); session.ID != uuid.Nil {
		// Check if session is expired
		if time.Now().After(session.ExpiresAt) {
			// Delete expired session
			err := h.Store.DeleteSession(ctx, session.ID)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to delete expired session: %v", err)
			}
		} else if session.IsBlocked {
			// Session exists and is not blocked
			return nil, status.Errorf(codes.PermissionDenied, "session is blocked")
		} else {
			// Session exists and is not blocked
			return nil, status.Errorf(codes.PermissionDenied, "user session already exists")
		}
	}

	accessToken, aTkPayload, err := h.TokenMaker.CreateToken(user.Username, h.Config.AccessTokenDuration)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create access token: %v", err)
	}

	refreshToken, rTkPayload, err := h.TokenMaker.CreateToken(user.Username, h.Config.RefreshTokenDuration)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create refresh token: %v", err)
	}

	metadata := h.extractMetadata(ctx)

	arg := db.CreateSessionParams{
		ID:           uuid.MustParse(rTkPayload.ID),
		Username:     user.Username,
		RefreshToken: refreshToken,
		UserAgent:    metadata.UserClient,
		ClientIp:     metadata.ClientIP,
		IsBlocked:    false,
		ExpiresAt:    rTkPayload.ExpiresAt.Time,
	}

	session, err := h.Store.CreateSession(ctx, arg)

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create session: %v", err)
	}

	response := &pb.LoginUserResponse{
		SessionID:             session.ID.String(),
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  timestamppb.New(aTkPayload.ExpiresAt.Time),
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: timestamppb.New(rTkPayload.ExpiresAt.Time),
		User: convertUser(db.User{
			Username:          user.Username,
			FullName:          user.FullName,
			Email:             user.Email,
			PasswordChangedAt: user.PasswordChangedAt,
			CreatedAt:         user.CreatedAt,
		}),
	}

	return response, nil
}

func (h *UserHandler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	authPayload, err := h.authorizeUser(ctx)

	if err != nil {
		return nil, myErr.UnAuthorizedError(err)
	}

	violations := validateUpdateUserRequest(req)

	if violations != nil {
		return nil, myErr.InvalidAgrumentError(violations)
	}

	if authPayload.UserName != req.GetUserName() {
		return nil, status.Errorf(codes.PermissionDenied, "unauthorized user")
	}

	arg := db.UpdateUserParams{
		Username: req.GetUserName(),
		FullName: pgtype.Text{
			String: req.GetFullName(),
			Valid:  req.FullName != nil,
		},
		Email: pgtype.Text{
			String: req.GetEmail(),
			Valid:  req.Email != nil,
		},
	}

	user, err := h.Store.UpdateUser(ctx, arg)

	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, status.Errorf(codes.NotFound, "user not found")
		}

		return nil, status.Errorf(codes.Internal, "failed to update user: %v", err)
	}

	response := &pb.UpdateUserResponse{
		User: convertUser(user),
	}
	return response, nil
}

func (h *UserHandler) VerifyUserEmail(ctx context.Context, req *pb.VerifyUserEmailRequest) (*pb.VerifyUserEmailResponse, error) {
	violations := validateVerifyUserEmailRequest(req)

	if violations != nil {
		return nil, myErr.InvalidAgrumentError(violations)
	}

	arg := db.VerifyUserEmailTxParams{
		EmailId:    req.GetEmailId(),
		SecretCode: req.GetSecretCode(),
	}

	verifyEmail, err := h.Store.VerifyUserEmailTx(ctx, arg)

	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, status.Errorf(codes.NotFound, "email verification not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get email verification: %v", err)
	}

	response := &pb.VerifyUserEmailResponse{
		IsVerified: verifyEmail.User.IsEmailVerified,
	}

	return response, nil
}
func validateCreateUserRequest(req *pb.CreateUserRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := validations.ValidateUsername(req.GetUserName()); err != nil {
		violations = append(violations, myErr.FieldViolation("userName", err))
	}

	if err := validations.ValidatePassword(req.GetPassword()); err != nil {
		violations = append(violations, myErr.FieldViolation("password", err))
	}

	if err := validations.ValidateFullName(req.GetFullName()); err != nil {
		violations = append(violations, myErr.FieldViolation("fullName", err))
	}

	if err := validations.ValidateEmail(req.GetEmail()); err != nil {
		violations = append(violations, myErr.FieldViolation("email", err))
	}

	return violations
}

func validateLoginUserRequest(req *pb.LoginUserRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := validations.ValidateUsername(req.GetUserName()); err != nil {
		violations = append(violations, myErr.FieldViolation("userName", err))
	}

	if err := validations.ValidatePassword(req.GetPassword()); err != nil {
		violations = append(violations, myErr.FieldViolation("password", err))
	}

	return violations
}

func validateUpdateUserRequest(req *pb.UpdateUserRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := validations.ValidateUsername(req.GetUserName()); err != nil {
		violations = append(violations, myErr.FieldViolation("userName", err))
	}

	if req.FullName != nil {
		if err := validations.ValidateFullName(req.GetFullName()); err != nil {
			violations = append(violations, myErr.FieldViolation("fullName", err))
		}
	}

	if req.Email != nil {
		if err := validations.ValidateEmail(req.GetEmail()); err != nil {
			violations = append(violations, myErr.FieldViolation("email", err))
		}
	}

	return violations
}

func validateVerifyUserEmailRequest(req *pb.VerifyUserEmailRequest) (violations []*errdetails.BadRequest_FieldViolation) {
	if err := validations.ValidateEmailId(req.GetEmailId()); err != nil {
		violations = append(violations, myErr.FieldViolation("emailId", err))
	}

	if err := validations.ValidateSecretCode(req.GetSecretCode()); err != nil {
		violations = append(violations, myErr.FieldViolation("secretCode", err))
	}

	return violations
}
