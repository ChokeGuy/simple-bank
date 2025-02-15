package transfer

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

	req "github.com/ChokeGuy/simple-bank/api/transfer/dto"
	"github.com/ChokeGuy/simple-bank/api/user"
	mockdb "github.com/ChokeGuy/simple-bank/db/mock"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	pkg "github.com/ChokeGuy/simple-bank/pkg/config"
	"github.com/ChokeGuy/simple-bank/pkg/middlewares/auth"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	server "github.com/ChokeGuy/simple-bank/server/http"
	"github.com/ChokeGuy/simple-bank/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func RandomEntry(accountID int64, amount int64) db.Entry {
	return db.Entry{
		ID:        util.RandomInt(1, 1000),
		AccountID: accountID,
		Amount:    amount,
	}
}

func RandomAccountWithParams(ID int64, currency, owner string) db.Account {
	return db.Account{
		ID:       ID,
		Owner:    owner,
		Balance:  util.RandomMoney(),
		Currency: currency,
	}
}

// RandomTxResult generates a random TransferTxResult
func RandomTxResult(t *testing.T) db.TransferTxResult {
	fromUser, _ := user.RandomUser(t)
	toUser, _ := user.RandomUser(t)
	fromAccount := RandomAccountWithParams(550, util.CAD, fromUser.Username)
	toAccount := RandomAccountWithParams(259, util.CAD, toUser.Username)

	var randBalance int64 = util.RandomInt(1, fromAccount.Balance)

	fromEntry := RandomEntry(550, -randBalance)
	toEntry := RandomEntry(259, randBalance)

	transfer := db.Transfer{
		ID:            util.RandomInt(1, 1000),
		FromAccountID: fromAccount.Balance,
		ToAccountID:   toAccount.Balance,
		Amount:        randBalance,
	}

	result := db.TransferTxResult{
		Transfer:    transfer,
		FromAccount: fromAccount,
		ToAccount:   toAccount,
		FromEntry:   fromEntry,
		ToEntry:     toEntry,
	}

	return result
}

