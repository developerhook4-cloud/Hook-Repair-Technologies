package models

import (
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	"hook-repair/db"
)

// Tipos de movimiento (se guardan como texto).
const (
	MovementEntry      = "Entry"
	MovementExit       = "Exit"
	MovementAdjustment = "Adjustment"
)

// Movement — tabla inventory_movements (con el nombre del producto).
type Movement struct {
	ID            string    `json:"id"`
	ProductID     string    `json:"productId"`
	ProductName   string    `json:"productName"`
	Type          string    `json:"type"`
	Quantity      int       `json:"quantity"`
	PreviousStock int       `json:"previousStock"`
	CurrentStock  int       `json:"currentStock"`
	Reason        *string   `json:"reason"`
	CreatedAtUtc  time.Time `json:"createdAtUtc"`
}

// MovementRequest — nuevo movimiento manual.
type MovementRequest struct {
	ProductID string  `json:"productId"`
	Type      string  `json:"type"`
	Quantity  int     `json:"quantity"`
	Reason    *string `json:"reason"`
}

// canonicalMovementType acepta "Entry"/"entry"/0, etc.
func canonicalMovementType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "entry", "0", "entrada":
		return MovementEntry
	case "exit", "1", "salida":
		return MovementExit
	case "adjustment", "2", "ajuste":
		return MovementAdjustment
	}
	return ""
}

const movementSelect = `
	SELECT mv.id, mv.product_id, COALESCE(p.name, ''), mv.type, mv.quantity,
	       mv.previous_stock, mv.current_stock, mv.reason, mv.created_at
	FROM inventory_movements mv
	LEFT JOIN products p ON p.id = mv.product_id`

func scanMovement(sc interface{ Scan(...any) error }) (*Movement, error) {
	var m Movement
	var reason sql.NullString
	if err := sc.Scan(&m.ID, &m.ProductID, &m.ProductName, &m.Type, &m.Quantity, &m.PreviousStock, &m.CurrentStock, &reason, &m.CreatedAtUtc); err != nil {
		return nil, err
	}
	m.Reason = nullStr(reason)
	m.CreatedAtUtc = m.CreatedAtUtc.UTC()
	return &m, nil
}

// GetMovements — movimientos de la organización (opcionalmente de un producto), del más reciente al más viejo.
func GetMovements(orgID, productID string) ([]Movement, error) {
	q := movementSelect + ` WHERE mv.organization_id = ?`
	args := []any{orgID}
	if productID != "" {
		q += ` AND mv.product_id = ?`
		args = append(args, productID)
	}
	rows, err := db.DB.Query(q+` ORDER BY mv.created_at DESC, mv.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Movement{}
	for rows.Next() {
		m, err := scanMovement(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// GetMovement por id.
func GetMovement(orgID, id string) (*Movement, error) {
	m, err := scanMovement(db.DB.QueryRow(movementSelect+` WHERE mv.organization_id = ? AND mv.id = ?`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// CreateMovement — reglas:
// entrada suma, salida resta, ajuste suma la cantidad (puede ser negativa).
// El stock nunca queda negativo. El producto se bloquea mientras se mueve.
func CreateMovement(orgID string, r MovementRequest) (*Movement, error) {
	if !IsID(r.ProductID) {
		return nil, Invalid("El producto es obligatorio.")
	}
	t := canonicalMovementType(r.Type)
	if t == "" {
		return nil, Invalid("El tipo de movimiento es obligatorio y debe ser válido.")
	}
	if (t == MovementEntry || t == MovementExit) && r.Quantity <= 0 {
		return nil, Invalid("La cantidad debe ser mayor a cero en entradas y salidas.")
	}
	if t == MovementAdjustment && r.Quantity == 0 {
		return nil, Invalid("La cantidad de un ajuste no puede ser cero.")
	}
	if runeLen(strVal(r.Reason)) > 500 {
		return nil, Invalid("El motivo no puede pasar de 500 caracteres.")
	}
	var id string
	err := withTx(func(tx *sql.Tx) error {
		var stock int
		var serialized bool
		// Un producto eliminado ya no recibe movimientos (KAN-94).
		err := tx.QueryRow(`SELECT in_stock, is_serialized FROM products WHERE organization_id = ? AND id = ? AND deleted_at IS NULL FOR UPDATE`, orgID, r.ProductID).Scan(&stock, &serialized)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		// Un movimiento cambiaría el stock sin tocar sus IMEI: el stock de un
		// producto serializado siempre es igual a su número de IMEI (KAN-144).
		// Mientras los movimientos no capturen IMEI (KAN-137) se rechazan.
		if serialized {
			return Invalid("Los productos serializados se ajustan editando sus números de serie (IMEI).")
		}
		delta := r.Quantity
		if t == MovementExit {
			delta = -r.Quantity
		}
		next := int64(stock) + int64(delta)
		if next > math.MaxInt32 {
			return Conflict("El stock resultante no es válido.")
		}
		if next < 0 {
			return Conflict("No hay stock suficiente para este movimiento.")
		}
		if _, err := tx.Exec(`UPDATE products SET in_stock = ? WHERE id = ?`, next, r.ProductID); err != nil {
			return err
		}
		id, err = insert(tx, `INSERT INTO inventory_movements
			(organization_id, product_id, type, quantity, previous_stock, current_stock, reason, created_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			orgID, r.ProductID, t, r.Quantity, stock, next, trimOrNil(r.Reason), time.Now().UTC())
		return err
	})
	if err != nil {
		return nil, err
	}
	return GetMovement(orgID, id)
}
