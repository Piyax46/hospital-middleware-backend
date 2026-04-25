package models

import "time"

// Patient mirrors the Hospital Information System (HIS) patient record structure.
// Field names and JSON keys match the Hospital A API response exactly.
type Patient struct {
	ID           uint      `json:"id"            gorm:"primaryKey"`
	FirstNameTH  string    `json:"first_name_th"  gorm:"column:first_name_th"`
	MiddleNameTH string    `json:"middle_name_th" gorm:"column:middle_name_th"`
	LastNameTH   string    `json:"last_name_th"   gorm:"column:last_name_th"`
	FirstNameEN  string    `json:"first_name_en"  gorm:"column:first_name_en;not null"`
	MiddleNameEN string    `json:"middle_name_en" gorm:"column:middle_name_en"`
	LastNameEN   string    `json:"last_name_en"   gorm:"column:last_name_en;not null"`
	DateOfBirth  time.Time `json:"date_of_birth"  gorm:"column:date_of_birth"`
	PatientHN    string    `json:"patient_hn"     gorm:"column:patient_hn;not null;index"`
	NationalID   string    `json:"national_id"    gorm:"column:national_id;index"`
	PassportID   string    `json:"passport_id"    gorm:"column:passport_id;index"`
	PhoneNumber  string    `json:"phone_number"   gorm:"column:phone_number"`
	Email        string    `json:"email"          gorm:"column:email"`
	Gender       string    `json:"gender"         gorm:"column:gender"` // M or F
	HospitalID   uint      `json:"hospital_id"    gorm:"column:hospital_id;not null"`
	Hospital     Hospital  `json:"hospital,omitempty" gorm:"foreignKey:HospitalID"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
