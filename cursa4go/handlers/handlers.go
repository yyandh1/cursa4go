package handlers

import (
	"cursa4go/config"
	"cursa4go/middleware"
	"cursa4go/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ========== ПУБЛИЧНЫЕ СТРАНИЦЫ ==========

// Главная страница
func HomePage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

// Страница входа
func LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", nil)
}

// Страница регистрации
func RegisterPage(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", nil)
}

// Регистрация пользователя
func Register(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Password hashing failed"})
		return
	}

	user := models.User{
		Username: input.Username,
		Password: string(hashedPassword),
		Role:     "user",
	}

	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User registered successfully"})
}

// Вход пользователя
func Login(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := config.DB.Where("username = ?", input.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	middleware.SaveSession(c, user.ID, user.Role)
	c.JSON(http.StatusOK, gin.H{"message": "Login successful", "role": user.Role})
}

// Выход
func Logout(c *gin.Context) {
	middleware.ClearSession(c)
	c.Redirect(http.StatusFound, "/login")
}

// ========== ПОЛЬЗОВАТЕЛЬСКИЕ МАРШРУТЫ ==========

// Дашборд пользователя
func Dashboard(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"username": user.Username,
	})
}

// Получить задачи текущего пользователя
func GetTasks(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var tasks []models.Task
	config.DB.Where("user_id = ?", user.ID).Find(&tasks)

	c.JSON(http.StatusOK, tasks)
}

// 🆕 Получить детали задачи
func GetTaskDetails(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID := c.Param("id")

	var task models.Task
	query := config.DB.Where("id = ?", taskID)

	// Админ может видеть любую задачу
	if user.Role != "admin" {
		query = query.Where("user_id = ?", user.ID)
	}

	if err := query.Preload("User").First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Загружаем заметки
	var notes []models.Note
	config.DB.Where("task_id = ?", taskID).Preload("User").Order("created_at desc").Find(&notes)

	c.JSON(http.StatusOK, gin.H{
		"task":  task,
		"notes": notes,
	})
}

// Создать задачу
func CreateTask(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var input struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if input.Status == "" {
		input.Status = "todo"
	}

	userID := user.ID
	task := models.Task{
		Title:       input.Title,
		Description: input.Description,
		Status:      "todo",
		UserID:      &userID,  // ← ВЗЯТЬ АДРЕС
	}


	if err := config.DB.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task created", "task": task})
}

// Обновить задачу
func UpdateTask(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID := c.Param("id")

	var task models.Task
	query := config.DB.Where("id = ?", taskID)

	if user.Role != "admin" {
		query = query.Where("user_id = ?", user.ID)
	}

	if err := query.First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found or access denied"})
		return
	}

	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 🟢 Пользователь может менять только статус задач от админа
	if user.Role != "admin" && task.CreatedByAdmin {
		if input.Status != "" {
			task.Status = input.Status
		}
	} else {
		if input.Title != "" {
			task.Title = input.Title
		}
		if input.Description != "" {
			task.Description = input.Description
		}
		if input.Status != "" {
			task.Status = input.Status
		}
	}

	if err := config.DB.Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task updated", "task": task})
}

// Удалить задачу
func DeleteTask(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID := c.Param("id")

	var task models.Task
	query := config.DB.Where("id = ?", taskID)

	if user.Role != "admin" {
		query = query.Where("user_id = ?", user.ID)
	}

	if err := query.First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found or access denied"})
		return
	}

	// Удаляем связанные заметки
	config.DB.Where("task_id = ?", task.ID).Delete(&models.Note{})

	if err := config.DB.Delete(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task deleted"})
}

// ========== ЗАМЕТКИ ==========

// Добавить заметку к задаче
func AddNote(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID := c.Param("id")

	var input struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&input); err != nil || input.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content required"})
		return
	}

	var task models.Task
	query := config.DB.Where("id = ?", taskID)

	// 🟢 Админ может добавлять заметки к любой задаче
	if user.Role != "admin" {
		query = query.Where("user_id = ?", user.ID)
	}

	if err := query.First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found or access denied"})
		return
	}

	note := models.Note{
		TaskID:  task.ID,
		UserID:  user.ID,
		Content: input.Content,
	}

	if err := config.DB.Create(&note).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add note"})
		return
	}

	config.DB.Preload("User").First(&note, note.ID)

	c.JSON(http.StatusOK, gin.H{"message": "Note added", "note": note})
}
// GetTaskNotes - получить заметки к задаче (админ и пользователь)
func GetTaskNotes(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID := c.Param("id")

	var task models.Task
	query := config.DB.Where("id = ?", taskID)

	// Админ может видеть заметки любой задачи
	if user.Role != "admin" {
		query = query.Where("user_id = ?", user.ID)
	}

	if err := query.First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found or access denied"})
		return
	}

	// Загружаем заметки с информацией о пользователях
	var notes []models.Note
	config.DB.Where("task_id = ?", taskID).
		Preload("User").
		Order("created_at desc").
		Find(&notes)

	c.JSON(http.StatusOK, notes)
}
