package handlers

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"hook-repair/auth"
	"hook-repair/env"
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
		// HSTS solo en producción (HTTPS garantizado); en local rompería http://localhost.
		if env.IsProduction() {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// TrustedProxies — redes de los proxies de confianza (Railway y cualquier proxy
// interno). main.go las pasa a gin.SetTrustedProxies; aquí se usan para decidir
// si se cree en X-Forwarded-Host.
var TrustedProxies = []string{"10.0.0.0/8", "100.64.0.0/10", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.1"}

var trustedNets = sync.OnceValue(func() []*net.IPNet {
	var nets []*net.IPNet
	for _, s := range TrustedProxies {
		if !strings.Contains(s, "/") {
			s += "/32"
		}
		if _, n, err := net.ParseCIDR(s); err == nil {
			nets = append(nets, n)
		}
	}
	return nets
})

// fromTrustedProxy dice si la conexión directa viene de un proxy de confianza.
func fromTrustedProxy(c *gin.Context) bool {
	ip := net.ParseIP(c.RemoteIP())
	if ip == nil {
		return false
	}
	for _, n := range trustedNets() {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// hostOf saca el host (con puerto) de un Origin o Referer; "" si no es una URL válida.
func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}

// SameOrigin — defensa contra CSRF. En las peticiones que cambian estado
// (POST, PUT, PATCH, DELETE) a /api/* y POST /logout exige que el encabezado
// Origin (o, si falta, Referer) sea del mismo host que la petición; los
// navegadores siempre mandan Origin en esos métodos. Sin ninguno de los dos, o
// de otro host, responde 403. El host esperado es el de la petición
// (c.Request.Host) y, si la conexión viene de un proxy de confianza, también
// X-Forwarded-Host. Las peticiones de lectura (GET/HEAD) no se tocan.
func SameOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			c.Next()
			return
		}
		if p := c.Request.URL.Path; !strings.HasPrefix(p, "/api/") && p != "/logout" {
			c.Next()
			return
		}
		got := ""
		if origin := c.GetHeader("Origin"); origin != "" {
			got = hostOf(origin) // "null" u otro valor raro no da host: se rechaza
		} else if ref := c.GetHeader("Referer"); ref != "" {
			got = hostOf(ref)
		}
		if got != "" {
			if strings.EqualFold(got, c.Request.Host) {
				c.Next()
				return
			}
			if fh := c.GetHeader("X-Forwarded-Host"); fh != "" && fromTrustedProxy(c) {
				// Puede traer varios valores separados por coma: vale el primero.
				if strings.EqualFold(got, strings.TrimSpace(strings.Split(fh, ",")[0])) {
					c.Next()
					return
				}
			}
		}
		problem(c, http.StatusForbidden, "Forbidden", "Solicitud rechazada (origen no válido).")
	}
}

// LoadUser lee la sesión y, si hay, carga al usuario (activo, con rol y
// organización activos) en el contexto. Si ya no es válido, o la versión de
// sesiones de la cookie ya no es la de la base (logout, cambio de contraseña,
// desactivación: KAN-143), borra la sesión.
func LoadUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if id, version := auth.Session(c.Request); id != "" {
			if u, err := models.GetSessionUser(id); err == nil && u.SessionVersion == version {
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

// RequirePlatform — 403 si el usuario no es de la organización de plataforma
// (administración de todas las organizaciones).
func RequirePlatform() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := currentUser(c); u == nil || !u.IsPlatform {
			problem(c, http.StatusForbidden, "Forbidden", "Esta sección es solo para la administración de la plataforma.")
			return
		}
		c.Next()
	}
}

// RequireOrgAdmin — 403 si el usuario no es administrador de su organización
// (puede editar usuarios y roles).
func RequireOrgAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !currentUser(c).IsAdmin() {
			problem(c, http.StatusForbidden, "Forbidden", "Solo los administradores de la organización pueden hacer esto.")
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

// RequirePagePlatform — si no es de la organización de plataforma, de regreso
// al dashboard.
func RequirePagePlatform() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := currentUser(c); u == nil || !u.IsPlatform {
			c.Redirect(http.StatusFound, "/dashboard")
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
