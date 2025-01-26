package api

import (
	"database/sql"
	"net/http"

	dto "github.com/ChokeGuy/simple-bank/api/dto/account"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	res "github.com/ChokeGuy/simple-bank/pkg/http_response"

	"github.com/gin-gonic/gin"
)

func (server *Server) MapRoutes(router *gin.Engine) {
	router.POST("/account", server.createAccount)
	router.GET("/account/:id", server.getAccount)
	router.GET("/accounts", server.listAccount)
}

func (server *Server) createAccount(ctx *gin.Context) {
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

	account, err := server.store.CreateAccount(ctx, arg)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(account, "Account created successfully"))
}

func (server *Server) getAccount(ctx *gin.Context) {
	var req dto.GetAccountRequest

	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	account, err := server.store.GetAccount(ctx, req.ID)

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

func (server *Server) listAccount(ctx *gin.Context) {
	var req dto.ListAccountRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	arg := db.ListAccountsParams{
		Limit:  req.Size,
		Offset: (req.Page - 1) * req.Size,
	}

	accounts, err := server.store.ListAccounts(ctx, arg)

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
