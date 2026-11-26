package models

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"hook-repair/db"
)

// Estados de la orden y del pago (se guardan como texto).
const (
	POPending           = "Pending"
	POWaiting           = "Waiting"
	POInProgress        = "InProgress"
	POReceivedPartially = "ReceivedPartially"
	POReceived          = "Received"
	POInRma             = "InRma"
	POCanceled          = "Canceled"

	PaymentUnpaid = "Unpaid"
	PaymentPaid   = "Paid"
)

// PurchaseOrderLine — renglón de la orden.
type PurchaseOrderLine struct {
	ID          string  `json:"id"`
	ProductID   string  `json:"productId"`
	ProductName string  `json:"productName"`
	Quantity    int     `json:"quantity"`
	UnitCost    float64 `json:"unitCost"`
	Subtotal    float64 `json:"subtotal"`
	LineTotal   float64 `json:"lineTotal"` // alias de subtotal (lo usa el modal de detalle)
}

// PurchaseOrder — tabla purchase_orders con proveedor, usuario y renglones.
// El número de la orden es su id.
type PurchaseOrder struct {
	ID                  string              `json:"id"`
	PurchaseOrderNumber int                 `json:"purchaseOrderNumber"`
	SupplierID          string              `json:"supplierId"`
	SupplierName        string              `json:"supplierName"`
	CreatedByUserID     string              `json:"createdByUserId"`
	UserName            string              `json:"userName"`
	CreatedAtUtc        time.Time           `json:"createdAtUtc"`
	Status              string              `json:"status"`
	PaymentStatus       string              `json:"paymentStatus"`
	Total               float64             `json:"total"`
	Lines               []PurchaseOrderLine `json:"lines"`
}

// PurchaseOrderProduct — producto pedido (el costo sale del producto).
type PurchaseOrderProduct struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

// PurchaseOrderRequest — nueva orden.
type PurchaseOrderRequest struct {
	SupplierID    string                 `json:"supplierId"`
	Products      []PurchaseOrderProduct `json:"products"`
	Status        string                 `json:"status"`        // opcional: Pending (default), Waiting, InProgress
	PaymentStatus string                 `json:"paymentStatus"` // opcional: Unpaid (default), Paid
}

// manualStatuses — estados que se eligen a mano. Recibido y Recibido
// parcialmente los pone la recepción de mercancía.
var manualStatuses = map[string]bool{POPending: true, POWaiting: true, POInProgress: true, POCanceled: true}

// poTransitions — cambios de estado válidos a mano (KAN-145). Cancelada y
// Recibida son estados finales: no tienen salida. Recibida parcialmente solo
// avanza con otra recepción de mercancía, y En RMA no se elige ni se deja a mano.
var poTransitions = map[string]map[string]bool{
	POPending:    {POWaiting: true, POInProgress: true, POCanceled: true},
	POWaiting:    {POPending: true, POInProgress: true, POCanceled: true},
	POInProgress: {POPending: true, POWaiting: true, POCanceled: true},
}

var paymentStatuses = map[string]bool{PaymentUnpaid: true, PaymentPaid: true}

// PurchaseOrderStatusRequest — cambiar estado y/o pago de una orden.
type PurchaseOrderStatusRequest struct {
	Status        string `json:"status"`
	PaymentStatus string `json:"paymentStatus"`
}

// UpdatePurchaseOrderStatus cambia el estado de la orden y/o el del pago.
func UpdatePurchaseOrderStatus(orgID, id string, r PurchaseOrderStatusRequest) (*PurchaseOrder, error) {
	o, err := GetPurchaseOrder(orgID, id)
	if err != nil {
		return nil, err
	}
	status, payment := o.Status, o.PaymentStatus
	if r.Status != "" && r.Status != o.Status {
		if !manualStatuses[r.Status] {
			return nil, Invalid("Ese estado no se puede elegir a mano.")
		}
		switch o.Status {
		case POCanceled:
			return nil, Invalid("La orden ya está cancelada y no se puede cambiar.")
		case POReceived:
			return nil, Invalid("La orden ya está recibida y no se puede cambiar.")
		case POReceivedPartially:
			return nil, Invalid("La orden ya tiene mercancía recibida; su estado ya no se puede cambiar.")
		}
		if !poTransitions[o.Status][r.Status] {
			return nil, Invalid("Ese cambio de estado no es válido para esta orden.")
		}
		status = r.Status
	}
	if r.PaymentStatus != "" {
		if !paymentStatuses[r.PaymentStatus] {
			return nil, Invalid("El estado del pago no es válido.")
		}
		payment = r.PaymentStatus
	}
	if _, err := db.DB.Exec(`UPDATE purchase_orders SET status = ?, payment_status = ? WHERE organization_id = ? AND id = ?`,
		status, payment, orgID, id); err != nil {
		return nil, err
	}
	return GetPurchaseOrder(orgID, id)
}

