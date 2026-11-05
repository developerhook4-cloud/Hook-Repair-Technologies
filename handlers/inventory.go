package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"hook-repair/models"
)

// ── Productos: /api/:orgId/products ────────────────────────────────────

// ListProducts — GET (?active=true para solo los activos).
func ListProducts(c *gin.Context) {
	out, err := models.GetProducts(orgOf(c), c.Query("active") == "true")
	if err != nil {
		respondError(c, err, "product")
		return
	}
	c.JSON(http.StatusOK, out)
}

func GetProduct(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	out, err := models.GetProduct(orgOf(c), id)
	if err != nil {
		respondError(c, err, "product")
		return
	}
	c.JSON(http.StatusOK, out)
}

func CreateProduct(c *gin.Context) {
	var in models.ProductRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.CreateProduct(orgOf(c), in)
	if err != nil {
		respondError(c, err, "product")
		return
	}
	mutation(c, http.StatusCreated, out.ID, "Producto creado exitosamente.")
}

func UpdateProduct(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.ProductRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.UpdateProduct(orgOf(c), id, in)
	if err != nil {
		respondError(c, err, "product")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Producto actualizado exitosamente.")
}

var UpdateProductStatus = catalogStatus("products", "product", "Estatus del producto actualizado exitosamente.")

// ── Movimientos: /api/:orgId/inventory/movements ───────────────────────

// ListMovements — GET (?productId= para filtrar).
func ListMovements(c *gin.Context) {
	productID := c.Query("productId")
	if productID != "" && !models.IsID(productID) {
		problem(c, http.StatusBadRequest, "Invalid inventory movement", "productId no es válido.")
		return
	}
	out, err := models.GetMovements(orgOf(c), productID)
	if err != nil {
		respondError(c, err, "inventory movement")
		return
	}
	c.JSON(http.StatusOK, out)
}

func GetMovement(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	out, err := models.GetMovement(orgOf(c), id)
	if err != nil {
		respondError(c, err, "inventory movement")
		return
	}
	c.JSON(http.StatusOK, out)
}

func CreateMovement(c *gin.Context) {
	var in models.MovementRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.CreateMovement(orgOf(c), in)
	if err != nil {
		respondError(c, err, "inventory movement")
		return
	}
	c.JSON(http.StatusCreated, out)
}

// ── Órdenes de compra: /api/:orgId/purchase-orders ─────────────────────

func ListPurchaseOrders(c *gin.Context) {
	out, err := models.GetPurchaseOrders(orgOf(c))
	if err != nil {
		respondError(c, err, "purchase order")
		return
	}
	c.JSON(http.StatusOK, out)
}

func GetPurchaseOrder(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	out, err := models.GetPurchaseOrder(orgOf(c), id)
	if err != nil {
		respondError(c, err, "purchase order")
		return
	}
	c.JSON(http.StatusOK, out)
}

func CreatePurchaseOrder(c *gin.Context) {
	var in models.PurchaseOrderRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.CreatePurchaseOrder(orgOf(c), currentUser(c).ID, in)
	if err != nil {
		respondError(c, err, "purchase order")
		return
	}
	mutation(c, http.StatusCreated, out.ID, "Orden de compra creada exitosamente.")
}

// ── Recepciones (GRN): /api/:orgId/goods-receipts ──────────────────────

// ListGoodsReceipts — GET (?purchaseOrderId= para filtrar).
func ListGoodsReceipts(c *gin.Context) {
	poID := c.Query("purchaseOrderId")
	if poID != "" && !models.IsID(poID) {
		problem(c, http.StatusBadRequest, "Invalid goods receipt", "purchaseOrderId no es válido.")
		return
	}
	out, err := models.GetGoodsReceipts(orgOf(c), poID)
	if err != nil {
		respondError(c, err, "goods receipt")
		return
	}
	c.JSON(http.StatusOK, out)
}

func GetGoodsReceipt(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	out, err := models.GetGoodsReceipt(orgOf(c), id)
	if err != nil {
		respondError(c, err, "goods receipt")
		return
	}
	c.JSON(http.StatusOK, out)
}

func CreateGoodsReceipt(c *gin.Context) {
	var in models.GoodsReceiptRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.CreateGoodsReceipt(orgOf(c), currentUser(c).ID, in)
	if err != nil {
		respondError(c, err, "goods receipt")
		return
	}
	mutation(c, http.StatusCreated, out.ID, "Recepción registrada exitosamente.")
}
