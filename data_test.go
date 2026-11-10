package main

import (
	"net/http"
	"strings"
	"testing"

	"hook-repair/models"
)

// Pruebas del Bloque 3 (datos y validaciones): KAN-94, KAN-145, KAN-144 y KAN-95.

// create hace un POST que debe salir bien (200/201) y devuelve el id creado.
func (s *shop) create(t *testing.T, path string, body any) string {
	t.Helper()
	r := s.app.do(s.Admin, "POST", s.path(path), body)
	if r.Code != http.StatusOK && r.Code != http.StatusCreated {
		t.Fatalf("POST %s: %s", path, r)
	}
	var out struct{ ID string }
	r.decode(t, &out)
	return out.ID
}

// productBody arma un producto válido; extra pisa o agrega campos.
func productBody(extra map[string]any) map[string]any {
	b := map[string]any{"name": "Producto " + unique(), "price": 10, "generalSalePrice": 20, "distributorSalePrice": 15, "inStock": 0}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

func (s *shop) newProduct(t *testing.T, extra map[string]any) string {
	t.Helper()
	return s.create(t, "/products", productBody(extra))
}

func (s *shop) getProduct(t *testing.T, id string) models.Product {
	t.Helper()
	r := s.app.do(s.Admin, "GET", s.path("/products/"+id), nil)
	expect(t, "GET producto", r, http.StatusOK)
	var p models.Product
	r.decode(t, &p)
	return p
}

// ── KAN-94: baja lógica ────────────────────────────────────────────────

func TestBajaLogica_MovimientoSobreProductoBorrado(t *testing.T) {
	s := newShop(t)
	a := s.app
	pid := s.newProduct(t, nil)
	mv := map[string]any{"productId": pid, "type": "Entry", "quantity": 2}
	expect(t, "movimiento sobre producto vivo", a.do(s.Admin, "POST", s.path("/inventory/movements"), mv), http.StatusCreated)
	expect(t, "borrar producto", a.do(s.Admin, "DELETE", s.path("/products/"+pid), nil), http.StatusOK)
	r := a.do(s.Admin, "POST", s.path("/inventory/movements"), mv)
	expect(t, "movimiento sobre producto borrado", r, http.StatusNotFound)
}

func TestBajaLogica_RolBorradoDa404(t *testing.T) {
	s := newShop(t)
	a := s.app
	rid := s.newRole(t, "Temporal", models.ViewUsers)
	expect(t, "borrar rol", a.do(s.Admin, "DELETE", s.path("/roles/"+rid), nil), http.StatusOK)
	expect(t, "desactivar rol borrado", a.do(s.Admin, "PATCH", s.path("/roles/"+rid+"/status"), map[string]any{"active": false}), http.StatusNotFound)
	expect(t, "activar rol borrado", a.do(s.Admin, "PATCH", s.path("/roles/"+rid+"/status"), map[string]any{"active": true}), http.StatusNotFound)
	expect(t, "activar rol inexistente", a.do(s.Admin, "PATCH", s.path("/roles/99999999/status"), map[string]any{"active": true}), http.StatusNotFound)
}

func TestBajaLogica_EditarConReferenciaBorradaNombraElCampo(t *testing.T) {
	s := newShop(t)
	a := s.app
	mk := map[string]func() string{
		"marca":     func() string { return s.create(t, "/brands", map[string]any{"name": "Marca " + unique()}) },
		"categoría": func() string { return s.create(t, "/categories", map[string]any{"name": "Cat " + unique(), "icon": "x"}) },
		"proveedor": func() string {
			return s.create(t, "/suppliers", map[string]any{"name": "Prov " + unique(), "phone1": "6621234567", "email": "p" + unique() + "@ejemplo.com"})
		},
	}
	kinds := map[string]string{"marca": "brands", "categoría": "categories", "proveedor": "suppliers"}
	keys := map[string]string{"marca": "brandId", "categoría": "categoryId", "proveedor": "supplierId"}
	for field, make := range mk {
		id := make()
		pid := s.newProduct(t, map[string]any{keys[field]: id})
		svc := s.create(t, "/services", map[string]any{"name": "Servicio " + unique(), "retailPrice": 1, "publicPrice": 1, "distributorPrice": 1})
		expect(t, "borrar "+field, a.do(s.Admin, "DELETE", s.path("/"+kinds[field]+"/"+id), nil), http.StatusOK)

		r := a.do(s.Admin, "PUT", s.path("/products/"+pid), productBody(map[string]any{keys[field]: id}))
		expect(t, "editar producto con "+field+" borrado", r, http.StatusBadRequest)
		if !strings.Contains(r.String(), field) {
			t.Errorf("el mensaje del producto debería nombrar %q: %s", field, r)
		}
		expect(t, "crear producto con "+field+" borrado", a.do(s.Admin, "POST", s.path("/products"), productBody(map[string]any{keys[field]: id})), http.StatusBadRequest)

		if field == "proveedor" {
			continue // los servicios no llevan proveedor
		}
		body := map[string]any{"name": "Servicio " + unique(), "retailPrice": 1, "publicPrice": 1, "distributorPrice": 1, keys[field]: id}
		r = a.do(s.Admin, "PUT", s.path("/services/"+svc), body)
		expect(t, "editar servicio con "+field+" borrado", r, http.StatusBadRequest)
		if !strings.Contains(r.String(), field) {
			t.Errorf("el mensaje del servicio debería nombrar %q: %s", field, r)
		}
		expect(t, "crear servicio con "+field+" borrado", a.do(s.Admin, "POST", s.path("/services"), body), http.StatusBadRequest)
	}
}

func TestBajaLogica_ModeloBorradoEnProducto(t *testing.T) {
	s := newShop(t)
	a := s.app
	brand := s.create(t, "/brands", map[string]any{"name": "Marca " + unique()})
	model := s.create(t, "/models", map[string]any{"name": "Modelo " + unique(), "brandId": brand})
	pid := s.newProduct(t, map[string]any{"brandId": brand, "modelIds": []string{model}})
	expect(t, "borrar modelo", a.do(s.Admin, "DELETE", s.path("/models/"+model), nil), http.StatusOK)
	r := a.do(s.Admin, "PUT", s.path("/products/"+pid), productBody(map[string]any{"brandId": brand, "modelIds": []string{model}}))
	expect(t, "editar producto con modelo borrado", r, http.StatusBadRequest)
	if !strings.Contains(r.String(), "modelo") {
		t.Errorf("el mensaje debería nombrar el modelo: %s", r)
	}
}
