package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"hospital-backend/database"
	"hospital-backend/his"
	"hospital-backend/models"
)

// PatientSearchResponse wraps the paginated patient results.
type PatientSearchResponse struct {
	Data  []models.Patient `json:"data"`
	Total int64            `json:"total"`
}

// SearchPatients returns a gin.HandlerFunc that:
//  1. (When national_id or passport_id is supplied) fetches the authoritative
//     record from the Hospital A HIS API and upserts it into the local DB,
//     keeping the middleware in sync with the HIS at query time.
//  2. Queries the local DB with any combination of the optional search fields.
//  3. Scopes every query to the authenticated staff member's hospital.
//
// HIS errors are non-fatal: the endpoint continues to serve local DB results.
func SearchPatients(hisClient his.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		hospitalIDVal, exists := c.Get("hospital_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Hospital ID not found in token"})
			return
		}
		hospitalID, ok := hospitalIDVal.(uint)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid hospital ID type"})
			return
		}

		// ── Optional query parameters (all fields from spec) ─────────────────
		nationalID  := c.Query("national_id")
		passportID  := c.Query("passport_id")
		firstName   := c.Query("first_name")
		middleName  := c.Query("middle_name")
		lastName    := c.Query("last_name")
		dateOfBirth := c.Query("date_of_birth") // expected format: YYYY-MM-DD
		phoneNumber := c.Query("phone_number")
		email       := c.Query("email")
		patientHN   := c.Query("patient_hn")

		// ── HIS Sync ──────────────────────────────────────────────────────────
		// When an ID is provided, pull the authoritative record from the HIS
		// and upsert it into the local DB before executing the local query.
		// This ensures the middleware always reflects up-to-date HIS data.
		if hisClient != nil {
			lookupID := nationalID
			if lookupID == "" {
				lookupID = passportID
			}
			if lookupID != "" {
				if hisPt, err := hisClient.SearchPatient(lookupID); err == nil && hisPt != nil {
					upsertPatientFromHIS(hospitalID, hisPt)
				}
				// HIS errors are intentionally swallowed here; local DB is the fallback.
			}
		}

		// ── Local DB query (always scoped to staff's hospital) ───────────────
		query := database.DB.Model(&models.Patient{}).Where("hospital_id = ?", hospitalID)

		if nationalID != "" {
			query = query.Where("national_id = ?", nationalID)
		}
		if passportID != "" {
			query = query.Where("passport_id = ?", passportID)
		}
		if firstName != "" {
			query = query.Where(
				"LOWER(first_name_en) LIKE LOWER(?) OR LOWER(first_name_th) LIKE LOWER(?)",
				"%"+firstName+"%", "%"+firstName+"%",
			)
		}
		if middleName != "" {
			query = query.Where(
				"LOWER(middle_name_en) LIKE LOWER(?) OR LOWER(middle_name_th) LIKE LOWER(?)",
				"%"+middleName+"%", "%"+middleName+"%",
			)
		}
		if lastName != "" {
			query = query.Where(
				"LOWER(last_name_en) LIKE LOWER(?) OR LOWER(last_name_th) LIKE LOWER(?)",
				"%"+lastName+"%", "%"+lastName+"%",
			)
		}
		if dateOfBirth != "" {
			dob, err := time.Parse("2006-01-02", dateOfBirth)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date_of_birth format, expected YYYY-MM-DD"})
				return
			}
			query = query.Where("DATE(date_of_birth) = ?", dob.Format("2006-01-02"))
		}
		if phoneNumber != "" {
			query = query.Where("phone_number = ?", phoneNumber)
		}
		if email != "" {
			query = query.Where("LOWER(email) LIKE LOWER(?)", "%"+email+"%")
		}
		if patientHN != "" {
			query = query.Where("patient_hn = ?", patientHN)
		}

		var total int64
		query.Count(&total)

		var patients []models.Patient
		if err := query.Find(&patients).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search patients"})
			return
		}

		c.JSON(http.StatusOK, PatientSearchResponse{Data: patients, Total: total})
	}
}

// upsertPatientFromHIS inserts or updates a patient record using data received
// from the HIS API, matched on (national_id OR passport_id) + hospital_id.
func upsertPatientFromHIS(hospitalID uint, p *his.PatientResponse) {
	// Guard: cannot create a patient without an identifier or a required HN.
	if (p.NationalID == "" && p.PassportID == "") || p.PatientHN == "" {
		return
	}

	dob, _ := time.Parse("2006-01-02", p.DateOfBirth)

	var patient models.Patient
	err := database.DB.Where(
		"(national_id = ? OR passport_id = ?) AND hospital_id = ?",
		p.NationalID, p.PassportID, hospitalID,
	).First(&patient).Error

	// Populate / overwrite all HIS fields.
	patient.FirstNameTH  = p.FirstNameTH
	patient.MiddleNameTH = p.MiddleNameTH
	patient.LastNameTH   = p.LastNameTH
	patient.FirstNameEN  = p.FirstNameEN
	patient.MiddleNameEN = p.MiddleNameEN
	patient.LastNameEN   = p.LastNameEN
	patient.DateOfBirth  = dob
	patient.PatientHN    = p.PatientHN
	patient.NationalID   = p.NationalID
	patient.PassportID   = p.PassportID
	patient.PhoneNumber  = p.PhoneNumber
	patient.Email        = p.Email
	patient.Gender       = p.Gender
	patient.HospitalID   = hospitalID

	if err != nil {
		database.DB.Create(&patient) // new record
	} else {
		database.DB.Save(&patient) // update existing record
	}
}
