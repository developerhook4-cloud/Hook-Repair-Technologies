package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hook-repair/auth"
	"hook-repair/models"
)

// Pruebas de integración del Bloque 2: invalidación de sesiones (KAN-143),
// límite de intentos de login (KAN-129) y CSRF por Origin (KAN-138).

// meStatus — GET /api/auth/me con esa cookie.
func (a *testApp) meStatus(cookie *http.Cookie) int {
	return a.do(cookie, "GET", "/api/auth/me", nil).Code
}

// loginRaw intenta un login desde una IP concreta (el limitador cuenta por IP) y
// devuelve la respuesta completa.
func (a *testApp) loginRaw(ip, email, password string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	req.RemoteAddr = ip + ":40000"
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

// sessionCookie saca la cookie de sesión de una respuesta (nil si no la puso o
// si solo la borra).
func sessionCookie(r resp) *http.Cookie {
	for _, c := range (&http.Response{Header: r.Hdr}).Cookies() {
		if c.Name == auth.SessionName && c.MaxAge >= 0 && c.Value != "" {
			return c
		}
	}
	return nil
}

// basico crea un rol mínimo y un usuario con él; devuelve id, email y cookie.
func (s *shop) basico(t *testing.T) (id, email string, cookie *http.Cookie) {
	t.Helper()
	return s.newUser(t, s.newRole(t, "Basico", models.ViewCustomers))
}

// ── KAN-143: invalidar sesiones ────────────────────────────────────────

func TestLogoutInvalidaTodasLasCookies(t *testing.T) {
	s := newShop(t)
	a := s.app
	_, email, c1 := s.basico(t)
	c2 := a.login(t, email, testPassword) // segundo dispositivo
	expect(t, "cookie 1 antes", resp{Code: a.meStatus(c1)}, http.StatusOK)
	expect(t, "cookie 2 antes", resp{Code: a.meStatus(c2)}, http.StatusOK)

	expect(t, "logout", a.do(c1, "POST", "/api/auth/logout", nil), http.StatusOK)

	expect(t, "cookie vieja tras logout", resp{Code: a.meStatus(c1)}, http.StatusUnauthorized)
	expect(t, "cookie de otro dispositivo tras logout", resp{Code: a.meStatus(c2)}, http.StatusUnauthorized)
	// Iniciar sesión de nuevo funciona.
	expect(t, "login nuevo", resp{Code: a.meStatus(a.login(t, email, testPassword))}, http.StatusOK)
}

func TestLogoutPagina(t *testing.T) {
	s := newShop(t)
	a := s.app
	_, _, c := s.basico(t)

	// GET /logout ya no cierra la sesión (CSRF): solo redirige.
	expect(t, "GET /logout", a.do(c, "GET", "/logout", nil), http.StatusFound)
	expect(t, "sesión intacta tras GET /logout", resp{Code: a.meStatus(c)}, http.StatusOK)

	// POST /logout sin Origin: rechazado.
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	expect(t, "POST /logout sin Origin", resp{Code: rec.Code}, http.StatusForbidden)
	expect(t, "sesión intacta tras POST sin Origin", resp{Code: a.meStatus(c)}, http.StatusOK)

	// POST /logout con Origin: cierra y manda al login.
	r := a.do(c, "POST", "/logout", nil)
	expect(t, "POST /logout", r, http.StatusSeeOther)
	if loc := r.Hdr.Get("Location"); loc != "/login" {
		t.Errorf("POST /logout redirige a %q, quiero /login", loc)
	}
	expect(t, "cookie tras POST /logout", resp{Code: a.meStatus(c)}, http.StatusUnauthorized)
}

func TestAdminCambiaContrasenaInvalidaSesionDelEmpleado(t *testing.T) {
	s := newShop(t)
	a := s.app
	id, email, old := s.basico(t)
	expect(t, "empleado con sesión", resp{Code: a.meStatus(old)}, http.StatusOK)

	expect(t, "admin cambia contraseña",
		a.do(s.Admin, "PUT", s.path("/users/"+id), map[string]any{"name": "Empleado", "email": email, "password": "Nueva-Clave-987"}),
		http.StatusOK)

	expect(t, "cookie vieja del empleado", resp{Code: a.meStatus(old)}, http.StatusUnauthorized)
	expect(t, "el admin sigue con sesión", resp{Code: a.meStatus(s.Admin)}, http.StatusOK)
	expect(t, "login con la contraseña nueva", resp{Code: a.meStatus(a.login(t, email, "Nueva-Clave-987"))}, http.StatusOK)
}

func TestEditarSinContrasenaNoInvalidaSesion(t *testing.T) {
	s := newShop(t)
	a := s.app
	id, email, c := s.basico(t)
	expect(t, "admin edita nombre",
		a.do(s.Admin, "PUT", s.path("/users/"+id), map[string]any{"name": "Otro nombre", "email": email}), http.StatusOK)
	expect(t, "sesión intacta", resp{Code: a.meStatus(c)}, http.StatusOK)
}

func TestDesactivarYEliminarInvalidanSesion(t *testing.T) {
	s := newShop(t)
	a := s.app

	id, email, c := s.basico(t)
	expect(t, "desactivar", a.do(s.Admin, "PATCH", s.path("/users/"+id+"/status"), map[string]any{"active": false}), http.StatusNoContent)
	expect(t, "cookie de usuario desactivado", resp{Code: a.meStatus(c)}, http.StatusUnauthorized)
	// Al reactivarlo, la cookie de antes NO revive (la versión ya cambió).
	expect(t, "reactivar", a.do(s.Admin, "PATCH", s.path("/users/"+id+"/status"), map[string]any{"active": true}), http.StatusNoContent)
	expect(t, "cookie vieja tras reactivar", resp{Code: a.meStatus(c)}, http.StatusUnauthorized)
	expect(t, "login tras reactivar", resp{Code: a.meStatus(a.login(t, email, testPassword))}, http.StatusOK)

	id2, _, c2 := s.basico(t)
	expect(t, "eliminar", a.do(s.Admin, "DELETE", s.path("/users/"+id2), nil), http.StatusOK)
	expect(t, "cookie de usuario eliminado", resp{Code: a.meStatus(c2)}, http.StatusUnauthorized)
}

func TestPlataformaCambiaContrasenaDeUsuarioDeOrganizacion(t *testing.T) {
	s := newShop(t)
	a := s.app
	plat := a.login(t, seedAdminEmail, seedAdminPassword)
	id := s.adminUserID(t)
	expect(t, "plataforma cambia contraseña del admin del taller",
		a.do(plat, "PUT", "/api/organizations/"+s.OrgID+"/users/"+id, map[string]any{"name": "Admin", "email": s.Email, "password": "Nueva-Clave-987"}),
		http.StatusNoContent)
	expect(t, "cookie vieja del admin del taller", resp{Code: a.meStatus(s.Admin)}, http.StatusUnauthorized)
	expect(t, "la plataforma sigue con sesión", resp{Code: a.meStatus(plat)}, http.StatusOK)
}

func TestCambioDeContrasenaPropioConservaLaSesionActual(t *testing.T) {
	s := newShop(t)
	a := s.app
	_, email, here := s.basico(t)
	other := a.login(t, email, testPassword) // otro dispositivo

	r := a.do(here, "PUT", "/api/users/me", map[string]any{
		"name": "Yo", "email": email, "oldPassword": testPassword, "newPassword": "Nueva-Clave-987"})
	expect(t, "cambiar contraseña desde el perfil", r, http.StatusOK)

	fresh := sessionCookie(r)
	if fresh == nil {
		t.Fatal("el cambio de contraseña propio debía volver a guardar la sesión actual (Set-Cookie)")
	}
	expect(t, "sesión actual (cookie renovada)", resp{Code: a.meStatus(fresh)}, http.StatusOK)
	expect(t, "otro dispositivo", resp{Code: a.meStatus(other)}, http.StatusUnauthorized)
	expect(t, "cookie anterior", resp{Code: a.meStatus(here)}, http.StatusUnauthorized)
	expect(t, "login con la contraseña nueva", resp{Code: a.meStatus(a.login(t, email, "Nueva-Clave-987"))}, http.StatusOK)
}

func TestEditarPerfilSinContrasenaConservaTodo(t *testing.T) {
	s := newShop(t)
	a := s.app
	_, email, c := s.basico(t)
	other := a.login(t, email, testPassword)
	expect(t, "editar perfil", a.do(c, "PUT", "/api/users/me", map[string]any{"name": "Yo mismo", "email": email}), http.StatusOK)
	expect(t, "misma cookie", resp{Code: a.meStatus(c)}, http.StatusOK)
	expect(t, "otro dispositivo", resp{Code: a.meStatus(other)}, http.StatusOK)
}

func TestAdminCambiaSuPropiaContrasenaDesdeEmpleados(t *testing.T) {
	s := newShop(t)
	a := s.app
	id := s.adminUserID(t)
	r := a.do(s.Admin, "PUT", s.path("/users/"+id), map[string]any{"name": "Admin", "email": s.Email, "password": "Nueva-Clave-987"})
	expect(t, "admin se cambia la contraseña", r, http.StatusOK)
	fresh := sessionCookie(r)
	if fresh == nil {
		t.Fatal("se esperaba la cookie renovada del propio usuario")
	}
	expect(t, "sesión renovada", resp{Code: a.meStatus(fresh)}, http.StatusOK)
}

// ── KAN-129: límite de intentos de login ───────────────────────────────

func TestLimiteDeLoginPorEmail(t *testing.T) {
	s := newShop(t)
	a := s.app
	_, email, _ := s.basico(t)
	ip := "198.51.100.10"
	ghost := "fantasma" + unique() + "@ejemplo.com" // no existe

	// 4 fallos + 1 login correcto: el correcto limpia el contador de ESE email.
	for i := 0; i < 4; i++ {
		if rec := a.loginRaw(ip, email, "mala"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("fallo %d: %d", i+1, rec.Code)
		}
	}
	if rec := a.loginRaw(ip, email, testPassword); rec.Code != http.StatusOK {
		t.Fatalf("login correcto tras 4 fallos: %d %s", rec.Code, rec.Body.String())
	}
	// Con el contador limpio, hacen falta 5 fallos más para bloquearlo.
	for i := 0; i < 5; i++ {
		if rec := a.loginRaw(ip, email, "mala"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("fallo %d tras limpiar: %d", i+1, rec.Code)
		}
	}
	// Bloqueado: 429 incluso con la contraseña correcta.
	if rec := a.loginRaw(ip, email, testPassword); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("email bloqueado: %d, quiero 429", rec.Code)
	}
	// Otra IP, mismo email: sigue bloqueado (el contador es por email).
	if rec := a.loginRaw("198.51.100.11", email, testPassword); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("email bloqueado desde otra IP: %d, quiero 429", rec.Code)
	}
	// Otro email desde la misma IP no queda bloqueado por eso.
	_, other, _ := s.basico(t)
	if rec := a.loginRaw(ip, other, testPassword); rec.Code != http.StatusOK {
		t.Fatalf("otro email desde la misma IP: %d, quiero 200", rec.Code)
	}

	// Un email inexistente se bloquea igual (mismo 429: no revela qué cuentas existen).
	for i := 0; i < 5; i++ {
		if rec := a.loginRaw("198.51.100.12", ghost, "mala"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("fantasma fallo %d: %d", i+1, rec.Code)
		}
	}
	if rec := a.loginRaw("198.51.100.12", ghost, "mala"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("email inexistente bloqueado: %d, quiero 429", rec.Code)
	}
}

