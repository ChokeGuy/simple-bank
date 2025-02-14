package user

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	dto "github.com/ChokeGuy/simple-bank/api/user/dto"
	db "github.com/ChokeGuy/simple-bank/db/sqlc"
	pkg "github.com/ChokeGuy/simple-bank/pkg/config"
	res "github.com/ChokeGuy/simple-bank/pkg/http_response"
	"github.com/ChokeGuy/simple-bank/pkg/middlewares/auth"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/ChokeGuy/simple-bank/pkg/token/paseto"
	sv "github.com/ChokeGuy/simple-bank/server/http"
	"github.com/ChokeGuy/simple-bank/util"
	pw "github.com/ChokeGuy/simple-bank/util/password"
	"github.com/ChokeGuy/simple-bank/worker"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

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
	router.POST("/auth/refresh-token", h.refreshNewToken)
	router.GET("/user", h.getUserByUserName)

	authRoutes := router.Group("/").Use(auth.AuthMiddleWare(h.TokenMaker))
	authRoutes.PATCH("/user/update", h.updateUser)
}

// Create radom user
func RandomUser(t *testing.T) (db.User, string) {
	password := util.RandomPassword()
	hashedPassword, err := pw.HashPassword(password)
	require.NoError(t, err)

	return db.User{
		Username:       util.RandomOwner(),
		FullName:       util.RandomOwner(),
		Email:          util.RandomEmail(),
		HashedPassword: hashedPassword,
	}, password
}

func RandomToken(t *testing.T, userName string) string {
	cfg, err := pkg.LoadConfig("../..")
	require.NoError(t, err)

	paseto, err := paseto.NewPasetoMaker(cfg.SymetricKey)
	require.NoError(t, err)

	token, _, err := paseto.CreateToken(userName, time.Hour)
	require.NoError(t, err)

	return token
}

func RandomSession(t *testing.T, userName string) (db.Session, string) {
	token := RandomToken(t, userName)
	return db.Session{
		ID:           uuid.New(),
		Username:     userName,
		RefreshToken: token,
		UserAgent:    util.RandomString(6),
		ClientIp:     util.RandomString(6),
		IsBlocked:    false,
		ExpiresAt:    time.Now().Add(time.Hour),
	}, token
}

func (h *UserHandler) createUser(ctx *gin.Context) {
	var req dto.CreateUserRequest

	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, res.ErrorResponse(http.StatusBadRequest, err.Error()))
		return
	}

	hashedPassword, err := pw.HashPassword(req.Password)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	arg := db.CreateUserTxParams{
		CreateUserParams: db.CreateUserParams{
			Username:       req.UserName,
			FullName:       req.FullName,
			Email:          req.Email,
			HashedPassword: hashedPassword,
		},
		AfterCreate: func(user db.User) error {
			//Send verification email to user
			taskPayload := &worker.PayloadSendVerifyEmail{
				UserName: user.Username,
			}

			opts := []asynq.Option{
				asynq.MaxRetry(10),
				asynq.ProcessIn(10 * time.Second),
				asynq.Queue(worker.QueueDefault),
			}

			return h.TaskDistributor.DistributeTaskSendVerifyEmail(ctx, taskPayload, opts...)
		},
	}

	user, err := h.Store.CreateUserTx(ctx, arg)

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
		UserName:          user.User.Username,
		FullName:          user.User.FullName,
		Email:             user.User.Email,
		PasswordChangedAt: user.User.PasswordChangedAt.String(),
		CreatedAt:         user.User.CreatedAt.String(),
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(response, "User created successfully"))
}

