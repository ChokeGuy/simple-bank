package user

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/ChokeGuy/simple-bank/consts"
	mockdb "github.com/ChokeGuy/simple-bank/db/mock"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	"github.com/ChokeGuy/simple-bank/pb"
	pkg "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	server "github.com/ChokeGuy/simple-bank/server/grpc"
	"github.com/ChokeGuy/simple-bank/util"
	"github.com/ChokeGuy/simple-bank/worker"
	mockwk "github.com/ChokeGuy/simple-bank/worker/mock"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// TestCreateUserApi tests the CreateUser API handler
func TestCreateUserApi(t *testing.T) {
	user, password := RandomUser(t)

	testCases := []struct {
		name          string
		body          *pb.CreateUserRequest
		buildStubs    func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor)
		checkResponse func(t *testing.T, res *pb.CreateUserResponse, err error)
	}{
		{
			name: "OK",
			body: &pb.CreateUserRequest{
				UserName: user.Username,
				FullName: user.FullName,
				Password: password,
				Email:    user.Email,
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				arg := db.CreateUserTxParams{
					CreateUserParams: db.CreateUserParams{
						Username:       user.Username,
						FullName:       user.FullName,
						HashedPassword: user.HashedPassword,
						Email:          user.Email,
					},
				}

				store.EXPECT().
					CreateUserTx(gomock.Any(), EqCreateUserTxParams(arg, password, user)).
					Times(1).
					Return(db.CreateUserTxResult{User: user}, nil)

				//Send verification email to user
				taskPayload := &worker.PayloadSendVerifyEmail{
					UserName: user.Username,
				}

				taskDistributor.EXPECT().
					DistributeTaskSendVerifyEmail(
						gomock.Any(),
						EqPayloadSendVerifyEmail(taskPayload),
						gomock.Any(),
					).
					Times(1).
					Return(nil)

			},
			checkResponse: func(t *testing.T, res *pb.CreateUserResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)

				createdUser := res.GetUser()

				require.Equal(t, user.Username, createdUser.UserName)
				require.Equal(t, user.FullName, createdUser.FullName)
				require.Equal(t, user.Email, createdUser.Email)

			},
		},
		{
			name: "InternalError",
			body: &pb.CreateUserRequest{
				UserName: user.Username,
				FullName: user.FullName,
				Password: password,
				Email:    user.Email,
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateUserTxResult{}, sql.ErrConnDone)

				taskDistributor.EXPECT().
					DistributeTaskSendVerifyEmail(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
					).Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.CreateUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.Internal, st.Code())
			},
		},
		{
			name: "BadRequest",
			body: &pb.CreateUserRequest{},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				taskDistributor.EXPECT().
					DistributeTaskSendVerifyEmail(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
					).Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.CreateUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.InvalidArgument, st.Code())
			},
		}, {
			name: "DuplicateUsername",
			body: &pb.CreateUserRequest{
				UserName: user.Username,
				FullName: user.FullName,
				Password: password,
				Email:    user.Email,
			},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				store.EXPECT().
					CreateUserTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateUserTxResult{}, db.ErrUniqueViolation)

				taskDistributor.EXPECT().
					DistributeTaskSendVerifyEmail(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.CreateUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.AlreadyExists, st.Code())
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			storeCtrl := gomock.NewController(t)
			defer storeCtrl.Finish()

			store := mockdb.NewMockStore(storeCtrl)

			taskCtrl := gomock.NewController(t)
			defer taskCtrl.Finish()

			taskDistributor := mockwk.NewMockTaskDistributor(taskCtrl)
			//build stubs
			tc.buildStubs(store, taskDistributor)

			//start new server
			cfg, err := pkg.LoadConfig("../../")
			require.NoError(t, err)

			server := server.NewTestServer(t, store, &cfg, taskDistributor)
			userHandler := NewUserHandler(server)
			res, err := userHandler.CreateUser(context.Background(), tc.body)

			tc.checkResponse(t, res, err)
		})
	}
}