func TestLimiteDeLoginPorIPNoSeReiniciaConUnLoginCorrecto(t *testing.T) {
	s := newShop(t)
	a := s.app
	_, mine, _ := s.basico(t)
	ip := "198.51.100.20"

	// 19 fallos con correos distintos (ninguno llega a su propio tope por email).
	for i := 0; i < 19; i++ {
		if rec := a.loginRaw(ip, "x"+unique()+"@ejemplo.com", "mala"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("fallo %d: %d", i+1, rec.Code)
		}
	}
	// Un login correcto propio NO borra los fallos de la IP...
	if rec := a.loginRaw(ip, mine, testPassword); rec.Code != http.StatusOK {
		t.Fatalf("login propio: %d", rec.Code)
	}
	// ...así que el 20.º fallo cierra la IP.
	if rec := a.loginRaw(ip, "x"+unique()+"@ejemplo.com", "mala"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("fallo 20: %d", rec.Code)
	}
	if rec := a.loginRaw(ip, mine, testPassword); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("IP bloqueada: %d, quiero 429", rec.Code)
	}
	// Otra IP no se ve afectada.
	if rec := a.loginRaw("198.51.100.21", mine, testPassword); rec.Code != http.StatusOK {
		t.Fatalf("otra IP: %d, quiero 200", rec.Code)
	}
}

