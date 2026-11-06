package models

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"hook-repair/db"
)

// ── Clientes ───────────────────────────────────────────────────────────

// CustomerType: 0 = Público ("public" en la base), 1 = Distribuidor ("distributor").
type CustomerType int

const (
	CustomerPublic      CustomerType = 0
	CustomerDistributor CustomerType = 1
)

// UnmarshalJSON acepta 0/1 o "Public"/"Distributor".
func (t *CustomerType) UnmarshalJSON(b []byte) error {
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		if n != 0 && n != 1 {
			return Invalid("customerType no es válido.")
		}
		*t = CustomerType(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return Invalid("customerType no es válido.")
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "public", "0":
		*t = CustomerPublic
	case "distributor", "1":
		*t = CustomerDistributor
	default:
		return Invalid("customerType no es válido.")
	}
	return nil
}

func (t CustomerType) dbValue() string {
	if t == CustomerDistributor {
		return "distributor"
	}
	return "public"
}

func customerTypeFromDB(s string) CustomerType {
	if strings.EqualFold(s, "distributor") {
		return CustomerDistributor
	}
	return CustomerPublic
}

// NullableAddress — dirección que puede venir en null.
type NullableAddress struct {
	Address  *string `json:"address"`
	City     *string `json:"city"`
	State    *string `json:"state"`
	PostCode *string `json:"postCode"`
	Country  *string `json:"country"`
}

// Client — tabla customers.
type Client struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	CustomerType     CustomerType     `json:"customerType"`
	Phone1           string           `json:"phone1"`
	Phone2           *string          `json:"phone2"`
	LegalName        *string          `json:"legalName"`
	Email            *string          `json:"email"`
	Address          *NullableAddress `json:"address"`
	TitularName      *string          `json:"titularName"`
	AuthorizedPeople []string         `json:"authorizedPeople"`
	Rfc              *string          `json:"rfc"`
	TaxRegime        *string          `json:"taxRegime"`
	ExpenseType      *string          `json:"expenseType"`
	Active           bool             `json:"active"`
}

// ClientRequest — cuerpo para crear/editar un cliente.
type ClientRequest struct {
	Name             *string          `json:"name"`
	CustomerType     CustomerType     `json:"customerType"`
	Phone1           *string          `json:"phone1"`
	Phone2           *string          `json:"phone2"`
	LegalName        *string          `json:"legalName"`
	Email            *string          `json:"email"`
	Address          *NullableAddress `json:"address"`
	TitularName      *string          `json:"titularName"`
	AuthorizedPeople []string         `json:"authorizedPeople"`
	Rfc              *string          `json:"rfc"`
	TaxRegime        *string          `json:"taxRegime"`
	ExpenseType      *string          `json:"expenseType"`
}

func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(*p)
}

// validate — reglas del cliente.
func (r *ClientRequest) validate() error {
	name := strVal(r.Name)
	if name == "" || runeLen(name) > 200 {
		return Invalid("El nombre debe tener entre 1 y 200 caracteres.")
	}
	if err := validPhone("teléfono", strVal(r.Phone1), true); err != nil {
		return err
	}
	if err := validPhone("celular", strVal(r.Phone2), false); err != nil {
		return err
	}
	if e := strVal(r.Email); e != "" && (runeLen(e) > 254 || !IsEmail(e)) {
		return Invalid("El email debe ser válido (máximo 254 caracteres).")
	}
	// Facturación: si trae cualquiera de los datos fiscales, se piden los tres.
	rfc := strings.ToUpper(strVal(r.Rfc))
	if rfc != "" || strVal(r.TaxRegime) != "" || strVal(r.ExpenseType) != "" {
		if rfc == "" || strVal(r.TaxRegime) == "" || strVal(r.ExpenseType) == "" {
			return Invalid("Para facturar se necesitan el RFC, el régimen fiscal y el uso de CFDI.")
		}
		if !ValidRFC(rfc) {
			return Invalid("El RFC no es válido: deben ser 12 caracteres (persona moral) o 13 (persona física), con el formato del SAT.")
		}
		r.Rfc = &rfc
	}
	if a := r.Address; a != nil {
		for _, v := range []*string{a.Address, a.City, a.State, a.PostCode, a.Country} {
			if runeLen(strVal(v)) > 500 {
				return Invalid("Los campos de la dirección no pueden pasar de 500 caracteres.")
			}
		}
	}
	return nil
}

