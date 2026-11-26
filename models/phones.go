package models

import (
	"database/sql"
	"strings"

	"hook-repair/db"
)

// Teléfonos múltiples (KAN-101). Cada organización, cliente o proveedor puede
// tener hasta maxPhones números en la tabla phones. Las columnas phone1 / phone2
// de cada tabla se conservan y se mantienen iguales a los dos primeros números
// (el principal primero), para que listas, búsquedas y demás código sigan igual.

const maxPhones = 10

// Tipos de teléfono válidos (enum phones.kind).
var phoneKinds = map[string]bool{"fijo": true, "celular": true, "whatsapp": true}

// Dueños de teléfonos (enum phones.owner_type).
const (
	ownerOrganization = "organization"
	ownerCustomer     = "customer"
	ownerSupplier     = "supplier"
)

// Phone — un número de teléfono de una organización, cliente o proveedor.
type Phone struct {
	Kind      string `json:"kind"`
	Number    string `json:"number"`
	IsPrimary bool   `json:"isPrimary"`
}

// phoneRules — cómo se resuelven los teléfonos de cada entidad.
type phoneRules struct {
	required bool   // exige al menos un número
	label1   string // etiqueta del campo phone1 (mensajes de error de la vía antigua)
	label2   string // etiqueta del campo phone2
	kind2    string // tipo que se asigna a phone2 cuando llega por la vía antigua
}

// resolvePhones deja la lista final de teléfonos: normalizada, validada y con el
// principal primero. Si `phones` es nil (el cuerpo no trae "phones") se arma con
// los campos antiguos phone1 / phone2 (compatibilidad). Reglas:
//   - máximo 10; cada número pasa validPhone y se guarda con normalizePhone;
//   - un número repetido en la misma entidad es 400;
//   - dos principales es 400; si ninguno viene marcado, el primero es el principal;
//   - si la entidad lo exige (clientes y proveedores) debe haber al menos uno.
func resolvePhones(phones []Phone, phone1, phone2 string, ru phoneRules) ([]Phone, error) {
	if phones == nil {
		return legacyToPhones(phone1, phone2, ru)
	}
	if len(phones) > maxPhones {
		return nil, Invalid("Solo se permiten hasta %d teléfonos.", maxPhones)
	}
	out := make([]Phone, 0, len(phones))
	seen := map[string]bool{}
	primaries := 0
	for i, p := range phones {
		p.Kind = strings.ToLower(strings.TrimSpace(p.Kind))
		if !phoneKinds[p.Kind] {
			return nil, Invalid("El tipo del teléfono %d no es válido (fijo, celular o whatsapp).", i+1)
		}
		n, err := normalizePhone("teléfono", p.Number, true)
		if err != nil {
			return nil, err
		}
		if seen[n] {
			return nil, Invalid("El teléfono %s está repetido.", n)
		}
		seen[n] = true
		p.Number = n
		if p.IsPrimary {
			primaries++
		}
		out = append(out, p)
	}
	if primaries > 1 {
		return nil, Invalid("Solo un teléfono puede ser el principal.")
	}
	if len(out) == 0 {
		if ru.required {
			return nil, Invalid("Se necesita al menos un teléfono.")
		}
		return out, nil
	}
	if primaries == 0 {
		out[0].IsPrimary = true
	}
	return orderPhones(out), nil
}

// legacyToPhones arma la lista a partir de phone1 / phone2 (cuerpos sin "phones").
func legacyToPhones(phone1, phone2 string, ru phoneRules) ([]Phone, error) {
	p1, err := normalizePhone(ru.label1, phone1, ru.required)
	if err != nil {
		return nil, err
	}
	p2, err := normalizePhone(ru.label2, phone2, false)
	if err != nil {
		return nil, err
	}
	out := []Phone{}
	if p1 != "" {
		out = append(out, Phone{Kind: "fijo", Number: p1, IsPrimary: true})
	}
	if p2 != "" && p2 != p1 {
		out = append(out, Phone{Kind: ru.kind2, Number: p2, IsPrimary: len(out) == 0})
	}
	return out, nil
}

// orderPhones pone el principal al inicio y conserva el orden del resto.
func orderPhones(in []Phone) []Phone {
	if len(in) == 0 {
		return in
	}
	pi := 0
	for i, p := range in {
		if p.IsPrimary {
			pi = i
			break
		}
	}
	out := make([]Phone, 0, len(in))
	out = append(out, in[pi])
	for i, p := range in {
		if i != pi {
			p.IsPrimary = false
			out = append(out, p)
		}
	}
	out[0].IsPrimary = true
	return out
}

// legacyPhones devuelve los valores de phone1 / phone2 que se guardan en las
// columnas de la entidad: los dos primeros números ya ordenados.
func legacyPhones(ps []Phone) (p1, p2 string) {
	if len(ps) > 0 {
		p1 = ps[0].Number
	}
	if len(ps) > 1 {
		p2 = ps[1].Number
	}
	return
}

// savePhones reemplaza todos los teléfonos del dueño (baja lógica de los
// anteriores) dentro de la transacción de quien guarda la entidad.
func savePhones(tx *sql.Tx, ownerType, orgID, ownerID string, ps []Phone) error {
	if err := deletePhones(tx, ownerType, ownerID); err != nil {
		return err
	}
	for i, p := range ps {
		if _, err := tx.Exec(`INSERT INTO phones (organization_id, owner_type, owner_id, kind, number, is_primary, position)
			VALUES (?,?,?,?,?,?,?)`, orgID, ownerType, ownerID, p.Kind, p.Number, p.IsPrimary, i); err != nil {
			return err
		}
	}
	return nil
}

// deletePhones da de baja lógica a los teléfonos de un dueño (al reemplazarlos
// o al eliminar la entidad).
func deletePhones(tx *sql.Tx, ownerType, ownerID string) error {
	_, err := tx.Exec(`UPDATE phones SET deleted_at = UTC_TIMESTAMP()
		WHERE owner_type = ? AND owner_id = ? AND deleted_at IS NULL`, ownerType, ownerID)
	return err
}

// loadPhones trae los teléfonos de varios dueños con una sola consulta (sin N+1),
// ordenados por position. Todo dueño de ids tiene entrada (lista vacía si no hay).
func loadPhones(ownerType string, ids []string) (map[string][]Phone, error) {
	out := make(map[string][]Phone, len(ids))
	for _, id := range ids {
		out[id] = []Phone{}
	}
	if len(ids) == 0 {
		return out, nil
	}
	args := []any{ownerType}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.DB.Query(`SELECT owner_id, kind, number, is_primary FROM phones
		WHERE owner_type = ? AND deleted_at IS NULL AND owner_id IN (`+placeholders(len(ids))+`)
		ORDER BY owner_id, position, id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var owner string
		var p Phone
		if err := rows.Scan(&owner, &p.Kind, &p.Number, &p.IsPrimary); err != nil {
			return nil, err
		}
		out[owner] = append(out[owner], p)
	}
	return out, rows.Err()
}
