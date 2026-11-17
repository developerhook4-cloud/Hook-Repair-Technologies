package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"hook-repair/models"
)

// GetPostalCode — GET /api/postal-codes/:cp
// Cualquier usuario con sesión (no depende de la organización ni de permisos).
// 400 si no son 5 dígitos, 404 si no existe, 503 si el catálogo aún no se carga.
func GetPostalCode(c *gin.Context) {
	cp := c.Param("cp")
	if !models.ValidPostalCode(cp) {
		problem(c, http.StatusBadRequest, "Invalid postal code", "El código postal debe tener 5 dígitos.")
		return
	}
	pc, err := models.LookupPostalCode(cp)
	switch {
	case errors.Is(err, models.ErrPostalCatalogEmpty):
		problem(c, http.StatusServiceUnavailable, "Service unavailable", "Catálogo no disponible.")
	case errors.Is(err, models.ErrNotFound):
		problem(c, http.StatusNotFound, "Not found", "Código postal no encontrado.")
	case err != nil:
		respondError(c, err, "postal code")
	default:
		c.JSON(http.StatusOK, pc)
	}
}
