package models

import (
	"database/sql"
	"errors"
	"strings"

	"hook-repair/db"
)

// ── Categorías ─────────────────────────────────────────────────────────

// Category — tabla categories.
type Category struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Icon   string `json:"icon"`
	Active bool   `json:"active"`
}

// CategoryRequest — nombre + clase del ícono de Font Awesome ("fas fa-mobile").
type CategoryRequest struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

func validCatalogName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || runeLen(n) > 150 {
		return "", Invalid("El nombre debe tener entre 1 y 150 caracteres.")
	}
	return n, nil
}

// GetCategories — todas las de la organización.
func GetCategories(orgID string) ([]Category, error) {
	rows, err := db.DB.Query(`SELECT id, name, icon, active FROM categories WHERE organization_id = ? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Category{}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Icon, &c.Active); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetCategory por id.
func GetCategory(orgID, id string) (*Category, error) {
	var c Category
	err := db.DB.QueryRow(`SELECT id, name, icon, active FROM categories WHERE organization_id = ? AND id = ?`, orgID, id).
		Scan(&c.ID, &c.Name, &c.Icon, &c.Active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// CreateCategory crea una categoría.
func CreateCategory(orgID string, r CategoryRequest) (*Category, error) {
	name, err := validCatalogName(r.Name)
	if err != nil {
		return nil, err
	}
	id, err := insert(db.DB, `INSERT INTO categories (organization_id, name, icon, active) VALUES (?,?,?,1)`,
		orgID, name, strings.TrimSpace(r.Icon))
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, Conflict("La categoría ya existe.")
		}
		return nil, err
	}
	return GetCategory(orgID, id)
}

// UpdateCategory edita nombre e ícono.
func UpdateCategory(orgID, id string, r CategoryRequest) (*Category, error) {
	name, err := validCatalogName(r.Name)
	if err != nil {
		return nil, err
	}
	res, err := db.DB.Exec(`UPDATE categories SET name=?, icon=? WHERE organization_id=? AND id=?`,
		name, strings.TrimSpace(r.Icon), orgID, id)
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, Conflict("La categoría ya existe.")
		}
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return GetCategory(orgID, id)
}

// ── Marcas ─────────────────────────────────────────────────────────────

// Brand — tabla brands.
type Brand struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// BrandRequest — solo el nombre.
type BrandRequest struct {
	Name string `json:"name"`
}

// GetBrands — todas las de la organización.
func GetBrands(orgID string) ([]Brand, error) {
	rows, err := db.DB.Query(`SELECT id, name, active FROM brands WHERE organization_id = ? ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Brand{}
	for rows.Next() {
		var b Brand
		if err := rows.Scan(&b.ID, &b.Name, &b.Active); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBrand por id.
func GetBrand(orgID, id string) (*Brand, error) {
	var b Brand
	err := db.DB.QueryRow(`SELECT id, name, active FROM brands WHERE organization_id = ? AND id = ?`, orgID, id).
		Scan(&b.ID, &b.Name, &b.Active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &b, err
}

// CreateBrand crea una marca.
func CreateBrand(orgID string, r BrandRequest) (*Brand, error) {
	name, err := validCatalogName(r.Name)
	if err != nil {
		return nil, err
	}
	id, err := insert(db.DB, `INSERT INTO brands (organization_id, name, active) VALUES (?,?,1)`, orgID, name)
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, Conflict("La marca ya existe.")
		}
		return nil, err
	}
	return GetBrand(orgID, id)
}

// UpdateBrand cambia el nombre.
func UpdateBrand(orgID, id string, r BrandRequest) (*Brand, error) {
	name, err := validCatalogName(r.Name)
	if err != nil {
		return nil, err
	}
	res, err := db.DB.Exec(`UPDATE brands SET name=? WHERE organization_id=? AND id=?`, name, orgID, id)
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, Conflict("La marca ya existe.")
		}
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return GetBrand(orgID, id)
}

// ── Modelos (dispositivos) ─────────────────────────────────────────────

// DeviceModel — tabla device_models (con el nombre de su marca).
type DeviceModel struct {
	ID        string `json:"id"`
	BrandID   string `json:"brandId"`
	BrandName string `json:"brandName"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
}

// ModelRequest — nombre + marca.
type ModelRequest struct {
	Name    string `json:"name"`
	BrandID string `json:"brandId"`
}

const modelSelect = `SELECT m.id, m.brand_id, COALESCE(b.name, ''), m.name, m.active
	FROM device_models m LEFT JOIN brands b ON b.id = m.brand_id`

// GetDeviceModels — todos los de la organización.
func GetDeviceModels(orgID string) ([]DeviceModel, error) {
	rows, err := db.DB.Query(modelSelect+` WHERE m.organization_id = ? ORDER BY m.name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DeviceModel{}
	for rows.Next() {
		var m DeviceModel
		if err := rows.Scan(&m.ID, &m.BrandID, &m.BrandName, &m.Name, &m.Active); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetDeviceModel por id.
func GetDeviceModel(orgID, id string) (*DeviceModel, error) {
	var m DeviceModel
	err := db.DB.QueryRow(modelSelect+` WHERE m.organization_id = ? AND m.id = ?`, orgID, id).
		Scan(&m.ID, &m.BrandID, &m.BrandName, &m.Name, &m.Active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}

func (r *ModelRequest) validate(orgID string) (string, error) {
	name, err := validCatalogName(r.Name)
	if err != nil {
		return "", err
	}
	if !IsID(r.BrandID) {
		return "", Invalid("La marca es obligatoria.")
	}
	if _, err := GetBrand(orgID, r.BrandID); err != nil {
		return "", Invalid("La marca no pertenece a esta organización.")
	}
	return name, nil
}

// CreateDeviceModel crea un modelo.
func CreateDeviceModel(orgID string, r ModelRequest) (*DeviceModel, error) {
	name, err := r.validate(orgID)
	if err != nil {
		return nil, err
	}
	id, err := insert(db.DB, `INSERT INTO device_models (organization_id, brand_id, name, active) VALUES (?,?,?,1)`,
		orgID, r.BrandID, name)
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, Conflict("El modelo ya existe para esta marca.")
		}
		return nil, err
	}
	return GetDeviceModel(orgID, id)
}

// UpdateDeviceModel cambia nombre y marca.
func UpdateDeviceModel(orgID, id string, r ModelRequest) (*DeviceModel, error) {
	name, err := r.validate(orgID)
	if err != nil {
		return nil, err
	}
	res, err := db.DB.Exec(`UPDATE device_models SET name=?, brand_id=? WHERE organization_id=? AND id=?`, name, r.BrandID, orgID, id)
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, Conflict("El modelo ya existe para esta marca.")
		}
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return GetDeviceModel(orgID, id)
}

// ── Estado (activar / desactivar) de cualquier catálogo ────────────────

// catalogTables — tablas que se pueden activar/desactivar con SetCatalogStatus.
var catalogTables = map[string]string{
	"categories": "categories",
	"brands":     "brands",
	"models":     "device_models",
	"services":   "repair_services",
	"clients":    "customers",
	"suppliers":  "suppliers",
	"products":   "products",
}

// SetCatalogStatus activa o desactiva un registro de una de las tablas de arriba.
func SetCatalogStatus(kind, orgID, id string, active bool) error {
	table, ok := catalogTables[kind]
	if !ok {
		return errors.New("catálogo desconocido: " + kind)
	}
	res, err := db.DB.Exec(`UPDATE `+table+` SET active=? WHERE organization_id=? AND id=?`, active, orgID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
