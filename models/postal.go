package models

import (
	"database/sql"
	"errors"

	"hook-repair/db"
)

// ErrPostalCatalogEmpty — el catálogo de códigos postales aún no se carga (→ 503).
var ErrPostalCatalogEmpty = errors.New("catálogo de códigos postales vacío")

// Colonia — un asentamiento (colonia, fraccionamiento, pueblo...) de un código postal.
type Colonia struct {
	Nombre string `json:"nombre"`
	Tipo   string `json:"tipo"`
}

// PostalCode — respuesta de GET /api/postal-codes/:cp (KAN-102).
type PostalCode struct {
	CP        string    `json:"cp"`
	Estado    string    `json:"estado"`
	Municipio string    `json:"municipio"`
	Ciudad    string    `json:"ciudad"`
	Pais      string    `json:"pais"`
	Colonias  []Colonia `json:"colonias"`
}

// ValidPostalCode — exactamente 5 dígitos ASCII.
func ValidPostalCode(cp string) bool {
	if len(cp) != 5 {
		return false
	}
	for i := 0; i < 5; i++ {
		if cp[i] < '0' || cp[i] > '9' {
			return false
		}
	}
	return true
}

// LookupPostalCode busca un código postal en el catálogo SEPOMEX. Devuelve
// ErrNotFound si no existe y ErrPostalCatalogEmpty si el catálogo todavía no
// está cargado. Estado y municipio salen de la primera fila; la ciudad es la
// primera no vacía y, si el catálogo no trae ciudad (zonas rurales), se usa
// el municipio.
func LookupPostalCode(cp string) (*PostalCode, error) {
	if !ValidPostalCode(cp) {
		return nil, Invalid("El código postal debe tener 5 dígitos.")
	}
	rows, err := db.DB.Query(`SELECT colonia, tipo_asentamiento, municipio, ciudad, estado
		FROM postal_codes WHERE cp = ? ORDER BY colonia, id`, cp)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := &PostalCode{CP: cp, Pais: "México", Colonias: []Colonia{}}
	seen := map[Colonia]bool{}
	for rows.Next() {
		var col, tipo, mun, ciudad, estado string
		if err := rows.Scan(&col, &tipo, &mun, &ciudad, &estado); err != nil {
			return nil, err
		}
		if out.Estado == "" {
			out.Estado, out.Municipio = estado, mun
		}
		if out.Ciudad == "" {
			out.Ciudad = ciudad
		}
		if c := (Colonia{col, tipo}); !seen[c] {
			seen[c] = true
			out.Colonias = append(out.Colonias, c)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out.Colonias) == 0 {
		var one int
		err := db.DB.QueryRow(`SELECT 1 FROM postal_codes LIMIT 1`).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPostalCatalogEmpty
		}
		if err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	if out.Ciudad == "" {
		out.Ciudad = out.Municipio
	}
	return out, nil
}
