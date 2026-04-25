package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"

	"hospital-backend/database"
	"hospital-backend/handlers"
	"hospital-backend/his"
	"hospital-backend/middleware"
	"hospital-backend/models"
)

// ── Mock HIS Client ──────────────────────────────────────────────────────────

type mockHISClient struct {
	response *his.PatientResponse
	err      error
}

func (m *mockHISClient) SearchPatient(_ string) (*his.PatientResponse, error) {
	return m.response, m.err
}

// noopHISClient returns no data and no error — simulates HIS being absent.
var noopHISClient his.Client = &mockHISClient{}

// ── Router & Request Helpers ─────────────────────────────────────────────────

func setupPatientRouter(hisClient his.Client) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	patientGroup := r.Group("/patient")
	patientGroup.Use(middleware.AuthRequired())
	{
		patientGroup.GET("/search", handlers.SearchPatients(hisClient))
	}
	return r
}

// authGet performs a GET request with an optional Bearer token.
func authGet(r *gin.Engine, rawURL, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", rawURL, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)
	return w
}

// buildURL constructs a search URL with properly encoded query parameters.
func buildURL(path string, params map[string]string) string {
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return path + "?" + q.Encode()
}

// ── Data Setup Helper ────────────────────────────────────────────────────────

// setupHospitalData creates a hospital, one staff member, and optional patients.
// Returns a valid JWT token for that staff member.
func setupHospitalData(t *testing.T, hospitalName, staffUsername string, patients []models.Patient) string {
	t.Helper()

	hospital := models.Hospital{Name: hospitalName}
	assert.NoError(t, database.DB.Create(&hospital).Error)

	hashedPass, _ := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
	staff := models.Staff{
		Username:     staffUsername,
		PasswordHash: string(hashedPass),
		HospitalID:   hospital.ID,
	}
	assert.NoError(t, database.DB.Create(&staff).Error)

	for i := range patients {
		patients[i].HospitalID = hospital.ID
		assert.NoError(t, database.DB.Create(&patients[i]).Error)
	}

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
	assert.NoError(t, err)
	return tokenString
}

// ============================================================
// AUTHENTICATION TESTS
// ============================================================

