package handlers

import (
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"hook-repair/auth"
	"hook-repair/models"
)

// ── Límite de intentos de login ────────────────────────────────────────
// Solo cuentan los intentos fallidos, en dos contadores independientes:
//   - por email normalizado: 5 fallos cada 15 minutos (frena el adivinar la
//     contraseña de una cuenta, desde una o varias IP);
//   - por IP: 20 fallos cada 15 minutos (más holgado para no bloquear a una
//     oficina entera que comparte salida a internet).
//
// Un login correcto solo limpia el contador de SU email, nunca el de la IP: si
// no, alternar fallos con un login propio daría intentos ilimitados.
// El bloqueo por email responde igual exista o no la cuenta (no revela cuáles
// correos están registrados). Los contadores viven en memoria (se pierden al
// reiniciar) y las entradas vencidas se barren para que los mapas no crezcan.

const (
	loginMaxFailsEmail = 5
	loginMaxFailsIP    = 20
	loginWindow        = 15 * time.Minute
	// loginMaxKeys — tope de claves por mapa; lleno (tras barrer lo vencido) ya
	// no se siguen claves nuevas, así un barrido de correos inventados no agota
	// la memoria.
	loginMaxKeys = 10000
	// loginSweepEvery — cada cuánto se barren las entradas vencidas.
	loginSweepEvery = time.Minute
)

var loginLimiter = struct {
	sync.Mutex
	byIP, byEmail map[string][]time.Time
	lastSweep     time.Time
}{byIP: map[string][]time.Time{}, byEmail: map[string][]time.Time{}}

func normalizeLoginEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// recentFails deja en m[key] solo los fallos dentro de la ventana (y borra la
// clave si quedó vacía) y devuelve cuántos son. Pide el candado tomado.
func recentFails(m map[string][]time.Time, key string, now time.Time) int {
	old, ok := m[key]
	if !ok {
		return 0
	}
	recent := old[:0]
	for _, t := range old {
		if now.Sub(t) < loginWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) == 0 {
		delete(m, key)
		return 0
	}
	m[key] = recent
	return len(recent)
}

// sweepLogin barre todas las claves vencidas, a lo más una vez por minuto.
// Pide el candado tomado.
func sweepLogin(now time.Time) {
	if now.Sub(loginLimiter.lastSweep) < loginSweepEvery {
		return
	}
	loginLimiter.lastSweep = now
	for _, m := range []map[string][]time.Time{loginLimiter.byIP, loginLimiter.byEmail} {
		for k := range m {
			recentFails(m, k, now)
		}
	}
}

// loginBlocked — ¿esta IP o este email ya llegaron a su tope de fallos?
func loginBlocked(ip, email string) bool {
	now := time.Now()
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	sweepLogin(now)
	if recentFails(loginLimiter.byIP, ip, now) >= loginMaxFailsIP {
		return true
	}
	return email != "" && recentFails(loginLimiter.byEmail, email, now) >= loginMaxFailsEmail
}

// loginFailed anota un fallo para la IP y para el email (si hay).
func loginFailed(ip, email string) {
	now := time.Now()
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	sweepLogin(now)
	addFail(loginLimiter.byIP, ip, now)
	if email != "" {
		addFail(loginLimiter.byEmail, email, now)
	}
}

func addFail(m map[string][]time.Time, key string, now time.Time) {
	if _, known := m[key]; !known && len(m) >= loginMaxKeys {
		return
	}
	m[key] = append(m[key], now)
}

// loginSucceeded limpia solo el contador del email que entró bien.
func loginSucceeded(email string) {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	delete(loginLimiter.byEmail, email)
}

// sessionResponse — {user: ...}. La sesión vive en una cookie HttpOnly.
func sessionResponse(u *models.SessionUser) gin.H {
	return gin.H{"user": u}
}

// Login — POST /api/auth/login {email, password}.
func Login(c *gin.Context) {
	ip := c.ClientIP()
	if loginBlocked(ip, "") {
		tooManyLogins(c)
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !bindJSON(c, &in) {
		return
	}
	email := normalizeLoginEmail(in.Email)
	// Mismo 429 exista o no el email: el bloqueo no revela qué cuentas existen.
	if loginBlocked(ip, email) {
		tooManyLogins(c)
		return
	}
	u, err := models.Authenticate(in.Email, in.Password)
	if err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			respondError(c, err, "login")
			return
		}
		loginFailed(ip, email)
		problem(c, http.StatusUnauthorized, "Invalid credentials", "El email o la contraseña no son válidos.")
		return
	}
	loginSucceeded(email)
	if err := auth.Login(c.Writer, c.Request, u.ID, u.SessionVersion); err != nil {
		respondError(c, err, "login")
		return
	}
	c.JSON(http.StatusOK, sessionResponse(u))
}

func tooManyLogins(c *gin.Context) {
	problem(c, http.StatusTooManyRequests, "Too many requests", "Demasiados intentos fallidos. Espera unos minutos e intenta de nuevo.")
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

// endSession cierra la sesión: borra la cookie del navegador y sube la versión
// de sesiones del usuario, así que también dejan de valer sus cookies en
// cualquier otro dispositivo (KAN-143). La cookie se borra aunque falle la base.
func endSession(c *gin.Context) error {
	var err error
	if u := currentUser(c); u != nil {
		err = models.InvalidateSessions(u.ID)
	}
	_ = auth.Logout(c.Writer, c.Request)
	return err
}

// keepSession vuelve a guardar la sesión actual con la versión de sesiones
// vigente. Se usa cuando el usuario cambia su PROPIA contraseña: sube la versión
// (las demás sesiones caen) pero esta no debe caerse.
func keepSession(c *gin.Context, userID string) {
	if u := currentUser(c); u == nil || u.ID != userID {
		return
	}
	if v, err := models.SessionVersion(userID); err == nil {
		_ = auth.Login(c.Writer, c.Request, userID, v)
	}
}

// Logout — POST /api/auth/logout.
func Logout(c *gin.Context) {
	if err := endSession(c); err != nil {
		respondError(c, err, "logout")
		return
	}
	c.JSON(http.StatusOK, gin.H{"mensaje": "Sesión cerrada correctamente."})
}

// LogoutPage — POST /logout (formulario del menú de usuario, sin JavaScript):
// cierra la sesión y manda al login.
func LogoutPage(c *gin.Context) {
	if err := endSession(c); err != nil {
		respondError(c, err, "logout")
		return
	}
	c.Redirect(http.StatusSeeOther, "/login")
}

// LogoutRedirect — GET /logout. Ya NO cierra la sesión: un GET que cambia estado
// se puede disparar desde otro sitio (CSRF, p. ej. una imagen). Solo manda a
// /login (que, con sesión abierta, lleva al dashboard). Para salir se usa POST.
func LogoutRedirect(c *gin.Context) {
	c.Redirect(http.StatusFound, "/login")
}
