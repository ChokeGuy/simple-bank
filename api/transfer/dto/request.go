package transfer

type TransferRequest struct {
	FromAccountID int64  `json:"fromAccountId" binding:"required,min=1"`
	ToAccountID   int64  `json:"toAccountId" binding:"required,min=1"`
	Amount        int64  `json:"amount" binding:"required,gt=0"`
	Currency      string `json:"currency" binding:"required,currency"`
}

type GetTransferRequest struct {
	FromAccountID int64 `form:"fromAccountId" binding:"required,min=1"`
	ToAccountID   int64 `form:"toAccountId" binding:"required,min=1"`
}

type GetFromAccountTransferRequest struct {
	FromAccountID int64 `form:"fromAccountId" binding:"required,min=1"`
}

type GetToAccountTransferRequest struct {
	ToAccountID int64 `form:"toAccountId" binding:"required,min=1"`
}
