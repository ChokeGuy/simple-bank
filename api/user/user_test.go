package user

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	req "github.com/ChokeGuy/simple-bank/api/user/dto"
	mockdb "github.com/ChokeGuy/simple-bank/db/mock"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	pkg "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/ChokeGuy/simple-bank/pkg/middlewares/auth"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	server "github.com/ChokeGuy/simple-bank/server/http"
	"github.com/ChokeGuy/simple-bank/util"
	"github.com/ChokeGuy/simple-bank/worker"
	mockwk "github.com/ChokeGuy/simple-bank/worker/mock"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

// TestGetUserByUserNameApi tests the GetUserByUserName API handler
func TestGetUserByUserNameApi(t *testing.T) {
	user, _ := RandomUser(t)

	testCases := []struct {
		name          string
		userName      string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			userName: user.Username,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						FullName:       user.FullName,
						Email:          user.Email,
						HashedPassword: user.HashedPassword,
						CreatedAt:      user.CreatedAt,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchUser(t, recorder.Body, user)
			},
		},
		{
			name:     "NotFound",
			userName: user.Username,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:     "InternalError",
			userName: user.Username,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:     "BadRequest",
			userName: "",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
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
			userHandler.MapRoutes()

			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/user?userName=%s", tc.userName)

			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			server.Router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestCreateUserApi tests the CreateUser API handler
func TestCreateUserApi(t *testing.T) {
	user, password := RandomUser(t)

	testCases := []struct {
		name          string
		body          req.CreateUserRequest
		buildStubs    func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: req.CreateUserRequest{
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
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchCreateUser(t, recorder.Body, user)
			},
		},
		{
			name: "InternalError",
			body: req.CreateUserRequest{
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
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "BadRequest",
			body: req.CreateUserRequest{},
			buildStubs: func(store *mockdb.MockStore, taskDistributor *mockwk.MockTaskDistributor) {
				taskDistributor.EXPECT().
					DistributeTaskSendVerifyEmail(
						gomock.Any(),
						gomock.Any(),
						gomock.Any(),
					).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
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
			userHandler.MapRoutes()
			recorder := httptest.NewRecorder()

			url := "/user"

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			require.NoError(t, err)

			server.Router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestLoginUserApi tests the LoginUser API handler
func TestLoginUserApi(t *testing.T) {
	user, password := RandomUser(t)

	testCases := []struct {
		name          string
		body          req.LoginUserRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: req.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						FullName:       user.FullName,
						Email:          user.Email,
						HashedPassword: user.HashedPassword,
						CreatedAt:      user.CreatedAt,
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
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "ExpiredSession",
			body: req.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						FullName:       user.FullName,
						Email:          user.Email,
						HashedPassword: user.HashedPassword,
						CreatedAt:      user.CreatedAt,
					}, nil)

				sessionID := uuid.New()
				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetSessionByUserNameRow{
						ID:        sessionID,
						ExpiresAt: time.Now().Add(-24 * time.Hour), // Expired session
					}, nil)

				store.EXPECT().
					DeleteSession(gomock.Any(), gomock.Eq(sessionID)).
					Times(1).
					Return(nil)

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
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "DeleteExpiredSessionError",
			body: req.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						FullName:       user.FullName,
						Email:          user.Email,
						HashedPassword: user.HashedPassword,
						CreatedAt:      user.CreatedAt,
					}, nil)

				sessionID := uuid.New()
				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetSessionByUserNameRow{
						ID:        sessionID,
						ExpiresAt: time.Now().Add(-24 * time.Hour), // Expired session
					}, nil)

				store.EXPECT().
					DeleteSession(gomock.Any(), gomock.Eq(sessionID)).
					Times(1).
					Return(sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "BlockedSession",
			body: req.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						FullName:       user.FullName,
						Email:          user.Email,
						HashedPassword: user.HashedPassword,
						CreatedAt:      user.CreatedAt,
					}, nil)

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetSessionByUserNameRow{
						ID:        uuid.New(),
						IsBlocked: true,
						ExpiresAt: time.Now().Add(24 * time.Hour), // Add non-expired time
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "ActiveSession",
			body: req.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						FullName:       user.FullName,
						Email:          user.Email,
						HashedPassword: user.HashedPassword,
						CreatedAt:      user.CreatedAt,
					}, nil)

				store.EXPECT().
					GetSessionByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetSessionByUserNameRow{
						ID:        uuid.New(),
						IsBlocked: false,
						ExpiresAt: time.Now().Add(24 * time.Hour),
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "UserNotFound",
			body: req.LoginUserRequest{
				UserName: "nonexistent",
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetUserByUserNameRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "IncorrectPassword",
			body: req.LoginUserRequest{
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
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InternalError",
			body: req.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetUserByUserNameRow{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "SessionInternalError",
			body: req.LoginUserRequest{
				UserName: user.Username,
				Password: password,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:       user.Username,
						FullName:       user.FullName,
						Email:          user.Email,
						HashedPassword: user.HashedPassword,
						CreatedAt:      user.CreatedAt,
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
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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

			cfg, err := pkg.LoadConfig("../../")
			require.NoError(t, err)

			server := server.NewTestServer(t, store, &cfg, nil)
			userHandler := NewUserHandler(server)
			userHandler.MapRoutes()
			recorder := httptest.NewRecorder()

			url := "/auth/login"
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			require.NoError(t, err)

			server.Router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestRefreshTokenApi tests the RefreshToken API handler
func TestRefreshTokenApi(t *testing.T) {
	user, _ := RandomUser(t)

	session, refreshToken := RandomSession(t, user.Username)

	testCases := []struct {
		name          string
		body          req.RefreshTokenRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: req.RefreshTokenRequest{
				RefreshToken: refreshToken,
			},
			buildStubs: func(store *mockdb.MockStore) {
				session := db.Session{
					ID:           session.ID,
					Username:     user.Username,
					RefreshToken: refreshToken,
					IsBlocked:    false,
					ExpiresAt:    time.Now().Add(24 * time.Hour),
				}
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetSessionByIdRow{
						ID:           session.ID,
						Username:     session.Username,
						RefreshToken: session.RefreshToken,
						UserAgent:    session.UserAgent,
						ClientIp:     session.ClientIp,
						IsBlocked:    session.IsBlocked,
						ExpiresAt:    session.ExpiresAt,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "BadRequest",
			body: req.RefreshTokenRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidRefreshToken",
			body: req.RefreshTokenRequest{
				RefreshToken: "invalid_token",
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "ExpiredSession",
			body: req.RefreshTokenRequest{
				RefreshToken: refreshToken,
			},
			buildStubs: func(store *mockdb.MockStore) {
				session := db.Session{
					ID:           session.ID,
					Username:     user.Username,
					RefreshToken: refreshToken,
					IsBlocked:    false,
					ExpiresAt:    time.Now().Add(-24 * time.Hour),
				}
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetSessionByIdRow{
						ID:           session.ID,
						Username:     session.Username,
						RefreshToken: session.RefreshToken,
						UserAgent:    session.UserAgent,
						ClientIp:     session.ClientIp,
						IsBlocked:    session.IsBlocked,
						ExpiresAt:    session.ExpiresAt,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "BlockedSession",
			body: req.RefreshTokenRequest{
				RefreshToken: refreshToken,
			},
			buildStubs: func(store *mockdb.MockStore) {
				session := db.Session{
					ID:           session.ID,
					Username:     user.Username,
					RefreshToken: refreshToken,
					IsBlocked:    true,
					ExpiresAt:    time.Now().Add(24 * time.Hour),
				}
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetSessionByIdRow{
						ID:           session.ID,
						Username:     session.Username,
						RefreshToken: session.RefreshToken,
						UserAgent:    session.UserAgent,
						ClientIp:     session.ClientIp,
						IsBlocked:    session.IsBlocked,
						ExpiresAt:    session.ExpiresAt,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "SessionNotFound",
			body: req.RefreshTokenRequest{
				RefreshToken: refreshToken,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetSessionByIdRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "IncorrectSessionUser",
			body: req.RefreshTokenRequest{
				RefreshToken: refreshToken,
			},
			buildStubs: func(store *mockdb.MockStore) {
				session := db.Session{
					ID:           session.ID,
					Username:     "different_user", // Different username than in token claims
					RefreshToken: refreshToken,
					IsBlocked:    false,
					ExpiresAt:    time.Now().Add(24 * time.Hour),
				}
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetSessionByIdRow{
						ID:           session.ID,
						Username:     session.Username,
						RefreshToken: session.RefreshToken,
						UserAgent:    session.UserAgent,
						ClientIp:     session.ClientIp,
						IsBlocked:    session.IsBlocked,
						ExpiresAt:    session.ExpiresAt,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "MismatchedSessionToken",
			body: req.RefreshTokenRequest{
				RefreshToken: refreshToken,
			},
			buildStubs: func(store *mockdb.MockStore) {
				session := db.Session{
					ID:           session.ID,
					Username:     user.Username,
					RefreshToken: "different_token", // Different refresh token
					IsBlocked:    false,
					ExpiresAt:    time.Now().Add(24 * time.Hour),
				}
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetSessionByIdRow{
						ID:           session.ID,
						Username:     session.Username,
						RefreshToken: session.RefreshToken,
						UserAgent:    session.UserAgent,
						ClientIp:     session.ClientIp,
						IsBlocked:    session.IsBlocked,
						ExpiresAt:    session.ExpiresAt,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "InternalError",
			body: req.RefreshTokenRequest{
				RefreshToken: refreshToken,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetSessionById(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.GetSessionByIdRow{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
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

			cfg, err := pkg.LoadConfig("../../")
			require.NoError(t, err)

			server := server.NewTestServer(t, store, &cfg, nil)
			userHandler := NewUserHandler(server)
			userHandler.MapRoutes()
			recorder := httptest.NewRecorder()

			url := "/auth/refresh-token"
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			require.NoError(t, err)

			server.Router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// TestUpdateUserApi tests the UpdateUser API handler
func TestUpdateUserApi(t *testing.T) {
	user, _ := RandomUser(t)

	testCases := []struct {
		name          string
		body          req.UpdateUserRequest
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "UpdateFullName",
			body: req.UpdateUserRequest{
				UserName: user.Username,
				FullName: user.FullName,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, user.Username, user.Role, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.UpdateUserParams{
					Username: user.Username,
					FullName: pgtype.Text{String: user.FullName, Valid: user.FullName != ""},
				}

				store.EXPECT().GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:          user.Username,
						FullName:          user.FullName,
						Email:             user.Email,
						HashedPassword:    user.HashedPassword,
						PasswordChangedAt: user.PasswordChangedAt,
						CreatedAt:         user.CreatedAt,
					}, nil)
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(user, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchUser(t, recorder.Body, user)
			},
		},
		{
			name: "UpdateEmail",
			body: req.UpdateUserRequest{
				UserName: user.Username,
				Email:    user.Email,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, user.Username, user.Role, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.UpdateUserParams{
					Username: user.Username,
					Email:    pgtype.Text{String: user.Email, Valid: user.Email != ""},
				}

				store.EXPECT().GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:          user.Username,
						FullName:          user.FullName,
						Email:             user.Email,
						HashedPassword:    user.HashedPassword,
						PasswordChangedAt: user.PasswordChangedAt,
						CreatedAt:         user.CreatedAt,
					}, nil)
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(user, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchUser(t, recorder.Body, user)
			},
		},
		{
			name: "UpdateAllFields",
			body: req.UpdateUserRequest{
				UserName: user.Username,
				Email:    user.Email,
				FullName: user.FullName,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, user.Username, user.Role, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.UpdateUserParams{
					Username: user.Username,
					Email:    pgtype.Text{String: user.Email, Valid: user.Email != ""},
					FullName: pgtype.Text{String: user.FullName, Valid: user.FullName != ""},
				}

				store.EXPECT().GetUserByUserName(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.GetUserByUserNameRow{
						Username:          user.Username,
						FullName:          user.FullName,
						Email:             user.Email,
						HashedPassword:    user.HashedPassword,
						PasswordChangedAt: user.PasswordChangedAt,
						CreatedAt:         user.CreatedAt,
					}, nil)
				store.EXPECT().
					UpdateUser(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(user, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchUser(t, recorder.Body, user)
			},
		},
		{
			name: "UserNotFound",
			body: req.UpdateUserRequest{
				UserName: "nonexistent",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, user.FullName, user.Role, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetUserByUserName(gomock.Any(), gomock.Any()).Times(1).Return(db.GetUserByUserNameRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "UnAuthorizedUser",
			body: req.UpdateUserRequest{
				UserName: user.Username,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, "unauthorized_user", user.Role, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetUserByUserName(gomock.Any(), gomock.Any()).Times(1).Return(db.GetUserByUserNameRow{
					Username:          user.Username,
					FullName:          user.FullName,
					Email:             user.Email,
					HashedPassword:    user.HashedPassword,
					PasswordChangedAt: user.PasswordChangedAt,
				}, nil)
				store.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: req.UpdateUserRequest{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetUserByUserName(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "BadRequest",
			body: req.UpdateUserRequest{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, user.Username, user.Role, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetUserByUserName(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "GetUserInternalError",
			body: req.UpdateUserRequest{
				UserName: user.Username,
				FullName: user.FullName,
				Email:    user.Email,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, user.Username, user.Role, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetUserByUserName(gomock.Any(), gomock.Any()).Times(1).Return(db.GetUserByUserNameRow{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "InternalError",
			body: req.UpdateUserRequest{
				UserName: user.Username,
				FullName: user.FullName,
				Email:    user.Email,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, user.Username, user.Role, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetUserByUserName(gomock.Any(), gomock.Any()).Times(1).Return(db.GetUserByUserNameRow{
					Username:          user.Username,
					FullName:          user.FullName,
					Email:             user.Email,
					HashedPassword:    user.HashedPassword,
					PasswordChangedAt: user.PasswordChangedAt,
				}, nil)

				store.EXPECT().UpdateUser(gomock.Any(), gomock.Any()).Times(1).Return(db.User{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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

			cfg, err := pkg.LoadConfig("../../")
			require.NoError(t, err)

			server := server.NewTestServer(t, store, &cfg, nil)
			userHandler := NewUserHandler(server)
			userHandler.MapRoutes()
			recorder := httptest.NewRecorder()

			url := "/user/update"
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.TokenMaker)
			server.Router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestVerifyUserEmailApi(t *testing.T) {
	user, _ := RandomUser(t)
	verifyEmail := RandomVerifyEmail(t, user)

	testCases := []struct {
		name          string
		query         req.VerifyUserEmailRequest
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			query: req.VerifyUserEmailRequest{
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
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code == http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusOK, recorder.Code)

				requireBodyMatchVerifyEmail(t, recorder.Body, true)
			},
		},
		{
			name:  "BadRequest",
			query: req.VerifyUserEmailRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					VerifyUserEmailTx(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NotFound",
			query: req.VerifyUserEmailRequest{
				EmailId:    verifyEmail.ID,
				SecretCode: verifyEmail.SecretCode,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					VerifyUserEmailTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.VerifyUserEmailTxResult{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InternalError",
			query: req.VerifyUserEmailRequest{
				EmailId:    verifyEmail.ID,
				SecretCode: verifyEmail.SecretCode,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					VerifyUserEmailTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.VerifyUserEmailTxResult{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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
			userHandler.MapRoutes()

			recorder := httptest.NewRecorder()

			url := "/user/verify-email"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			// Add query parameters
			q := request.URL.Query()
			q.Add("emailId", fmt.Sprint(tc.query.EmailId))
			q.Add("secretCode", tc.query.SecretCode)
			request.URL.RawQuery = q.Encode()

			server.Router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// Helper function to check response body
func requireBodyMatchVerifyEmail(t *testing.T, body *bytes.Buffer, isVerified bool) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var response struct {
		Data struct {
			IsVerified bool `json:"isVerified"`
		} `json:"data"`
		Message    string `json:"message"`
		StatusCode int    `json:"statusCode"`
	}

	err = json.Unmarshal(data, &response)
	require.NoError(t, err)
	require.Equal(t, isVerified, response.Data.IsVerified)
}

// requireBodyMatchUser checks if the response body matches user
func requireBodyMatchUser(t *testing.T, body *bytes.Buffer, user db.User) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var response struct {
		Data       req.UserResponse `json:"data"`
		Message    string           `json:"message"`
		StatusCode int              `json:"statusCode"`
	}

	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	require.Equal(t, req.UserResponse{
		UserName:          user.Username,
		FullName:          user.FullName,
		Email:             user.Email,
		PasswordChangedAt: user.PasswordChangedAt.String(),
		CreatedAt:         user.CreatedAt.String(),
	}, response.Data)
}

func requireBodyMatchCreateUser(t *testing.T, body *bytes.Buffer, user db.User) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var response struct {
		Data       req.UserResponse `json:"data"`
		Message    string           `json:"message"`
		StatusCode int              `json:"statusCode"`
	}

	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	require.Equal(t, req.UserResponse{
		UserName:          user.Username,
		FullName:          user.FullName,
		Email:             user.Email,
		PasswordChangedAt: user.PasswordChangedAt.String(),
		CreatedAt:         user.CreatedAt.String(),
	}, response.Data)
}