const (
	poMaxProducts = 100
	poMaxQuantity = 100000
)

func (r *PurchaseOrderRequest) validate() error {
	if r.Status == "" {
		r.Status = POPending
	}
	if r.PaymentStatus == "" {
		r.PaymentStatus = PaymentUnpaid
	}
	if r.Status != POPending && r.Status != POWaiting && r.Status != POInProgress {
		return Invalid("Una orden nueva solo puede estar Pendiente, En espera o En progreso.")
	}
	if !paymentStatuses[r.PaymentStatus] {
		return Invalid("El estado del pago no es válido.")
	}
	if !IsID(r.SupplierID) {
		return Invalid("El proveedor es obligatorio.")
	}
	if len(r.Products) == 0 || len(r.Products) > poMaxProducts {
		return Invalid("La orden debe tener entre 1 y %d productos.", poMaxProducts)
	}
	seen := map[string]bool{}
	for _, p := range r.Products {
		if !IsID(p.ProductID) {
			return Invalid("Cada renglón debe tener un producto.")
		}
		if seen[p.ProductID] {
			return Invalid("La orden no puede repetir productos.")
		}
		seen[p.ProductID] = true
		if p.Quantity < 1 || p.Quantity > poMaxQuantity {
			return Invalid("La cantidad debe estar entre 1 y %d.", poMaxQuantity)
		}
	}
	return nil
}

const poSelect = `
	SELECT po.id, po.supplier_id, COALESCE(s.name, ''), po.created_by, COALESCE(u.name, ''),
	       po.created_at, po.status, po.payment_status, po.total
	FROM purchase_orders po
	LEFT JOIN suppliers s ON s.id = po.supplier_id
	LEFT JOIN users u ON u.id = po.created_by`

func scanPurchaseOrder(sc interface{ Scan(...any) error }) (*PurchaseOrder, error) {
	var o PurchaseOrder
	if err := sc.Scan(&o.ID, &o.SupplierID, &o.SupplierName, &o.CreatedByUserID, &o.UserName,
		&o.CreatedAtUtc, &o.Status, &o.PaymentStatus, &o.Total); err != nil {
		return nil, err
	}
	o.PurchaseOrderNumber, _ = strconv.Atoi(o.ID)
	o.Lines = []PurchaseOrderLine{}
	return &o, nil
}

// fillPurchaseOrderLines carga los renglones de varias órdenes en una consulta.
func fillPurchaseOrderLines(list []*PurchaseOrder) error {
	if len(list) == 0 {
		return nil
	}
	byID := map[string]*PurchaseOrder{}
	ids := make([]string, 0, len(list))
	for _, o := range list {
		byID[o.ID] = o
		ids = append(ids, o.ID)
	}
	rows, err := db.DB.Query(`SELECT l.purchase_order_id, l.id, l.product_id, COALESCE(p.name, ''), l.quantity, l.unit_cost
		FROM purchase_order_lines l LEFT JOIN products p ON p.id = l.product_id
		WHERE l.purchase_order_id IN (`+placeholders(len(ids))+`) ORDER BY p.name`, anySlice(ids)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var poID string
		var l PurchaseOrderLine
		if err := rows.Scan(&poID, &l.ID, &l.ProductID, &l.ProductName, &l.Quantity, &l.UnitCost); err != nil {
			return err
		}
		sub := math.Round(float64(l.Quantity)*l.UnitCost*100) / 100
		l.Subtotal, l.LineTotal = sub, sub
		if o := byID[poID]; o != nil {
			o.Lines = append(o.Lines, l)
		}
	}
	return rows.Err()
}

