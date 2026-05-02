package physio

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) CreateRehabilitationPlan(c *gin.Context) {
	var plan RehabilitationPlan
	if err := c.ShouldBindJSON(&plan); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if plan.Id == "" {
		plan.Id = newDocumentID()
	}
	if err := validatePlan(plan); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	exists, err := s.patientExists(ctx, plan.PatientId)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate patient reference", err)
		return
	}
	if !exists {
		jsonError(c, http.StatusNotFound, "referenced patient does not exist", nil)
		return
	}

	if err := createDocument(ctx, s.rehabilitationPlans, plan.Id, &plan); err != nil {
		switch {
		case errors.Is(err, errConflict):
			jsonError(c, http.StatusConflict, "rehabilitation plan already exists", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to create rehabilitation plan", err)
		}
		return
	}

	c.JSON(http.StatusCreated, plan)
}

func (s *Server) DeleteRehabilitationPlan(c *gin.Context) {
	planID := c.Param("planId")
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	hasSessions, err := s.planHasSessions(ctx, planID)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate rehabilitation plan references", err)
		return
	}
	if hasSessions {
		jsonError(c, http.StatusConflict, "plan cannot be deleted because related sessions still exist", nil)
		return
	}

	if err := deleteDocument(ctx, s.rehabilitationPlans, planID); err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "rehabilitation plan not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to delete rehabilitation plan", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) GetRehabilitationPlan(c *gin.Context) {
	planID := c.Param("planId")
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	plan, err := findDocumentByID[RehabilitationPlan](ctx, s.rehabilitationPlans, planID)
	if err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "rehabilitation plan not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to load rehabilitation plan", err)
		}
		return
	}

	c.JSON(http.StatusOK, plan)
}

func (s *Server) GetRehabilitationPlans(c *gin.Context) {
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	plans, err := s.plans(ctx)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to list rehabilitation plans", err)
		return
	}

	patientID := c.Query("patientId")
	status := RehabilitationPlanStatus(c.Query("status"))
	filtered := make([]RehabilitationPlan, 0, len(plans))
	for _, plan := range plans {
		if patientID != "" && plan.PatientId != patientID {
			continue
		}
		if status != "" && plan.Status != status {
			continue
		}
		filtered = append(filtered, plan)
	}

	c.JSON(http.StatusOK, filtered)
}

func (s *Server) UpdateRehabilitationPlan(c *gin.Context) {
	planID := c.Param("planId")
	var plan RehabilitationPlan
	if err := c.ShouldBindJSON(&plan); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if idMismatch(c, planID, plan.Id) {
		return
	}
	if err := validatePlan(plan); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	exists, err := s.patientExists(ctx, plan.PatientId)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate patient reference", err)
		return
	}
	if !exists {
		jsonError(c, http.StatusNotFound, "referenced patient does not exist", nil)
		return
	}

	if err := replaceDocument(ctx, s.rehabilitationPlans, planID, &plan); err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "rehabilitation plan not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to update rehabilitation plan", err)
		}
		return
	}

	c.JSON(http.StatusOK, plan)
}
