package physio

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext(method string, target string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	return ctx, recorder
}

func decodeJSONResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return payload
}

func TestValidatePatient(t *testing.T) {
	t.Run("accepts valid patient", func(t *testing.T) {
		err := validatePatient(Patient{
			Id:              "patient-001",
			FirstName:       "Anna",
			Surname:         "Novakova",
			BirthNumber:     "915101/1234",
			DateOfBirth:     "1991-01-01",
			Sex:             FEMALE,
			HealthInsurance: VSZP,
			Email:           "anna@example.com",
			Phone:           "+421900111222",
			FirstVisitDate:  "2026-05-01",
		})
		if err != nil {
			t.Fatalf("expected valid patient, got error: %v", err)
		}
	})

	t.Run("rejects invalid health insurance", func(t *testing.T) {
		err := validatePatient(Patient{
			FirstName:       "Anna",
			Surname:         "Novakova",
			BirthNumber:     "915101/1234",
			DateOfBirth:     "1991-01-01",
			Sex:             FEMALE,
			HealthInsurance: HealthInsurance("INVALID"),
			Email:           "anna@example.com",
			Phone:           "+421900111222",
			FirstVisitDate:  "2026-05-01",
		})
		if err == nil {
			t.Fatal("expected invalid health insurance to fail validation")
		}
	})
}

func TestValidatePlan(t *testing.T) {
	err := validatePlan(RehabilitationPlan{
		Id:        "plan-001",
		PatientId: "patient-001",
		Status:    PLAN_CANCELED,
		Notes:     "Canceled plan is still a valid enum value",
	})
	if err != nil {
		t.Fatalf("expected canceled plan status to be valid, got error: %v", err)
	}
}

func TestValidateSession(t *testing.T) {
	t.Run("rejects partial time range", func(t *testing.T) {
		start := time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)
		err := validateSession(RehabilitationSession{
			PlanId:             "plan-001",
			AmbulanceId:        "ambulance-001",
			TherapistId:        "therapist-001",
			StartDateTime:      &start,
			AttendanceStatus:   PLANNED,
			ConfirmationStatus: CONFIRMED,
		})
		if err == nil {
			t.Fatal("expected validation error when only one time boundary is present")
		}
	})

	t.Run("rejects inverted time range", func(t *testing.T) {
		start := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
		end := time.Date(2026, 5, 5, 9, 0, 0, 0, time.UTC)
		err := validateSession(RehabilitationSession{
			PlanId:             "plan-001",
			AmbulanceId:        "ambulance-001",
			TherapistId:        "therapist-001",
			StartDateTime:      &start,
			EndDateTime:        &end,
			AttendanceStatus:   PLANNED,
			ConfirmationStatus: CONFIRMED,
		})
		if err == nil {
			t.Fatal("expected validation error when startDateTime is not before endDateTime")
		}
	})
}

func TestIdMismatch(t *testing.T) {
	t.Run("returns bad request when body id is empty", func(t *testing.T) {
		ctx, recorder := newTestContext(http.MethodPut, "/api/patients/patient-001", "")
		if !idMismatch(ctx, "patient-001", "") {
			t.Fatal("expected idMismatch to report a problem")
		}
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
		}
	})

	t.Run("returns forbidden when path and body ids differ", func(t *testing.T) {
		ctx, recorder := newTestContext(http.MethodPut, "/api/patients/patient-001", "")
		if !idMismatch(ctx, "patient-001", "patient-002") {
			t.Fatal("expected idMismatch to report a problem")
		}
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
		}
	})
}

func TestCreatePatient_InvalidJSON_ReturnsBadRequest(t *testing.T) {
	server := &Server{timeout: time.Second}
	ctx, recorder := newTestContext(http.MethodPost, "/api/patients", "{")

	server.CreatePatient(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	payload := decodeJSONResponse(t, recorder)
	if payload["message"] != "invalid request body" {
		t.Fatalf("expected invalid request body message, got %v", payload["message"])
	}
}

func TestUpdatePatient_PathBodyMismatch_ReturnsForbidden(t *testing.T) {
	server := &Server{timeout: time.Second}
	ctx, recorder := newTestContext(http.MethodPut, "/api/patients/patient-001", `{
		"id":"patient-002",
		"firstName":"Anna",
		"surname":"Novakova",
		"birthNumber":"915101/1234",
		"dateOfBirth":"1991-01-01",
		"sex":"female",
		"healthInsurance":"VSZP",
		"email":"anna@example.com",
		"phone":"+421900111222",
		"firstVisitDate":"2026-05-01"
	}`)
	ctx.Params = []gin.Param{{Key: "patientId", Value: "patient-001"}}

	server.UpdatePatient(ctx)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestCreateRehabilitationPlan_MissingPatientID_ReturnsBadRequest(t *testing.T) {
	server := &Server{timeout: time.Second}
	ctx, recorder := newTestContext(http.MethodPost, "/api/rehabilitation-plans", `{
		"id":"plan-001",
		"patientId":"",
		"status":"active",
		"notes":"missing patient id"
	}`)

	server.CreateRehabilitationPlan(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestCreateRehabilitationSession_InvalidDateRange_ReturnsBadRequest(t *testing.T) {
	server := &Server{timeout: time.Second}
	ctx, recorder := newTestContext(http.MethodPost, "/api/rehabilitation-sessions", `{
		"id":"session-001",
		"planId":"plan-001",
		"ambulanceId":"ambulance-001",
		"therapistId":"therapist-001",
		"startDateTime":"2026-05-05T10:00:00Z",
		"endDateTime":"2026-05-05T09:00:00Z",
		"attendanceStatus":"planned",
		"confirmationStatus":"confirmed"
	}`)

	server.CreateRehabilitationSession(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
}

func TestCheckAvailability_InvalidInputs(t *testing.T) {
	server := &Server{timeout: time.Second}

	t.Run("invalid startDateTime", func(t *testing.T) {
		ctx, recorder := newTestContext(http.MethodGet, "/api/availability?startDateTime=not-a-date&endDateTime=2026-05-05T10:00:00Z", "")

		server.CheckAvailability(ctx)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
		}
	})

	t.Run("startDateTime must be before endDateTime", func(t *testing.T) {
		ctx, recorder := newTestContext(http.MethodGet, "/api/availability?startDateTime=2026-05-05T10:00:00Z&endDateTime=2026-05-05T09:00:00Z", "")

		server.CheckAvailability(ctx)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
		}
		payload := decodeJSONResponse(t, recorder)
		if payload["message"] != "startDateTime must be before endDateTime" {
			t.Fatalf("unexpected error message: %v", payload["message"])
		}
	})
}
