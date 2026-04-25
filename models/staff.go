package models

import "time"

type Staff struct {
	ID           uint     `json:"id" gorm:"primaryKey"`
	Username     string   `json:"username" gorm:"not null"`
	PasswordHash string   `json:"-" gorm:"not null"`
	HospitalID   uint     `json:"hospital_id" gorm:"not null"`
	Hospital     Hospital `json:"hospital" gorm:"foreignKey:HospitalID"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName overrides the table name for GORM
func (Staff) TableName() string {
	return "staffs"
}