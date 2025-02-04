package account

type CreateAccountRequest struct {
	Currency string `json:"currency" binding:"required,currency"`
}

type GetAccountRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type ListAccountRequest struct {
	Owner string `form:"owner"`
	Page  int32  `form:"page,default=1" binding:"min=1"`
	Size  int32  `form:"size" binding:"required,min=5,max=10"`
}

type DeleteAccountRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}
