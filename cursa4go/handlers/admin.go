package handlers

import (
	"cursa4go/config"
	"cursa4go/middleware"
	"cursa4go/models"
	"log"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"github.com/gin-contrib/sessions"
)

// ========================================
// 📊 СТРУКТУРЫ ДЛЯ СТАТИСТИКИ
// ========================================

type GroupStat struct {
	Group       models.Group
	Total       int64
	TodoCount   int64
	InProgCount int64
	DoneCount   int64
	Percentage  int
}

type IndividualStat struct {
	User        models.User
	Total       int64
	TodoCount   int64
	InProgCount int64
	DoneCount   int64
	Percentage  int
}

type TaskStats struct {
	Total       int
	TodoCount   int
	InProgCount int
	DoneCount   int
	Percentage  int
}

// ========================================
// 📊 АДМИН ДАШБОРД
// ========================================

// AdminDashboard - Главная страница админки с полной статистикой
func AdminDashboard(c *gin.Context) {
	// Получить текущего админа
	session := sessions.Default(c)
	adminID := session.Get("user_id")

	var admin models.User
	if err := config.DB.First(&admin, adminID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения данных администратора"})
		return
	}

	// ✅ ПОЛУЧИТЬ РЕАЛЬНУЮ СТАТИСТИКУ ИЗ БД
	
	// Количество пользователей
	var usersCount int64
	config.DB.Model(&models.User{}).Count(&usersCount)

	// Количество групп
	var groupsCount int64
	config.DB.Model(&models.Group{}).Count(&groupsCount)

	// Количество групповых задач (где group_id не NULL)
	var groupTasksCount int64
	config.DB.Model(&models.Task{}).Where("group_id IS NOT NULL AND group_id > 0").Count(&groupTasksCount)

	// Количество индивидуальных задач (где group_id IS NULL или 0)
	var individualTasksCount int64
	config.DB.Model(&models.Task{}).Where("group_id IS NULL OR group_id = 0").Count(&individualTasksCount)

	// Общее количество задач
	totalTasks := groupTasksCount + individualTasksCount

	// ✅ СТАТИСТИКА ПО ГРУППАМ
	var groups []models.Group
	config.DB.Preload("Users").Find(&groups)

	var groupStats []GroupStat
	for _, group := range groups {
		var total, todo, inProg, done int64
		
		// Считаем все задачи группы (общие + индивидуальные участников)
		config.DB.Model(&models.Task{}).Where("group_id = ?", group.ID).Count(&total)
		config.DB.Model(&models.Task{}).Where("group_id = ? AND status = ?", group.ID, "todo").Count(&todo)
		config.DB.Model(&models.Task{}).Where("group_id = ? AND status = ?", group.ID, "in_progress").Count(&inProg)
		config.DB.Model(&models.Task{}).Where("group_id = ? AND status = ?", group.ID, "done").Count(&done)

		percentage := 0
		if total > 0 {
			percentage = int((float64(done) / float64(total)) * 100)
		}

		groupStats = append(groupStats, GroupStat{
			Group:       group,
			Total:       total,
			TodoCount:   todo,
			InProgCount: inProg,
			DoneCount:   done,
			Percentage:  percentage,
		})
	}

	// ✅ ИНДИВИДУАЛЬНЫЕ ЗАДАЧИ ПО ПОЛЬЗОВАТЕЛЯМ
	var users []models.User
	config.DB.Find(&users)

	var individualStats []IndividualStat
	for _, user := range users {
		var total, todo, inProg, done int64
		
		// Считаем только индивидуальные задачи (где group_id IS NULL или 0)
		config.DB.Model(&models.Task{}).Where("user_id = ? AND (group_id IS NULL OR group_id = 0)", user.ID).Count(&total)
		config.DB.Model(&models.Task{}).Where("user_id = ? AND (group_id IS NULL OR group_id = 0) AND status = ?", user.ID, "todo").Count(&todo)
		config.DB.Model(&models.Task{}).Where("user_id = ? AND (group_id IS NULL OR group_id = 0) AND status = ?", user.ID, "in_progress").Count(&inProg)
		config.DB.Model(&models.Task{}).Where("user_id = ? AND (group_id IS NULL OR group_id = 0) AND status = ?", user.ID, "done").Count(&done)

		// Пропускаем пользователей без индивидуальных задач
		if total == 0 {
			continue
		}

		percentage := 0
		if total > 0 {
			percentage = int((float64(done) / float64(total)) * 100)
		}

		individualStats = append(individualStats, IndividualStat{
			User:        user,
			Total:       total,
			TodoCount:   todo,
			InProgCount: inProg,
			DoneCount:   done,
			Percentage:  percentage,
		})
	}

	log.Printf("📊 Админ-дашборд: Users=%d, Groups=%d, GroupTasks=%d, IndividualTasks=%d, GroupStats=%d, IndividualStats=%d", 
		usersCount, groupsCount, groupTasksCount, individualTasksCount, len(groupStats), len(individualStats))

	c.HTML(http.StatusOK, "admin_dashboard.html", gin.H{
		"admin":                admin,
		"usersCount":           usersCount,
		"groupsCount":          groupsCount,
		"groupTasksCount":      groupTasksCount,
		"individualTasksCount": individualTasksCount,
		"totalTasks":           totalTasks,
		"groupStats":           groupStats,
		"individualStats":      individualStats,
	})
}

