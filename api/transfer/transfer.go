package transfer

import (
	"database/sql"
	"fmt"
	"net/http"

	dto "github.com/ChokeGuy/simple-bank/api/transfer/dto"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	res "github.com/ChokeGuy/simple-bank/pkg/http_response"
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

	router.POST("/transfer", h.createTransfer)
	router.GET("/transfers", h.getTransfers)
	router.GET("/transfers/from", h.getFromAccountTransfers)
	router.GET("/transfers/to", h.getToAccountTransfers)
}

func (h *TransferHandler) createTransfer(ctx *gin.Context) {
	var req dto.TransferRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	if err := h.validTx(ctx, req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
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

	if _, err := h.getValidAccount(ctx, req.FromAccountID); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	if _, err := h.getValidAccount(ctx, req.ToAccountID); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
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

	if _, err := h.getValidAccount(ctx, req.FromAccountID); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
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

	if _, err := h.getValidAccount(ctx, req.ToAccountID); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	result, err := h.Store.GetTransfersByToAccountId(ctx, req.ToAccountID)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(result, "Transfer history retrieved successfully"))
}

func (h *TransferHandler) getValidAccount(ctx *gin.Context, id int64) (db.Account, error) {
	account, err := h.Store.GetAccount(ctx, id)
	if err == sql.ErrNoRows {
		return db.Account{}, fmt.Errorf("account with id %d not found", id)
	}

	return account, nil
}

func (h *TransferHandler) validTx(ctx *gin.Context, req dto.TransferRequest) error {
	// Validate "From" account
	fromAccount, err := h.getValidAccount(ctx, req.FromAccountID)
	if err != nil {
		return err
	}

	// Validate "To" account
	toAccount, err := h.getValidAccount(ctx, req.ToAccountID)
	if err != nil {
		return err
	}

	// Check for currency mismatch
	if fromAccount.Currency != req.Currency || toAccount.Currency != req.Currency {
		return fmt.Errorf("account currency mismatch")
	}

	// Check for sufficient balance
	if fromAccount.Balance < req.Amount {
		return fmt.Errorf("insufficient account balance")
	}

	return nil
}
