package models

import (
	"hook-repair/db"
)

const dashboardRecentMovementsLimit = 8
const dashboardLowStockLimit = 10

type DashboardLowStockProduct struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	InStock             int    `json:"inStock"`
	StockWarning        int    `json:"stockWarning"`
	MinimumReorderPoint int    `json:"minimumReorderPoint"`
}

type DashboardSummary struct {
	LowStockProductsCount int                        `json:"lowStockProductsCount"`
	PendingOrdersCount    int                        `json:"pendingOrdersCount"`
	ActiveClientsCount    int                        `json:"activeClientsCount"`
	LowStockProducts      []DashboardLowStockProduct `json:"lowStockProducts"`
	RecentMovements       []Movement                 `json:"recentMovements"`
}

func GetDashboardSummary(orgID string) (*DashboardSummary, error) {
	summary := &DashboardSummary{
		LowStockProducts: []DashboardLowStockProduct{},
		RecentMovements:  []Movement{},
	}

	lowStockCondition := `
		in_stock = 0 OR
		(stock_warning > 0 AND in_stock <= stock_warning) OR
		(reorder_point > 0 AND in_stock <= reorder_point)
	`

	if err := db.DB.QueryRow(`
		SELECT COUNT(*)
		FROM products
		WHERE organization_id = ? AND active = 1 AND (`+lowStockCondition+`)`,
		orgID,
	).Scan(&summary.LowStockProductsCount); err != nil {
		return nil, err
	}

	if err := db.DB.QueryRow(`
		SELECT COUNT(*)
		FROM purchase_orders
		WHERE organization_id = ?
		  AND status IN (?, ?, ?, ?)`,
		orgID,
		POPending,
		POWaiting,
		POInProgress,
		POReceivedPartially,
	).Scan(&summary.PendingOrdersCount); err != nil {
		return nil, err
	}

	if err := db.DB.QueryRow(`
		SELECT COUNT(*)
		FROM customers
		WHERE organization_id = ? AND active = 1 AND deleted_at IS NULL`,
		orgID,
	).Scan(&summary.ActiveClientsCount); err != nil {
		return nil, err
	}

	lowStockRows, err := db.DB.Query(`
		SELECT id, name, in_stock, stock_warning, reorder_point
		FROM products
		WHERE organization_id = ? AND active = 1 AND (`+lowStockCondition+`)
		ORDER BY in_stock ASC, name ASC
		LIMIT ?`,
		orgID,
		dashboardLowStockLimit,
	)
	if err != nil {
		return nil, err
	}
	defer lowStockRows.Close()

	for lowStockRows.Next() {
		var product DashboardLowStockProduct
		if err := lowStockRows.Scan(
			&product.ID,
			&product.Name,
			&product.InStock,
			&product.StockWarning,
			&product.MinimumReorderPoint,
		); err != nil {
			return nil, err
		}
		summary.LowStockProducts = append(
			summary.LowStockProducts,
			product,
		)
	}

	if err := lowStockRows.Err(); err != nil {
		return nil, err
	}

	movementRows, err := db.DB.Query(
		movementSelect+`
		 WHERE mv.organization_id = ?
		 ORDER BY mv.created_at DESC, mv.id DESC
		 LIMIT ?`,
		orgID,
		dashboardRecentMovementsLimit,
	)
	if err != nil {
		return nil, err
	}
	defer movementRows.Close()

	for movementRows.Next() {
		movement, err := scanMovement(movementRows)
		if err != nil {
			return nil, err
		}
		summary.RecentMovements = append(
			summary.RecentMovements,
			*movement,
		)
	}

	if err := movementRows.Err(); err != nil {
		return nil, err
	}

	return summary, nil
}