// ========================================
// 👥 УПРАВЛЕНИЕ ПОЛЬЗОВАТЕЛЯМИ
// ========================================

func GetAllUsers(c *gin.Context) {
	var users []models.User
	config.DB.Select("id, username, role, created_at").Find(&users)
	c.JSON(http.StatusOK, users)
}

func GetUserByID(c *gin.Context) {
	userID := c.Param("id")
	var user models.User

	if err := config.DB.Select("id, username, role, created_at").First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

func GetUsersListJSON(c *gin.Context) {
	var users []models.User
	if err := config.DB.Select("id, username, role").Order("username asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load users"})
		return
	}

	type UserResponse struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		Role     string `json:"role"`
	}

	var response []UserResponse
	for _, u := range users {
		response = append(response, UserResponse{
			ID:       u.ID,
			Username: u.Username,
			Role:     u.Role,
		})
	}

	c.JSON(http.StatusOK, response)
}

func CreateUser(c *gin.Context) {
	var input struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	user := models.User{
		Username: input.Username,
		Password: string(hashedPassword),
		Role:     input.Role,
	}

	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	log.Printf("✅ Админ создал пользователя %s (роль: %s)", user.Username, user.Role)
	c.JSON(http.StatusOK, user)
}

func DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if err := config.DB.Delete(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	log.Printf("✅ Админ удалил пользователя %s (ID: %s)", user.Username, userID)
	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

// Страница управления пользователями (HTML)
func GetUsersPage(c *gin.Context) {
	var users []models.User
	config.DB.Order("created_at DESC").Find(&users)

	// Получить текущего админа
	currentUser := middleware.GetCurrentUser(c)

	c.HTML(http.StatusOK, "admin_panel.html", gin.H{
		"users": users,
		"admin": currentUser,
	})
}

// MakeUserAdmin - Сделать пользователя администратором
func MakeUserAdmin(c *gin.Context) {
	userID := c.Param("id")

	// Проверить, существует ли пользователь
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Проверить, не является ли уже админом
	if user.Role == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User is already an admin"})
		return
	}

	// Обновить роль
	user.Role = "admin"
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user role"})
		return
	}

	log.Printf("✅ Пользователь %s назначен администратором", user.Username)
	c.JSON(http.StatusOK, gin.H{
		"message": "Пользователь " + user.Username + " назначен администратором",
		"user":    user,
	})
}

