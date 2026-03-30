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

// ✅ ИСПРАВЛЕНО: Получить задачи текущего пользователя + групповые задачи
func GetTasks(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 1. Получить ID групп, в которых состоит пользователь
	var userGroups []models.Group
	config.DB.Joins("JOIN user_groups ON user_groups.group_id = groups.id").
		Where("user_groups.user_id = ?", user.ID).
		Find(&userGroups)
	
	groupIDs := []uint{}
	for _, g := range userGroups {
		groupIDs = append(groupIDs, g.ID)
	}
	
	var tasks []models.Task
	
	// 2. Начинаем с индивидуальных задач пользователя (без группы)
	query := config.DB.Where("user_id = ? AND (group_id IS NULL OR group_id = 0)", user.ID)
	
	// 3. Добавляем групповые задачи (общие для группы + индивидуальные в группе)
	if len(groupIDs) > 0 {
		query = query.Or("(group_id IN (?) AND (user_id = ? OR user_id IS NULL))", groupIDs, user.ID)
	}
	
	query.Order("created_at DESC").Preload("User").Preload("Group").Find(&tasks)
	
	c.JSON(http.StatusOK, tasks)
}

// ✅ ИСПРАВЛЕНО: Получить детали задачи (с учётом групповых задач)
func GetTaskDetails(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID := c.Param("id")

	var task models.Task
	
	// Админ может видеть любую задачу
	if user.Role == "admin" {
		if err := config.DB.Where("id = ?", taskID).Preload("User").Preload("Group").First(&task).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}
	} else {
		// Получить ID групп пользователя
		var userGroups []models.Group
		config.DB.Joins("JOIN user_groups ON user_groups.group_id = groups.id").
			Where("user_groups.user_id = ?", user.ID).
			Find(&userGroups)
		
		groupIDs := []uint{}
		for _, g := range userGroups {
			groupIDs = append(groupIDs, g.ID)
		}
		
		// Проверяем доступ: задача принадлежит пользователю ИЛИ его группе
		query := config.DB.Where("id = ?", taskID).Where(
			config.DB.Where("user_id = ?", user.ID).
				Or("(group_id IN (?) AND (user_id = ? OR user_id IS NULL))", groupIDs, user.ID),
		)
		
		if err := query.Preload("User").Preload("Group").First(&task).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found or access denied"})
			return
		}
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
		Status:      input.Status,
		UserID:      &userID,
	}

	if err := config.DB.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Task created", "task": task})
}

// ✅ ИСПРАВЛЕНО: Обновить задачу (с учётом групповых задач)
func UpdateTask(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID := c.Param("id")

	var task models.Task
	
	// Админ может обновлять любую задачу
	if user.Role == "admin" {
		if err := config.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}
	} else {
		// Получить ID групп пользователя
		var userGroups []models.Group
		config.DB.Joins("JOIN user_groups ON user_groups.group_id = groups.id").
			Where("user_groups.user_id = ?", user.ID).
			Find(&userGroups)
		
		groupIDs := []uint{}
		for _, g := range userGroups {
			groupIDs = append(groupIDs, g.ID)
		}
		
		// Проверяем доступ
		query := config.DB.Where("id = ?", taskID).Where(
			config.DB.Where("user_id = ?", user.ID).
				Or("(group_id IN (?) AND (user_id = ? OR user_id IS NULL))", groupIDs, user.ID),
		)
		
		if err := query.First(&task).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found or access denied"})
			return
		}
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

	// 🟢 Пользователь может менять только статус задач от админа или групповых задач
	if user.Role != "admin" && (task.CreatedByAdmin || task.GroupID != nil) {
		if input.Status != "" {
			task.Status = input.Status
		}
	} else {
		// Полное редактирование для своих задач или если админ
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

// ✅ ИСПРАВЛЕНО: Удалить задачу (с учётом групповых задач)
func DeleteTask(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID := c.Param("id")

	var task models.Task
	
	// Админ может удалять любую задачу
	if user.Role == "admin" {
		if err := config.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}
	} else {
		// Пользователь может удалять ТОЛЬКО свои индивидуальные задачи
		if err := config.DB.Where("id = ? AND user_id = ? AND (group_id IS NULL OR group_id = 0)", taskID, user.ID).First(&task).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found or access denied"})
			return
		}
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

// ✅ ИСПРАВЛЕНО: Добавить заметку к задаче (с учётом групповых задач)
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
	
	// Админ может добавлять заметки к любой задаче
	if user.Role == "admin" {
		if err := config.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}
	} else {
		// Получить ID групп пользователя
		var userGroups []models.Group
		config.DB.Joins("JOIN user_groups ON user_groups.group_id = groups.id").
			Where("user_groups.user_id = ?", user.ID).
			Find(&userGroups)
		
		groupIDs := []uint{}
		for _, g := range userGroups {
			groupIDs = append(groupIDs, g.ID)
		}
		
		// Проверяем доступ к задаче
		query := config.DB.Where("id = ?", taskID).Where(
			config.DB.Where("user_id = ?", user.ID).
				Or("(group_id IN (?) AND (user_id = ? OR user_id IS NULL))", groupIDs, user.ID),
		)
		
		if err := query.First(&task).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found or access denied"})
			return
		}
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

// ✅ ИСПРАВЛЕНО: GetTaskNotes - получить заметки к задаче (с учётом групповых задач)
func GetTaskNotes(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID := c.Param("id")

	var task models.Task
	
	// Админ может видеть заметки любой задачи
	if user.Role == "admin" {
		if err := config.DB.Where("id = ?", taskID).First(&task).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
			return
		}
	} else {
		// Получить ID групп пользователя
		var userGroups []models.Group
		config.DB.Joins("JOIN user_groups ON user_groups.group_id = groups.id").
			Where("user_groups.user_id = ?", user.ID).
			Find(&userGroups)
		
		groupIDs := []uint{}
		for _, g := range userGroups {
			groupIDs = append(groupIDs, g.ID)
		}
		
		// Проверяем доступ
		query := config.DB.Where("id = ?", taskID).Where(
			config.DB.Where("user_id = ?", user.ID).
				Or("(group_id IN (?) AND (user_id = ? OR user_id IS NULL))", groupIDs, user.ID),
		)
		
		if err := query.First(&task).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Task not found or access denied"})
			return
		}
	}

	// Загружаем заметки с информацией о пользователях
	var notes []models.Note
	config.DB.Where("task_id = ?", taskID).
		Preload("User").
		Order("created_at desc").
		Find(&notes)

	c.JSON(http.StatusOK, notes)
}
