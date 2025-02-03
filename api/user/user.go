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
	Server *sv.Server
}

func NewUserHandler(server *sv.Server) *UserHandler {
	return &UserHandler{Server: server}
}

func (h *UserHandler) MapRoutes() {
	router := h.Server.Router

	router.POST("/user", h.createUser)
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

	result, err := h.Server.Store.CreateUser(ctx, arg)

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

	response := dto.CreateUserResponse{
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

	result, err := h.Server.Store.GetUserByUserName(ctx, req.UserName)

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
