package database

import (
	"fmt"
	"log"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"hospital-backend/models"
)

var DB *gorm.DB
var isTestDB bool

func Connect() {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	migrate()
}

// ConnectTestDB connects to a SQLite in-memory database for testing.
func ConnectTestDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to test database:", err)
	}
	isTestDB = true

	migrate()
}

// migrate runs AutoMigrate and creates composite unique indexes.
func migrate() {
	DB.AutoMigrate(&models.Hospital{}, &models.Staff{}, &models.Patient{})

	sqlDB, err := DB.DB()
	if err != nil {
		return
	}
	// Staff: username must be unique within a hospital
	sqlDB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_username_hospital ON staffs(username, hospital_id)")
	// Patient: HN must be unique within a hospital
	sqlDB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_patient_hn_hospital ON patients(patient_hn, hospital_id)")
}

// CleanTestDB truncates all tables between test runs.
func CleanTestDB() {
	if isTestDB {
		DB.Exec("DELETE FROM patients")
		DB.Exec("DELETE FROM staffs")
		DB.Exec("DELETE FROM hospitals")
	} else {
		DB.Exec("TRUNCATE TABLE patients, staffs, hospitals RESTART IDENTITY CASCADE")
	}
}