// RemoveAdminRole - Снять права администратора
func RemoveAdminRole(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID пользователя"})
		return
	}

	// Получить текущего админа из сессии
	session := sessions.Default(c)
	currentAdminID := session.Get("user_id")

	// Проверить, что админ не снимает права сам у себя
	if currentAdminID != nil && uint(userID) == currentAdminID.(uint) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Нельзя снять права администратора у самого себя"})
		return
	}

	// Найти пользователя
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}

	// Проверить, что это администратор
	if user.Role != "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Этот пользователь не является администратором"})
		return
	}

	// Снять права администратора
	user.Role = "user"
	if err := config.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка снятия прав администратора"})
		return
	}

	log.Printf("✅ Права администратора сняты у пользователя %s (ID: %d)", user.Username, user.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Права администратора успешно сняты",
		"user":    user,
	})
}

// ========================================
// 👥 УПРАВЛЕНИЕ ГРУППАМИ
// ========================================

func GetAllGroups(c *gin.Context) {
	var groups []models.Group
	config.DB.Preload("Users").Find(&groups)
	c.JSON(http.StatusOK, groups)
}

func GetGroupByID(c *gin.Context) {
	groupID := c.Param("id")
	var group models.Group

	if err := config.DB.Preload("Users").First(&group, groupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	c.JSON(http.StatusOK, group)
}

// Страница управления группами (HTML)
func GetGroupsPage(c *gin.Context) {
	var groups []models.Group

	// Загрузить группы с пользователями
	if err := config.DB.Preload("Users").Find(&groups).Error; err != nil {
		log.Printf("❌ Ошибка загрузки групп: %v", err)
		c.HTML(http.StatusInternalServerError, "admin_groups.html", gin.H{
			"error":  "Ошибка загрузки групп",
			"groups": []models.Group{},
		})
		return
	}

	log.Printf("✅ Загружено групп: %d", len(groups))
	c.HTML(http.StatusOK, "admin_groups.html", gin.H{
		"groups": groups,
	})
}

// GetGroupStats - Статистика группы
func GetGroupStats(c *gin.Context) {
	groupID := c.Param("id")
	
	var group models.Group
	if err := config.DB.Preload("Users").First(&group, groupID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Группа не найдена"})
		return
	}

	// Получить статистику по каждому пользователю в группе
	type UserTaskStat struct {
		User        models.User
		Total       int64
		TodoCount   int64
		InProgCount int64
		DoneCount   int64
		Percentage  int
	}

	userStats := []UserTaskStat{}
	for _, user := range group.Users {
		var total, todoCount, inProgCount, doneCount int64
		
		// ✅ Считаем индивидуальные задачи пользователя + общие групповые задачи
		config.DB.Model(&models.Task{}).
			Where("group_id = ? AND (user_id = ? OR user_id IS NULL)", groupID, user.ID).
			Count(&total)
		config.DB.Model(&models.Task{}).
			Where("group_id = ? AND (user_id = ? OR user_id = 0 OR user_id IS NULL) AND status = ?", groupID, user.ID, "todo").
			Count(&todoCount)
		config.DB.Model(&models.Task{}).
			Where("group_id = ? AND (user_id = ? OR user_id = 0 OR user_id IS NULL) AND status = ?", groupID, user.ID, "in_progress").
			Count(&inProgCount)
		config.DB.Model(&models.Task{}).
			Where("group_id = ? AND (user_id = ? OR user_id = 0 OR user_id IS NULL) AND status = ?", groupID, user.ID, "done").
			Count(&doneCount)

		percentage := 0
		if total > 0 {
			percentage = int((float64(doneCount) / float64(total)) * 100)
		}

		userStats = append(userStats, UserTaskStat{
			User:        user,
			Total:       total,
			TodoCount:   todoCount,
			InProgCount: inProgCount,
			DoneCount:   doneCount,
			Percentage:  percentage,
		})
	}

	c.HTML(http.StatusOK, "admin_group_stats.html", gin.H{
		"group":     group,
		"userStats": userStats,
	})
}

// GetGroupDetailsPage - Страница деталей группы
func GetGroupDetailsPage(c *gin.Context) {
	groupID := c.Param("id")
	
	var group models.Group
	if err := config.DB.Preload("Users").First(&group, groupID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Группа не найдена"})
		return
	}
	
	// Получить всех пользователей
	var allUsers []models.User
	config.DB.Find(&allUsers)
	
	// Исключить уже добавленных в группу
	existingUserIDs := make(map[uint]bool)
	for _, u := range group.Users {
		existingUserIDs[u.ID] = true
	}
	
	var availableUsers []models.User
	for _, u := range allUsers {
		if !existingUserIDs[u.ID] {
			availableUsers = append(availableUsers, u)
		}
	}
	
	c.HTML(http.StatusOK, "admin_group_details.html", gin.H{
		"group":    group,
		"allUsers": availableUsers,
	})
}

func CreateGroup(c *gin.Context) {
	var input struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	group := models.Group{
		Name:        input.Name,
		Description: input.Description,
	}
	
	if err := config.DB.Create(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create group"})
		return
	}
	
	log.Printf("✅ Админ создал группу '%s'", group.Name)
	c.JSON(http.StatusOK, group)
}

func DeleteGroup(c *gin.Context) {
	groupID := c.Param("id")
	
	var group models.Group
	if err := config.DB.First(&group, groupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	if err := config.DB.Delete(&group).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete group"})
		return
	}

	log.Printf("✅ Админ удалил группу '%s' (ID: %s)", group.Name, groupID)
	c.JSON(http.StatusOK, gin.H{"message": "Group deleted"})
}

func AddUserToGroup(c *gin.Context) {
	groupID := c.Param("id")
	userID := c.Param("userID")

	var group models.Group
	var user models.User

	if err := config.DB.First(&group, groupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	config.DB.Model(&group).Association("Users").Append(&user)
	log.Printf("✅ Пользователь %s добавлен в группу '%s'", user.Username, group.Name)
	c.JSON(http.StatusOK, gin.H{"message": "User added to group"})
}

func RemoveUserFromGroup(c *gin.Context) {
	groupID := c.Param("id")
	userID := c.Param("userId")

	var group models.Group
	var user models.User

	if err := config.DB.First(&group, groupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	config.DB.Model(&group).Association("Users").Delete(&user)
	log.Printf("✅ Пользователь %s удалён из группы '%s'", user.Username, group.Name)
	c.JSON(http.StatusOK, gin.H{"message": "User removed from group"})
}

// ========================================
// 📋 УПРАВЛЕНИЕ ЗАДАЧАМИ
// ========================================

// AdminCreateTask - Создать ИНДИВИДУАЛЬНУЮ задачу для конкретного пользователя
func AdminCreateTask(c *gin.Context) {
	var req struct {
		UserID      uint   `json:"user_id" binding:"required"`
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные: " + err.Error()})
		return
	}

	// Проверить, существует ли пользователь
	var user models.User
	if err := config.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}

	// Установить статус по умолчанию
	if req.Status == "" {
		req.Status = "todo"
	}

	// ✅ Создать индивидуальную задачу (с user_id, без group_id)
	task := models.Task{
		Title:          req.Title,
		Description:    req.Description,
		Status:         req.Status,
		UserID:         &req.UserID,  // ← ВЗЯТЬ АДРЕС
		GroupID:        nil,
		CreatedByAdmin: true,
	}


	if err := config.DB.Create(&task).Error; err != nil {
		log.Printf("❌ Ошибка создания задачи: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания задачи"})
		return
	}

	log.Printf("✅ Админ создал индивидуальную задачу '%s' для пользователя %s (ID: %d)", 
		task.Title, user.Username, user.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Индивидуальная задача успешно создана",
		"task":    task,
	})
}

