// Package his provides a client for querying Hospital Information Systems (HIS).
package his

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PatientResponse matches the Hospital A HIS API JSON response exactly.
// Route: GET https://hospital-a.api.co.th/patient/search/{id}
// The {id} parameter can be either a national_id or passport_id.
type PatientResponse struct {
	FirstNameTH  string `json:"first_name_th"`
	MiddleNameTH string `json:"middle_name_th"`
	LastNameTH   string `json:"last_name_th"`
	FirstNameEN  string `json:"first_name_en"`
	MiddleNameEN string `json:"middle_name_en"`
	LastNameEN   string `json:"last_name_en"`
	DateOfBirth  string `json:"date_of_birth"` // YYYY-MM-DD
	PatientHN    string `json:"patient_hn"`
	NationalID   string `json:"national_id"`
	PassportID   string `json:"passport_id"`
	PhoneNumber  string `json:"phone_number"`
	Email        string `json:"email"`
	Gender       string `json:"gender"` // "M" or "F"
}

// Client is the interface for querying a Hospital Information System.
// Using an interface allows the handler to be tested with a mock.
type Client interface {
	// SearchPatient fetches a patient record by national_id or passport_id.
	// Returns (nil, nil) when the patient is not found (HTTP 404).
	SearchPatient(id string) (*PatientResponse, error)
}

// hospitalAClient is the concrete implementation for Hospital A.
type hospitalAClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewHospitalAClient returns a Client configured for Hospital A's HIS API.
func NewHospitalAClient() Client {
	return &hospitalAClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    "https://hospital-a.api.co.th",
	}
}

// SearchPatient calls GET /patient/search/{id} on the Hospital A HIS.
func (c *hospitalAClient) SearchPatient(id string) (*PatientResponse, error) {
	url := fmt.Sprintf("%s/patient/search/%s", c.baseURL, id)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HIS request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound:
		return nil, nil // patient not found in HIS — not an error
	case http.StatusOK:
		var patient PatientResponse
		if err := json.NewDecoder(resp.Body).Decode(&patient); err != nil {
			return nil, fmt.Errorf("failed to decode HIS response: %w", err)
		}
		return &patient, nil
	default:
		return nil, fmt.Errorf("HIS returned unexpected status %d", resp.StatusCode)
	}
}
