package tests

import (
	"os"
	"testing"

	"hospital-backend/database"
)

func TestMain(m *testing.M) {
	// Set test env
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_USER", "user")
	os.Setenv("DB_PASSWORD", "password")
	os.Setenv("DB_NAME", "hospital_test")
	os.Setenv("DB_PORT", "5432")
	os.Setenv("JWT_SECRET", "test-secret-key")

	database.ConnectTestDB()

	// Run tests
	code := m.Run()

	os.Exit(code)
}
