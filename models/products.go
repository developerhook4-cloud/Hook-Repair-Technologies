package models

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"hook-repair/db"
)

// Product — tabla products con nombres de categoría, marca, proveedor,
// modelos compatibles y números de serie.
type Product struct {
	ID                   string   `json:"id"`
	ProductNumber        int      `json:"productNumber"`
	Name                 string   `json:"name"`
	CategoryID           *string  `json:"categoryId"`
	CategoryName         *string  `json:"categoryName"`
	BrandID              *string  `json:"brandId"`
	BrandName            *string  `json:"brandName"`
	ModelID              *string  `json:"modelId"`
	ModelName            *string  `json:"modelName"`
	ModelIDs             []string `json:"modelIds"`
	ModelNames           []string `json:"modelNames"`
	SupplierID           *string  `json:"supplierId"`
	SupplierName         *string  `json:"supplierName"`
	Upc                  string   `json:"upc"`
	ImageURL             *string  `json:"imageUrl"`
	Condition            *string  `json:"condition"`
	InStock              int      `json:"inStock"`
	StockWarning         int      `json:"stockWarning"`
	MinimumReorderPoint  int      `json:"minimumReorderPoint"`
	Price                float64  `json:"price"`
	GeneralSalePrice     float64  `json:"generalSalePrice"`
	DistributorSalePrice float64  `json:"distributorSalePrice"`
	IsSerialized         bool     `json:"isSerialized"`
	SerialNumbers        []string `json:"serialNumbers"`
	Active               bool     `json:"active"`
	StockStatus          string   `json:"stockStatus"`
	Status               string   `json:"status"` // alias de stockStatus (lo usa la tabla)
}

// ProductRequest — cuerpo para crear/editar. "upc" se ignora: lo calcula la base.
type ProductRequest struct {
	Name                 *string  `json:"name"`
	CategoryID           *string  `json:"categoryId"`
	BrandID              *string  `json:"brandId"`
	ModelID              *string  `json:"modelId"`
	ModelIDs             []string `json:"modelIds"`
	SupplierID           *string  `json:"supplierId"`
	ImageURL             *string  `json:"imageUrl"`
	Condition            *string  `json:"condition"`
	InStock              *int     `json:"inStock"`
	StockWarning         *int     `json:"stockWarning"`
	MinimumReorderPoint  *int     `json:"minimumReorderPoint"`
	Price                *float64 `json:"price"`
	GeneralSalePrice     *float64 `json:"generalSalePrice"`
	DistributorSalePrice *float64 `json:"distributorSalePrice"`
	IsSerialized         *bool    `json:"isSerialized"`
	Active               *bool    `json:"active"`
	SerialNumbers        []string `json:"serialNumbers"`
}

// allModelIDs une modelId (frontend) + modelIds, sin repetidos ni vacíos.
func (r *ProductRequest) allModelIDs() []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if IsID(id) && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, id := range r.ModelIDs {
		add(id)
	}
	if r.ModelID != nil {
		add(*r.ModelID)
	}
	return out
}

func intVal(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// validateCommon — reglas comunes de crear y editar.
func (r *ProductRequest) validateCommon() error {
	name := strVal(r.Name)
	if name == "" || runeLen(name) > 250 {
		return Invalid("El nombre es obligatorio (1 a 250 caracteres).")
	}
	for _, p := range []struct {
		label string
		v     *float64
	}{{"costo", r.Price}, {"precio general", r.GeneralSalePrice}, {"precio distribuidor", r.DistributorSalePrice}} {
		if p.v == nil || *p.v < 0 || !has2Decimals(*p.v) {
			return Invalid("El %s es obligatorio, debe ser 0 o mayor y tener máximo 2 decimales.", p.label)
		}
	}
	if intVal(r.StockWarning) < 0 {
		return Invalid("El warning de stock debe ser 0 o mayor.")
	}
	if intVal(r.MinimumReorderPoint) < 0 {
		return Invalid("El punto de reorden debe ser 0 o mayor.")
	}
	if runeLen(strVal(r.Condition)) > 100 {
		return Invalid("La condición no puede pasar de 100 caracteres.")
	}
	if img := strVal(r.ImageURL); img != "" {
		u, err := url.Parse(img)
		if err != nil || len(img) > 2048 || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") {
			return Invalid("imageUrl debe ser una URL http o https (máximo 2048 caracteres).")
		}
	}
	return nil
}

