package user

type UserResponse struct {
	UserName          string `json:"userName"`
	FullName          string `json:"fullName"`
	Email             string `json:"email"`
	PasswordChangedAt string `json:"passwordChangedAt"`
	CreatedAt         string `json:"createdAt"`
}

type GetUserByUserNameResponse = UserResponse

type LoginUserResponse struct {
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
	User         UserResponse `json:"user"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"accessToken"`
}
