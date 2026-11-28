package models

import (
	"database/sql"
	"errors"
	"log"
	"strings"

	"hook-repair/db"
)

// Address es la dirección completa (columnas address_* en organizations y customers).
type Address struct {
	Address string `json:"address"`
	// Neighborhood — colonia (KAN-102).
	Neighborhood string `json:"neighborhood"`
	City         string `json:"city"`
	State        string `json:"state"`
	PostCode     string `json:"postCode"`
	Country      string `json:"country"`
}

// Organization — fila de organizations.
type Organization struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	Website   string  `json:"website"`
	Active    bool    `json:"active"`
	Phone1    string  `json:"phone1"`
	Phone     string  `json:"phone"` // alias de phone1 (el frontend usa "phone")
	Phone2    string  `json:"phone2"`
	Phones    []Phone `json:"phones"` // todos los teléfonos (KAN-101); phone1/phone2 son los dos primeros
	Address   Address `json:"address"`
	Employees int     `json:"employees"`
}

// SaveOrganizationRequest — cuerpo para crear/editar una organización.
// Acepta "phone1" o "phone" (el formulario de Configuración manda "phone").
type SaveOrganizationRequest struct {
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Website string   `json:"website"`
	Phone1  string   `json:"phone1"`
	Phone   string   `json:"phone"`
	Phone2  string   `json:"phone2"`
	Active  *bool    `json:"active"`
	Address *Address `json:"address"`
	// Phones — teléfonos múltiples (KAN-101). Si no viene, se usan phone1 / phone2.
	Phones []Phone `json:"phones"`

	phones []Phone // lista ya validada (la llena normalize)
}

func (r *SaveOrganizationRequest) normalize() error {
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.TrimSpace(r.Email)
	if r.Phone1 == "" {
		r.Phone1 = r.Phone
	}
	if r.Name == "" || runeLen(r.Name) > 200 {
		return Invalid("El nombre es obligatorio (máximo 200 caracteres).")
	}
	if r.Email == "" || !IsEmail(r.Email) || runeLen(r.Email) > 200 {
		return Invalid("El email es obligatorio y debe ser válido.")
	}
	var err error
	if r.phones, err = resolvePhones(r.Phones, r.Phone1, r.Phone2,
		phoneRules{label1: "teléfono", label2: "celular", kind2: "celular"}); err != nil {
		return err
	}
	// phone1 / phone2 se mantienen iguales a los dos primeros números.
	r.Phone1, r.Phone2 = legacyPhones(r.phones)
	if runeLen(r.Website) > 500 {
		return Invalid("La página web no puede pasar de 500 caracteres.")
	}
	if r.Address == nil {
		r.Address = &Address{}
	}
	// Largos de las columnas address_* de organizations (db/schema.sql): con
	// MySQL estricto pasarse da error 500 en vez de un 400 claro (KAN-145).
	for _, f := range []struct {
		label string
		v     *string
		max   int
	}{
		{"La dirección", &r.Address.Address, 300}, {"La colonia", &r.Address.Neighborhood, 200}, {"La ciudad", &r.Address.City, 100},
		{"El estado", &r.Address.State, 100}, {"El código postal", &r.Address.PostCode, 20},
		{"El país", &r.Address.Country, 100},
	} {
		*f.v = strings.TrimSpace(*f.v)
		if runeLen(*f.v) > f.max {
			return Invalid("%s no puede pasar de %d caracteres.", f.label, f.max)
		}
	}
	return nil
}

const orgSelect = `
	SELECT o.id, o.name, o.email, o.website, o.active, o.phone1, o.phone2,
	       o.address, o.neighborhood, o.city, o.state, o.post_code, o.country,
	       (SELECT COUNT(*) FROM users u WHERE u.organization_id = o.id AND u.deleted_at IS NULL)
	FROM organizations o`