// AdminCreateGroupTask - Создать ОБЩУЮ задачу для всей группы (БЕЗ привязки к пользователю)
func AdminCreateGroupTask(c *gin.Context) {
	var req struct {
		GroupID     uint   `json:"group_id" binding:"required"`
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ Ошибка валидации: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверные данные: " + err.Error()})
		return
	}

	// Проверить, существует ли группа
	var group models.Group
	if err := config.DB.First(&group, req.GroupID).Error; err != nil {
		log.Printf("❌ Группа %d не найдена", req.GroupID)
		c.JSON(http.StatusNotFound, gin.H{"error": "Группа не найдена"})
		return
	}

	// Установить статус по умолчанию
	if req.Status == "" {
		req.Status = "todo"
	}

	// ✅ СОЗДАЁМ ОДНУ ОБЩУЮ ЗАДАЧУ БЕЗ ПРИВЯЗКИ К ПОЛЬЗОВАТЕЛЮ (user_id = 0)
	// ✅ СОЗДАЁМ ОДНУ ОБЩУЮ ЗАДАЧУ БЕЗ ПРИВЯЗКИ К ПОЛЬЗОВАТЕЛЮ (user_id = NULL)
	task := models.Task{
		Title:          req.Title,
		Description:    req.Description,
		Status:         req.Status,
		UserID:         nil, // ← ПРАВИЛЬНО! (NULL в БД)
		GroupID:        &req.GroupID,
		CreatedByAdmin: true,
	}


	if err := config.DB.Create(&task).Error; err != nil {
		log.Printf("❌ Ошибка создания групповой задачи: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания задачи"})
		return
	}

	log.Printf("✅ Админ создал ОБЩУЮ групповую задачу '%s' для группы '%s' (ID: %d, TaskID: %d)", 
		task.Title, group.Name, group.ID, task.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Общая задача для группы '%s' успешно создана", group.Name),
		"task":    task,
	})
}

