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
	Address  string `json:"address"`
	City     string `json:"city"`
	State    string `json:"state"`
	PostCode string `json:"postCode"`
	Country  string `json:"country"`
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
	r.Phone1, r.Phone2 = strings.TrimSpace(r.Phone1), strings.TrimSpace(r.Phone2)
	if err := validPhone("teléfono", r.Phone1, false); err != nil {
		return err
	}
	if err := validPhone("celular", r.Phone2, false); err != nil {
		return err
	}
	if runeLen(r.Website) > 500 {
		return Invalid("La página web no puede pasar de 500 caracteres.")
	}
	if r.Address == nil {
		r.Address = &Address{}
	}
	return nil
}

const orgSelect = `
	SELECT o.id, o.name, o.email, o.website, o.active, o.phone1, o.phone2,
	       o.address, o.city, o.state, o.post_code, o.country,
	       (SELECT COUNT(*) FROM users u WHERE u.organization_id = o.id AND u.deleted_at IS NULL)
	FROM organizations o`

func scanOrganization(sc interface{ Scan(...any) error }) (*Organization, error) {
	var o Organization
	var website, p1, p2, a, c, s, pc, co sql.NullString
	if err := sc.Scan(&o.ID, &o.Name, &o.Email, &website, &o.Active, &p1, &p2, &a, &c, &s, &pc, &co, &o.Employees); err != nil {
		return nil, err
	}
	o.Website, o.Phone1, o.Phone2 = strOrEmpty(website), strOrEmpty(p1), strOrEmpty(p2)
	o.Phone = o.Phone1
	o.Address = Address{strOrEmpty(a), strOrEmpty(c), strOrEmpty(s), strOrEmpty(pc), strOrEmpty(co)}
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
	return out, rows.Err()
}

// GetOrganization por id.
func GetOrganization(id string) (*Organization, error) {
	o, err := scanOrganization(db.DB.QueryRow(orgSelect+` WHERE o.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return o, err
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
			(name, email, website, active, phone1, phone2, address, city, state, post_code, country)
			VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			r.Name, r.Email, r.Website, active, r.Phone1, r.Phone2,
			r.Address.Address, r.Address.City, r.Address.State, r.Address.PostCode, r.Address.Country)
		if err != nil {
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
}

// UpdateOrganization edita los datos (no el estado).
func UpdateOrganization(id string, r SaveOrganizationRequest) (*Organization, error) {
	if err := r.normalize(); err != nil {
		return nil, err
	}
	if _, err := GetOrganization(id); err != nil {
		return nil, err
	}
	_, err := db.DB.Exec(`UPDATE organizations SET
			name=?, email=?, website=?, phone1=?, phone2=?,
			address=?, city=?, state=?, post_code=?, country=?
		WHERE id=?`,
		r.Name, r.Email, r.Website, r.Phone1, r.Phone2,
		r.Address.Address, r.Address.City, r.Address.State, r.Address.PostCode, r.Address.Country, id)
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