// validateReferences revisa que categoría, marca, proveedor y modelos existan
// y estén activos en la organización (y que los modelos sean de esa marca).
func (r *ProductRequest) validateReferences(orgID string, modelIDs []string) error {
	// Cada mensaje nombra el campo que falla (KAN-94): un registro eliminado ya
	// no existe para la validación, y uno inactivo no se puede elegir.
	if id := idOrNil(r.CategoryID); id != nil {
		c, err := GetCategory(orgID, id.(string))
		switch {
		case errors.Is(err, ErrNotFound):
			return Invalid("La categoría seleccionada ya no existe; elige otra.")
		case err != nil:
			return err
		case !c.Active:
			return Invalid("La categoría seleccionada está inactiva; elige otra.")
		}
	}
	if id := idOrNil(r.BrandID); id != nil {
		b, err := GetBrand(orgID, id.(string))
		switch {
		case errors.Is(err, ErrNotFound):
			return Invalid("La marca seleccionada ya no existe; elige otra.")
		case err != nil:
			return err
		case !b.Active:
			return Invalid("La marca seleccionada está inactiva; elige otra.")
		}
	}
	if id := idOrNil(r.SupplierID); id != nil {
		s, err := GetSupplier(orgID, id.(string))
		switch {
		case errors.Is(err, ErrNotFound):
			return Invalid("El proveedor seleccionado ya no existe; elige otro.")
		case err != nil:
			return err
		case !s.Active:
			return Invalid("El proveedor seleccionado está inactivo; elige otro.")
		}
	}
	for _, mid := range modelIDs {
		m, err := GetDeviceModel(orgID, mid)
		switch {
		case errors.Is(err, ErrNotFound):
			return Invalid("Un modelo seleccionado ya no existe; quítalo o elige otro.")
		case err != nil:
			return err
		case !m.Active:
			return Invalid("El modelo %s está inactivo; quítalo o elige otro.", m.Name)
		}
		if b := idOrNil(r.BrandID); b != nil && m.BrandID != b.(string) {
			return Invalid("El modelo %s no pertenece a la marca seleccionada.", m.Name)
		}
	}
	return nil
}

func stockStatus(inStock, warning int) string {
	switch {
	case inStock == 0:
		return "Agotado"
	case inStock <= warning:
		return "Bajo Stock"
	default:
		return "En Stock"
	}
}

const productSelect = `
	SELECT p.id, p.name,
	       c.id, c.name, b.id, b.name,
	       s.id, s.name, COALESCE(p.upc, ''), p.image_url, p.item_condition,
	       p.in_stock, p.stock_warning, p.reorder_point,
	       p.cost_price, p.general_price, p.distributor_price, p.is_serialized, p.active
	FROM products p
	LEFT JOIN categories c ON c.id = p.category_id AND c.deleted_at IS NULL
	LEFT JOIN brands b ON b.id = p.brand_id AND b.deleted_at IS NULL
	LEFT JOIN suppliers s ON s.id = p.supplier_id AND s.deleted_at IS NULL`

func scanProduct(sc interface{ Scan(...any) error }) (*Product, error) {
	var p Product
	var cat, catName, brand, brandName, sup, supName, img, cond sql.NullString
	if err := sc.Scan(&p.ID, &p.Name, &cat, &catName, &brand, &brandName, &sup, &supName,
		&p.Upc, &img, &cond, &p.InStock, &p.StockWarning, &p.MinimumReorderPoint,
		&p.Price, &p.GeneralSalePrice, &p.DistributorSalePrice, &p.IsSerialized, &p.Active); err != nil {
		return nil, err
	}
	p.ProductNumber, _ = strconv.Atoi(p.ID)
	p.CategoryID, p.CategoryName = nullStr(cat), nullStr(catName)
	p.BrandID, p.BrandName = nullStr(brand), nullStr(brandName)
	p.SupplierID, p.SupplierName = nullStr(sup), nullStr(supName)
	p.ImageURL, p.Condition = nullStr(img), nullStr(cond)
	p.ModelIDs, p.ModelNames, p.SerialNumbers = []string{}, []string{}, []string{}
	p.StockStatus = stockStatus(p.InStock, p.StockWarning)
	p.Status = p.StockStatus
	return &p, nil
}