// GetPurchaseOrders — órdenes de la organización, de la más nueva a la más vieja.
func GetPurchaseOrders(orgID string) ([]PurchaseOrder, error) {
	rows, err := db.DB.Query(poSelect+` WHERE po.organization_id = ? ORDER BY po.created_at DESC, po.id DESC`, orgID)
	if err != nil {
		return nil, err
	}
	ptrs := []*PurchaseOrder{}
	for rows.Next() {
		o, err := scanPurchaseOrder(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		ptrs = append(ptrs, o)
	}
	rows.Close()
	if err := fillPurchaseOrderLines(ptrs); err != nil {
		return nil, err
	}
	out := make([]PurchaseOrder, len(ptrs))
	for i, o := range ptrs {
		out[i] = *o
	}
	return out, nil
}

// GetPurchaseOrder por id.
func GetPurchaseOrder(orgID, id string) (*PurchaseOrder, error) {
	o, err := scanPurchaseOrder(db.DB.QueryRow(poSelect+` WHERE po.organization_id = ? AND po.id = ?`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return o, fillPurchaseOrderLines([]*PurchaseOrder{o})
}

// CreatePurchaseOrder — crea la orden (por defecto Pendiente y Sin pagar); no mueve stock.
// El costo unitario de cada renglón es el costo actual del producto.
func CreatePurchaseOrder(orgID, userID string, r PurchaseOrderRequest) (*PurchaseOrder, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	if s, err := GetSupplier(orgID, r.SupplierID); err != nil || !s.Active {
		return nil, Invalid("El proveedor no existe o está inactivo.")
	}
	var id string
	err := withTx(func(tx *sql.Tx) error {
		type line struct {
			productID string
			qty       int
			cost      float64
		}
		total := 0.0
		lines := make([]line, 0, len(r.Products))
		for _, p := range r.Products {
			var cost float64
			var active bool
			err := tx.QueryRow(`SELECT cost_price, active FROM products WHERE organization_id = ? AND id = ?`, orgID, p.ProductID).
				Scan(&cost, &active)
			if errors.Is(err, sql.ErrNoRows) || (err == nil && !active) {
				return Invalid("El producto %s no existe o está inactivo.", p.ProductID)
			}
			if err != nil {
				return err
			}
			lines = append(lines, line{p.ProductID, p.Quantity, cost})
			total += float64(p.Quantity) * cost
		}
		var err error
		id, err = insert(tx, `INSERT INTO purchase_orders
			(organization_id, supplier_id, created_by, created_at, status, payment_status, total)
			VALUES (?,?,?,?,?,?,?)`,
			orgID, r.SupplierID, userID, time.Now().UTC(), r.Status, r.PaymentStatus, math.Round(total*100)/100)
		if err != nil {
			return err
		}
		for _, l := range lines {
			if _, err := tx.Exec(`INSERT INTO purchase_order_lines (purchase_order_id, product_id, quantity, unit_cost)
				VALUES (?,?,?,?)`, id, l.productID, l.qty, l.cost); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return GetPurchaseOrder(orgID, id)
}

// ── Recepciones de mercancía (GRN) ─────────────────────────────────────

// GoodsReceiptLine — renglón recibido.
type GoodsReceiptLine struct {
	ID                  string `json:"id"`
	PurchaseOrderLineID string `json:"purchaseOrderLineId"`
	ProductID           string `json:"productId"`
	ProductName         string `json:"productName"`
	OrderedQuantity     int    `json:"orderedQuantity"`
	ReceivedQuantity    int    `json:"receivedQuantity"`
}

// GoodsReceipt — tabla goods_receipts. El número de la recepción es su id.
type GoodsReceipt struct {
	ID                  string             `json:"id"`
	GoodsReceiptNumber  int                `json:"goodsReceiptNumber"`
	PurchaseOrderID     string             `json:"purchaseOrderId"`
	ReceivedByUserID    string             `json:"receivedByUserId"`
	CreatedAtUtc        time.Time          `json:"createdAtUtc"`
	Notes               *string            `json:"notes"`
	PurchaseOrderStatus string             `json:"purchaseOrderStatus"`
	Lines               []GoodsReceiptLine `json:"lines"`
}

// GoodsReceiptLineRequest — cantidad recibida de un renglón (solo si hubo diferencia).
type GoodsReceiptLineRequest struct {
	PurchaseOrderLineID string `json:"purchaseOrderLineId"`
	ReceivedQuantity    *int   `json:"receivedQuantity"`
}

// GoodsReceiptRequest — si Lines viene null se recibe todo lo pendiente.
type GoodsReceiptRequest struct {
	PurchaseOrderID string                    `json:"purchaseOrderId"`
	Lines           []GoodsReceiptLineRequest `json:"lines"`
	Notes           *string                   `json:"notes"`
}

func (r *GoodsReceiptRequest) validate() error {
	if !IsID(r.PurchaseOrderID) {
		return Invalid("La orden de compra es obligatoria.")
	}
	if runeLen(strVal(r.Notes)) > 500 {
		return Invalid("Las notas no pueden pasar de 500 caracteres.")
	}
	if len(r.Lines) > poMaxProducts {
		return Invalid("No puede haber más de %d renglones.", poMaxProducts)
	}
	seen := map[string]bool{}
	for _, l := range r.Lines {
		if !IsID(l.PurchaseOrderLineID) {
			return Invalid("Cada renglón necesita su purchaseOrderLineId.")
		}
		if seen[l.PurchaseOrderLineID] {
			return Invalid("Los renglones no se pueden repetir.")
		}
		seen[l.PurchaseOrderLineID] = true
		if l.ReceivedQuantity != nil && (*l.ReceivedQuantity < 0 || *l.ReceivedQuantity > poMaxQuantity) {
			return Invalid("La cantidad recibida debe estar entre 0 y %d.", poMaxQuantity)
		}
	}
	return nil
}

const grSelect = `
	SELECT gr.id, gr.purchase_order_id, gr.received_by, gr.created_at, gr.notes, COALESCE(po.status, '')
	FROM goods_receipts gr
	LEFT JOIN purchase_orders po ON po.id = gr.purchase_order_id`

func scanGoodsReceipt(sc interface{ Scan(...any) error }) (*GoodsReceipt, error) {
	var g GoodsReceipt
	var notes sql.NullString
	if err := sc.Scan(&g.ID, &g.PurchaseOrderID, &g.ReceivedByUserID, &g.CreatedAtUtc, &notes, &g.PurchaseOrderStatus); err != nil {
		return nil, err
	}
	g.GoodsReceiptNumber, _ = strconv.Atoi(g.ID)
	g.Notes = nullStr(notes)
	g.Lines = []GoodsReceiptLine{}
	return &g, nil
}

// fillGoodsReceiptLines carga los renglones de varias recepciones en una consulta.
func fillGoodsReceiptLines(list []*GoodsReceipt) error {
	if len(list) == 0 {
		return nil
	}
	byID := map[string]*GoodsReceipt{}
	ids := make([]string, 0, len(list))
	for _, g := range list {
		byID[g.ID] = g
		ids = append(ids, g.ID)
	}
	rows, err := db.DB.Query(`SELECT l.goods_receipt_id, l.id, l.purchase_order_line_id, l.product_id, COALESCE(p.name, ''),
		       l.ordered_quantity, l.received_quantity
		FROM goods_receipt_lines l LEFT JOIN products p ON p.id = l.product_id
		WHERE l.goods_receipt_id IN (`+placeholders(len(ids))+`) ORDER BY p.name`, anySlice(ids)...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var grID string
		var l GoodsReceiptLine
		if err := rows.Scan(&grID, &l.ID, &l.PurchaseOrderLineID, &l.ProductID, &l.ProductName, &l.OrderedQuantity, &l.ReceivedQuantity); err != nil {
			return err
		}
		if g := byID[grID]; g != nil {
			g.Lines = append(g.Lines, l)
		}
	}
	return rows.Err()
}

// GetGoodsReceipts — recepciones de la organización (opcionalmente de una orden).
func GetGoodsReceipts(orgID, purchaseOrderID string) ([]GoodsReceipt, error) {
	q := grSelect + ` WHERE gr.organization_id = ?`
	args := []any{orgID}
	if purchaseOrderID != "" {
		q += ` AND gr.purchase_order_id = ?`
		args = append(args, purchaseOrderID)
	}
	rows, err := db.DB.Query(q+` ORDER BY gr.created_at DESC, gr.id DESC`, args...)
	if err != nil {
		return nil, err
	}
	ptrs := []*GoodsReceipt{}
	for rows.Next() {
		g, err := scanGoodsReceipt(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		ptrs = append(ptrs, g)
	}
	rows.Close()
	if err := fillGoodsReceiptLines(ptrs); err != nil {
		return nil, err
	}
	out := make([]GoodsReceipt, len(ptrs))
	for i, g := range ptrs {
		out[i] = *g
	}
	return out, nil
}

// GetGoodsReceipt por id.
func GetGoodsReceipt(orgID, id string) (*GoodsReceipt, error) {
	g, err := scanGoodsReceipt(db.DB.QueryRow(grSelect+` WHERE gr.organization_id = ? AND gr.id = ?`, orgID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return g, fillGoodsReceiptLines([]*GoodsReceipt{g})
}

var receivableStatuses = map[string]bool{POPending: true, POWaiting: true, POInProgress: true, POReceivedPartially: true}

// CreateGoodsReceipt — bloquea la orden, autocompleta lo pendiente, no deja
// recibir de más, suma al stock con un movimiento de entrada por renglón y
// marca la orden como Recibida o Recibida parcialmente.
func CreateGoodsReceipt(orgID, userID string, r GoodsReceiptRequest) (*GoodsReceipt, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	var id string
	err := withTx(func(tx *sql.Tx) error {
		var status string
		err := tx.QueryRow(`SELECT status FROM purchase_orders WHERE id = ? AND organization_id = ? FOR UPDATE`,
			r.PurchaseOrderID, orgID).Scan(&status)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !receivableStatuses[status] {
			return Conflict("Una orden con estado %s no puede recibir mercancía.", status)
		}

		type poLine struct {
			id, productID, productName string
			qty, received, stock       int
			serialized                 bool
		}
		rows, err := tx.Query(`
			SELECT l.id, l.product_id, COALESCE(p.name, ''), l.quantity,
			       COALESCE((SELECT SUM(g.received_quantity) FROM goods_receipt_lines g WHERE g.purchase_order_line_id = l.id), 0),
			       p.in_stock, p.is_serialized
			FROM purchase_order_lines l
			JOIN products p ON p.id = l.product_id
			WHERE l.purchase_order_id = ?
			ORDER BY p.name
			FOR UPDATE`, r.PurchaseOrderID)
		if err != nil {
			return err
		}
		lines := []poLine{}
		for rows.Next() {
			var l poLine
			if err := rows.Scan(&l.id, &l.productID, &l.productName, &l.qty, &l.received, &l.stock, &l.serialized); err != nil {
				rows.Close()
				return err
			}
			lines = append(lines, l)
		}
		rows.Close()

		requested := map[string]*int{}
		if r.Lines != nil {
			known := map[string]bool{}
			for _, l := range lines {
				known[l.id] = true
			}
			for _, l := range r.Lines {
				if !known[l.PurchaseOrderLineID] {
					return Invalid("El renglón %s no pertenece a la orden de compra.", l.PurchaseOrderLineID)
				}
				requested[l.PurchaseOrderLineID] = l.ReceivedQuantity
			}
		}

		now := time.Now().UTC()
		id, err = insert(tx, `INSERT INTO goods_receipts (organization_id, purchase_order_id, received_by, notes, created_at)
			VALUES (?,?,?,?,?)`, orgID, r.PurchaseOrderID, userID, trimOrNil(r.Notes), now)
		if err != nil {
			return err
		}
		fully := true
		receivedAny := false
		for _, l := range lines {
			pending := l.qty - l.received
			if pending <= 0 {
				continue
			}
			qty := pending
			if v, ok := requested[l.id]; ok && v != nil {
				qty = *v
			}
			if qty > pending {
				return Invalid("La cantidad recibida de %s no puede pasar de lo pendiente (%d).", l.productName, pending)
			}
			if qty < pending {
				fully = false
			}
			if qty == 0 {
				continue
			}
			if l.serialized {
				return Invalid("%s es serializado; todavía no se pueden recibir productos serializados.", l.productName)
			}
			receivedAny = true
			if _, err := tx.Exec(`INSERT INTO goods_receipt_lines
				(goods_receipt_id, purchase_order_line_id, product_id, ordered_quantity, received_quantity)
				VALUES (?,?,?,?,?)`, id, l.id, l.productID, l.qty, qty); err != nil {
				return err
			}
			next := l.stock + qty
			if _, err := tx.Exec(`UPDATE products SET in_stock = ? WHERE id = ?`, next, l.productID); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO inventory_movements
				(organization_id, product_id, type, quantity, previous_stock, current_stock, reason, created_at)
				VALUES (?,?,'Entry',?,?,?,?,?)`,
				orgID, l.productID, qty, l.stock, next, fmt.Sprintf("Recepción de orden de compra #%s", r.PurchaseOrderID), now); err != nil {
				return err
			}
		}
		if !receivedAny {
			return Invalid("La recepción debe recibir al menos una unidad.")
		}
		newStatus := POReceivedPartially
		if fully {
			newStatus = POReceived
		}
		_, err = tx.Exec(`UPDATE purchase_orders SET status = ? WHERE id = ?`, newStatus, r.PurchaseOrderID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return GetGoodsReceipt(orgID, id)
}