func scanOrganization(sc interface{ Scan(...any) error }) (*Organization, error) {
	var o Organization
	var website, p1, p2, a, nb, c, s, pc, co sql.NullString
	if err := sc.Scan(&o.ID, &o.Name, &o.Email, &website, &o.Active, &p1, &p2, &a, &nb, &c, &s, &pc, &co, &o.Employees); err != nil {
		return nil, err
	}
	o.Website, o.Phone1, o.Phone2 = strOrEmpty(website), strOrEmpty(p1), strOrEmpty(p2)
	o.Phone = o.Phone1
	o.Address = Address{Address: strOrEmpty(a), Neighborhood: strOrEmpty(nb), City: strOrEmpty(c), State: strOrEmpty(s), PostCode: strOrEmpty(pc), Country: strOrEmpty(co)}
	return &o, nil
}

// GetAllOrganizations — todas (vista de plataforma).
func GetAllOrganizations() ([]Organization, error) {
	rows, err := db.DB.Query(orgSelect + ` ORDER BY o.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Organization{}
	for rows.Next() {
		o, err := scanOrganization(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	ids := make([]string, len(out))
	for i := range out {
		ids[i] = out[i].ID
	}
	phones, err := loadPhones(ownerOrganization, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Phones = phones[out[i].ID]
	}
	return out, nil
}

// GetOrganization por id.
func GetOrganization(id string) (*Organization, error) {
	o, err := scanOrganization(db.DB.QueryRow(orgSelect+` WHERE o.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	phones, err := loadPhones(ownerOrganization, []string{o.ID})
	if err != nil {
		return nil, err
	}
	o.Phones = phones[o.ID]
	return o, nil
}

// CreateOrganization crea la organización y sus roles por defecto.
func CreateOrganization(r SaveOrganizationRequest) (*Organization, error) {
	if err := r.normalize(); err != nil {
		return nil, err
	}
	active := true
	if r.Active != nil {
		active = *r.Active
	}
	var id string
	err := withTx(func(tx *sql.Tx) error {
		var err error
		id, err = insert(tx, `INSERT INTO organizations
			(name, email, website, active, phone1, phone2, address, neighborhood, city, state, post_code, country)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			r.Name, r.Email, r.Website, active, r.Phone1, r.Phone2,
			r.Address.Address, r.Address.Neighborhood, r.Address.City, r.Address.State, r.Address.PostCode, r.Address.Country)
		if err != nil {
			return err
		}
		if err := savePhones(tx, ownerOrganization, id, id, r.phones); err != nil {
			return err
		}
		return registerDefaultRoles(tx, id)
	})
	if err != nil {
		return nil, err
	}
	return GetOrganization(id)
}

// registerDefaultRoles crea los roles por defecto que la organización no tenga.
// No toca los roles que ya existen: los permisos que el administrador quite o
// agregue se respetan en cada arranque. Los permisos nuevos de un rol por
// defecto se agregan una sola vez con backfillDefaultPermissions.
func registerDefaultRoles(tx *sql.Tx, orgID string) error {
	for _, dr := range DefaultRoles {
		var exists bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles WHERE organization_id = ? AND LOWER(name) = LOWER(?) AND deleted_at IS NULL)`,
			orgID, dr.Name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO roles (organization_id, name, permissions, active) VALUES (?,?,?,1)`,
			orgID, dr.Name, toJSON(dr.Permissions)); err != nil {
			return err
		}
	}
	return nil
}

// addedDefaultPermissions — permisos que se sumaron a los roles por defecto
// después de que ya existían organizaciones (KAN-110). Cada entrada se aplica
// una sola vez a los roles existentes; el marcador queda en app_migrations.
var addedDefaultPermissions = []struct {
	Marker string
	Roles  map[string][]string
}{
	{"kan-110-permisos-lectura", map[string][]string{
		"Gerente": {ViewRoles},
		"Técnico": {ViewCatalogs, ViewSuppliers},
		"Cajero":  {ViewCatalogs, ViewSuppliers},
	}},
}

// backfillDefaultPermissions agrega a los roles por defecto que ya existen los
// permisos de addedDefaultPermissions que les falten, una sola vez por marcador.
// Conserva los permisos actuales del rol y no duplica.
func backfillDefaultPermissions() error {
	for _, add := range addedDefaultPermissions {
		err := withTx(func(tx *sql.Tx) error {
			var done bool
			if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM app_migrations WHERE name = ?)`, add.Marker).Scan(&done); err != nil {
				return err
			}
			if done {
				return nil
			}
			for name, perms := range add.Roles {
				rows, err := tx.Query(`SELECT id, permissions FROM roles WHERE LOWER(name) = LOWER(?) AND deleted_at IS NULL`, name)
				if err != nil {
					return err
				}
				type roleRow struct {
					id    string
					perms []string
				}
				found := []roleRow{}
				for rows.Next() {
					var id string
					var stored sql.NullString
					if err := rows.Scan(&id, &stored); err != nil {
						rows.Close()
						return err
					}
					found = append(found, roleRow{id, jsonList(stored)})
				}
				rows.Close()
				if err := rows.Err(); err != nil {
					return err
				}
				for _, r := range found {
					merged := mergeDefaultPermissions(r.perms, perms)
					if len(merged) == len(r.perms) {
						continue
					}
					if _, err := tx.Exec(`UPDATE roles SET permissions = ? WHERE id = ?`, toJSON(merged), r.id); err != nil {
						return err
					}
				}
			}
			_, err := tx.Exec(`INSERT INTO app_migrations (name) VALUES (?)`, add.Marker)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// mergeDefaultPermissions agrega a current los permisos de defaults que le falten.
func mergeDefaultPermissions(
	current []string,
	defaults []string,
) []string {
	merged := append([]string{}, current...)

	for _, permission := range defaults {
		if !HasPermission(merged, permission) {
			merged = append(merged, permission)
		}
	}

	return merged
}

// EnsureDefaultRoles agrega a cada organización los roles por defecto que le
// falten (Administrador, Gerente, Técnico, Cajero). Corre al arrancar.
func EnsureDefaultRoles() {
	rows, err := db.DB.Query(`SELECT id FROM organizations`)
	if err != nil {
		log.Printf("Roles por defecto: %v", err)
		return
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	for _, id := range ids {
		if err := withTx(func(tx *sql.Tx) error { return registerDefaultRoles(tx, id) }); err != nil {
			log.Printf("Roles por defecto (organización %s): %v", id, err)
		}
	}
	if err := backfillDefaultPermissions(); err != nil {
		log.Printf("Permisos nuevos de roles por defecto: %v", err)
	}
}

// UpdateOrganization edita los datos (no el estado).
func UpdateOrganization(id string, r SaveOrganizationRequest) (*Organization, error) {
	if err := r.normalize(); err != nil {
		return nil, err
	}
	if _, err := GetOrganization(id); err != nil {
		return nil, err
	}
	err := withTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE organizations SET
			name=?, email=?, website=?, phone1=?, phone2=?,
			address=?, neighborhood=?, city=?, state=?, post_code=?, country=?
		WHERE id=?`,
			r.Name, r.Email, r.Website, r.Phone1, r.Phone2,
			r.Address.Address, r.Address.Neighborhood, r.Address.City, r.Address.State, r.Address.PostCode, r.Address.Country, id); err != nil {
			return err
		}
		return savePhones(tx, ownerOrganization, id, id, r.phones)
	})
	if err != nil {
		return nil, err
	}
	return GetOrganization(id)
}

// SetOrganizationStatus activa o desactiva una organización.
func SetOrganizationStatus(id string, active bool) (*Organization, error) {
	if _, err := GetOrganization(id); err != nil {
		return nil, err
	}
	if _, err := db.DB.Exec(`UPDATE organizations SET active=? WHERE id=?`, active, id); err != nil {
		return nil, err
	}
	return GetOrganization(id)
}
