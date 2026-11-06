package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"hook-repair/models"
)

// ── /api/organizations (vista de plataforma: todas las organizaciones) ──

// ListOrganizations — GET /api/organizations
func ListOrganizations(c *gin.Context) {
	orgs, err := models.GetAllOrganizations()
	if err != nil {
		respondError(c, err, "organization")
		return
	}
	c.JSON(http.StatusOK, orgs)
}

// GetOrganization — GET /api/organizations/:id
func GetOrganization(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	o, err := models.GetOrganization(id)
	if err != nil {
		respondError(c, err, "organization")
		return
	}
	c.JSON(http.StatusOK, o)
}

// CreateOrganization — POST /api/organizations (crea también sus roles por defecto).
func CreateOrganization(c *gin.Context) {
	var in models.SaveOrganizationRequest
	if !bindJSON(c, &in) {
		return
	}
	o, err := models.CreateOrganization(in)
	if err != nil {
		respondError(c, err, "organization")
		return
	}
	c.JSON(http.StatusCreated, o)
}

// UpdateOrganization — PUT /api/organizations/:id
func UpdateOrganization(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.SaveOrganizationRequest
	if !bindJSON(c, &in) {
		return
	}
	o, err := models.UpdateOrganization(id, in)
	if err != nil {
		respondError(c, err, "organization")
		return
	}
	c.JSON(http.StatusOK, o)
}

// UpdateOrganizationStatus — PATCH /api/organizations/:id/status
func UpdateOrganizationStatus(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	active, ok := statusFromRequest(c, false)
	if !ok {
		return
	}
	o, err := models.SetOrganizationStatus(id, active)
	if err != nil {
		respondError(c, err, "organization")
		return
	}
	c.JSON(http.StatusOK, o)
}

// ListOrganizationUsers — GET /api/organizations/:id/users
func ListOrganizationUsers(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if _, err := models.GetOrganization(id); err != nil {
		respondError(c, err, "organization")
		return
	}
	users, err := models.GetUsers(id)
	if err != nil {
		respondError(c, err, "user")
		return
	}
	c.JSON(http.StatusOK, users)
}

// CreateOrganizationUser — POST /api/organizations/:id/users
// Si no trae rol se le asigna "Administrador" de esa organización.
func CreateOrganizationUser(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if _, err := models.GetOrganization(id); err != nil {
		respondError(c, err, "organization")
		return
	}
	var in models.SaveUserRequest
	if !bindJSON(c, &in) {
		return
	}
	if _, err := models.CreateUser(id, in); err != nil {
		respondError(c, err, "user")
		return
	}
	c.Status(http.StatusNoContent)
}

// UpdateOrganizationUser — PUT /api/organizations/:id/users/:userId
func UpdateOrganizationUser(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	userID, ok := idParam(c, "userId")
	if !ok {
		return
	}
	var in models.SaveUserRequest
	if !bindJSON(c, &in) {
		return
	}
	u, err := models.UpdateUser(id, userID, in)
	if err != nil {
		respondError(c, err, "user")
		return
	}
	if in.Active != nil && *in.Active != u.Active {
		if !*in.Active && userID == currentUser(c).ID {
			problem(c, http.StatusBadRequest, "Invalid user", "No puedes desactivar tu propio usuario.")
			return
		}
		if err := models.SetUserStatus(id, userID, *in.Active); err != nil {
			respondError(c, err, "user")
			return
		}
	}
	c.Status(http.StatusNoContent)
}

// UpdateOrganizationUserStatus — PATCH /api/organizations/:id/users/:userId/status
func UpdateOrganizationUserStatus(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	userID, ok := idParam(c, "userId")
	if !ok {
		return
	}
	active, ok := statusFromRequest(c, false)
	if !ok {
		return
	}
	if err := models.SetUserStatus(id, userID, active); err != nil {
		respondError(c, err, "user")
		return
	}
	c.Status(http.StatusNoContent)
}

// ── /api/organizations/mine (la organización del usuario con sesión) ──

// GetMyOrganization — GET /api/organizations/mine
func GetMyOrganization(c *gin.Context) {
	o, err := models.GetOrganization(currentUser(c).OrganizationID)
	if err != nil {
		respondError(c, err, "organization")
		return
	}
	c.JSON(http.StatusOK, o)
}

// UpdateMyOrganization — PUT /api/organizations/mine
func UpdateMyOrganization(c *gin.Context) {
	var in models.SaveOrganizationRequest
	if !bindJSON(c, &in) {
		return
	}
	o, err := models.UpdateOrganization(currentUser(c).OrganizationID, in)
	if err != nil {
		respondError(c, err, "organization")
		return
	}
	c.JSON(http.StatusOK, o)
}
