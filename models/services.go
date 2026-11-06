package models

import (
	"database/sql"
	"errors"
	"strings"

	"hook-repair/db"
)

// Service — tabla repair_services (servicios de reparación), con los nombres de
// categoría / marca / modelo para la tabla.
type Service struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	CategoryID       *string `json:"categoryId"`
	CategoryName     string  `json:"categoryName"`
	BrandID          *string `json:"brandId"`
	BrandName        string  `json:"brandName"`
	ModelID          *string `json:"modelId"`
	ModelName        string  `json:"modelName"`
	RetailPrice      float64 `json:"retailPrice"`
	PublicPrice      float64 `json:"publicPrice"`
	DistributorPrice float64 `json:"distributorPrice"`
	Active           bool    `json:"active"`
}

// SaveServiceRequest — cuerpo para crear/editar un servicio.
type SaveServiceRequest struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	CategoryID       *string  `json:"categoryId"`
	BrandID          *string  `json:"brandId"`
	ModelID          *string  `json:"modelId"`
	RetailPrice      *float64 `json:"retailPrice"`
	PublicPrice      *float64 `json:"publicPrice"`
	DistributorPrice *float64 `json:"distributorPrice"`
}

const serviceSelect = `
	SELECT s.id, s.name, s.description,
	       s.category_id, COALESCE(c.name, ''),
	       s.brand_id, COALESCE(b.name, ''),
	       s.model_id, COALESCE(m.name, ''),
	       s.retail_price, s.public_price, s.distributor_price, s.active
	FROM repair_services s
	LEFT JOIN categories c ON c.id = s.category_id
	LEFT JOIN brands b ON b.id = s.brand_id
	LEFT JOIN device_models m ON m.id = s.model_id`

func scanService(sc interface{ Scan(...any) error }) (*Service, error) {
	var s Service
	var cat, brand, model sql.NullString
	if err := sc.Scan(&s.ID, &s.Name, &s.Description, &cat, &s.CategoryName, &brand, &s.BrandName, &model, &s.ModelName,
		&s.RetailPrice, &s.PublicPrice, &s.DistributorPrice, &s.Active); err != nil {
		return nil, err
	}
	s.CategoryID, s.BrandID, s.ModelID = nullStr(cat), nullStr(brand), nullStr(model)
	return &s, nil
}

// GetServices — todos los servicios de la organización.
func GetServices(orgID string) ([]Service, error) {
	rows, err := db.DB.Query(serviceSelect+` WHERE s.organization_id = ? AND s.deleted_at IS NULL ORDER BY s.name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Service{}
	for rows.Next() {
		s, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// GetService por id.
func GetService(orgID, id string) (*Service, error) {
	s, err := scanService(db.DB.QueryRow(serviceSelect+` WHERE s.organization_id = ? AND s.id = ? AND s.deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

func (r *SaveServiceRequest) validate(orgID string) error {
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	if r.Name == "" || runeLen(r.Name) > 200 {
		return Invalid("El nombre es obligatorio (máximo 200 caracteres).")
	}
	if runeLen(r.Description) > 2000 {
		return Invalid("La descripción no puede pasar de 2000 caracteres.")
	}
	for _, p := range []struct {
		label string
		v     *float64
	}{{"precio", r.RetailPrice}, {"precio público", r.PublicPrice}, {"precio distribuidor", r.DistributorPrice}} {
		if p.v == nil || *p.v < 0 || *p.v > 999999999 {
			return Invalid("El %s es obligatorio y debe ser 0 o mayor.", p.label)
		}
	}
	if id := idOrNil(r.CategoryID); id != nil {
		if _, err := GetCategory(orgID, id.(string)); err != nil {
			return Invalid("La categoría no existe en esta organización.")
		}
	}
	if id := idOrNil(r.BrandID); id != nil {
		if _, err := GetBrand(orgID, id.(string)); err != nil {
			return Invalid("La marca no existe en esta organización.")
		}
	}
	if id := idOrNil(r.ModelID); id != nil {
		if _, err := GetDeviceModel(orgID, id.(string)); err != nil {
			return Invalid("El modelo no existe en esta organización.")
		}
	}
	return nil
}

// CreateService crea un servicio.
func CreateService(orgID string, r SaveServiceRequest) (*Service, error) {
	if err := r.validate(orgID); err != nil {
		return nil, err
	}
	id, err := insert(db.DB, `INSERT INTO repair_services
		(organization_id, name, description, category_id, brand_id, model_id, retail_price, public_price, distributor_price, active)
		VALUES (?,?,?,?,?,?,?,?,?,1)`,
		orgID, r.Name, r.Description, idOrNil(r.CategoryID), idOrNil(r.BrandID), idOrNil(r.ModelID),
		*r.RetailPrice, *r.PublicPrice, *r.DistributorPrice)
	if err != nil {
		return nil, err
	}
	return GetService(orgID, id)
}

// UpdateService edita un servicio (incluye categoría, marca y modelo).
func UpdateService(orgID, id string, r SaveServiceRequest) (*Service, error) {
	if err := r.validate(orgID); err != nil {
		return nil, err
	}
	res, err := db.DB.Exec(`UPDATE repair_services SET
			name=?, description=?, category_id=?, brand_id=?, model_id=?,
			retail_price=?, public_price=?, distributor_price=?
		WHERE organization_id=? AND id=? AND deleted_at IS NULL`,
		r.Name, r.Description, idOrNil(r.CategoryID), idOrNil(r.BrandID), idOrNil(r.ModelID),
		*r.RetailPrice, *r.PublicPrice, *r.DistributorPrice, orgID, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return GetService(orgID, id)
}
