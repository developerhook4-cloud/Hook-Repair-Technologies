package auth

import (
	"crypto/rand"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/sessions"

	"hook-repair/env"
)

// Store es el almacén de sesiones basado en cookies firmadas (mismo esquema
// que Avante). Se inicializa una sola vez con InitStore() en main.go.
var Store *sessions.CookieStore

// SessionName es el nombre de la cookie de sesión.
const SessionName = "hook_session"

// minSecretLen — largo mínimo (en bytes) de SESSION_SECRET.
const minSecretLen = 32

// sessionMaxAge — duración de la sesión: 7 días (lo mismo que duraba el refresh token).
const sessionMaxAge = 86400 * 7

// InitStore prepara el store de sesiones. La clave de firma sale de
// SESSION_SECRET (mínimo 32 bytes). En producción es OBLIGATORIA: si falta o es
// corta la app no arranca. Fuera de producción, si falta, se genera una clave
// aleatoria (las sesiones se pierden al reiniciar). Nunca hay una clave fija.
func InitStore() {
	secret := os.Getenv("SESSION_SECRET")
	if len(secret) < minSecretLen {
		if env.IsProduction() {
			log.Fatal("SESSION_SECRET es obligatorio en producción y debe medir al menos 32 caracteres. Genera uno con: openssl rand -base64 48")
		}
		key := make([]byte, 48)
		if _, err := rand.Read(key); err != nil {
			log.Fatalf("No se pudo generar una clave de sesión aleatoria: %v", err)
		}
		secret = string(key)
		log.Println("ADVERTENCIA: SESSION_SECRET falta o mide menos de 32 caracteres; se usa una clave aleatoria temporal (las sesiones se pierden al reiniciar). Define SESSION_SECRET antes de desplegar.")
	}

	Store = sessions.NewCookieStore([]byte(secret))
	// El codec de securecookie también caduca a 7 días (por defecto aceptaba 30).
	Store.MaxAge(sessionMaxAge)
	Store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   sessionMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(),
	}
}

// Login guarda en la sesión el id del usuario y su versión de sesiones
// (users.session_version): si luego cambia en la base, la cookie deja de valer.
func Login(w http.ResponseWriter, r *http.Request, userID string, version int64) error {
	s, _ := Store.Get(r, SessionName)
	s.Values["user_id"] = userID
	s.Values["sv"] = version
	return s.Save(r, w)
}

// Logout borra la sesión (la cookie del navegador). Invalidar las demás
// cookies del usuario es aparte: models.InvalidateSessions.
func Logout(w http.ResponseWriter, r *http.Request) error {
	s, _ := Store.Get(r, SessionName)
	s.Values = map[any]any{}
	s.Options.MaxAge = -1
	return s.Save(r, w)
}

// UserID devuelve el id guardado en la sesión ("" si no hay sesión).
func UserID(r *http.Request) string {
	id, _ := Session(r)
	return id
}

// Session devuelve el id y la versión de sesiones guardados en la cookie ("" y 0
// si no hay sesión). Una cookie emitida antes de existir la versión no la trae:
// cuenta como 0, que es el valor inicial de la base.
func Session(r *http.Request) (userID string, version int64) {
	s, err := Store.Get(r, SessionName)
	if err != nil {
		return "", 0
	}
	userID, _ = s.Values["user_id"].(string)
	version, _ = s.Values["sv"].(int64)
	return userID, version
}

// cookieSecure — la cookie solo viaja por HTTPS: siempre en producción, y fuera
// de producción cuando COOKIE_SECURE=true.
func cookieSecure() bool {
	return env.IsProduction() || strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true")
}
