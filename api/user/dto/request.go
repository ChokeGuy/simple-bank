package user

type CreateUserRequest struct {
	UserName string `json:"userName" binding:"required,alphanum"`
	FullName string `json:"fullName" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,password"`
}

type GetUserByUserNameRequest struct {
	UserName string `form:"userName" binding:"required,alphanum"`
}
