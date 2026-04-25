package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"hospital-backend/database"
	"hospital-backend/handlers"
	"hospital-backend/his"
	"hospital-backend/middleware"
)

func main() {
	database.Connect()

	// Initialise the Hospital A HIS client.
	hisClient := his.NewHospitalAClient()

	r := gin.Default()

	// Staff endpoints (no auth required)
	staffGroup := r.Group("/staff")
	{
		staffGroup.POST("/create", handlers.CreateStaff)
		staffGroup.POST("/login", handlers.Login)
	}

	// Patient endpoints (JWT auth required)
	patientGroup := r.Group("/patient")
	patientGroup.Use(middleware.AuthRequired())
	{
		patientGroup.GET("/search", handlers.SearchPatients(hisClient))
	}

	log.Println("Server starting on port 8080")
	r.Run(":8080")
}
