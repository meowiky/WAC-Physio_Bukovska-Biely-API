package physio

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) CreatePatient(c *gin.Context) {
	var patient Patient
	if err := c.ShouldBindJSON(&patient); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if patient.Id == "" {
		patient.Id = newDocumentID()
	}
	if err := validatePatient(patient); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	conflict, err := s.patientBirthNumberConflict(ctx, patient, "")
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate patient uniqueness", err)
		return
	}
	if conflict {
		jsonError(c, http.StatusConflict, "patient with the same birth number already exists", nil)
		return
	}

	if err := createDocument(ctx, s.patients, patient.Id, &patient); err != nil {
		switch {
		case errors.Is(err, errConflict):
			jsonError(c, http.StatusConflict, "patient already exists", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to create patient", err)
		}
		return
	}

	c.JSON(http.StatusCreated, patient)
}

func (s *Server) DeletePatient(c *gin.Context) {
	patientID := c.Param("patientId")
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	hasPlans, err := s.patientHasPlans(ctx, patientID)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate patient references", err)
		return
	}
	if hasPlans {
		jsonError(c, http.StatusConflict, "patient cannot be deleted because related rehabilitation plans still exist", nil)
		return
	}

	if err := deleteDocument(ctx, s.patients, patientID); err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "patient not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to delete patient", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) GetPatient(c *gin.Context) {
	patientID := c.Param("patientId")
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	patient, err := findDocumentByID[Patient](ctx, s.patients, patientID)
	if err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "patient not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to load patient", err)
		}
		return
	}

	c.JSON(http.StatusOK, patient)
}

func (s *Server) GetPatients(c *gin.Context) {
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	patients, err := s.patientsList(ctx)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to list patients", err)
		return
	}

	c.JSON(http.StatusOK, patients)
}

func (s *Server) UpdatePatient(c *gin.Context) {
	patientID := c.Param("patientId")
	var patient Patient
	if err := c.ShouldBindJSON(&patient); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if idMismatch(c, patientID, patient.Id) {
		return
	}
	if err := validatePatient(patient); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	conflict, err := s.patientBirthNumberConflict(ctx, patient, patientID)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate patient uniqueness", err)
		return
	}
	if conflict {
		jsonError(c, http.StatusConflict, "patient with the same birth number already exists", nil)
		return
	}

	if err := replaceDocument(ctx, s.patients, patientID, &patient); err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "patient not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to update patient", err)
		}
		return
	}

	c.JSON(http.StatusOK, patient)
}