// TestCreateTransfer tests the CreateTransfer API handler
func TestCreateTransfer(t *testing.T) {
	// Create a new transferResult
	result := RandomTxResult(t)

	testCases := []struct {
		name          string
		body          req.TransferRequest
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: req.TransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
				Amount:        result.Transfer.Amount,
				Currency:      result.FromAccount.Currency,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.TransferTxParams{
					FromAccountID: result.Transfer.FromAccountID,
					ToAccountID:   result.Transfer.ToAccountID,
					Amount:        result.Transfer.Amount,
				}

				store.EXPECT().GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().GetAccount(gomock.Any(), gomock.Eq(arg.ToAccountID)).
					Times(1).
					Return(result.ToAccount, nil)

				store.EXPECT().
					TransferTx(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(result, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMatchTxResult(t, recorder.Body, result)
			},
		},
		{
			name: "UnAuthorizedUser",
			body: req.TransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
				Amount:        result.Transfer.Amount,
				Currency:      result.FromAccount.Currency,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, "unauthorized_user", util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.TransferTxParams{
					FromAccountID: result.Transfer.FromAccountID,
					ToAccountID:   result.Transfer.ToAccountID,
				}

				store.EXPECT().GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().GetAccount(gomock.Any(), gomock.Eq(arg.ToAccountID)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: req.TransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
				Amount:        result.Transfer.Amount,
				Currency:      result.FromAccount.Currency,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.TransferTxParams{
					FromAccountID: result.Transfer.FromAccountID,
				}

				store.EXPECT().GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "FromAccountNotFound",
			body: req.TransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
				Amount:        result.Transfer.Amount,
				Currency:      result.FromAccount.Currency,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.TransferTxParams{
					FromAccountID: result.Transfer.FromAccountID,
				}

				store.EXPECT().GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(1).
					Return(db.Account{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ToAccountError",
			body: req.TransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
				Amount:        result.Transfer.Amount,
				Currency:      result.FromAccount.Currency,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.TransferTxParams{
					FromAccountID: result.Transfer.FromAccountID,
					ToAccountID:   result.Transfer.ToAccountID,
				}

				store.EXPECT().GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().GetAccount(gomock.Any(), gomock.Eq(arg.ToAccountID)).
					Times(1).
					Return(db.Account{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "AmountBadRequest",
			body: req.TransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
				Amount:        -1,
				Currency:      result.FromAccount.Currency,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.TransferTxParams{
					FromAccountID: result.Transfer.FromAccountID,
					ToAccountID:   result.Transfer.ToAccountID,
					Amount:        result.Transfer.Amount,
				}

				store.EXPECT().
					TransferTx(gomock.Any(), gomock.Eq(arg)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "CurrencyBadRequest",
			body: req.TransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
				Amount:        result.Transfer.Amount,
				Currency:      "CAD1",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.TransferTxParams{
					FromAccountID: result.Transfer.FromAccountID,
					ToAccountID:   result.Transfer.ToAccountID,
					Amount:        result.Transfer.Amount,
				}

				store.EXPECT().
					TransferTx(gomock.Any(), gomock.Eq(arg)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InternalError",
			body: req.TransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
				Amount:        result.Transfer.Amount,
				Currency:      result.FromAccount.Currency,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.TransferTxParams{
					FromAccountID: result.Transfer.FromAccountID,
					ToAccountID:   result.Transfer.ToAccountID,
					Amount:        result.Transfer.Amount,
				}

				store.EXPECT().GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().GetAccount(gomock.Any(), gomock.Eq(arg.ToAccountID)).
					Times(1).
					Return(result.ToAccount, nil)

				store.EXPECT().
					TransferTx(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(db.TransferTxResult{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
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
			//build stubs
			tc.buildStubs(store)

			//start new server
			cfg, err := pkg.LoadConfig("../../")
			require.NoError(t, err)

			server := server.NewTestServer(t, store, &cfg, nil)

			transferHandler := NewTransferHandler(server)
			transferHandler.MapRoutes()
			recorder := httptest.NewRecorder()

			url := "/transfer"

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.TokenMaker)
			server.Router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetTransfers(t *testing.T) {
	// Create a new transferResult
	result := RandomTxResult(t)

	testCases := []struct {
		name          string
		body          req.GetTransferRequest
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: req.GetTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.GetTransfersParams{
					FromAccountID: result.Transfer.FromAccountID,
					ToAccountID:   result.Transfer.ToAccountID,
				}

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg.ToAccountID)).
					Times(1).
					Return(result.ToAccount, nil)

				store.EXPECT().
					GetTransfers(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.GetTransfersRow{
						{
							FromAccountID: result.Transfer.FromAccountID,
							ToAccountID:   result.Transfer.ToAccountID,
							Amount:        result.Transfer.Amount,
							CreatedAt:     result.Transfer.CreatedAt,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMathTransfers(t, recorder.Body, []db.Transfer{result.Transfer})
			},
		},
		{
			name: "UnauthorizedUser",
			body: req.GetTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, "unauthorized_user", util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.GetTransfersParams{
					FromAccountID: result.Transfer.FromAccountID,
					ToAccountID:   result.Transfer.ToAccountID,
				}

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg.ToAccountID)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: req.GetTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.GetTransfersParams{
					FromAccountID: result.Transfer.FromAccountID,
				}

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "FromAccountNotFound",
			body: req.GetTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.GetTransfersParams{
					FromAccountID: result.Transfer.FromAccountID,
				}

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(1).
					Return(db.Account{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ToAccountNotFound",
			body: req.GetTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.GetTransfersParams{
					FromAccountID: result.Transfer.FromAccountID,
					ToAccountID:   result.Transfer.ToAccountID,
				}

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg.ToAccountID)).
					Times(1).
					Return(db.Account{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InternalError",
			body: req.GetTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
				ToAccountID:   result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.GetTransfersParams{
					FromAccountID: result.Transfer.FromAccountID,
					ToAccountID:   result.Transfer.ToAccountID,
				}

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg.FromAccountID)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg.ToAccountID)).
					Times(1).
					Return(result.ToAccount, nil)

				store.EXPECT().
					GetTransfers(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.GetTransfersRow{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "BadRequest",
			body: req.GetTransferRequest{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := db.GetTransfersParams{}

				store.EXPECT().
					GetTransfers(gomock.Any(), gomock.Eq(arg)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
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

			transferHandler := NewTransferHandler(server)
			transferHandler.MapRoutes()
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/transfers?fromAccountId=%d&toAccountId=%d",
				tc.body.FromAccountID,
				tc.body.ToAccountID)

			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.TokenMaker)
			server.Router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetFromAccountTransfers(t *testing.T) {
	// Create a new transferResult
	result := RandomTxResult(t)

	testCases := []struct {
		name          string
		body          req.GetFromAccountTransferRequest
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: req.GetFromAccountTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.FromAccountID

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().
					GetTransfersByFromAccountId(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.GetTransfersByFromAccountIdRow{
						{
							FromAccountID: result.Transfer.FromAccountID,
							ToAccountID:   result.Transfer.ToAccountID,
							Amount:        result.Transfer.Amount,
							CreatedAt:     result.Transfer.CreatedAt,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMathTransfers(t, recorder.Body, []db.Transfer{result.Transfer})
			},
		},
		{
			name: "UnAuthorizedUser",
			body: req.GetFromAccountTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, "unauthorized_user", util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.FromAccountID

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().
					GetTransfersByFromAccountId(gomock.Any(), gomock.Eq(arg)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: req.GetFromAccountTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.FromAccountID

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "FromAccountNotFound",
			body: req.GetFromAccountTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.FromAccountID

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(db.Account{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InternalError",
			body: req.GetFromAccountTransferRequest{
				FromAccountID: result.Transfer.FromAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.FromAccountID

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(result.FromAccount, nil)

				store.EXPECT().
					GetTransfersByFromAccountId(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.GetTransfersByFromAccountIdRow{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "BadRequest",
			body: req.GetFromAccountTransferRequest{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.FromAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.FromAccountID

				store.EXPECT().
					GetTransfersByFromAccountId(gomock.Any(), gomock.Eq(arg)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
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

			transferHandler := NewTransferHandler(server)
			transferHandler.MapRoutes()
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/transfers/from?fromAccountId=%d",
				tc.body.FromAccountID)

			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.TokenMaker)
			server.Router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetToAccountTransfers(t *testing.T) {
	// Create a new transferResult
	result := RandomTxResult(t)

	testCases := []struct {
		name          string
		body          req.GetToAccountTransferRequest
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: req.GetToAccountTransferRequest{
				ToAccountID: result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.ToAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.ToAccountID

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(result.ToAccount, nil)

				store.EXPECT().
					GetTransfersByToAccountId(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.GetTransfersByToAccountIdRow{
						{
							FromAccountID: result.Transfer.FromAccountID,
							ToAccountID:   result.Transfer.ToAccountID,
							Amount:        result.Transfer.Amount,
							CreatedAt:     result.Transfer.CreatedAt,
						},
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusOK, recorder.Code)
				requireBodyMathTransfers(t, recorder.Body, []db.Transfer{result.Transfer})
			},
		},
		{
			name: "UnAuthorizedUser",
			body: req.GetToAccountTransferRequest{
				ToAccountID: result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, "unauthorized_user", util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.ToAccountID

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(result.ToAccount, nil)

				store.EXPECT().
					GetTransfersByToAccountId(gomock.Any(), gomock.Eq(arg)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: req.GetToAccountTransferRequest{
				ToAccountID: result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.ToAccountID

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg)).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "FromAccountNotFound",
			body: req.GetToAccountTransferRequest{
				ToAccountID: result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.ToAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.ToAccountID

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(db.Account{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InternalError",
			body: req.GetToAccountTransferRequest{
				ToAccountID: result.Transfer.ToAccountID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.ToAccount.Owner, util.DepositorRole, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.ToAccountID

				store.EXPECT().
					GetAccount(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return(result.ToAccount, nil)

				store.EXPECT().
					GetTransfersByToAccountId(gomock.Any(), gomock.Eq(arg)).
					Times(1).
					Return([]db.GetTransfersByToAccountIdRow{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name: "BadRequest",
			body: req.GetToAccountTransferRequest{},
			buildStubs: func(store *mockdb.MockStore) {
				arg := result.Transfer.ToAccountID

				store.EXPECT().
					GetTransfersByToAccountId(gomock.Any(), gomock.Eq(arg)).
					Times(0)
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				auth.AddAuthorization(t, request, tokenMaker, auth.AuthTypeBearer, result.ToAccount.Owner, util.DepositorRole, time.Minute)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				if recorder.Code != http.StatusOK {
					t.Log("Response body: ", recorder.Body.String())
				}
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

			transferHandler := NewTransferHandler(server)
			transferHandler.MapRoutes()
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/transfers/to?toAccountId=%d",
				tc.body.ToAccountID)

			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.TokenMaker)
			server.Router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func requireBodyMatchTxResult(t *testing.T, body *bytes.Buffer, txResult db.TransferTxResult) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var response struct {
		Data       db.TransferTxResult `json:"data"`
		Message    string              `json:"message"`
		StatusCode int                 `json:"statusCode"`
	}

	err = json.Unmarshal(data, &response)

	require.NoError(t, err)
	require.Equal(t, txResult, response.Data)

}

func requireBodyMathTransfers(t *testing.T, body *bytes.Buffer, transfers []db.Transfer) {
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var response struct {
		Data       []db.Transfer `json:"data"`
		Message    string        `json:"message"`
		StatusCode int           `json:"statusCode"`
	}

	err = json.Unmarshal(data, &response)

	require.NoError(t, err)

	for i, transfer := range transfers {
		require.Equal(t, transfer.Amount, response.Data[i].Amount)
		require.Equal(t, transfer.FromAccountID, response.Data[i].FromAccountID)
		require.Equal(t, transfer.ToAccountID, response.Data[i].ToAccountID)

		require.NotEmpty(t, transfer.ID)
		require.WithinDuration(t, transfer.CreatedAt, response.Data[i].CreatedAt, time.Second)
	}
}
