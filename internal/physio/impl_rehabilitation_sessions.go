package physio

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) CreateRehabilitationSession(c *gin.Context) {
	var session RehabilitationSession
	if err := c.ShouldBindJSON(&session); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if session.Id == "" {
		session.Id = newDocumentID()
	}
	if err := validateSession(session); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	if !s.validateSessionReferencesForWrite(c, ctx, session) {
		return
	}
	if !s.validateSessionAvailabilityForWrite(c, ctx, session, "") {
		return
	}

	if err := createDocument(ctx, s.rehabSessions, session.Id, &session); err != nil {
		switch {
		case errors.Is(err, errConflict):
			jsonError(c, http.StatusConflict, "rehabilitation session already exists", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to create rehabilitation session", err)
		}
		return
	}

	c.JSON(http.StatusCreated, session)
}

func (s *Server) DeleteRehabilitationSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	if err := deleteDocument(ctx, s.rehabSessions, sessionID); err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "rehabilitation session not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to delete rehabilitation session", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) GetRehabilitationSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	session, err := findDocumentByID[RehabilitationSession](ctx, s.rehabSessions, sessionID)
	if err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "rehabilitation session not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to load rehabilitation session", err)
		}
		return
	}

	c.JSON(http.StatusOK, session)
}

func (s *Server) GetRehabilitationSessions(c *gin.Context) {
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	sessions, err := s.sessions(ctx)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to list rehabilitation sessions", err)
		return
	}

	planID := c.Query("planId")
	therapistID := c.Query("therapistId")
	ambulanceID := c.Query("ambulanceId")
	attendanceStatus := AttendanceStatus(c.Query("attendanceStatus"))
	confirmationStatus := SessionConfirmationStatus(c.Query("confirmationStatus"))

	filtered := make([]RehabilitationSession, 0, len(sessions))
	for _, session := range sessions {
		if planID != "" && session.PlanId != planID {
			continue
		}
		if therapistID != "" && session.TherapistId != therapistID {
			continue
		}
		if ambulanceID != "" && session.AmbulanceId != ambulanceID {
			continue
		}
		if attendanceStatus != "" && session.AttendanceStatus != attendanceStatus {
			continue
		}
		if confirmationStatus != "" && session.ConfirmationStatus != confirmationStatus {
			continue
		}
		filtered = append(filtered, session)
	}

	c.JSON(http.StatusOK, filtered)
}

func (s *Server) UpdateRehabilitationSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	var session RehabilitationSession
	if err := c.ShouldBindJSON(&session); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if idMismatch(c, sessionID, session.Id) {
		return
	}
	if err := validateSession(session); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	if !s.validateSessionReferencesForWrite(c, ctx, session) {
		return
	}
	if !s.validateSessionAvailabilityForWrite(c, ctx, session, sessionID) {
		return
	}

	if err := replaceDocument(ctx, s.rehabSessions, sessionID, &session); err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "rehabilitation session not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to update rehabilitation session", err)
		}
		return
	}

	c.JSON(http.StatusOK, session)
}

func (s *Server) validateSessionReferencesForWrite(c *gin.Context, ctx context.Context, session RehabilitationSession) bool {
	planExists, err := s.planExists(ctx, session.PlanId)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate rehabilitation plan reference", err)
		return false
	}
	if !planExists {
		jsonError(c, http.StatusNotFound, "referenced rehabilitation plan does not exist", nil)
		return false
	}

	therapistExists, err := s.therapistExists(ctx, session.TherapistId)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate therapist reference", err)
		return false
	}
	if !therapistExists {
		jsonError(c, http.StatusNotFound, "referenced therapist does not exist", nil)
		return false
	}

	ambulanceExists, err := s.ambulanceExists(ctx, session.AmbulanceId)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate ambulance reference", err)
		return false
	}
	if !ambulanceExists {
		jsonError(c, http.StatusNotFound, "referenced ambulance does not exist", nil)
		return false
	}

	return true
}

func (s *Server) validateSessionAvailabilityForWrite(c *gin.Context, ctx context.Context, session RehabilitationSession, excludeSessionID string) bool {
	if session.StartDateTime == nil || session.EndDateTime == nil {
		return true
	}

	ambulanceConflict, therapistConflict, err := s.availabilityForInterval(
		ctx,
		*session.StartDateTime,
		*session.EndDateTime,
		session.AmbulanceId,
		session.TherapistId,
		excludeSessionID,
	)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate session availability", err)
		return false
	}
	if ambulanceConflict != nil || therapistConflict != nil {
		jsonError(c, http.StatusConflict, "selected therapist or ambulance is not available for the requested time interval", nil)
		return false
	}
	return true
}