// fillProductDetails carga los modelos compatibles y las series de una lista
// de productos con dos consultas (en vez de una por producto).
func fillProductDetails(list []*Product) error {
	if len(list) == 0 {
		return nil
	}
	byID := map[string]*Product{}
	ids := make([]string, 0, len(list))
	for _, p := range list {
		byID[p.ID] = p
		ids = append(ids, p.ID)
	}
	in := placeholders(len(ids))

	rows, err := db.DB.Query(`SELECT pm.product_id, m.id, m.name
		FROM product_models pm JOIN device_models m ON m.id = pm.model_id AND m.deleted_at IS NULL
		WHERE pm.product_id IN (`+in+`) ORDER BY m.name`, anySlice(ids)...)
	if err != nil {
		return err
	}
	for rows.Next() {
		var pid, mid, name string
		if err := rows.Scan(&pid, &mid, &name); err != nil {
			rows.Close()
			return err
		}
		if p := byID[pid]; p != nil {
			p.ModelIDs = append(p.ModelIDs, mid)
			p.ModelNames = append(p.ModelNames, name)
		}
	}
	rows.Close()

	rows, err = db.DB.Query(`SELECT product_id, imei FROM product_serials WHERE product_id IN (`+in+`) ORDER BY imei`, anySlice(ids)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var pid, imei string
		if err := rows.Scan(&pid, &imei); err != nil {
			return err
		}
		if p := byID[pid]; p != nil {
			p.SerialNumbers = append(p.SerialNumbers, imei)
		}
	}
	for _, p := range list {
		if len(p.ModelIDs) > 0 {
			p.ModelID, p.ModelName = &p.ModelIDs[0], &p.ModelNames[0]
		}
	}
	return rows.Err()
}

// Categoría, marca, proveedor y modelos eliminados no se muestran en la lectura
// (el producto conserva el id guardado; solo deja de verse): KAN-145.

// GetProducts — productos de la organización. Con onlyActive solo los activos.
func GetProducts(orgID string, onlyActive bool) ([]Product, error) {
	q := productSelect + ` WHERE p.organization_id = ? AND p.deleted_at IS NULL`
	if onlyActive {
		q += ` AND p.active = 1`
	}
	rows, err := db.DB.Query(q+` ORDER BY p.name`, orgID)
	if err != nil {
		return nil, err
	}
	ptrs := []*Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		ptrs = append(ptrs, p)
	}
	rows.Close()
	if err := fillProductDetails(ptrs); err != nil {
		return nil, err
	}
	out := make([]Product, len(ptrs))
	for i, p := range ptrs {
		out[i] = *p
	}
	return out, nil
}

