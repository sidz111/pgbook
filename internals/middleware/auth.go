package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(secret string, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Cookie madhun token kadha
		tokenString, err := c.Cookie("access_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// 2. Token Parse kara
		token, _ := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userRole := claims["role"].(string)

			// 3. Role Check kara
			roleAllowed := false
			for _, r := range roles {
				if r == userRole {
					roleAllowed = true
					break
				}
			}

			if !roleAllowed {
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
				c.Abort()
				return
			}

			c.Set("userID", claims["sub"])
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
		}
	}
}