// ── KAN-138: CSRF por Origin ───────────────────────────────────────────

func TestCSRFOrigen(t *testing.T) {
	s := newShop(t)
	a := s.app
	post := func(headers map[string]string) int {
		body, _ := json.Marshal(map[string]any{"name": "Cat " + unique()})
		req := httptest.NewRequest(http.MethodPost, s.path("/categories"), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(s.Admin)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		a.router.ServeHTTP(rec, req)
		return rec.Code
	}
	if code := post(nil); code != http.StatusForbidden {
		t.Errorf("sin Origin: %d, quiero 403", code)
	}
	if code := post(map[string]string{"Origin": "https://evil.example"}); code != http.StatusForbidden {
		t.Errorf("Origin ajeno: %d, quiero 403", code)
	}
	if code := post(map[string]string{"Origin": "null"}); code != http.StatusForbidden {
		t.Errorf("Origin null: %d, quiero 403", code)
	}
	if code := post(map[string]string{"Referer": "https://evil.example/pagina"}); code != http.StatusForbidden {
		t.Errorf("Referer ajeno: %d, quiero 403", code)
	}
	if code := post(map[string]string{"Origin": testOrigin}); code != http.StatusOK {
		t.Errorf("Origin correcto: %d, quiero 200", code)
	}
	// Sin Origin, un Referer del mismo host también vale.
	if code := post(map[string]string{"Referer": testOrigin + "/settings/categories"}); code != http.StatusOK {
		t.Errorf("Referer del mismo host: %d, quiero 200", code)
	}
	// X-Forwarded-Host de un cliente directo (no es un proxy de confianza) no se cree.
	if code := post(map[string]string{"Origin": "https://evil.example", "X-Forwarded-Host": "evil.example"}); code != http.StatusForbidden {
		t.Errorf("X-Forwarded-Host falsificado: %d, quiero 403", code)
	}
	// Las lecturas no exigen Origin.
	req := httptest.NewRequest(http.MethodGet, s.path("/categories"), nil)
	req.AddCookie(s.Admin)
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET sin Origin: %d, quiero 200", rec.Code)
	}
}

func TestCSRFLoginSinOrigen(t *testing.T) {
	a := app(t)
	b, _ := json.Marshal(map[string]string{"email": seedAdminEmail, "password": seedAdminPassword})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("login sin Origin: %d, quiero 403", rec.Code)
	}
}

// ── KAN-138: Authenticate con usuario inexistente / inactivo ───────────

func TestAuthenticateInexistenteEInactivoDevuelvenNotFound(t *testing.T) {
	s := newShop(t)
	id, email, _ := s.basico(t)
	if _, err := models.Authenticate("no-existe"+unique()+"@ejemplo.com", "Cualquiera-123"); err != models.ErrNotFound {
		t.Errorf("email inexistente: %v", err)
	}
	expect(t, "desactivar", s.app.do(s.Admin, "PATCH", s.path("/users/"+id+"/status"), map[string]any{"active": false}), http.StatusNoContent)
	if _, err := models.Authenticate(email, testPassword); err != models.ErrNotFound {
		t.Errorf("usuario inactivo: %v", err)
	}
}
