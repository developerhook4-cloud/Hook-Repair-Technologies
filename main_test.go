package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"hook-repair/auth"
	"hook-repair/db"
	"hook-repair/models"
)

// Pruebas de integración: necesitan MySQL. Se saltan si no existe la variable
// HOOK_TEST_MYSQL, con un DSN sin base de datos, p. ej.:
//
//	HOOK_TEST_MYSQL='root:root@tcp(127.0.0.1:33306)/' go test ./... -count=1
//
// Cada corrida crea una base nueva (hook_test_<timestamp>) y la borra al final.

const (
	seedAdminEmail    = "admin@hookrepair.com"
	seedAdminPassword = "Seed-Test-123"
	testPassword      = "Prueba-12345"
	// testOrigin — Origin de las peticiones de prueba: httptest usa Host "example.com".
	testOrigin = "http://example.com"
)

var (
	testServerOnce sync.Once
	testServer     *testApp
	testServerErr  error
	adminDSN       string // DSN sin base de datos (para CREATE/DROP DATABASE)
	createdDBs     []string
	uniq           int
	uniqMu         sync.Mutex
)

func TestMain(m *testing.M) {
	adminDSN = os.Getenv("HOOK_TEST_MYSQL")
	gin.SetMode(gin.TestMode)
	code := m.Run()
	dropTestDatabases()
	os.Exit(code)
}

// dropTestDatabases borra las bases que crearon las pruebas.
func dropTestDatabases() {
	if adminDSN == "" || len(createdDBs) == 0 {
		return
	}
	if db.DB != nil {
		db.DB.Close()
	}
	if err := db.ConnectDSN(adminDSN); err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo limpiar las bases de prueba:", err)
		return
	}
	for _, name := range createdDBs {
		if _, err := db.DB.Exec("DROP DATABASE IF EXISTS `" + name + "`"); err != nil {
			fmt.Fprintln(os.Stderr, "no se pudo borrar", name, err)
		}
	}
	db.DB.Close()
}

// newDatabase crea una base nueva y deja db.DB apuntando a ella.
func newDatabase() error {
	if err := db.ConnectDSN(adminDSN); err != nil {
		return err
	}
	name := fmt.Sprintf("hook_test_%d_%d", time.Now().UnixNano(), len(createdDBs))
	if _, err := db.DB.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		return err
	}
	createdDBs = append(createdDBs, name)
	db.DB.Close()
	return db.ConnectDSN(adminDSN + name)
}

// testApp — el router real sobre una base limpia, con el seed ya corrido.
type testApp struct {
	router     *gin.Engine
	platformID string // id de la organización de plataforma
}

// app devuelve la app de pruebas (se arma una sola vez por corrida).
func app(t *testing.T) *testApp {
	t.Helper()
	if adminDSN == "" {
		t.Skip("HOOK_TEST_MYSQL no está definida: se salta la prueba de integración")
	}
	testServerOnce.Do(func() {
		if testServerErr = newDatabase(); testServerErr != nil {
			return
		}
		os.Setenv("SESSION_SECRET", "secreto-de-pruebas-0123456789-abcdefghij")
		os.Setenv("SEED_ADMIN_PASSWORD", seedAdminPassword)
		os.Unsetenv("RAILWAY_ENVIRONMENT_NAME")
		os.Unsetenv("RAILWAY_ENVIRONMENT")
		os.Unsetenv("APP_ENV")
		db.EnsureSchema()
		models.SeedRootAdmin()
		models.EnsureDefaultRoles()
		auth.InitStore()
		testServer = &testApp{router: setupRouter()}
		testServerErr = db.DB.QueryRow(`SELECT id FROM organizations WHERE is_platform = 1`).Scan(&testServer.platformID)
	})
	if testServerErr != nil {
		t.Fatalf("no se pudo preparar la base de pruebas: %v", testServerErr)
	}
	return testServer
}

// unique devuelve un sufijo distinto en cada llamada (emails y nombres únicos).
func unique() string {
	uniqMu.Lock()
	defer uniqMu.Unlock()
	uniq++
	return fmt.Sprintf("%d%d", time.Now().UnixNano()%1_000_000, uniq)
}

// ── Peticiones ─────────────────────────────────────────────────────────

type resp struct {
	Code int
	Body []byte
	Hdr  http.Header
}

