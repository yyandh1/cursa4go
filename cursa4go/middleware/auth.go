package middleware

import (
	"cursa4go/config"
	"cursa4go/models"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// Сохранить сессию
func SaveSession(c *gin.Context, userID uint, role string) {
	session := sessions.Default(c)
	session.Set("user_id", userID)
	session.Set("role", role)
	session.Save()
}

// Очистить сессию
func ClearSession(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
}

// Получить текущего пользователя
func GetCurrentUser(c *gin.Context) *models.User {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	
	if userID == nil {
		return nil
	}
	
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		return nil
	}
	
	return &user
}

// Требуется авторизация
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		
		if userID == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// Требуется роль администратора
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userID := session.Get("user_id")
		
		if userID == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		
		var user models.User
		if err := config.DB.First(&user, userID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}
		
		if user.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		
		c.Set("current_user", user)
		c.Next()
	}
}
