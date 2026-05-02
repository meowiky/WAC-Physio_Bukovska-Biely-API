package physio

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) CreateTherapist(c *gin.Context) {
	var therapist Therapist
	if err := c.ShouldBindJSON(&therapist); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if therapist.Id == "" {
		therapist.Id = newDocumentID()
	}
	if err := validateTherapist(therapist); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	if err := createDocument(ctx, s.therapists, therapist.Id, &therapist); err != nil {
		switch {
		case errors.Is(err, errConflict):
			jsonError(c, http.StatusConflict, "therapist already exists", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to create therapist", err)
		}
		return
	}

	c.JSON(http.StatusCreated, therapist)
}

func (s *Server) DeleteTherapist(c *gin.Context) {
	therapistID := c.Param("therapistId")
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	hasSessions, err := s.therapistHasSessions(ctx, therapistID)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate therapist references", err)
		return
	}
	if hasSessions {
		jsonError(c, http.StatusConflict, "therapist cannot be deleted because related rehabilitation sessions still exist", nil)
		return
	}

	if err := deleteDocument(ctx, s.therapists, therapistID); err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "therapist not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to delete therapist", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) GetTherapist(c *gin.Context) {
	therapistID := c.Param("therapistId")
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	therapist, err := findDocumentByID[Therapist](ctx, s.therapists, therapistID)
	if err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "therapist not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to load therapist", err)
		}
		return
	}

	c.JSON(http.StatusOK, therapist)
}

func (s *Server) GetTherapists(c *gin.Context) {
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	therapists, err := s.therapistsList(ctx)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to list therapists", err)
		return
	}

	c.JSON(http.StatusOK, therapists)
}

func (s *Server) UpdateTherapist(c *gin.Context) {
	therapistID := c.Param("therapistId")
	var therapist Therapist
	if err := c.ShouldBindJSON(&therapist); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if idMismatch(c, therapistID, therapist.Id) {
		return
	}
	if err := validateTherapist(therapist); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	if err := replaceDocument(ctx, s.therapists, therapistID, &therapist); err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "therapist not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to update therapist", err)
		}
		return
	}

	c.JSON(http.StatusOK, therapist)
}
