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
		Secure:   strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true"),
	}
}

// Login guarda el id del usuario en la sesión.
func Login(w http.ResponseWriter, r *http.Request, userID string) error {
	s, _ := Store.Get(r, SessionName)
	s.Values["user_id"] = userID
	return s.Save(r, w)
}

// Logout borra la sesión.
func Logout(w http.ResponseWriter, r *http.Request) error {
	s, _ := Store.Get(r, SessionName)
	s.Values = map[any]any{}
	s.Options.MaxAge = -1
	return s.Save(r, w)
}

// UserID devuelve el id guardado en la sesión ("" si no hay sesión).
func UserID(r *http.Request) string {
	s, err := Store.Get(r, SessionName)
	if err != nil {
		return ""
	}
	id, _ := s.Values["user_id"].(string)
	return id
}
