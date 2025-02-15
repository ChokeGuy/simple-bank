package user

import (
	"time"

	"github.com/google/uuid"
)

type UserResponse struct {
	UserName          string `json:"userName"`
	FullName          string `json:"fullName"`
	Email             string `json:"email"`
	PasswordChangedAt string `json:"passwordChangedAt"`
	CreatedAt         string `json:"createdAt"`
}

type GetUserByUserNameResponse = UserResponse

type LoginUserResponse struct {
	SessionID             uuid.UUID    `json:"sessionId"`
	AccessToken           string       `json:"accessToken"`
	AccessTokenExpiresAt  time.Time    `json:"accessTokenExpiresAt"`
	RefreshToken          string       `json:"refreshToken"`
	RefreshTokenExpiresAt time.Time    `json:"refreshTokenExpiresAt"`
	User                  UserResponse `json:"user"`
}

type RefreshTokenResponse struct {
	AccessToken          string    `json:"accessToken"`
	AccessTokenExpiresAt time.Time `json:"accessTokenExpiresAt"`
}

type VerifyUserEmailResponse struct {
	IsVerified bool `json:"isVerified"`
}
