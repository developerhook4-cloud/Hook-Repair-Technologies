package handlers

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// Pruebas unitarias (sin base de datos) de SecurityHeaders (HSTS), SameOrigin
// (CSRF) y el limitador de login.

func clearProdEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"RAILWAY_ENVIRONMENT_NAME", "RAILWAY_ENVIRONMENT", "APP_ENV"} {
		t.Setenv(k, "")
	}
}

func headersFor(t *testing.T) http.Header {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Header()
}

func TestHSTSSoloEnProduccion(t *testing.T) {
	clearProdEnv(t)
	if v := headersFor(t).Get("Strict-Transport-Security"); v != "" {
		t.Errorf("fuera de producción no debe haber HSTS, llegó %q", v)
	}
	t.Setenv("APP_ENV", "production")
	if v := headersFor(t).Get("Strict-Transport-Security"); v != "max-age=31536000; includeSubDomains" {
		t.Errorf("HSTS en producción = %q", v)
	}
	t.Setenv("APP_ENV", "")
	t.Setenv("RAILWAY_ENVIRONMENT_NAME", "production")
	if v := headersFor(t).Get("Strict-Transport-Security"); v == "" {
		t.Error("en Railway debe haber HSTS")
	}
}

func TestSameOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SameOrigin())
	ok := func(c *gin.Context) { c.String(http.StatusOK, "ok") }
	r.Any("/api/x", ok)
	r.Any("/logout", ok)
	r.Any("/otra", ok)

	cases := []struct {
		name    string
		method  string
		path    string
		remote  string
		headers map[string]string
		want    int
	}{
		{"POST api sin Origin", "POST", "/api/x", "", nil, 403},
		{"POST api Origin ajeno", "POST", "/api/x", "", map[string]string{"Origin": "https://evil.example"}, 403},
		{"POST api Origin null", "POST", "/api/x", "", map[string]string{"Origin": "null"}, 403},
		{"POST api Origin mismo host", "POST", "/api/x", "", map[string]string{"Origin": "http://example.com"}, 200},
		{"POST api Origin mismo host https", "POST", "/api/x", "", map[string]string{"Origin": "https://example.com"}, 200},
		{"POST api Origin otro puerto", "POST", "/api/x", "", map[string]string{"Origin": "http://example.com:8080"}, 403},
		{"POST api Referer mismo host", "POST", "/api/x", "", map[string]string{"Referer": "http://example.com/a/b"}, 200},
		{"POST api Referer ajeno", "POST", "/api/x", "", map[string]string{"Referer": "http://evil.example/a"}, 403},
		{"Origin manda sobre Referer", "POST", "/api/x", "", map[string]string{"Origin": "http://evil.example", "Referer": "http://example.com/"}, 403},
		{"PUT api sin Origin", "PUT", "/api/x", "", nil, 403},
		{"PATCH api sin Origin", "PATCH", "/api/x", "", nil, 403},
		{"DELETE api sin Origin", "DELETE", "/api/x", "", nil, 403},
		{"GET api sin Origin", "GET", "/api/x", "", nil, 200},
		{"POST /logout sin Origin", "POST", "/logout", "", nil, 403},
		{"POST /logout Origin correcto", "POST", "/logout", "", map[string]string{"Origin": "http://example.com"}, 200},
		{"GET /logout sin Origin", "GET", "/logout", "", nil, 200},
		{"POST fuera de /api sin Origin", "POST", "/otra", "", nil, 200},
		{"X-Forwarded-Host de cliente directo no se cree", "POST", "/api/x", "203.0.113.9:1234",
			map[string]string{"Origin": "https://app.example.org", "X-Forwarded-Host": "app.example.org"}, 403},
		{"X-Forwarded-Host de proxy de confianza", "POST", "/api/x", "10.1.2.3:1234",
			map[string]string{"Origin": "https://app.example.org", "X-Forwarded-Host": "app.example.org"}, 200},
		{"X-Forwarded-Host de proxy, Origin ajeno", "POST", "/api/x", "10.1.2.3:1234",
			map[string]string{"Origin": "https://evil.example", "X-Forwarded-Host": "app.example.org"}, 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil) // Host: example.com
			if tc.remote != "" {
				req.RemoteAddr = tc.remote
			}
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%s %s: %d, quiero %d (%s)", tc.method, tc.path, rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == 403 && !strings.Contains(rec.Body.String(), "Solicitud rechazada (origen no válido).") {
				t.Errorf("mensaje inesperado: %s", rec.Body.String())
			}
		})
	}
}

