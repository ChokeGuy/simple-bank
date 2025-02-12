package user

import (
	"context"
	"database/sql"
	"time"

	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	"github.com/ChokeGuy/simple-bank/pb"
	sv "github.com/ChokeGuy/simple-bank/server/grpc"
	pw "github.com/ChokeGuy/simple-bank/util/password"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/lib/pq"
)

type UserHandler struct {
	pb.UnimplementedSimpleBankServer
	*sv.Server
}

func NewUserHandler(server *sv.Server) *UserHandler {
	return &UserHandler{Server: server}
}

func (h *UserHandler) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	hashedPassword, err := pw.HashPassword(req.GetPassword())

	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to hash password: %v", err)
	}

	arg := db.CreateUserParams{
		Username:       req.GetUserName(),
		FullName:       req.GetFullName(),
		Email:          req.GetEmail(),
		HashedPassword: hashedPassword,
	}

	user, err := h.Store.CreateUser(ctx, arg)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code.Name() {
			case "unique_violation":
				return nil, status.Errorf(codes.AlreadyExists, "user already exists: %v", err)
			}
		}

		return nil, status.Errorf(codes.Internal, "failed to create user: %v", err)
	}

	response := &pb.CreateUserResponse{
		User: convertUser(user),
	}
	return response, nil
}

func (h *UserHandler) LoginUser(ctx context.Context, req *pb.LoginUserRequest) (*pb.LoginUserResponse, error) {
	user, err := h.Store.GetUserByUserName(ctx, req.GetUserName())

	if err != nil {
		if err == sql.ErrNoRows {
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