func (r resp) decode(t *testing.T, dst any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, dst); err != nil {
		t.Fatalf("respuesta no es JSON (%d): %s", r.Code, r.Body)
	}
}

func (r resp) String() string { return fmt.Sprintf("%d %s", r.Code, strings.TrimSpace(string(r.Body))) }

func (a *testApp) do(cookie *http.Cookie, method, path string, body any) resp {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Origin", testOrigin) // los navegadores mandan Origin en POST/PUT/PATCH/DELETE
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return resp{Code: rec.Code, Body: rec.Body.Bytes(), Hdr: rec.Header()}
}

// login inicia sesión y devuelve la cookie hook_session.
func (a *testApp) login(t *testing.T, email, password string) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	b, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(b))
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("Content-Type", "application/json")
	a.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login de %s falló: %d %s", email, rec.Code, rec.Body.String())
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == auth.SessionName {
			return c
		}
	}
	t.Fatalf("login de %s no devolvió la cookie %s", email, auth.SessionName)
	return nil
}

// expect revisa el código de estado.
func expect(t *testing.T, what string, r resp, want int) {
	t.Helper()
	if r.Code != want {
		t.Errorf("%s: quiero %d y llegó %s", what, want, r)
	}
}

// ── Datos de prueba ────────────────────────────────────────────────────

// shop — un taller con su Administrador ya con sesión.
type shop struct {
	app   *testApp
	OrgID string
	Admin *http.Cookie
	Email string
	roles map[string]models.Role
}

// newShop crea una organización (como plataforma) con su administrador.
func newShop(t *testing.T) *shop {
	t.Helper()
	a := app(t)
	plat := a.login(t, seedAdminEmail, seedAdminPassword)
	u := unique()
	r := a.do(plat, "POST", "/api/organizations", map[string]any{"name": "Taller " + u, "email": "taller" + u + "@ejemplo.com"})
	expect(t, "crear organización", r, http.StatusCreated)
	var org models.Organization
	r.decode(t, &org)
	email := "admin" + u + "@ejemplo.com"
	expect(t, "crear administrador del taller",
		a.do(plat, "POST", "/api/organizations/"+org.ID+"/users", map[string]any{"name": "Admin " + u, "email": email, "password": testPassword}),
		http.StatusNoContent)
	s := &shop{app: a, OrgID: org.ID, Email: email, Admin: a.login(t, email, testPassword)}
	s.loadRoles(t)
	return s
}

func (s *shop) loadRoles(t *testing.T) {
	t.Helper()
	r := s.app.do(s.Admin, "GET", s.path("/roles"), nil)
	expect(t, "listar roles", r, http.StatusOK)
	var out struct{ Roles []models.Role }
	r.decode(t, &out)
	s.roles = map[string]models.Role{}
	for _, ro := range out.Roles {
		s.roles[ro.Name] = ro
	}
}

func (s *shop) path(p string) string { return "/api/" + s.OrgID + p }

// newRole crea un rol (como el Administrador) y devuelve su id.
func (s *shop) newRole(t *testing.T, name string, perms ...string) string {
	t.Helper()
	r := s.app.do(s.Admin, "POST", s.path("/roles"), map[string]any{"name": name + unique(), "permissions": perms})
	expect(t, "crear rol "+name, r, http.StatusCreated)
	var out struct{ ID string }
	r.decode(t, &out)
	return out.ID
}

// newUser crea un usuario con ese rol (como el Administrador) y devuelve
// su id, su email y su cookie de sesión.
func (s *shop) newUser(t *testing.T, roleID string) (id, email string, cookie *http.Cookie) {
	t.Helper()
	u := unique()
	email = "user" + u + "@ejemplo.com"
	r := s.app.do(s.Admin, "POST", s.path("/users"), map[string]any{"name": "Usuario " + u, "email": email, "password": testPassword, "roleId": roleID})
	expect(t, "crear usuario", r, http.StatusCreated)
	var out models.User
	r.decode(t, &out)
	return out.ID, email, s.app.login(t, email, testPassword)
}

// adminUserID — id del Administrador del taller.
func (s *shop) adminUserID(t *testing.T) string {
	t.Helper()
	r := s.app.do(s.Admin, "GET", s.path("/users"), nil)
	var users []models.User
	r.decode(t, &users)
	for _, u := range users {
		if u.Email == s.Email {
			return u.ID
		}
	}
	t.Fatal("no se encontró al administrador del taller")
	return ""
}
