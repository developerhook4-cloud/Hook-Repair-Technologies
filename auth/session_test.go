package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/gorilla/sessions"
)

// clearEnv deja el entorno como una máquina local sin configuración.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"SESSION_SECRET", "RAILWAY_ENVIRONMENT_NAME", "RAILWAY_ENVIRONMENT", "APP_ENV"} {
		t.Setenv(k, "")
	}
}

// Sin SESSION_SECRET (fuera de producción) NO se puede volver a usar la clave
// fija que estaba en el código: una cookie firmada con ella debe ser rechazada.
func TestInitStoreSinSecretNoUsaClaveFija(t *testing.T) {
	clearEnv(t)
	old := sessions.NewCookieStore([]byte("cambia-esto-por-una-clave-larga-y-aleatoria-en-produccion"))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	s, _ := old.Get(req, SessionName)
	s.Values["user_id"] = "1"
	if err := s.Save(req, rec); err != nil {
		t.Fatal(err)
	}

	InitStore()
	forged := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		forged.AddCookie(c)
	}
	if id := UserID(forged); id != "" {
		t.Fatalf("la cookie firmada con la clave fija fue aceptada (user_id=%q)", id)
	}
}

// Una sesión creada con la clave aleatoria sí funciona de ida y vuelta.
func TestInitStoreSinSecretFuncionaYCaducaA7Dias(t *testing.T) {
	clearEnv(t)
	InitStore()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := Login(rec, req, "42"); err != nil {
		t.Fatal(err)
	}
	back := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		back.AddCookie(c)
		if c.MaxAge != 86400*7 {
			t.Errorf("MaxAge de la cookie = %d, quiero %d", c.MaxAge, 86400*7)
		}
	}
	if id := UserID(back); id != "42" {
		t.Fatalf("user_id = %q, quiero 42", id)
	}
}

// Cada arranque sin SESSION_SECRET genera una clave distinta.
func TestInitStoreClaveAleatoriaDistintaCadaVez(t *testing.T) {
	clearEnv(t)
	login := func() *http.Cookie {
		InitStore()
		rec := httptest.NewRecorder()
		if err := Login(rec, httptest.NewRequest(http.MethodGet, "/", nil), "1"); err != nil {
			t.Fatal(err)
		}
		return rec.Result().Cookies()[0]
	}
	c := login()
	InitStore() // otra clave
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(c)
	if UserID(req) != "" {
		t.Fatal("dos InitStore seguidos generaron la misma clave")
	}
}

// En producción, un SESSION_SECRET ausente o corto detiene el arranque.
func TestInitStoreProduccionSinSecretTermina(t *testing.T) {
	if os.Getenv("HOOK_TEST_FATAL") == "1" {
		InitStore()
		return
	}
	for _, secret := range []string{"", "muy-corto"} {
		cmd := exec.Command(os.Args[0], "-test.run=TestInitStoreProduccionSinSecretTermina")
		cmd.Env = append(os.Environ(), "HOOK_TEST_FATAL=1", "APP_ENV=production", "SESSION_SECRET="+secret)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("SESSION_SECRET=%q en producción debía terminar con error; salida: %s", secret, out)
		}
	}
}

// Con un secret válido en producción arranca normal.
func TestInitStoreProduccionConSecretValido(t *testing.T) {
	clearEnv(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("SESSION_SECRET", "0123456789abcdef0123456789abcdef")
	InitStore()
	if Store == nil {
		t.Fatal("Store nulo")
	}
}
