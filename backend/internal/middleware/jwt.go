package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type UserCompanyClaim struct {
	CompanyID string `json:"company_id"`
	Role      string `json:"role"`
}

type Claims struct {
	UserID      string             `json:"user_id"`
	Email       string             `json:"email"`
	Companies   []UserCompanyClaim `json:"companies"`
	Role        string             `json:"role"`
	Apps        []string           `json:"apps"`
	Permissions []string           `json:"permissions"`
	jwt.RegisteredClaims
}

func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Authorization header required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid authorization format"})
			return
		}

		tokenString := parts[1]
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid or expired token"})
			return
		}

		// Check if this app (fleet) is authorized for this user
		isSuperAdmin := claims.Role == "super_admin"
		hasAppAccess := false
		for _, appCode := range claims.Apps {
			if appCode == "inv" {
				hasAppAccess = true
				break
			}
		}

		if !isSuperAdmin && !hasAppAccess {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "You do not have access permission to Inventory Management System"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("companies", claims.Companies)
		c.Set("permissions", claims.Permissions)
		c.Next()
	}
}

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role == "super_admin" {
			c.Next()
			return
		}
		perms, exists := c.Get("permissions")
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
			return
		}
		permList, ok := perms.([]string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
			return
		}
		for _, p := range permList {
			if p == permission {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
	}
}

func RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "super_admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "Access denied"})
			return
		}
		c.Next()
	}
}

