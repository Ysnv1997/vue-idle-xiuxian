package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kowming/vue-idle-xiuxian/backend/internal/service"
)

func PassiveProgress(progressService *service.PassiveProgressService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if progressService == nil {
			c.Next()
			return
		}

		userID, ok := UserIDFromContext(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		// 活跃心跳仅用于统计展示，失败不应阻塞主流程。
		_ = progressService.TouchActivity(c.Request.Context(), userID)

		c.Next()
	}
}