// CreateTaskForUser - Создать индивидуальную задачу для пользователя (альтернативный эндпоинт)
func CreateTaskForUser(c *gin.Context) {
	userID := c.Param("id")
	
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

	// Преобразовать userID в uint
	uid, err := strconv.ParseUint(userID, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID пользователя"})
		return
	}

	// Проверить, существует ли пользователь
	var user models.User
	if err := config.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}

	// ✅ Создать индивидуальную задачу
	// ✅ Создать индивидуальную задачу
	userIDUint := uint(uid)
	task := models.Task{
		Title:          input.Title,
		Description:    input.Description,
		Status:         input.Status,
		UserID:         &userIDUint,  // ← ВЗЯТЬ АДРЕС
		GroupID:        nil,
		CreatedByAdmin: true,
	}


	if err := config.DB.Create(&task).Error; err != nil {
		log.Printf("❌ Ошибка создания индивидуальной задачи: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания задачи"})
		return
	}

	log.Printf("✅ Админ создал индивидуальную задачу '%s' для пользователя %s (ID: %d)", 
		task.Title, user.Username, user.ID)

	c.JSON(http.StatusOK, gin.H{
		"message": "Индивидуальная задача успешно создана",
		"task":    task,
	})
}

// GetUserTasks - Получить задачи пользователя (JSON)
func GetUserTasks(c *gin.Context) {
	userID := c.Param("id")
	individual := c.Query("individual") == "true"

	var tasks []models.Task
	query := config.DB.Where("user_id = ?", userID).Preload("Notes.User")

	if individual {
		query = query.Where("group_id IS NULL OR group_id = 0")
	}

	query.Find(&tasks)
	c.JSON(http.StatusOK, tasks)
}