func TestLoginUserApi(t *testing.T) {
	user, password := RandomUser(t)

	testCases := []struct {
		name          string
		body          *pb.LoginUserRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.LoginUserResponse, err error)
	}{
		{
			name: "OK",
			body: &pb.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:          user.Username,
						FullName:          user.FullName,
						HashedPassword:    user.HashedPassword,
						Email:             user.Email,
						IsEmailVerified:   user.IsEmailVerified,
						PasswordChangedAt: user.PasswordChangedAt,
						CreatedAt:         user.CreatedAt,
					}, nil)

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetSessionByUserNameRow{}, db.ErrRecordNotFound)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{
						ID:           uuid.New(),
						Username:     user.Username,
						RefreshToken: gomock.Any().String(),
						UserAgent:    gomock.Any().String(),
						ClientIp:     gomock.Any().String(),
						IsBlocked:    false,
						ExpiresAt:    time.Now().Add(24 * time.Hour),
					}, nil)

			},
			checkResponse: func(t *testing.T, res *pb.LoginUserResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)

				createdUser := res.GetUser()

				require.Equal(t, user.Username, createdUser.UserName)
				require.Equal(t, user.FullName, createdUser.FullName)
				require.Equal(t, user.Email, createdUser.Email)
			},
		},
		{
			name: "InternalError",
			body: &pb.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:          user.Username,
						FullName:          user.FullName,
						HashedPassword:    user.HashedPassword,
						Email:             user.Email,
						IsEmailVerified:   user.IsEmailVerified,
						PasswordChangedAt: user.PasswordChangedAt,
						CreatedAt:         user.CreatedAt,
					}, nil)

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetSessionByUserNameRow{}, db.ErrRecordNotFound)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, res *pb.LoginUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.Internal, st.Code())
			},
		},
		{
			name: "GetUserInternalError",
			body: &pb.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{}, sql.ErrConnDone)

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(0)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.LoginUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.Internal, st.Code())
			},
		},

		{
			name: "BadRequest",
			body: &pb.LoginUserRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.LoginUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.InvalidArgument, st.Code())
			},
		},
		{
			name: "UserNotFound",
			body: &pb.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Any()).
					Times(0)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.LoginUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.NotFound, st.Code())
			},
		},
		{
			name: "SessionBlocked",
			body: &pb.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						HashedPassword: user.HashedPassword,
					}, nil)

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetSessionByUserNameRow{
						ID:        uuid.New(),
						IsBlocked: true,
						ExpiresAt: time.Now().Add(time.Hour),
					}, nil)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.LoginUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.PermissionDenied, st.Code())
			},
		},
		{
			name: "SessionExpired",
			body: &pb.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						HashedPassword: user.HashedPassword,
					}, nil)

				expiredSession := db.GetSessionByUserNameRow{
					ID:        uuid.New(),
					IsBlocked: false,
					ExpiresAt: time.Now().Add(-time.Hour), // Expired 1 hour ago
				}

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(expiredSession, nil)

				store.EXPECT().
					DeleteSession(gomock.Any(), expiredSession.ID).
					Times(1).
					Return(nil)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Session{
						ID:           uuid.New(),
						Username:     user.Username,
						RefreshToken: gomock.Any().String(),
						ExpiresAt:    time.Now().Add(24 * time.Hour),
					}, nil)
			},
			checkResponse: func(t *testing.T, res *pb.LoginUserResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
				require.NotEmpty(t, res.SessionID)
				require.NotEmpty(t, res.AccessToken)
				require.NotEmpty(t, res.RefreshToken)
			},
		},
		{
			name: "DeleteSessionError",
			body: &pb.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						HashedPassword: user.HashedPassword,
					}, nil)

				expiredSession := db.GetSessionByUserNameRow{
					ID:        uuid.New(),
					IsBlocked: false,
					ExpiresAt: time.Now().Add(-time.Hour),
				}

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(expiredSession, nil)

				store.EXPECT().
					DeleteSession(gomock.Any(), expiredSession.ID).
					Times(1).
					Return(sql.ErrConnDone)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.LoginUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.Internal, st.Code())
			},
		},
		{
			name: "SessionAlreadyExists",
			body: &pb.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						HashedPassword: user.HashedPassword,
					}, nil)

				activeSession := db.GetSessionByUserNameRow{
					ID:        uuid.New(),
					IsBlocked: false,
					ExpiresAt: time.Now().Add(time.Hour),
				}

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(activeSession, nil)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.LoginUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.PermissionDenied, st.Code())
			},
		},
		{
			name: "InvalidPassword",
			body: &pb.LoginUserRequest{
				UserName: user.Username,
				Password: util.RandomPassword(),
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						HashedPassword: user.HashedPassword,
					}, nil)

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Any()).
					Times(0)

				store.EXPECT().
					CreateSession(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.LoginUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.InvalidArgument, st.Code())
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			//build stubs
			tc.buildStubs(store)

			//start new server
			cfg, err := pkg.LoadConfig("../../")
			require.NoError(t, err)

			server := server.NewTestServer(t, store, &cfg, nil)
			userHandler := NewUserHandler(server)
			res, err := userHandler.LoginUser(context.Background(), tc.body)

			tc.checkResponse(t, res, err)
		})
	}
}

