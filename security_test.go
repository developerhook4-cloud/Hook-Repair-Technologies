package main

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"

	"hook-repair/auth"
	"hook-repair/db"
	"hook-repair/models"
)

// ── KAN-127: organización de plataforma ────────────────────────────────

func TestPlataformaListaYCreaOrganizaciones(t *testing.T) {
	a := app(t)
	plat := a.login(t, seedAdminEmail, seedAdminPassword)
	expect(t, "GET /api/organizations (plataforma)", a.do(plat, "GET", "/api/organizations", nil), http.StatusOK)
	u := unique()
	expect(t, "POST /api/organizations (plataforma)",
		a.do(plat, "POST", "/api/organizations", map[string]any{"name": "Otro " + u, "email": "otro" + u + "@ejemplo.com"}),
		http.StatusCreated)

	var me struct{ User models.SessionUser }
	a.do(plat, "GET", "/api/auth/me", nil).decode(t, &me)
	if !me.User.IsPlatform {
		t.Error("el superadmin raíz debería tener isPlatform=true")
	}
	if r := a.do(plat, "GET", "/organizations", nil); r.Code != http.StatusOK {
		t.Errorf("la página /organizations debería abrir para plataforma: %s", r)
	}
}

func TestAdminDeTallerNoPuedeUsarOrganizaciones(t *testing.T) {
	s := newShop(t)
	a := s.app
	expect(t, "GET /api/organizations (taller)", a.do(s.Admin, "GET", "/api/organizations", nil), http.StatusForbidden)
	expect(t, "GET /api/organizations/:id (taller)", a.do(s.Admin, "GET", "/api/organizations/"+a.platformID, nil), http.StatusForbidden)

	var me struct{ User models.SessionUser }
	a.do(s.Admin, "GET", "/api/auth/me", nil).decode(t, &me)
	if me.User.IsPlatform {
		t.Error("un taller no debería tener isPlatform=true")
	}
}

func TestTallerNoPuedeCrearRolConOrganizations(t *testing.T) {
	s := newShop(t)
	a := s.app
	r := a.do(s.Admin, "POST", s.path("/roles"), map[string]any{"name": "Raro " + unique(), "permissions": []string{"Organizations.View"}})
	expect(t, "crear rol con Organizations.View", r, http.StatusBadRequest)

	// Editar un rol existente para agregarle Organizations.* tampoco.
	id := s.newRole(t, "Normal", models.ViewUsers)
	r = a.do(s.Admin, "PUT", s.path("/roles/"+id), map[string]any{"name": "Normal" + unique(), "permissions": []string{"Users.View", "Organizations.Update"}})
	expect(t, "editar rol con Organizations.Update", r, http.StatusBadRequest)

	// La lista de permisos del taller no incluye Organizations.*
	r = a.do(s.Admin, "GET", s.path("/roles/permissions"), nil)
	expect(t, "GET roles/permissions", r, http.StatusOK)
	var perms []string
	r.decode(t, &perms)
	if len(perms) == 0 {
		t.Fatal("la lista de permisos llegó vacía")
	}
	for _, p := range perms {
		if models.IsPlatformPermission(p) {
			t.Errorf("el taller no debería ver el permiso %s", p)
		}
	}

	// En cambio la plataforma sí los ve y puede crear roles con ellos.
	plat := a.login(t, seedAdminEmail, seedAdminPassword)
	r = a.do(plat, "GET", "/api/"+a.platformID+"/roles/permissions", nil)
	perms = nil
	r.decode(t, &perms)
	found := false
	for _, p := range perms {
		found = found || p == "Organizations.View"
	}
	if !found {
		t.Error("la plataforma debería ver Organizations.View en la lista de permisos")
	}
	r = a.do(plat, "POST", "/api/"+a.platformID+"/roles", map[string]any{"name": "Soporte " + unique(), "permissions": []string{"Organizations.View"}})
	expect(t, "plataforma crea rol con Organizations.View", r, http.StatusCreated)
}

