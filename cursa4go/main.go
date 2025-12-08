package main

import (
	"cursa4go/config"
	"cursa4go/handlers"
	"cursa4go/middleware"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
)

func main() {
	log.Println("🚀 Запуск Business Desk...")

	// Подключение к БД
	config.ConnectDB()
	log.Println("✅ База данных подключена")

	log.Println("📂 Загрузка статических файлов и шаблонов...")

	// Создание роутера
	r := gin.Default()

	// ✅ ИНИЦИАЛИЗАЦИЯ СЕССИЙ
	store := cookie.NewStore([]byte("super-secret-key-change-me-in-production-12345"))
	r.Use(sessions.Sessions("mysession", store))
	log.Println("✅ Сессии инициализированы")

	// Статические файлы
	r.Static("/static", "./static")

	// Шаблоны
	r.LoadHTMLGlob("templates/*")

	// Настройка маршрутов
	setupRoutes(r)

	// Запуск сервера
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	log.Printf("✅ Сервер запущен на http://localhost:%s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal("❌ Ошибка запуска сервера:", err)
	}
}

func setupRoutes(r *gin.Engine) {
	log.Println("🔓 Настройка публичных маршрутов...")
	setupPublicRoutes(r)

	log.Println("🔐 Настройка защищённых маршрутов...")
	setupUserRoutes(r)

	log.Println("👑 Настройка админских маршрутов...")
	setupAdminRoutes(r)
}

func setupPublicRoutes(r *gin.Engine) {
	r.GET("/", handlers.HomePage)
	r.GET("/login", handlers.LoginPage)
	r.POST("/login", handlers.Login)
	r.GET("/register", handlers.RegisterPage)
	r.POST("/register", handlers.Register)
	r.GET("/logout", handlers.Logout)   // ✅ GET для кнопки "Выход"
	r.POST("/logout", handlers.Logout)
}

func setupUserRoutes(r *gin.Engine) {
	// HTML страница
	userRoutes := r.Group("")
	userRoutes.Use(middleware.AuthRequired())
	{
		userRoutes.GET("/dashboard", handlers.Dashboard)
	}

	// API для dashboard (JSON)
	apiRoutes := r.Group("/api")
	apiRoutes.Use(middleware.AuthRequired())
	{
		apiRoutes.GET("/tasks", handlers.GetTasks)
		apiRoutes.GET("/tasks/:id", handlers.GetTaskDetails)
		apiRoutes.GET("/tasks/:id/notes", handlers.GetTaskNotes)  // ✅ Получить заметки
		apiRoutes.POST("/tasks", handlers.CreateTask)
		apiRoutes.PUT("/tasks/:id", handlers.UpdateTask)
		apiRoutes.DELETE("/tasks/:id", handlers.DeleteTask)
		apiRoutes.POST("/tasks/:id/notes", handlers.AddNote)
	}
}

func setupAdminRoutes(r *gin.Engine) {
	adminRoutes := r.Group("/admin")
	adminRoutes.Use(middleware.AuthRequired())
	adminRoutes.Use(middleware.AdminRequired())
	{
		// Дашборд
		adminRoutes.GET("/dashboard", handlers.AdminDashboard)

		// Пользователи
		adminRoutes.GET("/users", handlers.GetAllUsers)
		adminRoutes.GET("/users/page", handlers.GetUsersPage)
		adminRoutes.GET("/users/list", handlers.GetUsersListJSON)
		adminRoutes.GET("/users/:id", handlers.GetUserByID)
		adminRoutes.POST("/users", handlers.CreateUser)
		adminRoutes.POST("/users/:id/make-admin", handlers.MakeUserAdmin)
		adminRoutes.POST("/users/:id/remove-admin", handlers.RemoveAdminRole)
		adminRoutes.DELETE("/users/:id", handlers.DeleteUser)
		
		// Задачи пользователя
		adminRoutes.GET("/users/:id/tasks", handlers.GetUserTasks)
		adminRoutes.GET("/users/:id/tasks/page", handlers.GetUserTasksPage)
		adminRoutes.GET("/users/:id/tasks/individual", handlers.GetUserIndividualTasksPage)
		adminRoutes.POST("/users/:id/tasks", handlers.CreateTaskForUser)

		// ✅ СОЗДАНИЕ ЗАДАЧ
		adminRoutes.POST("/tasks", handlers.AdminCreateTask)
		adminRoutes.POST("/tasks/group", handlers.AdminCreateGroupTask)

		// ✅ УПРАВЛЕНИЕ ЗАДАЧАМИ
		adminRoutes.PUT("/tasks/:id", handlers.AdminUpdateTask)
		adminRoutes.DELETE("/tasks/:id", handlers.AdminDeleteTask)

		// Группы
		adminRoutes.GET("/groups", handlers.GetAllGroups)
		adminRoutes.GET("/groups/page", handlers.GetGroupsPage)
		adminRoutes.POST("/groups", handlers.CreateGroup)
		adminRoutes.GET("/groups/:id", handlers.GetGroupByID)
		adminRoutes.GET("/groups/:id/stats", handlers.GetGroupStats)
		adminRoutes.GET("/groups/:id/page", handlers.GetGroupDetailsPage)
		adminRoutes.DELETE("/groups/:id", handlers.DeleteGroup)
		adminRoutes.POST("/groups/:id/users/:userID", handlers.AddUserToGroup)
		adminRoutes.DELETE("/groups/:id/users/:userId", handlers.RemoveUserFromGroup)
		adminRoutes.GET("/groups/:id/users/:userId/tasks", handlers.GetUserTasksInGroup)

		// Заметки к задачам
		adminRoutes.GET("/tasks/:id/notes", handlers.AdminGetTaskNotes)
		adminRoutes.POST("/tasks/:id/notes", handlers.AdminAddNoteToTask)
	}
}
