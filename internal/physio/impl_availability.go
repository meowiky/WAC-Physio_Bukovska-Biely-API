package physio

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) CheckAvailability(c *gin.Context) {
	startDateTime, err := parseDateTime(c.Query("startDateTime"))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid startDateTime", err)
		return
	}
	endDateTime, err := parseDateTime(c.Query("endDateTime"))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid endDateTime", err)
		return
	}
	if !startDateTime.Before(endDateTime) {
		jsonError(c, http.StatusBadRequest, "startDateTime must be before endDateTime", nil)
		return
	}

	ambulanceID := c.Query("ambulanceId")
	therapistID := c.Query("therapistId")
	excludeSessionID := c.Query("excludeSessionId")

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	if ambulanceID != "" {
		exists, err := s.ambulanceExists(ctx, ambulanceID)
		if err != nil {
			jsonError(c, http.StatusBadGateway, "failed to validate ambulance reference", err)
			return
		}
		if !exists {
			jsonError(c, http.StatusNotFound, "specified ambulance does not exist", nil)
			return
		}
	}

	if therapistID != "" {
		exists, err := s.therapistExists(ctx, therapistID)
		if err != nil {
			jsonError(c, http.StatusBadGateway, "failed to validate therapist reference", err)
			return
		}
		if !exists {
			jsonError(c, http.StatusNotFound, "specified therapist does not exist", nil)
			return
		}
	}

	ambulanceConflict, therapistConflict, err := s.availabilityForInterval(
		ctx,
		startDateTime,
		endDateTime,
		ambulanceID,
		therapistID,
		excludeSessionID,
	)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to check availability", err)
		return
	}

	result := AvailabilityResult{
		StartDateTime: startDateTime,
		EndDateTime:   endDateTime,
		Ambulance: AvailabilityCheckItem{
			ResourceId:  ambulanceID,
			IsAvailable: ambulanceConflict == nil,
		},
		Therapist: AvailabilityCheckItem{
			ResourceId:  therapistID,
			IsAvailable: therapistConflict == nil,
		},
	}

	if ambulanceConflict != nil {
		conflictID := ambulanceConflict.Id
		result.Ambulance.ConflictingSessionId = &conflictID
	}
	if therapistConflict != nil {
		conflictID := therapistConflict.Id
		result.Therapist.ConflictingSessionId = &conflictID
	}

	c.JSON(http.StatusOK, result)
}
