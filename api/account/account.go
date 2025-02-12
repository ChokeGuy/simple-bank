package account

import (
	"database/sql"
	"net/http"

	dto "github.com/ChokeGuy/simple-bank/api/account/dto"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	res "github.com/ChokeGuy/simple-bank/pkg/http_response"
	"github.com/ChokeGuy/simple-bank/pkg/middlewares/auth"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	sv "github.com/ChokeGuy/simple-bank/server/http"
	"github.com/ChokeGuy/simple-bank/util"
	"github.com/lib/pq"

	"github.com/gin-gonic/gin"
)

type AccountHandler struct {
	*sv.Server
}

func NewAccountHandler(server *sv.Server) *AccountHandler {
	return &AccountHandler{Server: server}
}

func RandomAccount(owner string) db.Account {
	return db.Account{
		ID:       util.RandomInt(1, 1000),
		Owner:    owner,
		Balance:  util.RandomMoney(),
		Currency: util.RandomCurrency(),
	}
}

func (h *AccountHandler) MapRoutes() {
	router := h.Router

	authRoutes := router.Group("/").Use(auth.AuthMiddleWare(h.TokenMaker))

	authRoutes.POST("/account", h.createAccount)
	authRoutes.GET("/account/:id", h.getAccount)
	authRoutes.GET("/accounts", h.listAccounts)
	authRoutes.DELETE("/account/:id", h.deleteAccount)
}

func (h *AccountHandler) createAccount(ctx *gin.Context) {
	var req dto.CreateAccountRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	authPayload := ctx.MustGet(auth.AuthPayloadKey).(*token.Payload)

	arg := db.CreateAccountParams{
		Owner:    authPayload.UserName,
		Balance:  0,
		Currency: req.Currency,
	}

	account, err := h.Store.CreateAccount(ctx, arg)

	if err != nil {
		if pErr, ok := err.(*pq.Error); ok {
			switch pErr.Code.Name() {
			case "foreign_key_violation", "unique_violation":
				ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, pErr.Message))
				return
			}
		}

		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(account, "Account created successfully"))
}

func (h *AccountHandler) getAccount(ctx *gin.Context) {
	var req dto.GetAccountRequest

	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	account, err := h.Store.GetAccount(ctx, req.ID)

	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, res.ErrorResponse(http.StatusNotFound, "Account not found"))
			return
		}

		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}
	authPayload := ctx.MustGet(auth.AuthPayloadKey).(*token.Payload)

	if account.Owner != authPayload.UserName {
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Account does not belong to the authenticated user"))
		return
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(account, "Account retrieved successfully"))
}

func (h *AccountHandler) listAccounts(ctx *gin.Context) {
	var req dto.ListAccountRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	authPayload := ctx.MustGet(auth.AuthPayloadKey).(*token.Payload)

	arg := db.ListAccountsParams{
		Owner:  authPayload.UserName,
		Limit:  req.Size,
		Offset: (req.Page - 1) * req.Size,
	}

	accounts, err := h.Store.ListAccounts(ctx, arg)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	response := dto.ListAccountResponse{
		Accounts: accounts,
		Length:   len(accounts),
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(response, "Accounts retrieved successfully"))
}

func (h *AccountHandler) deleteAccount(ctx *gin.Context) {
	var req dto.DeleteAccountRequest
	var err error
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	account, err := h.Store.GetAccount(ctx, req.ID)

	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, res.ErrorResponse(http.StatusNotFound, "Account not found"))
			return
		}
	}

	authPayload := ctx.MustGet(auth.AuthPayloadKey).(*token.Payload)

	if account.Owner != authPayload.UserName {
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Account does not belong to the authenticated user"))
		return
	}

	err = h.Store.DeleteAccount(ctx, req.ID)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(nil, "Account deleted successfully"))
}