// GetUserTasksInGroup - HTML страница задач пользователя в группе
func GetUserTasksInGroup(c *gin.Context) {
	groupID := c.Param("id")
	userID := c.Param("userId")
	
	// Получить группу
	var group models.Group
	if err := config.DB.First(&group, groupID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Группа не найдена"})
		return
	}
	
	// Получить пользователя
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{"error": "Пользователь не найден"})
		return
	}
	
	// ✅ ПОЛУЧИТЬ ЗАДАЧИ: индивидуальные (user_id = userID) + общие групповые (user_id = 0)
	var tasks []models.Task
	if err := config.DB.
		Where("group_id = ? AND (user_id = ? OR user_id IS NULL)", groupID, userID).
		Order("created_at DESC").
		Find(&tasks).Error; err != nil {
		log.Printf("❌ Ошибка получения задач: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": "Ошибка получения задач"})
		return
	}
	
	// Посчитать статистику
	var total, todoCount, inProgCount, doneCount int
	total = len(tasks)
	
	for _, task := range tasks {
		switch task.Status {
		case "todo":
			todoCount++
		case "in_progress":
			inProgCount++
		case "done":
			doneCount++
		}
	}
	
	percentage := 0
	if total > 0 {
		percentage = (doneCount * 100) / total
	}
	
	stats := TaskStats{
		Total:       total,
		TodoCount:   todoCount,
		InProgCount: inProgCount,
		DoneCount:   doneCount,
		Percentage:  percentage,
	}
	
	log.Printf("✅ Админ просматривает задачи пользователя %s в группе %s (всего: %d, индивидуальные + общие)", 
		user.Username, group.Name, len(tasks))
	
	c.HTML(http.StatusOK, "admin_user_group_tasks.html", gin.H{
		"group": group,
		"user":  user,
		"tasks": tasks,
		"stats": stats,
	})
}

// GetUserIndividualTasksPage - HTML страница ИНДИВИДУАЛЬНЫХ задач пользователя (НЕ групповые)
func GetUserIndividualTasksPage(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неверный ID пользователя"})
		return
	}

	// Получить пользователя
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Пользователь не найден"})
		return
	}

	// ✅ ПОЛУЧИТЬ ТОЛЬКО ИНДИВИДУАЛЬНЫЕ ЗАДАЧИ (где group_id IS NULL и user_id = userID)
	var tasks []models.Task
	
	query := config.DB.Preload("User").
		Where("user_id = ?", userID).
		Where("group_id IS NULL OR group_id = 0")
	
	if err := query.Order("created_at DESC").Find(&tasks).Error; err != nil {
		log.Printf("❌ Ошибка загрузки индивидуальных задач для пользователя %d: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка загрузки задач: " + err.Error()})
		return
	}

	log.Printf("✅ Загружено %d индивидуальных задач для пользователя %s (ID: %d)", 
		len(tasks), user.Username, userID)

	// Подсчитать статистику
	stats := map[string]interface{}{
		"Total":       len(tasks),
		"TodoCount":   0,
		"InProgCount": 0,
		"DoneCount":   0,
		"Percentage":  0,
	}

	for _, task := range tasks {
		switch task.Status {
		case "todo":
			stats["TodoCount"] = stats["TodoCount"].(int) + 1
		case "in_progress":
			stats["InProgCount"] = stats["InProgCount"].(int) + 1
		case "done":
			stats["DoneCount"] = stats["DoneCount"].(int) + 1
		}
	}

	if len(tasks) > 0 {
		stats["Percentage"] = (stats["DoneCount"].(int) * 100) / len(tasks)
	}

	c.HTML(http.StatusOK, "admin_user_individual_tasks.html", gin.H{
		"user":  user,
		"tasks": tasks,
		"stats": stats,
	})
}

// GetUserTasksPage - HTML страница управления задачами пользователя (устаревший эндпоинт)
func GetUserTasksPage(c *gin.Context) {
	userID := c.Param("id")
	c.HTML(http.StatusOK, "admin_user_tasks.html", gin.H{"userID": userID})
}

