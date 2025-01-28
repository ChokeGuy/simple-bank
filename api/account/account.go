package account

import (
	"database/sql"
	"net/http"

	dto "github.com/ChokeGuy/simple-bank/api/account/dto"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	res "github.com/ChokeGuy/simple-bank/pkg/http_response"
	sv "github.com/ChokeGuy/simple-bank/server"
	"github.com/ChokeGuy/simple-bank/util"

	"github.com/gin-gonic/gin"
)

type AccountHandler struct {
	Server *sv.Server
}

func NewAccountHandler(server *sv.Server) *AccountHandler {
	return &AccountHandler{Server: server}
}

func RandomAccount() db.Account {
	return db.Account{
		ID:       util.RandomInt(1, 1000),
		Owner:    util.RandomOwner(),
		Balance:  util.RandomMoney(),
		Currency: util.RandomCurrency(),
	}
}

func (h *AccountHandler) MapRoutes() {
	router := h.Server.Router

	router.POST("/account", h.createAccount)
	router.GET("/account/:id", h.getAccount)
	router.GET("/accounts", h.listAccounts)
	router.DELETE("/account/:id", h.deleteAccount)
}

func (h *AccountHandler) createAccount(ctx *gin.Context) {
	var req dto.CreateAccountRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	arg := db.CreateAccountParams{
		Owner:    req.Owner,
		Balance:  0,
		Currency: req.Currency,
	}

	account, err := h.Server.Store.CreateAccount(ctx, arg)

	if err != nil {
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

	account, err := h.Server.Store.GetAccount(ctx, req.ID)

	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, res.ErrorResponse(http.StatusNotFound, "Account not found"))
			return
		}

		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
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

	arg := db.ListAccountsParams{
		Limit:  req.Size,
		Offset: (req.Page - 1) * req.Size,
	}

	accounts, err := h.Server.Store.ListAccounts(ctx, arg)

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

	_, err = h.Server.Store.GetAccount(ctx, req.ID)
	if err == sql.ErrNoRows {
		ctx.JSON(http.StatusNotFound, res.ErrorResponse(http.StatusNotFound, "Account not found"))
		return
	}

	err = h.Server.Store.DeleteAccount(ctx, req.ID)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(nil, "Account deleted successfully"))
}
