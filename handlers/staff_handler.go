package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"hospital-backend/database"
	"hospital-backend/middleware"
	"hospital-backend/models"
)

type CreateStaffRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Hospital string `json:"hospital" binding:"required"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Hospital string `json:"hospital" binding:"required"`
}

type StaffResponse struct {
	ID           uint   `json:"id"`
	Username     string `json:"username"`
	HospitalID   uint   `json:"hospital_id"`
	HospitalName string `json:"hospital_name"`
}

func CreateStaff(c *gin.Context) {
	var req CreateStaffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find or create hospital
	var hospital models.Hospital
	if err := database.DB.Where("name = ?", req.Hospital).First(&hospital).Error; err != nil {
		hospital = models.Hospital{Name: req.Hospital}
		if err := database.DB.Create(&hospital).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create hospital"})
			return
		}
	}

	// Check if username already exists in this hospital
	var existingStaff models.Staff
	if err := database.DB.Where("username = ? AND hospital_id = ?", req.Username, hospital.ID).First(&existingStaff).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists in this hospital"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	staff := models.Staff{
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
		HospitalID:   hospital.ID,
	}

	if err := database.DB.Create(&staff).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create staff"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Staff created successfully",
		"data": StaffResponse{
			ID:           staff.ID,
			Username:     staff.Username,
			HospitalID:   hospital.ID,
			HospitalName: hospital.Name,
		},
	})
}

func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find hospital
	var hospital models.Hospital
	if err := database.DB.Where("name = ?", req.Hospital).First(&hospital).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Find staff by username and hospital
	var staff models.Staff
	if err := database.DB.Where("username = ? AND hospital_id = ?", req.Username, hospital.ID).First(&staff).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(staff.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate JWT
	claims := middleware.Claims{
		StaffID:    staff.ID,
		HospitalID: hospital.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(middleware.JwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"staff": StaffResponse{
			ID:           staff.ID,
			Username:     staff.Username,
			HospitalID:   hospital.ID,
			HospitalName: hospital.Name,
		},
	})
}