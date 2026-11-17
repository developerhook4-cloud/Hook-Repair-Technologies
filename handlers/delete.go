package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"hook-repair/models"
)

// catalogDelete — DELETE .../:id para catálogos, clientes, proveedores,
// servicios y productos. El registro desaparece de las listas (se conserva
// en la base por historial).
func catalogDelete(kind, what, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := idParam(c, "id")
		if !ok {
			return
		}
		if err := models.DeleteCatalogRecord(kind, orgOf(c), id); err != nil {
			respondError(c, err, what)
			return
		}
		mutation(c, http.StatusOK, id, message)
	}
}

var (
	DeleteCategory = catalogDelete("categories", "category", "Categoría eliminada.")
	DeleteBrand    = catalogDelete("brands", "brand", "Marca eliminada.")
	DeleteModel    = catalogDelete("models", "model", "Modelo eliminado.")
	DeleteService  = catalogDelete("services", "service", "Servicio eliminado.")
	DeleteClient   = catalogDelete("clients", "client", "Cliente eliminado.")
	DeleteSupplier = catalogDelete("suppliers", "supplier", "Proveedor eliminado.")
	DeleteProduct  = catalogDelete("products", "product", "Producto eliminado.")
)

// DeleteRole — DELETE /api/:orgId/roles/:id (solo si no tiene usuarios).
func DeleteRole(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if err := models.DeleteRole(currentUser(c), orgOf(c), id); err != nil {
		respondError(c, err, "role")
		return
	}
	mutation(c, http.StatusOK, id, "Rol eliminado.")
}

// DeleteUser — DELETE /api/:orgId/users/:id
func DeleteUser(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	if id == currentUser(c).ID {
		problem(c, http.StatusBadRequest, "Invalid user", "No puedes eliminar tu propio usuario.")
		return
	}
	if err := models.DeleteUser(currentUser(c), orgOf(c), id); err != nil {
		respondError(c, err, "user")
		return
	}
	mutation(c, http.StatusOK, id, "Usuario eliminado.")
}

// DeleteOrganizationUser — DELETE /api/organizations/:id/users/:userId
func DeleteOrganizationUser(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	userID, ok := idParam(c, "userId")
	if !ok {
		return
	}
	if userID == currentUser(c).ID {
		problem(c, http.StatusBadRequest, "Invalid user", "No puedes eliminar tu propio usuario.")
		return
	}
	if err := models.DeleteUser(currentUser(c), id, userID); err != nil {
		respondError(c, err, "user")
		return
	}
	mutation(c, http.StatusOK, userID, "Usuario eliminado.")
}

// UpdatePurchaseOrderStatus — PATCH /api/:orgId/purchase-orders/:id
// {status?, paymentStatus?}
func UpdatePurchaseOrderStatus(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.PurchaseOrderStatusRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.UpdatePurchaseOrderStatus(orgOf(c), id, in)
	if err != nil {
		respondError(c, err, "purchase order")
		return
	}
	c.JSON(http.StatusOK, out)
}
