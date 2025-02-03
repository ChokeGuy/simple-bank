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

	req "github.com/ChokeGuy/simple-bank/api/user/dto"
	mockdb "github.com/ChokeGuy/simple-bank/db/mock"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	"github.com/ChokeGuy/simple-bank/server"
	"github.com/ChokeGuy/simple-bank/util"
	pw "github.com/ChokeGuy/simple-bank/util/password"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

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
					Return(db.GetUserByUserNameRow{}, sql.ErrNoRows)
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
			server := server.NewServer(store)
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
		buildStubs    func(store *mockdb.MockStore)
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
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.CreateUserParams{
					Username:       user.Username,
					FullName:       user.FullName,
					HashedPassword: user.HashedPassword,
					Email:          user.Email,
				}

				store.EXPECT().
					CreateUser(gomock.Any(), EqCreateUserParams(arg, password)).
					Times(1).
					Return(user, nil)
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
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.CreateUserParams{
					Username:       user.Username,
					FullName:       user.FullName,
					HashedPassword: user.HashedPassword,
					Email:          user.Email,
				}

				store.EXPECT().
					CreateUser(gomock.Any(), EqCreateUserParams(arg, password)).
					Times(1).
					Return(db.User{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
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
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.CreateUserParams{
					Username:       user.Username,
					FullName:       user.FullName,
					HashedPassword: user.HashedPassword,
					Email:          user.Email,
				}

				store.EXPECT().
					CreateUser(gomock.Any(), EqCreateUserParams(arg, password)).
					Times(1).
					Return(db.User{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "BadRequest",
			body: req.CreateUserRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					CreateUser(gomock.Any(), gomock.Any()).
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
			server := server.NewServer(store)
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

// requireBodyMatchUser checks if the response body matches user
func requireBodyMatchUser(t *testing.T, body *bytes.Buffer, user db.User) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var response struct {
		Data       req.GetUserByUserNameResponse `json:"data"`
		Message    string                        `json:"message"`
		StatusCode int                           `json:"statusCode"`
	}

	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	require.Equal(t, req.GetUserByUserNameResponse{
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
		Data       req.CreateUserResponse `json:"data"`
		Message    string                 `json:"message"`
		StatusCode int                    `json:"statusCode"`
	}

	err = json.Unmarshal(data, &response)
	require.NoError(t, err)

	require.Equal(t, req.CreateUserResponse{
		UserName:          user.Username,
		FullName:          user.FullName,
		Email:             user.Email,
		PasswordChangedAt: user.PasswordChangedAt.String(),
		CreatedAt:         user.CreatedAt.String(),
	}, response.Data)
}
