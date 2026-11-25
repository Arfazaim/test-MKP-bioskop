package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func GetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ConnectDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
		quoteDSNValue(GetEnv("DB_HOST", "localhost")),
		quoteDSNValue(GetEnv("DB_PORT", "5432")),
		quoteDSNValue(GetEnv("DB_USER", "postgres")),
		quoteDSNValue(os.Getenv("DB_PASSWORD")),
		quoteDSNValue(GetEnv("DB_NAME", "db_bioskop")),
		quoteDSNValue(GetEnv("DB_SSLMODE", "disable")),
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

func quoteDSNValue(v string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v)
	return "'" + escaped + "'"
}
