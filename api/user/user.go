package user

import (
	"database/sql"
	"net/http"

	dto "github.com/ChokeGuy/simple-bank/api/user/dto"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	res "github.com/ChokeGuy/simple-bank/pkg/http_response"
	sv "github.com/ChokeGuy/simple-bank/server"
	"github.com/ChokeGuy/simple-bank/util/password"
	"github.com/lib/pq"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	*sv.Server
}

func NewUserHandler(server *sv.Server) *UserHandler {
	return &UserHandler{Server: server}
}

func (h *UserHandler) MapRoutes() {
	router := h.Router

	router.POST("/user", h.createUser)
	router.POST("/auth/login", h.loginUser)
	router.POST("/auth/refresh", h.refreshNewToken)
	router.GET("/user", h.getUserByUserName)
}

func (h *UserHandler) createUser(ctx *gin.Context) {
	var req dto.CreateUserRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	hashedPassword, err := password.HashPassword(req.Password)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	arg := db.CreateUserParams{
		Username:       req.UserName,
		FullName:       req.FullName,
		Email:          req.Email,
		HashedPassword: hashedPassword,
	}

	result, err := h.Store.CreateUser(ctx, arg)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code.Name() {
			case "unique_violation":
				ctx.JSON(http.StatusForbidden, res.ErrorResponse(http.StatusForbidden, pqErr.Message))
				return
			}
		}

		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	response := dto.UserResponse{
		UserName:          result.Username,
		FullName:          result.FullName,
		Email:             result.Email,
		PasswordChangedAt: result.PasswordChangedAt.String(),
		CreatedAt:         result.CreatedAt.String(),
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(response, "User created successfully"))
}

func (h *UserHandler) getUserByUserName(ctx *gin.Context) {
	var req dto.GetUserByUserNameRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	result, err := h.Store.GetUserByUserName(ctx, req.UserName)

	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, res.ErrorResponse(http.StatusNotFound, "User not found"))
			return
		}
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	response := dto.GetUserByUserNameResponse{
		UserName:          result.Username,
		FullName:          result.FullName,
		Email:             result.Email,
		PasswordChangedAt: result.PasswordChangedAt.String(),
		CreatedAt:         result.CreatedAt.String(),
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(response, "User retrieved successfully"))
}

func (h *UserHandler) loginUser(ctx *gin.Context) {
	var req dto.LoginUserRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	user, err := h.Store.GetUserByUserName(ctx, req.UserName)

	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, res.ErrorResponse(http.StatusNotFound, "User not found"))
			return
		}
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	if err := password.CheckPassword(req.Password, user.HashedPassword); err != nil {
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Invalid password"))
		return
	}

	accessToken, err := h.TokenMaker.CreateToken(user.Username, h.Config.AccessTokenDuration)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	refreshToken, err := h.TokenMaker.CreateToken(user.Username, h.Config.RefreshTokenDuration)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	response := dto.LoginUserResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: dto.UserResponse{
			UserName:          user.Username,
			FullName:          user.FullName,
			Email:             user.Email,
			PasswordChangedAt: user.PasswordChangedAt.String(),
			CreatedAt:         user.CreatedAt.String(),
		},
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(response, "User logged in successfully"))
}

func (h *UserHandler) refreshNewToken(ctx *gin.Context) {
	var req dto.RefreshTokenRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	claims, err := h.TokenMaker.VerifyToken(req.RefreshToken)

	if err != nil {
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Invalid refresh token"))
		return
	}

	accessToken, err := h.TokenMaker.CreateToken(claims.UserName, h.Config.AccessTokenDuration)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	response := dto.RefreshTokenResponse{
		AccessToken: accessToken,
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(response, "Token refreshed successfully"))
}