// Helper function to add authorization metadata to context
func addAuthorizationMetadata(
	ctx context.Context,
	t *testing.T,
	tokenMaker token.Maker,
	username string,
	duration time.Duration,
) context.Context {
	token, payload, err := tokenMaker.CreateToken(username, duration)
	require.NoError(t, err)
	require.NotEmpty(t, payload)

	md := metadata.New(map[string]string{
		consts.AuthorizationHeader: fmt.Sprintf("%s %s", consts.AuthorizationType, token),
	})
	return metadata.NewIncomingContext(ctx, md)
}

// Update the test cases in TestUpdateUserApi
func TestUpdateUserApi(t *testing.T) {
	user, _ := RandomUser(t)

	testCases := []struct {
		name          string
		body          *pb.UpdateUserRequest
		setupContext  func(t *testing.T, tokenMaker token.Maker) context.Context
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, res *pb.UpdateUserResponse, err error)
	}{
		{
			name: "OK",
			body: &pb.UpdateUserRequest{
				UserName: user.Username,
				FullName: &user.FullName,
				Email:    &user.Email,
			},
			setupContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return addAuthorizationMetadata(
					context.Background(),
					t,
					tokenMaker,
					user.Username,
					time.Minute,
				)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.UpdateUserParams{
					Username: user.Username,
					FullName: pgtype.Text{
						String: user.FullName,
						Valid:  true,
					},
					Email: pgtype.Text{
						String: user.Email,
						Valid:  true,
					},
				}

				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(user, nil)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, res)
				updatedUser := res.GetUser()
				require.Equal(t, user.Username, updatedUser.UserName)
				require.Equal(t, user.FullName, updatedUser.FullName)
				require.Equal(t, user.Email, updatedUser.Email)
			},
		},
		{
			name: "NoAuthorization",
			body: &pb.UpdateUserRequest{
				UserName: user.Username,
				FullName: &user.FullName,
				Email:    &user.Email,
			},
			setupContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return context.Background()
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.Unauthenticated, st.Code())
			},
		},
		{
			name: "UnauthorizedUser",
			body: &pb.UpdateUserRequest{
				UserName: user.Username,
				FullName: &user.FullName,
				Email:    &user.Email,
			},
			setupContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return addAuthorizationMetadata(
					context.Background(),
					t,
					tokenMaker,
					"unauthorized_user", // Different username from request
					time.Minute,
				)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.PermissionDenied, st.Code())
			},
		},
		{
			name: "InvalidEmail",
			body: &pb.UpdateUserRequest{
				UserName: user.Username,
				Email:    proto.String("invalid-email"),
			},
			setupContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return addAuthorizationMetadata(
					context.Background(),
					t,
					tokenMaker,
					user.Username,
					time.Minute,
				)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.InvalidArgument, st.Code())
			},
		},
		{
			name: "ExpiredToken",
			body: &pb.UpdateUserRequest{
				UserName: user.Username,
				FullName: &user.FullName,
				Email:    &user.Email,
			},
			setupContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return addAuthorizationMetadata(
					context.Background(),
					t,
					tokenMaker,
					user.Username,
					-time.Minute, // Expired token
				)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.Unauthenticated, st.Code())
			},
		},
		{
			name: "UserNotFound",
			body: &pb.UpdateUserRequest{
				UserName: user.Username,
				FullName: &user.FullName,
				Email:    &user.Email,
			},
			setupContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return addAuthorizationMetadata(
					context.Background(),
					t,
					tokenMaker,
					user.Username,
					time.Minute,
				)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.UpdateUserParams{
					Username: user.Username,
					FullName: pgtype.Text{
						String: user.FullName,
						Valid:  true,
					},
					Email: pgtype.Text{
						String: user.Email,
						Valid:  true,
					},
				}

				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(db.User{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.NotFound, st.Code())
			},
		},
		{
			name: "InternalError",
			body: &pb.UpdateUserRequest{
				UserName: user.Username,
				FullName: &user.FullName,
				Email:    &user.Email,
			},
			setupContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return addAuthorizationMetadata(
					context.Background(),
					t,
					tokenMaker,
					user.Username,
					time.Minute,
				)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.User{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.Internal, st.Code())
			},
		},
		{
			name: "InvalidFormat",
			body: &pb.UpdateUserRequest{
				UserName: "invalid@username",
				FullName: &user.FullName,
				Email:    &user.Email,
			},
			setupContext: func(t *testing.T, tokenMaker token.Maker) context.Context {
				return addAuthorizationMetadata(
					context.Background(),
					t,
					tokenMaker,
					"invalid@username",
					time.Minute,
				)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, res *pb.UpdateUserResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.InvalidArgument, st.Code())
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			storeCtrl := gomock.NewController(t)
			defer storeCtrl.Finish()

			store := mockdb.NewMockStore(storeCtrl)
			tc.buildStubs(store)

			cfg, err := pkg.LoadConfig("../../")
			require.NoError(t, err)

			server := server.NewTestServer(t, store, &cfg, nil)
			userHandler := NewUserHandler(server)

			ctx := tc.setupContext(t, server.TokenMaker)
			res, err := userHandler.UpdateUser(ctx, tc.body)
			tc.checkResponse(t, res, err)
		})
	}
}