func cleanPeople(in []string) any {
	if in == nil {
		return nil
	}
	out := []string{}
	for _, p := range in {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return toJSON(out)
}

func addrField(a *NullableAddress, pick func(*NullableAddress) *string) any {
	if a == nil {
		return nil
	}
	return trimOrNil(pick(a))
}

const clientSelect = `
	SELECT id, name, customer_type, phone1, phone2, legal_name, email,
	       address, city, state, post_code, country,
	       titular_name, authorized_people, rfc, tax_regime, expense_type, active
	FROM customers`

func scanClient(sc interface{ Scan(...any) error }) (*Client, error) {
	var c Client
	var ctype string
	var p2, legal, email, a, ci, st, pc, co, titular, people, rfc, tax, exp sql.NullString
	if err := sc.Scan(&c.ID, &c.Name, &ctype, &c.Phone1, &p2, &legal, &email, &a, &ci, &st, &pc, &co,
		&titular, &people, &rfc, &tax, &exp, &c.Active); err != nil {
		return nil, err
	}
	c.CustomerType = customerTypeFromDB(ctype)
	c.Phone2, c.LegalName, c.Email = nullStr(p2), nullStr(legal), nullStr(email)
	c.TitularName, c.Rfc, c.TaxRegime, c.ExpenseType = nullStr(titular), nullStr(rfc), nullStr(tax), nullStr(exp)
	if people.Valid {
		c.AuthorizedPeople = jsonList(people)
	}
	if a.Valid || ci.Valid || st.Valid || pc.Valid || co.Valid {
		c.Address = &NullableAddress{nullStr(a), nullStr(ci), nullStr(st), nullStr(pc), nullStr(co)}
	}
	return &c, nil
}

// GetClients — todos los clientes de la organización.
func GetClients(orgID string) ([]Client, error) {
	rows, err := db.DB.Query(clientSelect+` WHERE organization_id = ? AND deleted_at IS NULL ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Client{}
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// GetClient por id.
func GetClient(orgID, id string) (*Client, error) {
	c, err := scanClient(db.DB.QueryRow(clientSelect+` WHERE organization_id = ? AND id = ? AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (r *ClientRequest) args() []any {
	a := r.Address
	return []any{
		r.CustomerType.dbValue(), strVal(r.Name), strVal(r.Phone1), trimOrNil(r.Phone2), trimOrNil(r.LegalName), trimOrNil(r.Email),
		addrField(a, func(x *NullableAddress) *string { return x.Address }),
		addrField(a, func(x *NullableAddress) *string { return x.City }),
		addrField(a, func(x *NullableAddress) *string { return x.State }),
		addrField(a, func(x *NullableAddress) *string { return x.PostCode }),
		addrField(a, func(x *NullableAddress) *string { return x.Country }),
		trimOrNil(r.TitularName), cleanPeople(r.AuthorizedPeople), trimOrNil(r.Rfc), trimOrNil(r.TaxRegime), trimOrNil(r.ExpenseType),
	}
}

// CreateClient crea un cliente.
func CreateClient(orgID string, r ClientRequest) (*Client, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	args := append([]any{orgID}, r.args()...)
	id, err := insert(db.DB, `INSERT INTO customers
		(organization_id, customer_type, name, phone1, phone2, legal_name, email,
		 address, city, state, post_code, country,
		 titular_name, authorized_people, rfc, tax_regime, expense_type, active)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,1)`, args...)
	if err != nil {
		return nil, err
	}
	return GetClient(orgID, id)
}

// UpdateClient edita un cliente.
func UpdateClient(orgID, id string, r ClientRequest) (*Client, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	args := append(r.args(), id, orgID)
	res, err := db.DB.Exec(`UPDATE customers SET
			customer_type=?, name=?, phone1=?, phone2=?, legal_name=?, email=?,
			address=?, city=?, state=?, post_code=?, country=?,
			titular_name=?, authorized_people=?, rfc=?, tax_regime=?, expense_type=?
		WHERE id=? AND organization_id=? AND deleted_at IS NULL`, args...)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return GetClient(orgID, id)
}

// ── Proveedores ────────────────────────────────────────────────────────

// Supplier — tabla suppliers.
type Supplier struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Phone1 string  `json:"phone1"`
	Phone2 *string `json:"phone2"`
	Email  string  `json:"email"`
	Active bool    `json:"active"`
}

// SupplierRequest — cuerpo para crear/editar un proveedor.
type SupplierRequest struct {
	Name   *string `json:"name"`
	Phone1 *string `json:"phone1"`
	Phone2 *string `json:"phone2"`
	Email  *string `json:"email"`
}

func (r *SupplierRequest) validate() error {
	name := strVal(r.Name)
	if name == "" || runeLen(name) > 200 {
		return Invalid("El nombre debe tener entre 1 y 200 caracteres.")
	}
	if err := validPhone("teléfono", strVal(r.Phone1), true); err != nil {
		return err
	}
	if err := validPhone("teléfono alternativo", strVal(r.Phone2), false); err != nil {
		return err
	}
	e := strVal(r.Email)
	if e == "" {
		return Invalid("El email es obligatorio.")
	}
	if runeLen(e) > 254 || !IsEmail(e) {
		return Invalid("El email debe ser válido (máximo 254 caracteres).")
	}
	return nil
}

const supplierSelect = `SELECT id, name, phone1, phone2, email, active FROM suppliers`

func scanSupplier(sc interface{ Scan(...any) error }) (*Supplier, error) {
	var s Supplier
	var p2 sql.NullString
	if err := sc.Scan(&s.ID, &s.Name, &s.Phone1, &p2, &s.Email, &s.Active); err != nil {
		return nil, err
	}
	s.Phone2 = nullStr(p2)
	return &s, nil
}

// GetSuppliers — todos los proveedores de la organización.
func GetSuppliers(orgID string) ([]Supplier, error) {
	rows, err := db.DB.Query(supplierSelect+` WHERE organization_id = ? AND deleted_at IS NULL ORDER BY name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Supplier{}
	for rows.Next() {
		s, err := scanSupplier(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// GetSupplier por id.
func GetSupplier(orgID, id string) (*Supplier, error) {
	s, err := scanSupplier(db.DB.QueryRow(supplierSelect+` WHERE organization_id = ? AND id = ? AND deleted_at IS NULL`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// CreateSupplier crea un proveedor.
func CreateSupplier(orgID string, r SupplierRequest) (*Supplier, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	id, err := insert(db.DB, `INSERT INTO suppliers (organization_id, name, phone1, phone2, email, active)
		VALUES (?,?,?,?,?,1)`, orgID, strVal(r.Name), strVal(r.Phone1), trimOrNil(r.Phone2), strVal(r.Email))
	if err != nil {
		return nil, err
	}
	return GetSupplier(orgID, id)
}

// UpdateSupplier edita un proveedor.
func UpdateSupplier(orgID, id string, r SupplierRequest) (*Supplier, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	res, err := db.DB.Exec(`UPDATE suppliers SET name=?, phone1=?, phone2=?, email=? WHERE organization_id=? AND id=? AND deleted_at IS NULL`,
		strVal(r.Name), strVal(r.Phone1), trimOrNil(r.Phone2), strVal(r.Email), orgID, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNotFound
	}
	return GetSupplier(orgID, id)
}
