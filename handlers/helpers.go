package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"hook-repair/models"
)

// problem responde con un error en JSON
// (application/problem+json con title / status / detail).
func problem(c *gin.Context, status int, title, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, gin.H{
		"type":   fmt.Sprintf("https://httpstatuses.com/%d", status),
		"title":  title,
		"status": status,
		"detail": detail,
	})
}

// respondError traduce los errores de models a la respuesta HTTP correcta.
func respondError(c *gin.Context, err error, what string) {
	var ve *models.ValidationError
	var ce *models.ConflictError
	switch {
	case errors.As(err, &ve):
		problem(c, http.StatusBadRequest, "Invalid "+what, ve.Msg)
	case errors.As(err, &ce):
		problem(c, http.StatusConflict, what+" conflict", ce.Msg)
	case errors.Is(err, models.ErrNotFound):
		problem(c, http.StatusNotFound, what+" not found", "El registro no existe.")
	case models.IsUniqueViolation(err):
		problem(c, http.StatusConflict, what+" conflict", "El registro ya existe.")
	default:
		log.Printf("[%s] %s %s: %v", what, c.Request.Method, c.Request.URL.Path, err)
		problem(c, http.StatusInternalServerError, "Server error", "Ocurrió un error inesperado. Intenta de nuevo.")
	}
}

// bindJSON lee el cuerpo como JSON; si está mal formado responde 400.
func bindJSON(c *gin.Context, dst any) bool {
	dec := json.NewDecoder(c.Request.Body)
	if err := dec.Decode(dst); err != nil {
		var ve *models.ValidationError
		if errors.As(err, &ve) {
			problem(c, http.StatusBadRequest, "Invalid request", ve.Msg)
		} else {
			problem(c, http.StatusBadRequest, "Invalid request", "El cuerpo de la petición no es válido.")
		}
		return false
	}
	return true
}

// statusFromRequest lee el estado de un PATCH .../status.
// Acepta {"active": bool}, {"isActive": bool} o ?active= / ?isActive= en la URL
// Si no viene
// nada, usa def.
func statusFromRequest(c *gin.Context, def bool) (bool, bool) {
	for _, key := range []string{"active", "isActive"} {
		if v := c.Query(key); v != "" {
			return v == "true" || v == "1", true
		}
	}
	body, _ := io.ReadAll(c.Request.Body)
	if len(body) == 0 {
		return def, true
	}
	var in struct {
		Active   *bool `json:"active"`
		IsActive *bool `json:"isActive"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		problem(c, http.StatusBadRequest, "Invalid request", "El cuerpo de la petición no es válido.")
		return false, false
	}
	switch {
	case in.Active != nil:
		return *in.Active, true
	case in.IsActive != nil:
		return *in.IsActive, true
	}
	return def, true
}

// idParam lee un parámetro de ruta que debe ser un id numérico; si no, responde 404.
func idParam(c *gin.Context, name string) (string, bool) {
	id := c.Param(name)
	if !models.IsID(id) {
		problem(c, http.StatusNotFound, "Not found", "El registro no existe.")
		return "", false
	}
	return id, true
}

// mutation responde {id, message}.
func mutation(c *gin.Context, status int, id, message string) {
	c.JSON(status, gin.H{"id": id, "message": message})
}