// Un rol viejo de un taller con Organizations.* (dato insertado a mano) queda inerte.
func TestRolViejoConOrganizationsQuedaInerte(t *testing.T) {
	s := newShop(t)
	a := s.app
	res, err := db.DB.Exec(`INSERT INTO roles (organization_id, name, permissions, active) VALUES (?, 'Viejo', ?, 1)`,
		s.OrgID, `["Users.View","Users.Update","Roles.View","Roles.Update","Organizations.View","Organizations.Update","Organizations.Delete"]`)
	if err != nil {
		t.Fatal(err)
	}
	rid, _ := res.LastInsertId()
	u := unique()
	email := "viejo" + u + "@ejemplo.com"
	hash, _ := models.HashPassword(testPassword)
	if _, err := db.DB.Exec(`INSERT INTO users (organization_id, role_id, name, email, password_hash, active) VALUES (?,?,?,?,?,1)`,
		s.OrgID, rid, "Viejo "+u, email, hash); err != nil {
		t.Fatal(err)
	}
	c := a.login(t, email, testPassword)

	expect(t, "GET /api/organizations", a.do(c, "GET", "/api/organizations", nil), http.StatusForbidden)
	expect(t, "POST /api/organizations", a.do(c, "POST", "/api/organizations", map[string]any{"name": "X", "email": "x@ejemplo.com"}), http.StatusForbidden)
	expect(t, "PUT organización de plataforma", a.do(c, "PUT", "/api/organizations/"+a.platformID, map[string]any{"name": "Hackeada", "email": "h@ejemplo.com"}), http.StatusForbidden)
	expect(t, "cambiar contraseña del superadmin raíz",
		a.do(c, "PUT", "/api/organizations/"+a.platformID+"/users/1", map[string]any{"name": "x", "email": seedAdminEmail, "password": "Hackeada-123"}),
		http.StatusForbidden)
	r := a.do(c, "GET", "/organizations", nil)
	if r.Code != http.StatusFound || r.Hdr.Get("Location") != "/dashboard" {
		t.Errorf("la página /organizations debería redirigir a /dashboard: %s (Location=%q)", r, r.Hdr.Get("Location"))
	}
	// El superadmin raíz sigue entrando con su contraseña.
	a.login(t, seedAdminEmail, seedAdminPassword)
}

// ── KAN-128: escalada de privilegios ───────────────────────────────────

func TestUsuarioConSoloUsersUpdateNoEscala(t *testing.T) {
	s := newShop(t)
	a := s.app
	adminID := s.adminUserID(t)
	rrhh := s.newRole(t, "RRHH", models.ViewUsers, models.UpdateUsers, models.DeleteUsers, models.ViewRoles)
	_, _, c := s.newUser(t, rrhh)

	// Cambiar la contraseña del Administrador.
	r := a.do(c, "PUT", s.path("/users/"+adminID), map[string]any{"name": "Admin", "email": s.Email, "password": "Hackeada-123"})
	expect(t, "cambiar contraseña de un Administrador", r, http.StatusForbidden)
	a.login(t, s.Email, testPassword) // la contraseña original sigue valiendo

	// Crear un usuario con rol Administrador.
	r = a.do(c, "POST", s.path("/users"), map[string]any{"name": "Nuevo", "email": "n" + unique() + "@ejemplo.com", "password": testPassword, "roleId": s.roles["Administrador"].ID})
	expect(t, "crear usuario con rol Administrador", r, http.StatusForbidden)
	// ... y sin roleId (el servidor usa Administrador por defecto).
	r = a.do(c, "POST", s.path("/users"), map[string]any{"name": "Nuevo", "email": "n" + unique() + "@ejemplo.com", "password": testPassword})
	expect(t, "crear usuario sin rol (Administrador por defecto)", r, http.StatusForbidden)

	// Desactivar y eliminar al Administrador.
	expect(t, "desactivar al Administrador", a.do(c, "PATCH", s.path("/users/"+adminID+"/status"), map[string]any{"active": false}), http.StatusForbidden)
	expect(t, "eliminar al Administrador", a.do(c, "DELETE", s.path("/users/"+adminID), nil), http.StatusForbidden)

	// Ascenderse a sí mismo a Administrador.
	selfID, _, selfC := s.newUser(t, rrhh)
	r = a.do(selfC, "PUT", s.path("/users/"+selfID), map[string]any{"name": "Yo", "email": "yo" + unique() + "@ejemplo.com", "roleId": s.roles["Administrador"].ID})
	expect(t, "ascenderse a Administrador", r, http.StatusForbidden)

	// Lo que sí puede: crear un usuario con un rol que él mismo tiene.
	r = a.do(c, "POST", s.path("/users"), map[string]any{"name": "Par", "email": "p" + unique() + "@ejemplo.com", "password": testPassword, "roleId": rrhh})
	expect(t, "crear usuario con su mismo rol", r, http.StatusCreated)
}

