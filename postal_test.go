package main

import (
	"net/http"
	"strings"
	"testing"

	"hook-repair/db"
	"hook-repair/models"
)

// KAN-102: catálogo de códigos postales (SEPOMEX) y colonia en la dirección.

// postalFixture — unas cuantas filas reales del catálogo (cp, colonia, tipo,
// municipio, ciudad, estado). Las pruebas no cargan el catálogo completo.
const postalFixture = "" +
	"06700\tRoma Norte\tColonia\tCuauhtémoc\tCiudad de México\tCiudad de México\n" +
	"06700\tRoma Sur\tColonia\tCuauhtémoc\tCiudad de México\tCiudad de México\n" +
	"83000\tHermosillo Centro\tColonia\tHermosillo\tHermosillo\tSonora\n" +
	"83000\tModelo\tColonia\tHermosillo\tHermosillo\tSonora\n" +
	"23000\tLa Paz Centro\tColonia\tLa Paz\t\tBaja California Sur\n"

func TestCodigoPostal_Consulta(t *testing.T) {
	s := newShop(t)
	r := s.app.do(s.Admin, "GET", "/api/postal-codes/83000", nil)
	expect(t, "consultar 83000", r, http.StatusOK)
	var pc models.PostalCode
	r.decode(t, &pc)
	if pc.CP != "83000" || pc.Estado != "Sonora" || pc.Municipio != "Hermosillo" || pc.Ciudad != "Hermosillo" || pc.Pais != "México" {
		t.Errorf("datos del CP: %+v", pc)
	}
	if len(pc.Colonias) != 2 || pc.Colonias[0].Nombre != "Hermosillo Centro" || pc.Colonias[0].Tipo != "Colonia" || pc.Colonias[1].Nombre != "Modelo" {
		t.Errorf("colonias del CP: %+v", pc.Colonias)
	}
	// Sin ciudad en el catálogo: se usa el municipio.
	r = s.app.do(s.Admin, "GET", "/api/postal-codes/23000", nil)
	expect(t, "consultar 23000", r, http.StatusOK)
	r.decode(t, &pc)
	if pc.Ciudad != "La Paz" || pc.Estado != "Baja California Sur" {
		t.Errorf("ciudad con respaldo en municipio: %+v", pc)
	}
}

func TestCodigoPostal_NoExisteFormatoYSesion(t *testing.T) {
	s := newShop(t)
	a := s.app
	expect(t, "CP inexistente", a.do(s.Admin, "GET", "/api/postal-codes/99999", nil), http.StatusNotFound)
	for _, bad := range []string{"123", "123456", "abcde", "8300a"} {
		expect(t, "CP con formato inválido "+bad, a.do(s.Admin, "GET", "/api/postal-codes/"+bad, nil), http.StatusBadRequest)
	}
	expect(t, "CP sin sesión", a.do(nil, "GET", "/api/postal-codes/83000", nil), http.StatusUnauthorized)
}

// Cualquier usuario con sesión puede consultar, sin permisos especiales.
func TestCodigoPostal_SinPermisosEspeciales(t *testing.T) {
	s := newShop(t)
	_, _, cookie := s.newUser(t, s.newRole(t, "Solo catálogos", models.ViewCatalogs))
	expect(t, "CP con un rol sin permisos", s.app.do(cookie, "GET", "/api/postal-codes/06700", nil), http.StatusOK)
}

func TestCodigoPostal_CargaIdempotente(t *testing.T) {
	app(t)
	count := func() int {
		var n int
		if err := db.DB.QueryRow(`SELECT COUNT(*) FROM postal_codes`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()
	if before == 0 {
		t.Fatal("el catálogo de pruebas debería estar cargado")
	}
	n, err := db.LoadPostalCatalog(strings.NewReader(postalFixture))
	if err != nil || n != 0 {
		t.Errorf("segunda carga: %d filas, error %v (quiero 0 y sin error)", n, err)
	}
	if after := count(); after != before {
		t.Errorf("la segunda carga duplicó filas: %d → %d", before, after)
	}
}

func TestCodigoPostal_ColoniaEnOrganizacionYCliente(t *testing.T) {
	s := newShop(t)
	a := s.app
	plat := a.login(t, seedAdminEmail, seedAdminPassword)

	addr := map[string]any{"address": "Av Reforma 100", "neighborhood": "Roma Norte", "city": "Ciudad de México", "state": "Ciudad de México", "postCode": "06700", "country": "México"}
	// Organización: crear por plataforma, editar desde Configuración y leer.
	u := unique()
	r := a.do(plat, "POST", "/api/organizations", map[string]any{"name": "Org " + u, "email": "org" + u + "@ejemplo.com", "address": addr})
	expect(t, "crear organización con colonia", r, http.StatusCreated)
	var org models.Organization
	r.decode(t, &org)
	if org.Address.Neighborhood != "Roma Norte" {
		t.Errorf("colonia al crear la organización: %q", org.Address.Neighborhood)
	}
	addr["neighborhood"] = "Roma Sur"
	r = a.do(s.Admin, "PUT", "/api/organizations/mine", map[string]any{"name": "Taller " + u, "email": "t" + u + "@ejemplo.com", "address": addr})
	expect(t, "editar mi organización con colonia", r, http.StatusOK)
	r = a.do(s.Admin, "GET", "/api/organizations/mine", nil)
	r.decode(t, &org)
	if org.Address.Neighborhood != "Roma Sur" {
		t.Errorf("colonia de mi organización: %q", org.Address.Neighborhood)
	}
	expect(t, "colonia de 201 caracteres", a.do(s.Admin, "PUT", "/api/organizations/mine", map[string]any{
		"name": "Taller " + u, "email": "t" + u + "@ejemplo.com", "address": map[string]any{"neighborhood": strings.Repeat("c", 201)}}), http.StatusBadRequest)

	// Cliente.
	cid := s.create(t, "/clients", map[string]any{"name": "Cliente " + u, "phone1": "6621234567", "address": addr})
	r = a.do(s.Admin, "GET", s.path("/clients/"+cid), nil)
	var c models.Client
	r.decode(t, &c)
	if c.Address == nil || c.Address.Neighborhood == nil || *c.Address.Neighborhood != "Roma Sur" {
		t.Errorf("colonia del cliente: %+v", c.Address)
	}
	addr["neighborhood"] = "Condesa"
	expect(t, "editar cliente", a.do(s.Admin, "PUT", s.path("/clients/"+cid), map[string]any{"name": "Cliente " + u, "phone1": "6621234567", "address": addr}), http.StatusOK)
	r = a.do(s.Admin, "GET", s.path("/clients/"+cid), nil)
	r.decode(t, &c)
	if c.Address == nil || c.Address.Neighborhood == nil || *c.Address.Neighborhood != "Condesa" {
		t.Errorf("colonia del cliente editado: %+v", c.Address)
	}
}
