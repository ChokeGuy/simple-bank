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

type LoginUserRequest struct {
	UserName string `json:"userName" binding:"required,alphanum"`
	Password string `json:"password" binding:"required,password"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type UpdateUserRequest struct {
	UserName string `json:"userName" binding:"alphanum"`
	FullName string `json:"fullName" binding:"omitempty,min=3,max=100"`
	Email    string `json:"email" binding:"omitempty,email"`
}

type VerifyUserEmailRequest struct {
	EmailId    int64  `form:"emailId" binding:"required"`
	SecretCode string `form:"secretCode" binding:"required"`
}