// ========================================
// 📝 ЗАМЕТКИ
// ========================================

// AdminGetTaskNotes - Админ получает заметки к задаче
func AdminGetTaskNotes(c *gin.Context) {
	taskID := c.Param("id")

	// Проверяем, что задача существует
	var task models.Task
	if err := config.DB.First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Загружаем заметки с информацией о пользователях
	var notes []models.Note
	if err := config.DB.Where("task_id = ?", taskID).
		Preload("User").
		Order("created_at DESC").
		Find(&notes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load notes"})
		return
	}

	c.JSON(http.StatusOK, notes)
}

// AdminAddNoteToTask - Админ добавляет заметку к задаче
func AdminAddNoteToTask(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	taskID := c.Param("id")

	var input struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Content is required"})
		return
	}

	// Проверяем, что задача существует
	var task models.Task
	if err := config.DB.First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Создаём заметку от имени админа
	note := models.Note{
		TaskID:  task.ID,
		UserID:  user.ID,
		Content: input.Content,
	}

	if err := config.DB.Create(&note).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add note"})
		return
	}

	// Загружаем связанные данные
	config.DB.Preload("User").First(&note, note.ID)

	log.Printf("✅ Админ %s добавил заметку к задаче %d", user.Username, task.ID)
	c.JSON(http.StatusCreated, note)
}

// AdminAddNote - Альтернативный эндпоинт для добавления заметки (устаревший)
func AdminAddNote(c *gin.Context) {
	user := middleware.GetCurrentUser(c)
	if user == nil || user.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
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
	if err := config.DB.First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
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

	c.JSON(http.StatusOK, gin.H{
		"message": "Note added",
		"note":    note,
	})
}
// ========================================
// ✏️ ОБНОВЛЕНИЕ И УДАЛЕНИЕ ЗАДАЧ
// ========================================

// AdminUpdateTask - Админ обновляет любую задачу
func AdminUpdateTask(c *gin.Context) {
	taskID := c.Param("id")

	var task models.Task
	if err := config.DB.First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Задача не найдена"})
		return
	}

	var input struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректные данные"})
		return
	}

	// Обновляем только переданные поля
	if input.Title != "" {
		task.Title = input.Title
	}
	if input.Description != "" {
		task.Description = input.Description
	}
	if input.Status != "" {
		// Валидация статуса
		if input.Status != "todo" && input.Status != "in_progress" && input.Status != "done" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный статус"})
			return
		}
		task.Status = input.Status
	}

	if err := config.DB.Save(&task).Error; err != nil {
		log.Printf("❌ Ошибка обновления задачи %d: %v", task.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления задачи"})
		return
	}

	log.Printf("✅ Админ обновил задачу '%s' (ID: %d)", task.Title, task.ID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Задача успешно обновлена",
		"task":    task,
	})
}

// AdminDeleteTask - Админ удаляет любую задачу
func AdminDeleteTask(c *gin.Context) {
	taskID := c.Param("id")

	var task models.Task
	if err := config.DB.First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Задача не найдена"})
		return
	}

	taskTitle := task.Title
	taskIDInt := task.ID

	// Удаляем все заметки, связанные с задачей
	if err := config.DB.Where("task_id = ?", taskID).Delete(&models.Note{}).Error; err != nil {
		log.Printf("⚠️ Ошибка удаления заметок задачи %d: %v", task.ID, err)
	}

	// Удаляем саму задачу
	if err := config.DB.Delete(&task).Error; err != nil {
		log.Printf("❌ Ошибка удаления задачи %d: %v", task.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка удаления задачи"})
		return
	}

	log.Printf("✅ Админ удалил задачу '%s' (ID: %d)", taskTitle, taskIDInt)
	c.JSON(http.StatusOK, gin.H{
		"message": "Задача успешно удалена",
	})
}
