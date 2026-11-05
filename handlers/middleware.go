package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"hook-repair/auth"
	"hook-repair/models"
)

const ctxUser = "session_user"

// currentUser devuelve el usuario con sesión (o nil).
func currentUser(c *gin.Context) *models.SessionUser {
	if v, ok := c.Get(ctxUser); ok {
		if u, ok := v.(*models.SessionUser); ok {
			return u
		}
	}
	return nil
}

// SecurityHeaders — encabezados de seguridad.
// Todo (JS, CSS, fuentes, íconos) se sirve desde el propio servidor, así que
// la política puede quedarse en 'self'.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

// LoadUser lee la sesión y, si hay, carga al usuario (activo, con rol y
// organización activos) en el contexto. Si ya no es válido, borra la sesión.
func LoadUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if id := auth.UserID(c.Request); id != "" {
			if u, err := models.GetSessionUser(id); err == nil {
				c.Set(ctxUser, u)
			} else {
				_ = auth.Logout(c.Writer, c.Request)
			}
		}
		c.Next()
	}
}

// RequireAPIAuth — 401 si no hay sesión.
func RequireAPIAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUser(c) == nil {
			problem(c, http.StatusUnauthorized, "Unauthorized", "Se requiere una sesión válida.")
			return
		}
		c.Next()
	}
}

// RequirePermission — 403 si el rol no tiene el permiso.
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !currentUser(c).Can(permission) {
			problem(c, http.StatusForbidden, "Forbidden", "No tienes permiso para esta acción.")
			return
		}
		c.Next()
	}
}

// RequireTenant — el :orgId de la ruta debe ser la organización del usuario.
func RequireTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := currentUser(c)
		if u == nil {
			problem(c, http.StatusUnauthorized, "Unauthorized", "Se requiere una sesión válida.")
			return
		}
		if !strings.EqualFold(c.Param("orgId"), u.OrganizationID) {
			problem(c, http.StatusForbidden, "Forbidden", "El usuario no pertenece a esta organización.")
			return
		}
		c.Next()
	}
}

// RequirePage — manda al login si no hay sesión.
func RequirePage() gin.HandlerFunc {
	return func(c *gin.Context) {
		if currentUser(c) == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequirePagePermission — sin el permiso, de regreso al dashboard.
func RequirePagePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !currentUser(c).Can(permission) {
			c.Redirect(http.StatusFound, "/dashboard")
			c.Abort()
			return
		}
		c.Next()
	}
}
