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
		"marca": func() string { return s.create(t, "/brands", map[string]any{"name": "Marca " + unique()}) },
		"categoría": func() string {
			return s.create(t, "/categories", map[string]any{"name": "Cat " + unique(), "icon": "x"})
		},
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

// ── KAN-145: validaciones que daban 500 o dejaban estados inválidos ────

func TestValidaciones_OrganizacionLargosYTelefono(t *testing.T) {
	s := newShop(t)
	a := s.app
	plat := a.login(t, seedAdminEmail, seedAdminPassword)
	u := unique()
	base := func(extra map[string]any) map[string]any {
		b := map[string]any{"name": "Org " + u, "email": "org" + u + "@ejemplo.com"}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	for field, max := range map[string]int{"address": 300, "city": 100, "state": 100, "postCode": 20, "country": 100} {
		r := a.do(plat, "POST", "/api/organizations", base(map[string]any{"address": map[string]any{field: strings.Repeat("x", max+1)}}))
		expect(t, "organización con "+field+" demasiado largo", r, http.StatusBadRequest)
	}
	expect(t, "organización con ciudad de 300 caracteres",
		a.do(plat, "POST", "/api/organizations", base(map[string]any{"address": map[string]any{"city": strings.Repeat("c", 300)}})), http.StatusBadRequest)
	// Los máximos exactos sí caben.
	expect(t, "organización con largos máximos", a.do(plat, "POST", "/api/organizations", base(map[string]any{
		"email":   "max" + u + "@ejemplo.com",
		"address": map[string]any{"address": strings.Repeat("a", 300), "city": strings.Repeat("c", 100), "state": strings.Repeat("s", 100), "postCode": strings.Repeat("9", 20), "country": strings.Repeat("p", 100)},
	})), http.StatusCreated)

	// Teléfono con separadores: se guarda solo con dígitos.
	r := a.do(plat, "PUT", "/api/organizations/"+s.OrgID, base(map[string]any{
		"name": "Org " + u + "b", "phone1": "662-123-4567", "phone2": "+52 (662) 123 45.67"}))
	expect(t, "editar organización con teléfono con guiones", r, http.StatusOK)
	var org models.Organization
	r.decode(t, &org)
	if org.Phone1 != "6621234567" || org.Phone2 != "+526621234567" {
		t.Errorf("teléfonos normalizados: %q y %q", org.Phone1, org.Phone2)
	}
	expect(t, "teléfono con + a la mitad", a.do(plat, "PUT", "/api/organizations/"+s.OrgID, base(map[string]any{"phone1": "662+123"})), http.StatusBadRequest)
	expect(t, "teléfono con 16 dígitos", a.do(plat, "PUT", "/api/organizations/"+s.OrgID, base(map[string]any{"phone1": "1234567890123456"})), http.StatusBadRequest)
}

func TestValidaciones_TelefonoYLargosEnClientesYProveedores(t *testing.T) {
	s := newShop(t)
	a := s.app
	cid := s.create(t, "/clients", map[string]any{"name": "Cliente " + unique(), "phone1": "(662) 123-4567", "phone2": "662 765 4321"})
	r := a.do(s.Admin, "GET", s.path("/clients/"+cid), nil)
	var c models.Client
	r.decode(t, &c)
	if c.Phone1 != "6621234567" || c.Phone2 == nil || *c.Phone2 != "6627654321" {
		t.Errorf("teléfonos del cliente: %q %v", c.Phone1, c.Phone2)
	}
	expect(t, "cliente con razón social larga", a.do(s.Admin, "POST", s.path("/clients"),
		map[string]any{"name": "C " + unique(), "phone1": "6621234567", "legalName": strings.Repeat("r", 201)}), http.StatusBadRequest)

	sid := s.create(t, "/suppliers", map[string]any{"name": "Prov " + unique(), "phone1": "662-123-4567", "phone2": "662-000-1111", "email": "p" + unique() + "@ejemplo.com"})
	r = a.do(s.Admin, "GET", s.path("/suppliers/"+sid), nil)
	var sp models.Supplier
	r.decode(t, &sp)
	if sp.Phone1 != "6621234567" || sp.Phone2 == nil || *sp.Phone2 != "6620001111" {
		t.Errorf("teléfonos del proveedor: %q %v", sp.Phone1, sp.Phone2)
	}
	// 15 dígitos con "+" y separadores: cabe en la columna solo si se limpia.
	expect(t, "proveedor con teléfono de 15 dígitos y separadores", a.do(s.Admin, "PUT", s.path("/suppliers/"+sid),
		map[string]any{"name": "Prov " + unique(), "phone1": "+1 (234) 567-8901-2345", "email": "q" + unique() + "@ejemplo.com"}), http.StatusOK)
	expect(t, "categoría con ícono largo", a.do(s.Admin, "POST", s.path("/categories"),
		map[string]any{"name": "Cat " + unique(), "icon": strings.Repeat("i", 101)}), http.StatusBadRequest)
}

func TestValidaciones_OrdenCancelada(t *testing.T) {
	s := newShop(t)
	a := s.app
	sup := s.create(t, "/suppliers", map[string]any{"name": "Prov " + unique(), "phone1": "6621234567", "email": "p" + unique() + "@ejemplo.com"})
	pid := s.newProduct(t, nil)
	newPO := func() string {
		return s.create(t, "/purchase-orders", map[string]any{"supplierId": sup, "products": []map[string]any{{"productId": pid, "quantity": 2}}})
	}
	patch := func(id, status string) resp {
		return a.do(s.Admin, "PATCH", s.path("/purchase-orders/"+id), map[string]any{"status": status})
	}

	po := newPO()
	expect(t, "Pending a Waiting", patch(po, "Waiting"), http.StatusOK)
	expect(t, "Waiting a Canceled", patch(po, "Canceled"), http.StatusOK)
	for _, to := range []string{"Pending", "Waiting", "InProgress"} {
		r := patch(po, to)
		expect(t, "Canceled a "+to, r, http.StatusBadRequest)
		if !strings.Contains(r.String(), "cancelada") {
			t.Errorf("el mensaje debería decir que está cancelada: %s", r)
		}
	}
	// El pago sí se puede registrar aunque la orden esté cancelada: no es un cambio de estado.
	expect(t, "pago de orden cancelada", a.do(s.Admin, "PATCH", s.path("/purchase-orders/"+po), map[string]any{"paymentStatus": "Paid"}), http.StatusOK)

	// Recibida también es final.
	po = newPO()
	expect(t, "recibir la orden", a.do(s.Admin, "POST", s.path("/goods-receipts"), map[string]any{"purchaseOrderId": po}), http.StatusCreated)
	r := patch(po, "Pending")
	expect(t, "Received a Pending", r, http.StatusBadRequest)
	if !strings.Contains(r.String(), "recibida") {
		t.Errorf("el mensaje debería decir que está recibida: %s", r)
	}
	expect(t, "Received a Canceled", patch(po, "Canceled"), http.StatusBadRequest)
}

func TestValidaciones_BajaLogicaOcultaNombresEnLecturas(t *testing.T) {
	s := newShop(t)
	a := s.app
	brand := s.create(t, "/brands", map[string]any{"name": "Marca " + unique()})
	cat := s.create(t, "/categories", map[string]any{"name": "Cat " + unique(), "icon": "x"})
	sup := s.create(t, "/suppliers", map[string]any{"name": "Prov " + unique(), "phone1": "6621234567", "email": "p" + unique() + "@ejemplo.com"})
	model := s.create(t, "/models", map[string]any{"name": "Modelo " + unique(), "brandId": brand})
	pid := s.newProduct(t, map[string]any{"brandId": brand, "categoryId": cat, "supplierId": sup, "modelIds": []string{model}})
	svc := s.create(t, "/services", map[string]any{"name": "Servicio " + unique(), "retailPrice": 1, "publicPrice": 1, "distributorPrice": 1, "brandId": brand, "categoryId": cat, "modelId": model})

	p := s.getProduct(t, pid)
	if p.BrandName == nil || p.CategoryName == nil || p.SupplierName == nil || len(p.ModelNames) != 1 {
		t.Fatalf("antes de borrar el producto debe mostrar sus referencias: %+v", p)
	}
	expect(t, "borrar proveedor", a.do(s.Admin, "DELETE", s.path("/suppliers/"+sup), nil), http.StatusOK)
	expect(t, "borrar categoría", a.do(s.Admin, "DELETE", s.path("/categories/"+cat), nil), http.StatusOK)
	expect(t, "borrar marca (y sus modelos)", a.do(s.Admin, "DELETE", s.path("/brands/"+brand), nil), http.StatusOK)

	p = s.getProduct(t, pid)
	if p.BrandName != nil || p.BrandID != nil || p.CategoryName != nil || p.SupplierName != nil || len(p.ModelNames) != 0 {
		t.Errorf("el producto no debe mostrar referencias borradas: marca=%v categoría=%v proveedor=%v modelos=%v", p.BrandName, p.CategoryName, p.SupplierName, p.ModelNames)
	}
	var list []models.Product
	a.do(s.Admin, "GET", s.path("/products"), nil).decode(t, &list)
	if len(list) != 1 || list[0].BrandName != nil {
		t.Errorf("la lista de productos tampoco debe mostrar la marca borrada: %+v", list)
	}
	r := a.do(s.Admin, "GET", s.path("/services/"+svc), nil)
	var sv models.Service
	r.decode(t, &sv)
	if sv.BrandName != "" || sv.CategoryName != "" || sv.ModelName != "" || sv.BrandID != nil {
		t.Errorf("el servicio no debe mostrar referencias borradas: %+v", sv)
	}
	// El producto se puede volver a guardar tal como lo devuelve la lectura.
	expect(t, "guardar producto sin las referencias borradas", a.do(s.Admin, "PUT", s.path("/products/"+pid),
		productBody(map[string]any{"brandId": p.BrandID, "categoryId": p.CategoryID})), http.StatusOK)
}

func TestValidaciones_NoDesactivarseConPUT(t *testing.T) {
	s := newShop(t)
	a := s.app
	me := s.adminUserID(t)
	body := map[string]any{"name": "Admin", "email": s.Email, "active": false}

	patch := a.do(s.Admin, "PATCH", s.path("/users/"+me+"/status"), map[string]any{"active": false})
	expect(t, "PATCH propio active=false", patch, http.StatusBadRequest)
	put := a.do(s.Admin, "PUT", s.path("/users/"+me), body)
	expect(t, "PUT propio active=false", put, http.StatusBadRequest)
	if put.Code != patch.Code || !strings.Contains(put.String(), "No puedes desactivar tu propio usuario") {
		t.Errorf("el PUT debe dar lo mismo que el PATCH: %s vs %s", put, patch)
	}
	// Sigue activo y con sesión.
	expect(t, "sesión del administrador", a.do(s.Admin, "GET", s.path("/users/"+me), nil), http.StatusOK)

	// Lo mismo desde la vista de plataforma, en la organización propia, sin cambiar nada antes.
	plat := a.login(t, seedAdminEmail, seedAdminPassword)
	listUsers := func() []models.User {
		var l []models.User
		a.do(plat, "GET", "/api/organizations/"+a.platformID+"/users", nil).decode(t, &l)
		return l
	}
	var rootID string
	for _, u := range listUsers() {
		if u.Email == seedAdminEmail {
			rootID = u.ID
		}
	}
	r := a.do(plat, "PUT", "/api/organizations/"+a.platformID+"/users/"+rootID,
		map[string]any{"name": "Cambiado", "email": seedAdminEmail, "active": false})
	expect(t, "PUT propio desde plataforma", r, http.StatusBadRequest)
	for _, u := range listUsers() {
		if u.ID == rootID && (u.Name == "Cambiado" || !u.Active) {
			t.Errorf("no debió cambiar nada: %+v", u)
		}
	}
}

// ── KAN-144: productos serializados (IMEI) ─────────────────────────────

func TestSerializados_StockCeroNoGuardaIMEI(t *testing.T) {
	s := newShop(t)
	a := s.app
	imei := "IMEI" + unique()
	expect(t, "stock 0 con 1 IMEI", a.do(s.Admin, "POST", s.path("/products"),
		productBody(map[string]any{"isSerialized": true, "inStock": 0, "serialNumbers": []string{imei}})), http.StatusBadRequest)
	expect(t, "stock 2 con 1 IMEI", a.do(s.Admin, "POST", s.path("/products"),
		productBody(map[string]any{"isSerialized": true, "inStock": 2, "serialNumbers": []string{imei}})), http.StatusBadRequest)
	// Stock 0 sin IMEI y stock 2 con 2 IMEI sí.
	pid := s.newProduct(t, map[string]any{"isSerialized": true, "inStock": 0})
	if p := s.getProduct(t, pid); len(p.SerialNumbers) != 0 || p.InStock != 0 {
		t.Errorf("producto serializado con stock 0: %+v", p)
	}
	pid = s.newProduct(t, map[string]any{"isSerialized": true, "inStock": 2, "serialNumbers": []string{imei, imei + "b"}})
	if p := s.getProduct(t, pid); len(p.SerialNumbers) != 2 || p.InStock != 2 {
		t.Errorf("producto serializado con stock 2: %+v", p)
	}
}

func TestSerializados_MovimientoManualRechazado(t *testing.T) {
	s := newShop(t)
	a := s.app
	imei := "IMEI" + unique()
	pid := s.newProduct(t, map[string]any{"isSerialized": true, "inStock": 3, "serialNumbers": []string{imei + "a", imei + "b", imei + "c"}})
	for _, mv := range []map[string]any{
		{"productId": pid, "type": "Entry", "quantity": 5},
		{"productId": pid, "type": "Exit", "quantity": 1},
		{"productId": pid, "type": "Adjustment", "quantity": -1},
	} {
		r := a.do(s.Admin, "POST", s.path("/inventory/movements"), mv)
		expect(t, "movimiento manual sobre serializado "+mv["type"].(string), r, http.StatusBadRequest)
		if !strings.Contains(r.String(), "serializados") {
			t.Errorf("el mensaje debería explicar la regla: %s", r)
		}
	}
	if p := s.getProduct(t, pid); p.InStock != 3 || len(p.SerialNumbers) != 3 {
		t.Errorf("el stock no debió cambiar: %+v", p)
	}
	// Un producto normal sigue aceptando movimientos.
	plain := s.newProduct(t, nil)
	expect(t, "movimiento sobre producto normal", a.do(s.Admin, "POST", s.path("/inventory/movements"),
		map[string]any{"productId": plain, "type": "Entry", "quantity": 5}), http.StatusCreated)
}

func TestSerializados_BorrarProductoLiberaIMEI(t *testing.T) {
	s := newShop(t)
	a := s.app
	imei := "IMEI" + unique()
	mk := func() resp {
		return a.do(s.Admin, "POST", s.path("/products"), productBody(map[string]any{"isSerialized": true, "inStock": 1, "serialNumbers": []string{imei}}))
	}
	first := mk()
	expect(t, "crear producto con IMEI", first, http.StatusCreated)
	expect(t, "IMEI repetido en producto vivo", mk(), http.StatusConflict)
	var out struct{ ID string }
	first.decode(t, &out)
	expect(t, "borrar producto", a.do(s.Admin, "DELETE", s.path("/products/"+out.ID), nil), http.StatusOK)
	expect(t, "recrear con el mismo IMEI", mk(), http.StatusCreated)
}
