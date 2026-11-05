package handlers

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"hook-repair/auth"
	"hook-repair/models"
)

// ── Límite de intentos de login ────────────────────────────────────────
// 5 intentos cada 15 minutos por IP; solo cuentan los intentos fallidos: un login correcto reinicia
// el contador.

const (
	loginMaxFails = 5
	loginWindow   = 15 * time.Minute
)

var loginLimiter = struct {
	sync.Mutex
	fails map[string][]time.Time
}{fails: map[string][]time.Time{}}

func loginBlocked(ip string) bool {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	recent := loginLimiter.fails[ip][:0]
	for _, t := range loginLimiter.fails[ip] {
		if time.Since(t) < loginWindow {
			recent = append(recent, t)
		}
	}
	loginLimiter.fails[ip] = recent
	return len(recent) >= loginMaxFails
}

func loginFailed(ip string) {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	loginLimiter.fails[ip] = append(loginLimiter.fails[ip], time.Now())
}

func loginSucceeded(ip string) {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	delete(loginLimiter.fails, ip)
}

// sessionResponse — {user: ...}. La sesión vive en una cookie HttpOnly.
func sessionResponse(u *models.SessionUser) gin.H {
	return gin.H{"user": u}
}

// Login — POST /api/auth/login {email, password}.
func Login(c *gin.Context) {
	ip := c.ClientIP()
	if loginBlocked(ip) {
		problem(c, http.StatusTooManyRequests, "Too many requests", "Demasiados intentos fallidos. Espera unos minutos e intenta de nuevo.")
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &in) {
		return
	}
	u, err := models.Authenticate(in.Email, in.Password)
	if err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			respondError(c, err, "login")
			return
		}
		loginFailed(ip)
		problem(c, http.StatusUnauthorized, "Invalid credentials", "El email o la contraseña no son válidos.")
		return
	}
	loginSucceeded(ip)
	if err := auth.Login(c.Writer, c.Request, u.ID); err != nil {
		respondError(c, err, "login")
		return
	}
	c.JSON(http.StatusOK, sessionResponse(u))
}

// Me — GET /api/auth/me (y POST /api/auth/refresh-token por compatibilidad):
// devuelve la sesión actual o 401.
func Me(c *gin.Context) {
	u := currentUser(c)
	if u == nil {
		problem(c, http.StatusUnauthorized, "Unauthorized", "Se requiere una sesión válida.")
		return
	}
	c.JSON(http.StatusOK, sessionResponse(u))
}

// Logout — POST /api/auth/logout.
func Logout(c *gin.Context) {
	_ = auth.Logout(c.Writer, c.Request)
	c.JSON(http.StatusOK, gin.H{"mensaje": "Sesión cerrada correctamente."})
}

// LogoutPage — GET /logout (enlace del menú de usuario).
func LogoutPage(c *gin.Context) {
	_ = auth.Logout(c.Writer, c.Request)
	c.Redirect(http.StatusFound, "/login")
}
