package account

type CreateAccountRequest struct {
	Owner    string `json:"owner" binding:"required"`
	Currency string `json:"currency" binding:"required,oneof=USD EUR CAD VND"`
}

type GetAccountRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type ListAccountRequest struct {
	Page int32 `form:"page" binding:"required,min=1"`
	Size int32 `form:"size" binding:"required,min=5,max=10"`
}