func TestUsuarioConRolesUpdateNoOtorgaLoQueNoTiene(t *testing.T) {
	s := newShop(t)
	a := s.app
	gestor := s.newRole(t, "Gestor", models.ViewUsers, models.ViewRoles, models.UpdateRoles, models.DeleteRoles)
	_, _, c := s.newUser(t, gestor)

	// Crear un rol con permisos que no tiene.
	r := a.do(c, "POST", s.path("/roles"), map[string]any{"name": "Poderoso " + unique(), "permissions": []string{"Inventory.Delete"}})
	expect(t, "crear rol con permisos que no tiene", r, http.StatusForbidden)

	// Editar el rol Administrador (más poderoso que él), aunque sea para bajarlo.
	r = a.do(c, "PUT", s.path("/roles/"+s.roles["Administrador"].ID), map[string]any{"name": "Administrador", "permissions": []string{"Users.View"}})
	expect(t, "editar rol Administrador", r, http.StatusForbidden)
	// Editar un rol propio agregando permisos que no tiene.
	r = a.do(c, "PUT", s.path("/roles/"+gestor), map[string]any{"name": "Gestor" + unique(), "permissions": []string{"Users.View", "Roles.Update", "Users.Delete"}})
	expect(t, "agregarse permisos a su propio rol", r, http.StatusForbidden)
	// Desactivar y eliminar un rol más poderoso.
	expect(t, "desactivar rol Administrador", a.do(c, "PATCH", s.path("/roles/"+s.roles["Administrador"].ID+"/status"), map[string]any{"active": false}), http.StatusForbidden)
	expect(t, "eliminar rol Administrador", a.do(c, "DELETE", s.path("/roles/"+s.roles["Administrador"].ID), nil), http.StatusForbidden)

	// Un subconjunto de lo que tiene sí.
	r = a.do(c, "POST", s.path("/roles"), map[string]any{"name": "Lector " + unique(), "permissions": []string{"Users.View"}})
	expect(t, "crear rol con un subconjunto de sus permisos", r, http.StatusCreated)
}

func TestAdministradorCreaRolSubconjuntoYLoAsigna(t *testing.T) {
	s := newShop(t)
	a := s.app
	roleID := s.newRole(t, "Mostrador", models.ViewInventory, models.ViewCustomers)
	id, email, _ := s.newUser(t, s.roles["Cajero"].ID)
	r := a.do(s.Admin, "PUT", s.path("/users/"+id), map[string]any{"name": "Cambiado", "email": email, "roleId": roleID})
	expect(t, "asignar el rol nuevo", r, http.StatusOK)
	var u models.User
	r.decode(t, &u)
	if u.RoleID != roleID {
		t.Errorf("roleId = %s, quiero %s", u.RoleID, roleID)
	}
}

// ── KAN-92: /api/organizations/mine ────────────────────────────────────

