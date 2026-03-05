package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

const userIDContextKey = "userID"
const linuxDoUserIDContextKey = "linuxDoUserID"

func Auth(tokenService *service.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			c.Abort()
			return
		}

		claims, err := tokenService.ValidateToken(parts[1], "access")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
			c.Abort()
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id in token"})
			c.Abort()
			return
		}

		c.Set(userIDContextKey, userID)
		c.Set(linuxDoUserIDContextKey, claims.LinuxDoUserID)
		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	value, ok := c.Get(userIDContextKey)
	if !ok {
		return uuid.Nil, false
	}
	userID, ok := value.(uuid.UUID)
	if !ok {
		return uuid.Nil, false
	}
	return userID, true
}

func LinuxDoUserIDFromContext(c *gin.Context) (string, bool) {
	value, ok := c.Get(linuxDoUserIDContextKey)
	if !ok {
		return "", false
	}
	linuxDoUserID, ok := value.(string)
	if !ok || linuxDoUserID == "" {
		return "", false
	}
	return linuxDoUserID, true
}