func resetLimiter() {
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	loginLimiter.byIP = map[string][]time.Time{}
	loginLimiter.byEmail = map[string][]time.Time{}
	loginLimiter.lastSweep = time.Time{}
}

func TestLimitadorContadoresIndependientes(t *testing.T) {
	resetLimiter()
	// 4 fallos del email + login correcto + 1 fallo: el correcto limpia SOLO el email.
	for i := 0; i < 4; i++ {
		loginFailed("1.1.1.1", "a@x.com")
	}
	if loginBlocked("1.1.1.1", "a@x.com") {
		t.Fatal("4 fallos no bloquean")
	}
	loginSucceeded("a@x.com")
	loginFailed("1.1.1.1", "a@x.com")
	if loginBlocked("1.1.1.1", "a@x.com") {
		t.Fatal("el login correcto debía limpiar el contador del email")
	}
	// La IP sigue acumulando: 5 fallos en total hasta ahora, no se reinició.
	loginLimiter.Lock()
	n := len(loginLimiter.byIP["1.1.1.1"])
	loginLimiter.Unlock()
	if n != 5 {
		t.Fatalf("fallos de la IP = %d, quiero 5 (el login correcto no la limpia)", n)
	}
	// 5 fallos del mismo email bloquean ese email, no otro desde la misma IP.
	loginFailed("1.1.1.1", "a@x.com")
	loginFailed("1.1.1.1", "a@x.com")
	loginFailed("1.1.1.1", "a@x.com")
	loginFailed("1.1.1.1", "a@x.com")
	if !loginBlocked("1.1.1.1", "a@x.com") {
		t.Fatal("5 fallos del email debían bloquearlo")
	}
	if loginBlocked("1.1.1.1", "b@x.com") {
		t.Fatal("otro email no debe quedar bloqueado")
	}
	// La IP se bloquea a los 20 fallos.
	resetLimiter()
	for i := 0; i < loginMaxFailsIP-1; i++ {
		loginFailed("2.2.2.2", "e"+strconv.Itoa(i)+"@x.com")
	}
	if loginBlocked("2.2.2.2", "nuevo@x.com") {
		t.Fatal("19 fallos de IP no bloquean")
	}
	loginFailed("2.2.2.2", "otro@x.com")
	if !loginBlocked("2.2.2.2", "nuevo@x.com") {
		t.Fatal("20 fallos de IP debían bloquear")
	}
	if loginBlocked("3.3.3.3", "nuevo@x.com") {
		t.Fatal("otra IP no debe quedar bloqueada")
	}
}

func TestLimitadorBarreVencidosYTieneTope(t *testing.T) {
	resetLimiter()
	old := time.Now().Add(-loginWindow - time.Minute)
	loginLimiter.Lock()
	loginLimiter.byIP["viejo"] = []time.Time{old}
	loginLimiter.byEmail["viejo@x.com"] = []time.Time{old}
	loginLimiter.Unlock()

	loginBlocked("9.9.9.9", "z@x.com") // dispara el barrido
	loginLimiter.Lock()
	if len(loginLimiter.byIP) != 0 || len(loginLimiter.byEmail) != 0 {
		t.Errorf("el barrido debía quitar lo vencido: ip=%d email=%d", len(loginLimiter.byIP), len(loginLimiter.byEmail))
	}
	loginLimiter.Unlock()

	// Tope: con el mapa lleno no se siguen claves nuevas.
	resetLimiter()
	loginLimiter.Lock()
	loginLimiter.lastSweep = time.Now() // sin barrido durante la prueba
	for i := 0; i < loginMaxKeys; i++ {
		loginLimiter.byEmail["k"+strconv.Itoa(i)] = []time.Time{time.Now()}
	}
	loginLimiter.Unlock()
	loginFailed("8.8.8.8", "nueva@x.com")
	loginLimiter.Lock()
	defer loginLimiter.Unlock()
	if len(loginLimiter.byEmail) != loginMaxKeys {
		t.Errorf("el mapa de emails creció pasando el tope: %d", len(loginLimiter.byEmail))
	}
	if _, ok := loginLimiter.byEmail["k0"]; !ok {
		t.Error("las claves existentes no deben perderse")
	}
}
