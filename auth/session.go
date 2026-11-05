package auth

import (
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/sessions"
)

// Store es el almacén de sesiones basado en cookies firmadas (mismo esquema
// que Avante). Se inicializa una sola vez con InitStore() en main.go.
var Store *sessions.CookieStore

// SessionName es el nombre de la cookie de sesión.
const SessionName = "hook_session"

// InitStore prepara el store de sesiones. La clave de firma sale de
// SESSION_SECRET — en producción SIEMPRE debe venir del entorno.
func InitStore() {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		secret = "cambia-esto-por-una-clave-larga-y-aleatoria-en-produccion"
	}

	Store = sessions.NewCookieStore([]byte(secret))
	Store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 días (lo mismo que duraba el refresh token)
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
