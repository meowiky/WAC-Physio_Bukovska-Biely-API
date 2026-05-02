package api

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed physio.openapi.json
var openapiSpec []byte

//go:embed swagger.html
var swaggerUI []byte

func HandleOpenApi(ctx *gin.Context) {
	ctx.Data(http.StatusOK, "application/json", openapiSpec)
}

func HandleSwaggerUI(ctx *gin.Context) {
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", swaggerUI)
}
