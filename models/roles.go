package models

import (
	"database/sql"
	"errors"
	"strings"

	"hook-repair/db"
)

// Role — fila de roles.
type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
	Active      bool     `json:"active"`
}

// SaveRoleRequest — cuerpo para crear/editar un rol.
type SaveRoleRequest struct {
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// normalize limpia el nombre, quita permisos repetidos y valida que existan.
func (r *SaveRoleRequest) normalize() error {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" || runeLen(r.Name) > 100 {
		return Invalid("El nombre del rol es obligatorio (máximo 100 caracteres).")
	}
	seen := map[string]bool{}
	clean := []string{}
	invalid := []string{}
	for _, p := range r.Permissions {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		c := CanonicalPermission(p)
		if c == "" {
			invalid = append(invalid, p)
			continue
		}
		if !seen[strings.ToLower(c)] {
			seen[strings.ToLower(c)] = true
			clean = append(clean, c)
		}
	}
	if len(invalid) > 0 {
		return Invalid("Los siguientes permisos no son válidos: %s", strings.Join(invalid, ", "))
	}
	if len(clean) == 0 {
		return Invalid("Selecciona al menos un permiso para el rol.")
	}
	r.Permissions = clean
	return nil
}

const roleSelect = `SELECT id, name, permissions, active FROM roles`

func scanRole(sc interface{ Scan(...any) error }) (*Role, error) {
	var r Role
	var perms sql.NullString
	if err := sc.Scan(&r.ID, &r.Name, &perms, &r.Active); err != nil {
		return nil, err
	}
	r.Permissions = jsonList(perms)
	return &r, nil
}

// GetRoles — roles de la organización, por nombre.
func GetRoles(orgID string) ([]Role, error) {
	rows, err := db.DB.Query(roleSelect+` WHERE organization_id = ? AND deleted_at IS NULL ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Role{}
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// GetRole por id dentro de la organización.
func GetRole(orgID, roleID string) (*Role, error) {
	r, err := scanRole(db.DB.QueryRow(roleSelect+` WHERE organization_id = ? AND id = ? AND deleted_at IS NULL`, orgID, roleID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

func roleNameTaken(orgID, name, exceptID string) (bool, error) {
	var exists bool
	err := db.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM roles
		WHERE organization_id = ? AND UPPER(name) = UPPER(?) AND deleted_at IS NULL AND (? = '' OR id <> ?))`,
		orgID, name, exceptID, exceptID).Scan(&exists)
	return exists, err
}

// CreateRole crea un rol nuevo.
func CreateRole(orgID string, r SaveRoleRequest) (*Role, error) {
	if err := r.normalize(); err != nil {
		return nil, err
	}
	taken, err := roleNameTaken(orgID, r.Name, "")
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, Conflict("Un rol con el mismo nombre ya existe en la organización.")
	}
	id, err := insert(db.DB, `INSERT INTO roles (organization_id, name, permissions, active) VALUES (?,?,?,1)`,
		orgID, r.Name, toJSON(r.Permissions))
	if err != nil {
		return nil, err
	}
	return GetRole(orgID, id)
}

// UpdateRole cambia nombre y permisos.
func UpdateRole(orgID, roleID string, r SaveRoleRequest) (*Role, error) {
	if err := r.normalize(); err != nil {
		return nil, err
	}
	if _, err := GetRole(orgID, roleID); err != nil {
		return nil, err
	}
	taken, err := roleNameTaken(orgID, r.Name, roleID)
	if err != nil {
		return nil, err
	}
	if !IsAdminRole(r.Permissions) {
		if err := ensureAdminRemains(orgID, "", roleID); err != nil {
			return nil, err
		}
	}
	if taken {
		return nil, Conflict("Un rol con el mismo nombre ya existe en la organización.")
	}
	if _, err := db.DB.Exec(`UPDATE roles SET name=?, permissions=? WHERE organization_id=? AND id=?`,
		r.Name, toJSON(r.Permissions), orgID, roleID); err != nil {
		return nil, err
	}
	return GetRole(orgID, roleID)
}

// SetRoleStatus activa o desactiva un rol.
func SetRoleStatus(orgID, roleID string, active bool) error {
	if !active {
		if err := ensureAdminRemains(orgID, "", roleID); err != nil {
			return err
		}
	}
	_, err := db.DB.Exec(`UPDATE roles SET active=? WHERE organization_id=? AND id=?`, active, orgID, roleID)
	return err
}

// FindRoleByName busca un rol de la organización por nombre (sin distinguir mayúsculas).
func FindRoleByName(orgID, name string) (*Role, error) {
	r, err := scanRole(db.DB.QueryRow(roleSelect+` WHERE organization_id = ? AND LOWER(name) = LOWER(?) AND deleted_at IS NULL LIMIT 1`, orgID, name))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}