// GetProduct por id.
func GetProduct(orgID, id string) (*Product, error) {
	p, err := scanProduct(db.DB.QueryRow(productSelect+` WHERE p.organization_id = ? AND p.id = ? AND p.deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, fillProductDetails([]*Product{p})
}

// cleanSerials quita vacíos y valida que no se repitan.
func cleanSerials(in []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if seen[strings.ToLower(s)] {
			return nil, Invalid("El IMEI / serie %s está repetido.", s)
		}
		seen[strings.ToLower(s)] = true
		out = append(out, s)
	}
	return out, nil
}

// makeUPC — '4' + (1 si es serializado, 0 si no) + el id con 10 dígitos.
func makeUPC(id string, serialized bool) string {
	n, _ := strconv.ParseInt(id, 10, 64)
	flag := "0"
	if serialized {
		flag = "1"
	}
	return fmt.Sprintf("4%s%010d", flag, n)
}

// CreateProduct crea el producto, su UPC, sus modelos, sus series (si es
// serializado) y el movimiento de "Stock inicial", todo en una transacción.
func CreateProduct(orgID string, r ProductRequest) (*Product, error) {
	if err := r.validateCommon(); err != nil {
		return nil, err
	}
	inStock := intVal(r.InStock)
	if inStock < 0 {
		return nil, Invalid("El stock inicial debe ser 0 o mayor.")
	}
	serialized := r.IsSerialized != nil && *r.IsSerialized
	serials, err := cleanSerials(r.SerialNumbers)
	if err != nil {
		return nil, err
	}
	if serialized && inStock > 0 && len(serials) != inStock {
		return nil, Invalid("Ingresa los %d IMEI / series del stock inicial (llevas %d).", inStock, len(serials))
	}
	modelIDs := r.allModelIDs()
	if err := r.validateReferences(orgID, modelIDs); err != nil {
		return nil, err
	}
	active := true
	if r.Active != nil {
		active = *r.Active
	}
	var id string
	err = withTx(func(tx *sql.Tx) error {
		var err error
		id, err = insert(tx, `INSERT INTO products
			(organization_id, name, category_id, brand_id, supplier_id, image_url, item_condition,
			 in_stock, stock_warning, reorder_point, cost_price, general_price, distributor_price, is_serialized, active)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			orgID, strVal(r.Name), idOrNil(r.CategoryID), idOrNil(r.BrandID), idOrNil(r.SupplierID),
			trimOrNil(r.ImageURL), trimOrNil(r.Condition), inStock, intVal(r.StockWarning), intVal(r.MinimumReorderPoint),
			*r.Price, *r.GeneralSalePrice, *r.DistributorSalePrice, serialized, active)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE products SET upc = ? WHERE id = ?`, makeUPC(id, serialized), id); err != nil {
			return err
		}
		for _, mid := range modelIDs {
			if _, err := tx.Exec(`INSERT INTO product_models (product_id, model_id) VALUES (?,?)`, id, mid); err != nil {
				return err
			}
		}
		if serialized {
			for _, s := range serials {
				if _, err := tx.Exec(`INSERT INTO product_serials (organization_id, product_id, imei, status) VALUES (?,?,?,'Disponible')`,
					orgID, id, s); err != nil {
					if IsUniqueViolation(err) {
						return Conflict("El IMEI / serie %s ya está registrado en otro producto.", s)
					}
					return err
				}
			}
		}
		if inStock > 0 {
			if _, err := tx.Exec(`INSERT INTO inventory_movements
				(organization_id, product_id, type, quantity, previous_stock, current_stock, reason, created_at)
				VALUES (?,?,'Entry',?,0,?,'Stock inicial',?)`, orgID, id, inStock, inStock, time.Now().UTC()); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return GetProduct(orgID, id)
}

// UpdateProduct edita los campos editables. El stock, el UPC y si es
// serializado NO cambian aquí (el stock se mueve con movimientos).
func UpdateProduct(orgID, id string, r ProductRequest) (*Product, error) {
	if err := r.validateCommon(); err != nil {
		return nil, err
	}
	current, err := GetProduct(orgID, id)
	if err != nil {
		return nil, err
	}
	if r.IsSerialized != nil && *r.IsSerialized != current.IsSerialized {
		return nil, Invalid("No se puede cambiar si el producto es serializado después de crearlo, porque define su UPC.")
	}
	modelIDs := r.allModelIDs()
	if err := r.validateReferences(orgID, modelIDs); err != nil {
		return nil, err
	}
	err = withTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE products SET
				name=?, category_id=?, brand_id=?, supplier_id=?, image_url=?, item_condition=?,
				stock_warning=?, reorder_point=?, cost_price=?, general_price=?, distributor_price=?
			WHERE organization_id=? AND id=?`,
			strVal(r.Name), idOrNil(r.CategoryID), idOrNil(r.BrandID), idOrNil(r.SupplierID),
			trimOrNil(r.ImageURL), trimOrNil(r.Condition), intVal(r.StockWarning), intVal(r.MinimumReorderPoint),
			*r.Price, *r.GeneralSalePrice, *r.DistributorSalePrice, orgID, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM product_models WHERE product_id = ?`, id); err != nil {
			return err
		}
		for _, mid := range modelIDs {
			if _, err := tx.Exec(`INSERT INTO product_models (product_id, model_id) VALUES (?,?)`, id, mid); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return GetProduct(orgID, id)
}