func TestVerifyUserEmailApi(t *testing.T) {
	user, _ := RandomUser(t)
	verifyEmail := RandomVerifyEmail(t, user)

	testCases := []struct {
		name          string
		query         *pb.VerifyUserEmailRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *pb.VerifyUserEmailResponse, err error)
	}{
		{
			name: "OK",
			query: &pb.VerifyUserEmailRequest{
				EmailId:    verifyEmail.ID,
				SecretCode: verifyEmail.SecretCode,
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.VerifyUserEmailTxParams{
					EmailId:    verifyEmail.ID,
					SecretCode: verifyEmail.SecretCode,
				}

				store.EXPECT().
					VerifyUserEmailTx(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(db.VerifyUserEmailTxResult{
						User: db.User{
							Username:          user.Username,
							FullName:          user.FullName,
							Email:             user.Email,
							HashedPassword:    user.HashedPassword,
							IsEmailVerified:   true,
							PasswordChangedAt: user.PasswordChangedAt,
							CreatedAt:         user.CreatedAt,
						},
						VerifyEmail: db.VerifyEmail{
							ID:         verifyEmail.ID,
							Username:   user.Username,
							Email:      user.Email,
							SecretCode: verifyEmail.SecretCode,
							ExpiredAt:  verifyEmail.ExpiredAt,
							IsUsed:     true,
							CreatedAt:  verifyEmail.CreatedAt,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *pb.VerifyUserEmailResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, recorder)
			},
		},
		{
			name:  "BadRequest",
			query: &pb.VerifyUserEmailRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					VerifyUserEmailTx(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *pb.VerifyUserEmailResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.InvalidArgument, st.Code())
			},
		},
		{
			name: "NotFound",
			query: &pb.VerifyUserEmailRequest{
				EmailId:    verifyEmail.ID,
				SecretCode: verifyEmail.SecretCode,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					VerifyUserEmailTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.VerifyUserEmailTxResult{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *pb.VerifyUserEmailResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.NotFound, st.Code())
			},
		},
		{
			name: "InternalError",
			query: &pb.VerifyUserEmailRequest{
				EmailId:    verifyEmail.ID,
				SecretCode: verifyEmail.SecretCode,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					VerifyUserEmailTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.VerifyUserEmailTxResult{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *pb.VerifyUserEmailResponse, err error) {
				require.Error(t, err)
				st, ok := status.FromError(err)
				require.True(t, ok)
				require.Equal(t, codes.Internal, st.Code())
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			// start test server and send request
			cfg, err := pkg.LoadConfig("../../")
			require.NoError(t, err)

			server := server.NewTestServer(t, store, &cfg, nil)

			userHandler := NewUserHandler(server)

			res, err := userHandler.VerifyUserEmail(context.Background(), tc.query)

			tc.checkResponse(t, res, err)
		})
	}
}
