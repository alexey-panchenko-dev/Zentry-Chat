package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHealthAdvertisesBirthDateRegistration(t *testing.T) {
	response := httptest.NewRecorder()
	healthHandler(response, httptest.NewRequest("GET", "/api/health", nil))
	var body struct {
		Status       string `json:"status"`
		Capabilities struct {
			BirthDates bool `json:"birth_dates"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || body.Status != "ok" || !body.Capabilities.BirthDates {
		t.Fatalf("registration capability missing: %s", response.Body.String())
	}
}
