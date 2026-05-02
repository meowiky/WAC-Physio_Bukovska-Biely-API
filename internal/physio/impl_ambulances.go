package physio

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) CreateAmbulance(c *gin.Context) {
	var ambulance Ambulance
	if err := c.ShouldBindJSON(&ambulance); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if ambulance.Id == "" {
		ambulance.Id = newDocumentID()
	}
	if err := validateAmbulance(ambulance); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	if err := createDocument(ctx, s.ambulances, ambulance.Id, &ambulance); err != nil {
		switch {
		case errors.Is(err, errConflict):
			jsonError(c, http.StatusConflict, "ambulance already exists", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to create ambulance", err)
		}
		return
	}

	c.JSON(http.StatusCreated, ambulance)
}

func (s *Server) DeleteAmbulance(c *gin.Context) {
	ambulanceID := c.Param("ambulanceId")
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	hasSessions, err := s.ambulanceHasSessions(ctx, ambulanceID)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to validate ambulance references", err)
		return
	}
	if hasSessions {
		jsonError(c, http.StatusConflict, "ambulance cannot be deleted because related rehabilitation sessions still exist", nil)
		return
	}

	if err := deleteDocument(ctx, s.ambulances, ambulanceID); err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "ambulance not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to delete ambulance", err)
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (s *Server) GetAmbulance(c *gin.Context) {
	ambulanceID := c.Param("ambulanceId")
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	ambulance, err := findDocumentByID[Ambulance](ctx, s.ambulances, ambulanceID)
	if err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "ambulance not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to load ambulance", err)
		}
		return
	}

	c.JSON(http.StatusOK, ambulance)
}

func (s *Server) GetAmbulances(c *gin.Context) {
	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	ambulances, err := s.ambulancesList(ctx)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "failed to list ambulances", err)
		return
	}

	c.JSON(http.StatusOK, ambulances)
}

func (s *Server) UpdateAmbulance(c *gin.Context) {
	ambulanceID := c.Param("ambulanceId")
	var ambulance Ambulance
	if err := c.ShouldBindJSON(&ambulance); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if idMismatch(c, ambulanceID, ambulance.Id) {
		return
	}
	if err := validateAmbulance(ambulance); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error(), nil)
		return
	}

	ctx, cancel := s.requestContext(c.Request.Context())
	defer cancel()

	if err := replaceDocument(ctx, s.ambulances, ambulanceID, &ambulance); err != nil {
		switch {
		case errors.Is(err, errNotFound):
			jsonError(c, http.StatusNotFound, "ambulance not found", err)
		default:
			jsonError(c, http.StatusBadGateway, "failed to update ambulance", err)
		}
		return
	}

	c.JSON(http.StatusOK, ambulance)
}
