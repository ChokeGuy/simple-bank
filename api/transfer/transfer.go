package transfer

import (
	"database/sql"
	"fmt"
	"net/http"

	dto "github.com/ChokeGuy/simple-bank/api/transfer/dto"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	res "github.com/ChokeGuy/simple-bank/pkg/http_response"
	"github.com/ChokeGuy/simple-bank/pkg/middlewares/auth"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	sv "github.com/ChokeGuy/simple-bank/server"
	"github.com/gin-gonic/gin"
)

type TransferHandler struct {
	*sv.Server
}

func NewTransferHandler(server *sv.Server) *TransferHandler {
	return &TransferHandler{Server: server}
}

func (h *TransferHandler) MapRoutes() {
	router := h.Router

	authRoutes := router.Group("/").Use(auth.AuthMiddleWare(h.TokenMaker))

	authRoutes.POST("/transfer", h.createTransfer)
	authRoutes.GET("/transfers", h.getTransfers)
	authRoutes.GET("/transfers/from", h.getFromAccountTransfers)
	authRoutes.GET("/transfers/to", h.getToAccountTransfers)
}

func (h *TransferHandler) createTransfer(ctx *gin.Context) {
	var req dto.TransferRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	statusCode, err := h.validTx(ctx, req)

	if err != nil {
		ctx.JSON(statusCode, res.ErrorResponse(statusCode, err.Error()))
		return
	}

	arg := db.TransferTxParams{
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		Amount:        req.Amount,
	}

	result, err := h.Store.TransferTx(ctx, arg)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(result, "Transfer created successfully"))
}

func (h *TransferHandler) getTransfers(ctx *gin.Context) {
	var req dto.GetTransferRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	if fromAccount, statusCode, err := h.getValidAccount(ctx, req.FromAccountID); err != nil {
		ctx.JSON(statusCode, res.ErrorResponse(statusCode, err.Error()))
		return
	} else {
		authPayload := ctx.MustGet(auth.AuthPayloadKey).(*token.Payload)
		if fromAccount.Owner != authPayload.UserName {
			ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "account does not belong to user"))
			return
		}
	}

	if _, statusCode, err := h.getValidAccount(ctx, req.ToAccountID); err != nil {
		ctx.JSON(statusCode, res.ErrorResponse(statusCode, err.Error()))
		return
	}

	arg := db.GetTransfersParams{
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
	}

	result, err := h.Store.GetTransfers(ctx, arg)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(result, "Transfer history retrieved successfully"))
}

func (h *TransferHandler) getFromAccountTransfers(ctx *gin.Context) {
	var req dto.GetFromAccountTransferRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	if fromAccount, statusCode, err := h.getValidAccount(ctx, req.FromAccountID); err != nil {
		ctx.JSON(statusCode, res.ErrorResponse(statusCode, err.Error()))
		return
	} else {
		authPayload := ctx.MustGet(auth.AuthPayloadKey).(*token.Payload)
		if fromAccount.Owner != authPayload.UserName {
			ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "account does not belong to user"))
			return
		}
	}

	result, err := h.Store.GetTransfersByFromAccountId(ctx, req.FromAccountID)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(result, "Transfer history retrieved successfully"))
}

func (h *TransferHandler) getToAccountTransfers(ctx *gin.Context) {
	var req dto.GetToAccountTransferRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	if toAccount, statusCode, err := h.getValidAccount(ctx, req.ToAccountID); err != nil {
		ctx.JSON(statusCode, res.ErrorResponse(statusCode, err.Error()))
		return
	} else {
		authPayload := ctx.MustGet(auth.AuthPayloadKey).(*token.Payload)
		if toAccount.Owner != authPayload.UserName {
			ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "account does not belong to user"))
			return
		}
	}

	result, err := h.Store.GetTransfersByToAccountId(ctx, req.ToAccountID)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(result, "Transfer history retrieved successfully"))
}

func (h *TransferHandler) getValidAccount(ctx *gin.Context, id int64) (db.Account, int, error) {
	account, err := h.Store.GetAccount(ctx, id)
	if err == sql.ErrNoRows {
		return db.Account{}, http.StatusBadRequest, fmt.Errorf("account with id %d not found", id)
	}

	return account, http.StatusOK, nil
}

func (h *TransferHandler) validTx(ctx *gin.Context, req dto.TransferRequest) (int, error) {
	// Validate "From" account
	fromAccount, statusCode, err := h.getValidAccount(ctx, req.FromAccountID)
	if err != nil {
		return statusCode, err
	}

	authPayload := ctx.MustGet(auth.AuthPayloadKey).(*token.Payload)
	if fromAccount.Owner != authPayload.UserName {
		return http.StatusUnauthorized, fmt.Errorf("account does not belong to user")
	}

	// Validate "To" account
	toAccount, statusCode, err := h.getValidAccount(ctx, req.ToAccountID)
	if err != nil {
		return statusCode, err
	}

	// Check for currency mismatch
	if fromAccount.Currency != req.Currency || toAccount.Currency != req.Currency {
		return http.StatusBadRequest, fmt.Errorf("account currency mismatch")
	}

	// Check for sufficient balance
	if fromAccount.Balance < req.Amount {
		return http.StatusBadRequest, fmt.Errorf("insufficient account balance")
	}

	return http.StatusOK, nil
}
