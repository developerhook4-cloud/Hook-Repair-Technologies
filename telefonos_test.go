package main

import (
	"net/http"
	"testing"

	"hook-repair/db"
	"hook-repair/models"
)

// Pruebas de teléfonos múltiples: KAN-101.

type phoneBody = map[string]any

func ph(kind, number string, primary bool) phoneBody {
	return phoneBody{"kind": kind, "number": number, "isPrimary": primary}
}

func phonesOf(t *testing.T, owner, id string) []models.Phone {
	t.Helper()
	rows, err := db.DB.Query(`SELECT kind, number, is_primary FROM phones
		WHERE owner_type = ? AND owner_id = ? AND deleted_at IS NULL ORDER BY position`, owner, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []models.Phone{}
	for rows.Next() {
		var p models.Phone
		if err := rows.Scan(&p.Kind, &p.Number, &p.IsPrimary); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func TestTelefonos_MigracionCopiaPhone1YPhone2(t *testing.T) {
	s := newShop(t)
	u := unique()
	// Filas "antiguas": solo phone1 / phone2, sin filas en phones.
	cid := s.create(t, "/clients", map[string]any{"name": "Cli " + u, "phone1": "6621111111"})
	sid := s.create(t, "/suppliers", map[string]any{"name": "Prov " + u, "phone1": "6622222222", "email": "m" + u + "@ejemplo.com"})
	for _, q := range []string{
		`UPDATE customers SET phone1 = '6621111111', phone2 = '6623333333' WHERE id = ` + cid,
		`UPDATE suppliers SET phone1 = '6622222222', phone2 = '6624444444' WHERE id = ` + sid,
		`UPDATE organizations SET phone1 = '', phone2 = '6625555555' WHERE id = ` + s.OrgID,
		// Se borran de verdad las filas de phones para simular una base previa a la tabla.
		`DELETE FROM phones WHERE owner_type = 'organization' AND owner_id = ` + s.OrgID,
		`DELETE FROM phones WHERE owner_type = 'customer' AND owner_id = ` + cid,
		`DELETE FROM phones WHERE owner_type = 'supplier' AND owner_id = ` + sid,
	} {
		if _, err := db.DB.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	check := func(when string) {
		t.Helper()
		want := map[string][]models.Phone{
			"customer":     {{Kind: "fijo", Number: "6621111111", IsPrimary: true}, {Kind: "celular", Number: "6623333333"}},
			"supplier":     {{Kind: "fijo", Number: "6622222222", IsPrimary: true}, {Kind: "fijo", Number: "6624444444"}},
			"organization": {{Kind: "celular", Number: "6625555555", IsPrimary: true}},
		}
		ids := map[string]string{"customer": cid, "supplier": sid, "organization": s.OrgID}
		for owner, w := range want {
			got := phonesOf(t, owner, ids[owner])
			if len(got) != len(w) {
				t.Fatalf("%s, %s: quiero %v y hay %v", when, owner, w, got)
			}
			for i := range w {
				if got[i] != w[i] {
					t.Errorf("%s, %s[%d]: quiero %v y hay %v", when, owner, i, w[i], got[i])
				}
			}
		}
	}
	db.Migrate()
	check("primera corrida")
	db.Migrate()
	check("segunda corrida (idempotente)")
}

func TestTelefonos_ClienteVariosTelefonos(t *testing.T) {
	s := newShop(t)
	a := s.app
	body := func(phones any) map[string]any {
		return map[string]any{"name": "Cli " + unique(), "phones": phones}
	}
	cid := s.create(t, "/clients", body([]phoneBody{
		ph("celular", "(662) 111-2222", false), ph("fijo", "662 333 4444", true), ph("whatsapp", "+52 662 555 6666", false)}))
	var c models.Client
	a.do(s.Admin, "GET", s.path("/clients/"+cid), nil).decode(t, &c)
	if len(c.Phones) != 3 || c.Phones[0] != (models.Phone{Kind: "fijo", Number: "6623334444", IsPrimary: true}) ||
		c.Phones[1].Number != "6621112222" || c.Phones[2] != (models.Phone{Kind: "whatsapp", Number: "+526625556666"}) {
		t.Fatalf("teléfonos del cliente: %+v", c.Phones)
	}
	if c.Phone1 != "6623334444" || c.Phone2 == nil || *c.Phone2 != "6621112222" {
		t.Errorf("phone1/phone2 deben seguir a los dos primeros: %q %v", c.Phone1, c.Phone2)
	}

	// La lista también trae los teléfonos.
	var list []models.Client
	a.do(s.Admin, "GET", s.path("/clients"), nil).decode(t, &list)
	if len(list) != 1 || len(list[0].Phones) != 3 {
		t.Errorf("lista de clientes: %+v", list)
	}

	// Editar reemplaza todo; sin marcar principal, el primero lo es.
	r := a.do(s.Admin, "PUT", s.path("/clients/"+cid), body([]phoneBody{ph("celular", "6627778888", false)}))
	expect(t, "editar cliente con un teléfono", r, http.StatusOK)
	a.do(s.Admin, "GET", s.path("/clients/"+cid), nil).decode(t, &c)
	if len(c.Phones) != 1 || !c.Phones[0].IsPrimary || c.Phones[0].Number != "6627778888" || c.Phone2 != nil {
		t.Errorf("tras editar: %+v phone2=%v", c.Phones, c.Phone2)
	}
	if n := len(phonesOf(t, "customer", cid)); n != 1 {
		t.Errorf("filas vivas en phones: %d", n)
	}

	// Eliminar el cliente da de baja sus teléfonos.
	expect(t, "eliminar cliente", a.do(s.Admin, "DELETE", s.path("/clients/"+cid), nil), http.StatusOK)
	if n := len(phonesOf(t, "customer", cid)); n != 0 {
		t.Errorf("teléfonos de un cliente eliminado: %d", n)
	}
}

func TestTelefonos_ValidacionesCliente(t *testing.T) {
	s := newShop(t)
	a := s.app
	post := func(what string, body map[string]any, want int) {
		t.Helper()
		body["name"] = "Cli " + unique()
		expect(t, what, a.do(s.Admin, "POST", s.path("/clients"), body), want)
	}
	post("dos principales", map[string]any{"phones": []phoneBody{ph("fijo", "6621111111", true), ph("celular", "6622222222", true)}}, http.StatusBadRequest)
	post("número inválido", map[string]any{"phones": []phoneBody{ph("fijo", "66a2111111", false)}}, http.StatusBadRequest)
	post("tipo inválido", map[string]any{"phones": []phoneBody{ph("radio", "6621111111", false)}}, http.StatusBadRequest)
	post("número vacío", map[string]any{"phones": []phoneBody{ph("fijo", "", false)}}, http.StatusBadRequest)
	post("número repetido", map[string]any{"phones": []phoneBody{ph("fijo", "662-111-1111", false), ph("celular", "(662) 111 1111", false)}}, http.StatusBadRequest)
	post("lista vacía", map[string]any{"phones": []phoneBody{}}, http.StatusBadRequest)
	post("sin teléfonos", map[string]any{}, http.StatusBadRequest)
	many := []phoneBody{}
	for i := 0; i < 11; i++ {
		many = append(many, ph("celular", "662100000"+string(rune('0'+i%10))+string(rune('0'+i/10)), false))
	}
	post("11 teléfonos", map[string]any{"phones": many}, http.StatusBadRequest)
	post("10 teléfonos", map[string]any{"phones": many[:10]}, http.StatusOK)
}

func TestTelefonos_ClienteViaAntiguaPhone1Phone2(t *testing.T) {
	s := newShop(t)
	a := s.app
	cid := s.create(t, "/clients", map[string]any{"name": "Cli " + unique(), "phone1": "662-111-2222", "phone2": "6623334444"})
	var c models.Client
	a.do(s.Admin, "GET", s.path("/clients/"+cid), nil).decode(t, &c)
	if c.Phone1 != "6621112222" || c.Phone2 == nil || *c.Phone2 != "6623334444" {
		t.Errorf("columnas: %q %v", c.Phone1, c.Phone2)
	}
	if len(c.Phones) != 2 || c.Phones[0] != (models.Phone{Kind: "fijo", Number: "6621112222", IsPrimary: true}) ||
		c.Phones[1] != (models.Phone{Kind: "celular", Number: "6623334444"}) {
		t.Errorf("phones: %+v", c.Phones)
	}
}

func TestTelefonos_ProveedorYOrganizacion(t *testing.T) {
	s := newShop(t)
	a := s.app
	plat := a.login(t, seedAdminEmail, seedAdminPassword)

	sid := s.create(t, "/suppliers", map[string]any{"name": "Prov " + unique(), "email": "p" + unique() + "@ejemplo.com",
		"phones": []phoneBody{ph("fijo", "6621111111", false), ph("whatsapp", "6622222222", true), ph("celular", "6623333333", false)}})
	var sp models.Supplier
	a.do(s.Admin, "GET", s.path("/suppliers/"+sid), nil).decode(t, &sp)
	if len(sp.Phones) != 3 || sp.Phones[0].Number != "6622222222" || !sp.Phones[0].IsPrimary ||
		sp.Phone1 != "6622222222" || sp.Phone2 == nil || *sp.Phone2 != "6621111111" {
		t.Errorf("proveedor: %+v %q %v", sp.Phones, sp.Phone1, sp.Phone2)
	}
	expect(t, "proveedor sin teléfonos", a.do(s.Admin, "POST", s.path("/suppliers"),
		map[string]any{"name": "Prov " + unique(), "email": "q" + unique() + "@ejemplo.com", "phones": []phoneBody{}}), http.StatusBadRequest)

	// Organización de plataforma: puede quedar sin teléfonos; "mine" usa lo mismo.
	org := func(extra map[string]any) map[string]any {
		b := map[string]any{"name": "Org " + unique(), "email": "o" + unique() + "@ejemplo.com"}
		for k, v := range extra {
			b[k] = v
		}
		return b
	}
	r := a.do(plat, "PUT", "/api/organizations/"+s.OrgID, org(map[string]any{"phones": []phoneBody{ph("fijo", "6621111111", true), ph("celular", "6622222222", false), ph("whatsapp", "6623333333", false)}}))
	expect(t, "editar organización con 3 teléfonos", r, http.StatusOK)
	var o models.Organization
	r.decode(t, &o)
	if len(o.Phones) != 3 || o.Phone1 != "6621111111" || o.Phone2 != "6622222222" {
		t.Errorf("organización: %+v %q %q", o.Phones, o.Phone1, o.Phone2)
	}
	expect(t, "organización con dos principales", a.do(plat, "PUT", "/api/organizations/"+s.OrgID,
		org(map[string]any{"phones": []phoneBody{ph("fijo", "6621111111", true), ph("celular", "6622222222", true)}})), http.StatusBadRequest)
	r = a.do(s.Admin, "PUT", "/api/organizations/mine", org(map[string]any{"phones": []phoneBody{ph("whatsapp", "6629999999", false)}}))
	expect(t, "editar mi organización", r, http.StatusOK)
	r = a.do(s.Admin, "GET", "/api/organizations/mine", nil)
	r.decode(t, &o)
	if len(o.Phones) != 1 || o.Phones[0] != (models.Phone{Kind: "whatsapp", Number: "6629999999", IsPrimary: true}) || o.Phone1 != "6629999999" || o.Phone2 != "" {
		t.Errorf("mi organización: %+v %q %q", o.Phones, o.Phone1, o.Phone2)
	}
	r = a.do(s.Admin, "PUT", "/api/organizations/mine", org(map[string]any{"phones": []phoneBody{}}))
	expect(t, "organización sin teléfonos", r, http.StatusOK)
	r.decode(t, &o)
	if len(o.Phones) != 0 || o.Phone1 != "" {
		t.Errorf("organización sin teléfonos: %+v %q", o.Phones, o.Phone1)
	}
	// Vía antigua con el alias "phone".
	r = a.do(s.Admin, "PUT", "/api/organizations/mine", org(map[string]any{"phone": "662-444-5555", "phone2": "6626667777"}))
	expect(t, "organización con alias phone", r, http.StatusOK)
	r.decode(t, &o)
	if len(o.Phones) != 2 || o.Phones[0].Number != "6624445555" || o.Phones[1].Kind != "celular" || o.Phone2 != "6626667777" {
		t.Errorf("alias phone: %+v", o.Phones)
	}
}
