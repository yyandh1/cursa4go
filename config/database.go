package config

import (
	"cursa4go/models"
	"log"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "host=db user=postgres password=4780 dbname=business_db port=5432 sslmode=disable"
	}

	var err error
	for i := 0; i < 10; i++ {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err == nil {
			break
		}
		log.Printf("Не удалось подключиться к БД (попытка %d/10): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatal("Не удалось подключиться к БД после 10 попыток:", err)
	}

	log.Println("✅ Подключено к БД")

	// Автомиграции
	DB.AutoMigrate(
		&models.User{},
		&models.Task{},
		&models.Note{},
		&models.Group{},
	)

	log.Println("✅ Миграции выполнены")

	// 🟢 Создаём администратора, если такого нет
	var count int64
	DB.Model(&models.User{}).Where("role = ?", "admin").Count(&count)
	if count == 0 {
		passwordHash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := models.User{
			Username: "admin",
			Password: string(passwordHash),
			Role:     "admin",
		}
		if err := DB.Create(&admin).Error; err != nil {
			log.Println("⚠️  Не удалось создать администратора:", err)
		} else {
			log.Println("✅ Админ 'admin' создан с паролем 'admin123'")
		}
	}
}