func TestSearchPatients_NoAuth(t *testing.T) {
	database.CleanTestDB()
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSearchPatients_InvalidToken(t *testing.T) {
	database.CleanTestDB()
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search", "invalid.token.here")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSearchPatients_ExpiredToken(t *testing.T) {
	database.CleanTestDB()

	hospital := models.Hospital{Name: "Expire Hospital"}
	database.DB.Create(&hospital)
	hashedPass, _ := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.DefaultCost)
	staff := models.Staff{Username: "expire_staff", PasswordHash: string(hashedPass), HospitalID: hospital.ID}
	database.DB.Create(&staff)

	claims := middleware.Claims{
		StaffID:    staff.ID,
		HospitalID: hospital.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(middleware.JwtSecret)

	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search", tok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// ============================================================
// NO-FILTER SEARCH
// ============================================================

func TestSearchPatients_NoParams_ReturnsAllInHospital(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "All Hospital", "all_staff", []models.Patient{
		{FirstNameEN: "P1", LastNameEN: "L1", PatientHN: "HN-X01"},
		{FirstNameEN: "P2", LastNameEN: "L2", PatientHN: "HN-X02"},
		{FirstNameEN: "P3", LastNameEN: "L3", PatientHN: "HN-X03"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(3), resp.Total)
	assert.Len(t, resp.Data, 3)
}

// ============================================================
// SEARCH BY national_id
// ============================================================

func TestSearchPatients_ByNationalID_Found(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "NID Hospital", "nid_staff", []models.Patient{
		{FirstNameEN: "John", LastNameEN: "Doe", PatientHN: "HN001", NationalID: "1234567890123"},
		{FirstNameEN: "Jane", LastNameEN: "Smith", PatientHN: "HN002", NationalID: "9876543210987"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?national_id=1234567890123", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "John", resp.Data[0].FirstNameEN)
}

func TestSearchPatients_ByNationalID_NotFound(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "NID Miss Hospital", "nidmiss_staff", []models.Patient{
		{FirstNameEN: "John", LastNameEN: "Doe", PatientHN: "HN003", NationalID: "1111111111111"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?national_id=0000000000000", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(0), resp.Total)
	assert.Empty(t, resp.Data)
}

// ============================================================
// SEARCH BY passport_id
// ============================================================

func TestSearchPatients_ByPassportID_Found(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "Passport Hospital", "pass_staff", []models.Patient{
		{FirstNameEN: "Alice", LastNameEN: "Wong", PatientHN: "HN101", PassportID: "AB1234567"},
		{FirstNameEN: "Bob", LastNameEN: "Lee", PatientHN: "HN102", PassportID: "CD9876543"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?passport_id=AB1234567", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "Alice", resp.Data[0].FirstNameEN)
}

func TestSearchPatients_ByPassportID_NotFound(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "Passport Miss Hospital", "passmiss_staff", []models.Patient{
		{FirstNameEN: "Alice", LastNameEN: "Wong", PatientHN: "HN103", PassportID: "AB1234567"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?passport_id=ZZZZZZZZZ", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(0), resp.Total)
	assert.Empty(t, resp.Data)
}

// ============================================================
// SEARCH BY first_name  (partial, case-insensitive, TH/EN)
// ============================================================

func TestSearchPatients_ByFirstName_EN(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "FName Hospital", "fname_staff", []models.Patient{
		{FirstNameEN: "Somchai", LastNameEN: "Jaidee", FirstNameTH: "สมชาย", PatientHN: "HN201"},
		{FirstNameEN: "Somsri", LastNameEN: "Rakdee", FirstNameTH: "สมศรี", PatientHN: "HN202"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?first_name=Somchai", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "Somchai", resp.Data[0].FirstNameEN)
}

func TestSearchPatients_ByFirstName_TH(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "FNameTH Hospital", "fnameth_staff", []models.Patient{
		{FirstNameEN: "Somchai", LastNameEN: "Jaidee", FirstNameTH: "สมชาย", PatientHN: "HN203"},
		{FirstNameEN: "Somsri", LastNameEN: "Rakdee", FirstNameTH: "สมศรี", PatientHN: "HN204"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, buildURL("/patient/search", map[string]string{"first_name": "สมชาย"}), token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "สมชาย", resp.Data[0].FirstNameTH)
}

func TestSearchPatients_ByFirstName_CaseInsensitive(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "Case Hospital", "case_staff", []models.Patient{
		{FirstNameEN: "UPPERCASE", LastNameEN: "Patient", PatientHN: "HN205"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?first_name=uppercase", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
}

// ============================================================
// SEARCH BY middle_name  (partial, case-insensitive, TH/EN)
// ============================================================

func TestSearchPatients_ByMiddleName_EN(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "Mid Hospital", "mid_staff", []models.Patient{
		{FirstNameEN: "John", MiddleNameEN: "Robert", LastNameEN: "Doe", PatientHN: "HN301"},
		{FirstNameEN: "Jane", MiddleNameEN: "Marie", LastNameEN: "Doe", PatientHN: "HN302"},
		{FirstNameEN: "Bob", MiddleNameEN: "", LastNameEN: "Smith", PatientHN: "HN303"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?middle_name=Robert", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "John", resp.Data[0].FirstNameEN)
	assert.Equal(t, "Robert", resp.Data[0].MiddleNameEN)
}

func TestSearchPatients_ByMiddleName_TH(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "MidTH Hospital", "midth_staff", []models.Patient{
		{FirstNameEN: "Somchai", MiddleNameTH: "กลาง", LastNameEN: "Jaidee", PatientHN: "HN304"},
		{FirstNameEN: "Somsri", MiddleNameTH: "ใจ", LastNameEN: "Rakdee", PatientHN: "HN305"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, buildURL("/patient/search", map[string]string{"middle_name": "กลาง"}), token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "กลาง", resp.Data[0].MiddleNameTH)
}

// ============================================================
// SEARCH BY last_name  (partial, TH/EN)
// ============================================================

func TestSearchPatients_ByLastName_EN(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "LName Hospital", "lname_staff", []models.Patient{
		{FirstNameEN: "John", LastNameEN: "Doe", PatientHN: "HN401"},
		{FirstNameEN: "Jane", LastNameEN: "Doe", PatientHN: "HN402"},
		{FirstNameEN: "Bob", LastNameEN: "Smith", PatientHN: "HN403"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?last_name=Doe", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(2), resp.Total)
}

func TestSearchPatients_ByLastName_TH(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "LNameTH Hospital", "lnameth_staff", []models.Patient{
		{FirstNameEN: "Somchai", LastNameTH: "ใจดี", PatientHN: "HN404"},
		{FirstNameEN: "Somsri", LastNameTH: "รักดี", PatientHN: "HN405"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, buildURL("/patient/search", map[string]string{"last_name": "ใจดี"}), token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "ใจดี", resp.Data[0].LastNameTH)
}

// ============================================================
// SEARCH BY date_of_birth
// ============================================================

func TestSearchPatients_ByDateOfBirth_Found(t *testing.T) {
	database.CleanTestDB()
	dob1, _ := time.Parse("2006-01-02", "1990-05-15")
	dob2, _ := time.Parse("2006-01-02", "1985-11-20")
	token := setupHospitalData(t, "DOB Hospital", "dob_staff", []models.Patient{
		{FirstNameEN: "John", LastNameEN: "A", PatientHN: "HN501", DateOfBirth: dob1},
		{FirstNameEN: "Jane", LastNameEN: "B", PatientHN: "HN502", DateOfBirth: dob2},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?date_of_birth=1990-05-15", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "John", resp.Data[0].FirstNameEN)
}

func TestSearchPatients_ByDateOfBirth_InvalidFormat(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "DOBErr Hospital", "doberr_staff", nil)
	r := setupPatientRouter(noopHISClient)

	// Wrong format: DD-MM-YYYY instead of YYYY-MM-DD
	w := authGet(r, "/patient/search?date_of_birth=15-05-1990", token)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Contains(t, resp["error"], "YYYY-MM-DD")
}

func TestSearchPatients_ByDateOfBirth_NotFound(t *testing.T) {
	database.CleanTestDB()
	dob, _ := time.Parse("2006-01-02", "1990-05-15")
	token := setupHospitalData(t, "DOBMiss Hospital", "dobmiss_staff", []models.Patient{
		{FirstNameEN: "John", LastNameEN: "A", PatientHN: "HN503", DateOfBirth: dob},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?date_of_birth=2000-01-01", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(0), resp.Total)
}

// ============================================================
// SEARCH BY phone_number
// ============================================================

func TestSearchPatients_ByPhoneNumber_Found(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "Phone Hospital", "phone_staff", []models.Patient{
		{FirstNameEN: "Tom", LastNameEN: "A", PatientHN: "HN601", PhoneNumber: "0812345678"},
		{FirstNameEN: "Jerry", LastNameEN: "B", PatientHN: "HN602", PhoneNumber: "0898765432"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?phone_number=0812345678", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "Tom", resp.Data[0].FirstNameEN)
}

func TestSearchPatients_ByPhoneNumber_NotFound(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "Phone Miss Hospital", "phonemiss_staff", []models.Patient{
		{FirstNameEN: "Tom", LastNameEN: "A", PatientHN: "HN603", PhoneNumber: "0812345678"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?phone_number=0000000000", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(0), resp.Total)
	assert.Empty(t, resp.Data)
}

// ============================================================
// SEARCH BY email  (partial, case-insensitive)
// ============================================================

func TestSearchPatients_ByEmail_Partial(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "Email Hospital", "email_staff", []models.Patient{
		{FirstNameEN: "Amy", LastNameEN: "C", PatientHN: "HN701", Email: "amy@hospital.com"},
		{FirstNameEN: "Ben", LastNameEN: "D", PatientHN: "HN702", Email: "ben@hospital.com"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?email=amy", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "Amy", resp.Data[0].FirstNameEN)
}

func TestSearchPatients_ByEmail_NotFound(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "Email Miss Hospital", "emailmiss_staff", []models.Patient{
		{FirstNameEN: "Amy", LastNameEN: "C", PatientHN: "HN703", Email: "amy@hospital.com"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?email=nobody@nowhere.com", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(0), resp.Total)
	assert.Empty(t, resp.Data)
}

// ============================================================
// SEARCH BY patient_hn  (exact match)
// ============================================================

func TestSearchPatients_ByPatientHN_Found(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "HN Hospital", "hn_staff", []models.Patient{
		{FirstNameEN: "Pat", LastNameEN: "E", PatientHN: "HN-A001"},
		{FirstNameEN: "Pete", LastNameEN: "F", PatientHN: "HN-A002"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?patient_hn=HN-A001", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "Pat", resp.Data[0].FirstNameEN)
}

func TestSearchPatients_ByPatientHN_NotFound(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "HN Miss Hospital", "hnmiss_staff", []models.Patient{
		{FirstNameEN: "Pat", LastNameEN: "E", PatientHN: "HN-A003"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?patient_hn=HN-DOES-NOT-EXIST", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(0), resp.Total)
	assert.Empty(t, resp.Data)
}

// ============================================================
// MULTIPLE CRITERIA
// ============================================================

func TestSearchPatients_MultipleCriteria(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "Multi Hospital", "multi_staff", []models.Patient{
		{FirstNameEN: "John", LastNameEN: "Doe", PatientHN: "HN-M01", NationalID: "1111100000001"},
		{FirstNameEN: "John", LastNameEN: "Smith", PatientHN: "HN-M02", NationalID: "1111100000002"},
		{FirstNameEN: "Jane", LastNameEN: "Doe", PatientHN: "HN-M03", NationalID: "1111100000003"},
	})
	r := setupPatientRouter(noopHISClient)
	w := authGet(r, "/patient/search?first_name=John&last_name=Doe", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "John", resp.Data[0].FirstNameEN)
	assert.Equal(t, "Doe", resp.Data[0].LastNameEN)
}

// ============================================================
// CROSS-HOSPITAL ISOLATION  (core business logic)
// ============================================================

func TestSearchPatients_CrossHospitalIsolation(t *testing.T) {
	database.CleanTestDB()

	tokenA := setupHospitalData(t, "Hospital Alpha", "staff_alpha", []models.Patient{
		{FirstNameEN: "AlphaPatient", LastNameEN: "One", PatientHN: "HNA001", NationalID: "1111111111111"},
	})
	_ = setupHospitalData(t, "Hospital Beta", "staff_beta", []models.Patient{
		{FirstNameEN: "BetaPatient", LastNameEN: "Two", PatientHN: "HNB001", NationalID: "2222222222222"},
	})

	r := setupPatientRouter(noopHISClient)

	// Hospital A staff can see their own patient
	w := authGet(r, "/patient/search?first_name=AlphaPatient", tokenA)
	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "AlphaPatient", resp.Data[0].FirstNameEN)

	// Hospital A staff cannot see Hospital B patient by national_id
	w2 := authGet(r, "/patient/search?national_id=2222222222222", tokenA)
	var resp2 handlers.PatientSearchResponse
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, int64(0), resp2.Total)
	assert.Empty(t, resp2.Data)

	// Hospital A staff cannot see Hospital B patient by name
	w3 := authGet(r, "/patient/search?first_name=BetaPatient", tokenA)
	var resp3 handlers.PatientSearchResponse
	json.Unmarshal(w3.Body.Bytes(), &resp3)
	assert.Equal(t, int64(0), resp3.Total)
}

// ============================================================
// HIS INTEGRATION TESTS
// ============================================================

// HIS returns a patient → gets created in local DB → returned in search.
func TestSearchPatients_HIS_CreatesNewLocalRecord(t *testing.T) {
	database.CleanTestDB()

	// Staff exists but patient does NOT yet exist locally.
	token := setupHospitalData(t, "HIS Hospital", "his_staff", nil)

	mockHIS := &mockHISClient{
		response: &his.PatientResponse{
			FirstNameTH:  "สมหมาย",
			MiddleNameTH: "กลาง",
			LastNameTH:   "ดีมาก",
			FirstNameEN:  "Sommai",
			MiddleNameEN: "Klang",
			LastNameEN:   "Deemax",
			DateOfBirth:  "1995-03-20",
			PatientHN:    "HN-HIS-001",
			NationalID:   "3333333333333",
			PassportID:   "",
			PhoneNumber:  "0811110000",
			Email:        "sommai@example.com",
			Gender:       "M",
		},
	}

	r := setupPatientRouter(mockHIS)
	w := authGet(r, "/patient/search?national_id=3333333333333", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "Sommai", resp.Data[0].FirstNameEN)
	assert.Equal(t, "Klang", resp.Data[0].MiddleNameEN)
	assert.Equal(t, "สมหมาย", resp.Data[0].FirstNameTH)
	assert.Equal(t, "กลาง", resp.Data[0].MiddleNameTH)
	assert.Equal(t, "M", resp.Data[0].Gender)
	assert.Equal(t, "sommai@example.com", resp.Data[0].Email)
}

// HIS returns updated data → existing local record is synced.
func TestSearchPatients_HIS_UpdatesExistingLocalRecord(t *testing.T) {
	database.CleanTestDB()

	// Patient already exists locally with a stale phone number.
	token := setupHospitalData(t, "HIS Update Hospital", "his_upd_staff", []models.Patient{
		{
			FirstNameEN: "Sommai", LastNameEN: "Deemax",
			PatientHN: "HN-HIS-002", NationalID: "4444444444444",
			PhoneNumber: "0800000000",
		},
	})

	// HIS returns the same patient with an updated phone number.
	mockHIS := &mockHISClient{
		response: &his.PatientResponse{
			FirstNameEN: "Sommai",
			LastNameEN:  "Deemax",
			PatientHN:   "HN-HIS-002",
			NationalID:  "4444444444444",
			PhoneNumber: "0899999999",
			Gender:      "M",
		},
	}

	r := setupPatientRouter(mockHIS)
	w := authGet(r, "/patient/search?national_id=4444444444444", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "0899999999", resp.Data[0].PhoneNumber)
}

// HIS lookup via passport_id also triggers a sync.
func TestSearchPatients_HIS_SyncByPassportID(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "HIS Pass Hospital", "his_pass_staff", nil)

	mockHIS := &mockHISClient{
		response: &his.PatientResponse{
			FirstNameEN: "Foreign",
			LastNameEN:  "Patient",
			PatientHN:   "HN-HIS-003",
			PassportID:  "PP9998887",
			Gender:      "F",
		},
	}

	r := setupPatientRouter(mockHIS)
	w := authGet(r, "/patient/search?passport_id=PP9998887", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "Foreign", resp.Data[0].FirstNameEN)
	assert.Equal(t, "F", resp.Data[0].Gender)
}

// HIS error is non-fatal — local DB results are still returned.
func TestSearchPatients_HIS_ErrorStillServesLocalResults(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "HIS Err Hospital", "his_err_staff", []models.Patient{
		{FirstNameEN: "Local", LastNameEN: "Only", PatientHN: "HN-LOCAL-001", NationalID: "5555555555555"},
	})

	// HIS is unavailable.
	mockHIS := &mockHISClient{err: fmt.Errorf("HIS service unavailable")}

	r := setupPatientRouter(mockHIS)
	w := authGet(r, "/patient/search?national_id=5555555555555", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Still returns the pre-existing local record — HIS error must not break the endpoint.
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "Local", resp.Data[0].FirstNameEN)
}

// HIS returns nil (patient not found in HIS) — existing local record is unchanged.
func TestSearchPatients_HIS_NotFoundStillServesLocalResults(t *testing.T) {
	database.CleanTestDB()
	token := setupHospitalData(t, "HIS NotFound Hospital", "his_nf_staff", []models.Patient{
		{FirstNameEN: "Existing", LastNameEN: "Patient", PatientHN: "HN-NF-001", NationalID: "6666666666666"},
	})

	// HIS returns nil, nil (patient not in HIS).
	mockHIS := &mockHISClient{response: nil, err: nil}

	r := setupPatientRouter(mockHIS)
	w := authGet(r, "/patient/search?national_id=6666666666666", token)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp handlers.PatientSearchResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, "Existing", resp.Data[0].FirstNameEN)
}