func TestOrganizacionPropiaSoloLaEditanAdministradores(t *testing.T) {
	s := newShop(t)
	a := s.app
	_, _, cajero := s.newUser(t, s.roles["Cajero"].ID)
	body := map[string]any{"name": "Renombrada " + unique(), "email": "nuevo" + unique() + "@ejemplo.com"}

	expect(t, "Cajero GET /mine", a.do(cajero, "GET", "/api/organizations/mine", nil), http.StatusOK)
	expect(t, "Cajero PUT /mine", a.do(cajero, "PUT", "/api/organizations/mine", body), http.StatusForbidden)
	expect(t, "Administrador PUT /mine", a.do(s.Admin, "PUT", "/api/organizations/mine", body), http.StatusOK)

	var org models.Organization
	a.do(cajero, "GET", "/api/organizations/mine", nil).decode(t, &org)
	if org.Name != body["name"] {
		t.Errorf("el nombre guardado es %q, quiero %q", org.Name, body["name"])
	}
}

// ── KAN-127: migración ─────────────────────────────────────────────────

// En una base anterior (sin is_platform) la migración marca la organización de
// admin@hookrepair.com y es idempotente.
func TestMigracionMarcaOrganizacionDePlataforma(t *testing.T) {
	a := app(t)
	if _, err := db.DB.Exec(`ALTER TABLE organizations DROP COLUMN is_platform`); err != nil {
		t.Fatal(err)
	}
	db.EnsureSchema()
	db.EnsureSchema() // idempotente
	var n int
	var id string
	if err := db.DB.QueryRow(`SELECT COUNT(*), MIN(id) FROM organizations WHERE is_platform = 1`).Scan(&n, &id); err != nil {
		t.Fatal(err)
	}
	if n != 1 || id != a.platformID {
		t.Errorf("organizaciones de plataforma = %d (id %s), quiero 1 (id %s)", n, id, a.platformID)
	}
}

// ── KAN-93: contraseña del seed ────────────────────────────────────────

func TestSeedSinPasswordGeneraUnaAleatoriaYLaLoguea(t *testing.T) {
	app(t) // asegura el entorno y la conexión
	saved := db.DB
	defer func() { db.DB = saved }()
	if err := newDatabase(); err != nil {
		t.Fatal(err)
	}
	db.EnsureSchema()

	old := os.Getenv("SEED_ADMIN_PASSWORD")
	os.Unsetenv("SEED_ADMIN_PASSWORD")
	defer os.Setenv("SEED_ADMIN_PASSWORD", old)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	models.SeedRootAdmin()
	out := buf.String()
	if strings.Contains(out, "AdminPassword123") {
		t.Fatal("la contraseña fija sigue en uso")
	}
	i := strings.Index(out, "contraseña generada ")
	if i < 0 {
		t.Fatalf("el log no trae la contraseña generada: %s", out)
	}
	pass := strings.Fields(out[i+len("contraseña generada "):])[0]
	if err := models.ValidatePassword(pass); err != nil {
		t.Errorf("la contraseña generada %q no cumple la política: %v", pass, err)
	}
	if _, err := models.Authenticate(seedAdminEmail, pass); err != nil {
		t.Errorf("no se puede entrar con la contraseña generada: %v", err)
	}
	if _, err := models.Authenticate(seedAdminEmail, "AdminPassword123"); err == nil {
		t.Error("AdminPassword123 no debería funcionar")
	}
	var platform bool
	if err := db.DB.QueryRow(`SELECT is_platform FROM organizations ORDER BY id LIMIT 1`).Scan(&platform); err != nil || !platform {
		t.Errorf("la organización raíz debería ser de plataforma (err=%v, is_platform=%v)", err, platform)
	}
}

// Una cookie inventada (firmada con otra clave) no da sesión.
func TestCookieInventadaNoEntra(t *testing.T) {
	a := app(t)
	if auth.Store == nil {
		t.Fatal("Store sin inicializar")
	}
	r := a.do(&http.Cookie{Name: auth.SessionName, Value: "MTcwMDAwMDAwMHxmb3JqYWRhfA=="}, "GET", "/api/organizations/mine", nil)
	expect(t, "cookie inventada", r, http.StatusUnauthorized)
}
