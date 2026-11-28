package main

import (
	"net/http"
	"testing"

	"hook-repair/db"
	"hook-repair/models"
)

// Pruebas de roles por defecto (KAN-110).

// sin quita perm de la lista.
func sin(perms []string, perm string) []string {
	out := []string{}
	for _, p := range perms {
		if p != perm {
			out = append(out, p)
		}
	}
	return out
}

// Un permiso que el administrador quita a un rol por defecto no vuelve al
// reiniciar la aplicación (EnsureDefaultRoles corre en cada arranque).
func TestRolesPorDefecto_PermisoQuitadoNoVuelveAlReiniciar(t *testing.T) {
	s := newShop(t)
	c := s.roles["Cajero"]
	expect(t, "quitar Suppliers.View a Cajero",
		s.app.do(s.Admin, "PUT", s.path("/roles/"+c.ID), map[string]any{"name": "Cajero", "permissions": sin(c.Permissions, models.ViewSuppliers)}),
		http.StatusOK)
	models.EnsureDefaultRoles()
	s.loadRoles(t)
	if models.HasPermission(s.roles["Cajero"].Permissions, models.ViewSuppliers) {
		t.Fatalf("tras reiniciar, Cajero recuperó %s que el administrador quitó", models.ViewSuppliers)
	}
}

// Los roles creados antes de KAN-110 reciben una sola vez los permisos nuevos
// y conservan los que ya tenían.
func TestRolesPorDefecto_BackfillUnaSolaVez(t *testing.T) {
	s := newShop(t)
	c := s.roles["Cajero"]
	viejo := sin(sin(c.Permissions, models.ViewSuppliers), models.ViewCatalogs)
	expect(t, "dejar Cajero como antes de KAN-110",
		s.app.do(s.Admin, "PUT", s.path("/roles/"+c.ID), map[string]any{"name": "Cajero", "permissions": viejo}),
		http.StatusOK)
	if _, err := db.DB.Exec(`DELETE FROM app_migrations WHERE name = 'kan-110-permisos-lectura'`); err != nil {
		t.Fatal(err)
	}

	models.EnsureDefaultRoles()
	s.loadRoles(t)
	got := s.roles["Cajero"].Permissions
	for _, p := range append([]string{models.ViewSuppliers, models.ViewCatalogs}, viejo...) {
		if !models.HasPermission(got, p) {
			t.Errorf("tras el backfill, Cajero debería tener %s; tiene %v", p, got)
		}
	}
	if len(got) != len(viejo)+2 {
		t.Errorf("se esperaban %d permisos sin duplicados, hay %d: %v", len(viejo)+2, len(got), got)
	}

	// Ya aplicado: si el administrador vuelve a quitarlo, no regresa.
	expect(t, "quitar Catalogs.View otra vez",
		s.app.do(s.Admin, "PUT", s.path("/roles/"+c.ID), map[string]any{"name": "Cajero", "permissions": sin(got, models.ViewCatalogs)}),
		http.StatusOK)
	models.EnsureDefaultRoles()
	s.loadRoles(t)
	if models.HasPermission(s.roles["Cajero"].Permissions, models.ViewCatalogs) {
		t.Fatalf("el backfill se aplicó dos veces: Cajero recuperó %s", models.ViewCatalogs)
	}
}
