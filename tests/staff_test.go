package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"hospital-backend/database"
	"hospital-backend/handlers"
)

func setupStaffRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	staffGroup := r.Group("/staff")
	{
		staffGroup.POST("/create", handlers.CreateStaff)
		staffGroup.POST("/login", handlers.Login)
	}
	return r
}

// ============================================================
// CREATE STAFF TESTS
// ============================================================

func TestCreateStaff_Success(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	reqBody := handlers.CreateStaffRequest{
		Username: "nurse_a",
		Password: "password123",
		Hospital: "Bangkok Hospital",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Equal(t, "Staff created successfully", response["message"])
	assert.NotNil(t, response["data"])

	// Verify returned data
	data := response["data"].(map[string]interface{})
	assert.Equal(t, "nurse_a", data["username"])
	assert.Equal(t, "Bangkok Hospital", data["hospital_name"])
}

func TestCreateStaff_SameUsernameInDifferentHospitals(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	// Create "admin" in Hospital A
	reqBody1 := handlers.CreateStaffRequest{
		Username: "admin",
		Password: "password123",
		Hospital: "Hospital A",
	}
	jsonBody1, _ := json.Marshal(reqBody1)

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonBody1))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)

	// Create "admin" in Hospital B — should succeed (different hospital)
	reqBody2 := handlers.CreateStaffRequest{
		Username: "admin",
		Password: "password456",
		Hospital: "Hospital B",
	}
	jsonBody2, _ := json.Marshal(reqBody2)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonBody2))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusCreated, w2.Code)
}

func TestCreateStaff_DuplicateUsernameInSameHospital(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	reqBody := handlers.CreateStaffRequest{
		Username: "nurse_dup",
		Password: "password123",
		Hospital: "Dup Hospital",
	}
	jsonBody, _ := json.Marshal(reqBody)

	// First create — should succeed
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonBody))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusCreated, w1.Code)

	// Second create same username same hospital — should fail
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonBody))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusConflict, w2.Code)
}

func TestCreateStaff_MissingUsername(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	body := map[string]string{
		"password": "password123",
		"hospital": "Test Hospital",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateStaff_MissingPassword(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	body := map[string]string{
		"username": "test_user",
		"hospital": "Test Hospital",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateStaff_MissingHospital(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	body := map[string]string{
		"username": "test_user",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateStaff_PasswordTooShort(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	reqBody := handlers.CreateStaffRequest{
		Username: "short_pw",
		Password: "123",
		Hospital: "Test Hospital",
	}
	jsonBody, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateStaff_EmptyBody(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ============================================================
// LOGIN TESTS
// ============================================================

func TestLogin_Success(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	// First create staff
	createBody := handlers.CreateStaffRequest{
		Username: "login_user",
		Password: "password123",
		Hospital: "Login Hospital",
	}
	jsonCreate, _ := json.Marshal(createBody)
	wCreate := httptest.NewRecorder()
	reqCreate, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonCreate))
	reqCreate.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wCreate, reqCreate)
	assert.Equal(t, http.StatusCreated, wCreate.Code)

	// Login
	loginBody := handlers.LoginRequest{
		Username: "login_user",
		Password: "password123",
		Hospital: "Login Hospital",
	}
	jsonLogin, _ := json.Marshal(loginBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/login", bytes.NewBuffer(jsonLogin))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.NotEmpty(t, response["token"])
	assert.NotNil(t, response["staff"])
}

func TestLogin_WrongPassword(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	// Create staff
	createBody := handlers.CreateStaffRequest{
		Username: "wrong_pw_user",
		Password: "correct_password",
		Hospital: "Test Hospital",
	}
	jsonCreate, _ := json.Marshal(createBody)
	wCreate := httptest.NewRecorder()
	reqCreate, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonCreate))
	reqCreate.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wCreate, reqCreate)

	// Login with wrong password
	loginBody := handlers.LoginRequest{
		Username: "wrong_pw_user",
		Password: "wrong_password",
		Hospital: "Test Hospital",
	}
	jsonLogin, _ := json.Marshal(loginBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/login", bytes.NewBuffer(jsonLogin))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_WrongHospital(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	// Create staff at Hospital A
	createBody := handlers.CreateStaffRequest{
		Username: "hosp_user",
		Password: "password123",
		Hospital: "Hospital A",
	}
	jsonCreate, _ := json.Marshal(createBody)
	wCreate := httptest.NewRecorder()
	reqCreate, _ := http.NewRequest("POST", "/staff/create", bytes.NewBuffer(jsonCreate))
	reqCreate.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wCreate, reqCreate)

	// Try login at Hospital B — should fail
	loginBody := handlers.LoginRequest{
		Username: "hosp_user",
		Password: "password123",
		Hospital: "Hospital B",
	}
	jsonLogin, _ := json.Marshal(loginBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/login", bytes.NewBuffer(jsonLogin))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_NonExistentUser(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	loginBody := handlers.LoginRequest{
		Username: "ghost_user",
		Password: "password123",
		Hospital: "No Hospital",
	}
	jsonLogin, _ := json.Marshal(loginBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/login", bytes.NewBuffer(jsonLogin))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLogin_MissingUsername(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	body := map[string]string{
		"password": "password123",
		"hospital": "Test Hospital",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_MissingPassword(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	body := map[string]string{
		"username": "test_user",
		"hospital": "Test Hospital",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_MissingHospital(t *testing.T) {
	database.CleanTestDB()
	r := setupStaffRouter()

	body := map[string]string{
		"username": "test_user",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/staff/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}