func (h *UserHandler) getUserByUserName(ctx *gin.Context) {
	var req dto.GetUserByUserNameRequest

	if err := ctx.ShouldBindQuery(&req); err != nil {
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

	response := dto.GetUserByUserNameResponse{
		UserName:          user.Username,
		FullName:          user.FullName,
		Email:             user.Email,
		PasswordChangedAt: user.PasswordChangedAt.String(),
		CreatedAt:         user.CreatedAt.String(),
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

	if err := pw.CheckPassword(req.Password, user.HashedPassword); err != nil {
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Invalid password"))
		return
	}

	if session, _ := h.Store.GetSessionByUserName(ctx, req.UserName); session.ID != uuid.Nil {
		// Check if session is expired
		if time.Now().After(session.ExpiresAt) {
			// Delete expired session
			err := h.Store.DeleteSession(ctx, session.ID)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, "Failed to delete expired session"))
				return
			}
		} else if session.IsBlocked {
			// Session exists and is not blocked
			ctx.JSON(http.StatusForbidden, res.ErrorResponse(http.StatusForbidden, "Session is blocked"))
			return
		} else {
			// Session exists and is not blocked
			ctx.JSON(http.StatusForbidden, res.ErrorResponse(http.StatusForbidden, "User session already exists"))
			return
		}
	}

	accessToken, aTkPayload, err := h.TokenMaker.CreateToken(user.Username, h.Config.AccessTokenDuration)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	refreshToken, rTkPayload, err := h.TokenMaker.CreateToken(user.Username, h.Config.RefreshTokenDuration)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	arg := db.CreateSessionParams{
		ID:           uuid.MustParse(rTkPayload.ID),
		Username:     user.Username,
		RefreshToken: refreshToken,
		UserAgent:    ctx.Request.UserAgent(),
		ClientIp:     ctx.ClientIP(),
		IsBlocked:    false,
		ExpiresAt:    rTkPayload.ExpiresAt.Time,
	}

	session, err := h.Store.CreateSession(ctx, arg)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	response := dto.LoginUserResponse{
		SessionID:             session.ID,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  aTkPayload.ExpiresAt.Time,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: rTkPayload.ExpiresAt.Time,
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

func (h *UserHandler) updateUser(ctx *gin.Context) {
	var req dto.UpdateUserRequest

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

	authPayload := ctx.MustGet(auth.AuthPayloadKey).(*token.Payload)

	if authPayload.UserName != user.Username {
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Unauthorized user"))
		return
	}

	arg := db.UpdateUserParams{
		Username: req.UserName,
		FullName: sql.NullString{String: req.FullName, Valid: req.FullName != ""},
		Email:    sql.NullString{String: req.Email, Valid: req.Email != ""},
	}

	updateUser, err := h.Store.UpdateUser(ctx, arg)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	response := dto.UserResponse{
		UserName:          updateUser.Username,
		FullName:          updateUser.FullName,
		Email:             updateUser.Email,
		PasswordChangedAt: updateUser.PasswordChangedAt.String(),
		CreatedAt:         updateUser.CreatedAt.String(),
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(response, "User updated successfully"))
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

	session, err := h.Store.GetSessionById(ctx, uuid.MustParse(claims.ID))

	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, res.ErrorResponse(http.StatusNotFound, "Session not found"))
			return
		}
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Invalid session"))
		return
	}

	if session.IsBlocked {
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Session is blocked"))
		return
	}

	if session.Username != claims.UserName {
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Incorrect session user"))
		return
	}

	if session.RefreshToken != req.RefreshToken {
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Mismatched session token"))
		return
	}

	if time.Now().After(session.ExpiresAt) {
		ctx.JSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, "Session expired"))
		return
	}

	accessToken, payload, err := h.TokenMaker.CreateToken(claims.UserName, h.Config.AccessTokenDuration)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, res.ErrorResponse(http.StatusInternalServerError, err.Error()))
		return
	}

	response := dto.RefreshTokenResponse{
		AccessToken:          accessToken,
		AccessTokenExpiresAt: payload.ExpiresAt.Time,
	}

	ctx.JSON(http.StatusOK, res.SuccessResponse(response, "Token refreshed successfully"))
}
