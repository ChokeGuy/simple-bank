package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	res "github.com/ChokeGuy/simple-bank/pkg/http_response"
	"github.com/ChokeGuy/simple-bank/pkg/token"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	AuthHeaderKey  = "authorization"
	AuthTypeBearer = "bearer"
	AuthPayloadKey = "auth_payload"
)

// AddAuthorization adds an authorization header to the request for testing
func AddAuthorization(
	t *testing.T,
	request *http.Request,
	tokenMaker token.Maker,
	authType string,
	username string,
	role string,
	duration time.Duration,
) {
	token, payload, err := tokenMaker.CreateToken(username, role, duration)
	require.NoError(t, err)
	require.NotEmpty(t, payload)

	authHeader := fmt.Sprintf("%s %s", authType, token)
	request.Header.Set(AuthHeaderKey, authHeader)
}

// AuthMiddleWare is a gin middleware for authentication
func AuthMiddleWare(tokenMaker token.Maker) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authHeader := ctx.GetHeader(AuthHeaderKey)

		if len(authHeader) == 0 {
			err := errors.New("authorization header is not provided")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, err.Error()))
			return
		}

		fields := strings.Fields(authHeader)
		if len(fields) != 2 {
			err := errors.New("invalid authorization header format")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, err.Error()))
			return
		}

		authType := strings.ToLower(fields[0])

		if authType != AuthTypeBearer {
			err := errors.New("unsupported authorization type")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, err.Error()))
			return
		}

		accessToken := fields[1]

		payload, err := tokenMaker.VerifyToken(accessToken)

		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, res.ErrorResponse(http.StatusUnauthorized, err.Error()))
			return
		}

		ctx.Set(AuthPayloadKey, payload)
		ctx.Next()
	}
}
