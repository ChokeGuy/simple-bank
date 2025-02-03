package user

type CreateUserResponse struct {
	UserName          string `json:"userName"`
	FullName          string `json:"fullName"`
	Email             string `json:"email"`
	PasswordChangedAt string `json:"passwordChangedAt"`
	CreatedAt         string `json:"createdAt"`
}

type GetUserByUserNameResponse = CreateUserResponse
