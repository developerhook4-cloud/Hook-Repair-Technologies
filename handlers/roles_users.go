package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"hook-repair/models"
)

func orgOf(c *gin.Context) string { return currentUser(c).OrganizationID }

// ── Roles: /api/:orgId/roles ───────────────────────────────────────────

// ListRoles — GET (responde {roles: [...]} igual que RolesResponse).
func ListRoles(c *gin.Context) {
	roles, err := models.GetRoles(orgOf(c))
	if err != nil {
		respondError(c, err, "role")
		return
	}
	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

// GetRole — GET /:roleId
func GetRole(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	r, err := models.GetRole(orgOf(c), id)
	if err != nil {
		respondError(c, err, "role")
		return
	}
	c.JSON(http.StatusOK, r)
}

// CreateRole — POST
func CreateRole(c *gin.Context) {
	var in models.SaveRoleRequest
	if !bindJSON(c, &in) {
		return
	}
	r, err := models.CreateRole(currentUser(c), orgOf(c), in)
	if err != nil {
		respondError(c, err, "role")
		return
	}
	mutation(c, http.StatusCreated, r.ID, "Rol creado correctamente.")
}

// UpdateRole — PUT /:roleId
func UpdateRole(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.SaveRoleRequest
	if !bindJSON(c, &in) {
		return
	}
	r, err := models.UpdateRole(currentUser(c), orgOf(c), id, in)
	if err != nil {
		respondError(c, err, "role")
		return
	}
	mutation(c, http.StatusOK, r.ID, "Rol actualizado correctamente.")
}

// UpdateRoleStatus — PATCH /:roleId/status
func UpdateRoleStatus(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	active, ok := statusFromRequest(c, false)
	if !ok {
		return
	}
	if err := models.SetRoleStatus(currentUser(c), orgOf(c), id, active); err != nil {
		respondError(c, err, "role")
		return
	}
	mutation(c, http.StatusOK, id, "Estado del rol actualizado correctamente.")
}

// ListPermissions — GET /permissions
func ListPermissions(c *gin.Context) {
	// Organizations.* solo existe en la organización de plataforma.
	if !currentUser(c).IsPlatform {
		c.JSON(http.StatusOK, models.WithoutPlatformPermissions(models.AllPermissions))
		return
	}
	c.JSON(http.StatusOK, models.AllPermissions)
}

// ── Usuarios (empleados): /api/:orgId/users ────────────────────────────

// ListUsers — GET
func ListUsers(c *gin.Context) {
	users, err := models.GetUsers(orgOf(c))
	if err != nil {
		respondError(c, err, "user")
		return
	}
	c.JSON(http.StatusOK, users)
}

// GetUser — GET /:id
func GetUser(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	u, err := models.GetUser(orgOf(c), id)
	if err != nil {
		respondError(c, err, "user")
		return
	}
	c.JSON(http.StatusOK, u)
}

// CreateUser — POST
func CreateUser(c *gin.Context) {
	var in models.SaveUserRequest
	if !bindJSON(c, &in) {
		return
	}
	u, err := models.CreateUser(currentUser(c), orgOf(c), in)
	if err != nil {
		respondError(c, err, "user")
		return
	}
	c.JSON(http.StatusCreated, u)
}

// UpdateUser — PUT /:id (si trae "active" también cambia el estado).
func UpdateUser(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.SaveUserRequest
	if !bindJSON(c, &in) {
		return
	}
	u, err := models.UpdateUser(currentUser(c), orgOf(c), id, in)
	if err != nil {
		respondError(c, err, "user")
		return
	}
	if in.Active != nil && *in.Active != u.Active {
		if err := models.SetUserStatus(currentUser(c), orgOf(c), id, *in.Active); err != nil {
			respondError(c, err, "user")
			return
		}
		u, _ = models.GetUser(orgOf(c), id)
	}
	c.JSON(http.StatusOK, u)
}

// UpdateUserStatus — PATCH /:id/status
func UpdateUserStatus(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	active, ok := statusFromRequest(c, false)
	if !ok {
		return
	}
	if id == currentUser(c).ID && !active {
		problem(c, http.StatusBadRequest, "Invalid user", "No puedes desactivar tu propio usuario.")
		return
	}
	if err := models.SetUserStatus(currentUser(c), orgOf(c), id, active); err != nil {
		respondError(c, err, "user")
		return
	}
	c.Status(http.StatusNoContent)
}

// UpdateProfile — PUT /api/users/me
func UpdateProfile(c *gin.Context) {
	var in models.UpdateProfileRequest
	if !bindJSON(c, &in) {
		return
	}
	if err := models.UpdateProfile(currentUser(c).ID, in); err != nil {
		if ve, ok := err.(*models.ValidationError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"message": ve.Msg})
			return
		}
		respondError(c, err, "profile")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Perfil actualizado exitosamente."})
}
