package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"hook-repair/models"
)

// catalogStatus — PATCH .../:id/status para catálogos, clientes, proveedores,
// servicios y productos. Sin cuerpo desactiva;
// con {"active": true} vuelve a activar.
func catalogStatus(kind, what, message string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := idParam(c, "id")
		if !ok {
			return
		}
		active, ok := statusFromRequest(c, false)
		if !ok {
			return
		}
		if err := models.SetCatalogStatus(kind, orgOf(c), id, active); err != nil {
			respondError(c, err, what)
			return
		}
		mutation(c, http.StatusOK, id, message)
	}
}

// ── Categorías: /api/:orgId/categories ─────────────────────────────────

func ListCategories(c *gin.Context) {
	out, err := models.GetCategories(orgOf(c))
	if err != nil {
		respondError(c, err, "category")
		return
	}
	c.JSON(http.StatusOK, out)
}

func GetCategory(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	out, err := models.GetCategory(orgOf(c), id)
	if err != nil {
		respondError(c, err, "category")
		return
	}
	c.JSON(http.StatusOK, out)
}

func CreateCategory(c *gin.Context) {
	var in models.CategoryRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.CreateCategory(orgOf(c), in)
	if err != nil {
		respondError(c, err, "category")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Categoría creada exitosamente.")
}

func UpdateCategory(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.CategoryRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.UpdateCategory(orgOf(c), id, in)
	if err != nil {
		respondError(c, err, "category")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Categoría actualizada exitosamente.")
}

var UpdateCategoryStatus = catalogStatus("categories", "category", "Estado de la categoría actualizado exitosamente.")

// ── Marcas: /api/:orgId/brands ─────────────────────────────────────────

func ListBrands(c *gin.Context) {
	out, err := models.GetBrands(orgOf(c))
	if err != nil {
		respondError(c, err, "brand")
		return
	}
	c.JSON(http.StatusOK, out)
}

func GetBrand(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	out, err := models.GetBrand(orgOf(c), id)
	if err != nil {
		respondError(c, err, "brand")
		return
	}
	c.JSON(http.StatusOK, out)
}

func CreateBrand(c *gin.Context) {
	var in models.BrandRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.CreateBrand(orgOf(c), in)
	if err != nil {
		respondError(c, err, "brand")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Marca creada exitosamente.")
}

func UpdateBrand(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.BrandRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.UpdateBrand(orgOf(c), id, in)
	if err != nil {
		respondError(c, err, "brand")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Marca actualizada exitosamente.")
}

var UpdateBrandStatus = catalogStatus("brands", "brand", "Estado de la marca actualizado exitosamente.")

// ── Modelos: /api/:orgId/models ────────────────────────────────────────

func ListModels(c *gin.Context) {
	out, err := models.GetDeviceModels(orgOf(c))
	if err != nil {
		respondError(c, err, "model")
		return
	}
	c.JSON(http.StatusOK, out)
}

func GetModel(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	out, err := models.GetDeviceModel(orgOf(c), id)
	if err != nil {
		respondError(c, err, "model")
		return
	}
	c.JSON(http.StatusOK, out)
}

func CreateModel(c *gin.Context) {
	var in models.ModelRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.CreateDeviceModel(orgOf(c), in)
	if err != nil {
		respondError(c, err, "model")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Modelo creado exitosamente.")
}

func UpdateModel(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.ModelRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.UpdateDeviceModel(orgOf(c), id, in)
	if err != nil {
		respondError(c, err, "model")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Modelo actualizado exitosamente.")
}

var UpdateModelStatus = catalogStatus("models", "model", "Estado del modelo actualizado exitosamente.")

// ── Servicios: /api/:orgId/services ────────────────────────────────────

func ListServices(c *gin.Context) {
	out, err := models.GetServices(orgOf(c))
	if err != nil {
		respondError(c, err, "service")
		return
	}
	c.JSON(http.StatusOK, out)
}

func GetService(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	out, err := models.GetService(orgOf(c), id)
	if err != nil {
		respondError(c, err, "service")
		return
	}
	c.JSON(http.StatusOK, out)
}

func CreateService(c *gin.Context) {
	var in models.SaveServiceRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.CreateService(orgOf(c), in)
	if err != nil {
		respondError(c, err, "service")
		return
	}
	mutation(c, http.StatusCreated, out.ID, "Servicio creado exitosamente.")
}

func UpdateService(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.SaveServiceRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.UpdateService(orgOf(c), id, in)
	if err != nil {
		respondError(c, err, "service")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Servicio actualizado exitosamente.")
}

var UpdateServiceStatus = catalogStatus("services", "service", "Estatus del servicio actualizado exitosamente.")

// ── Clientes: /api/:orgId/clients ──────────────────────────────────────

func ListClients(c *gin.Context) {
	out, err := models.GetClients(orgOf(c))
	if err != nil {
		respondError(c, err, "client")
		return
	}
	c.JSON(http.StatusOK, out)
}

func GetClient(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	out, err := models.GetClient(orgOf(c), id)
	if err != nil {
		respondError(c, err, "client")
		return
	}
	c.JSON(http.StatusOK, out)
}

func CreateClient(c *gin.Context) {
	var in models.ClientRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.CreateClient(orgOf(c), in)
	if err != nil {
		respondError(c, err, "client")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Cliente creado exitosamente.")
}

func UpdateClient(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.ClientRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.UpdateClient(orgOf(c), id, in)
	if err != nil {
		respondError(c, err, "client")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Cliente actualizado exitosamente.")
}

var UpdateClientStatus = catalogStatus("clients", "client", "Estado del cliente actualizado exitosamente.")

// ── Proveedores: /api/:orgId/suppliers ─────────────────────────────────

func ListSuppliers(c *gin.Context) {
	out, err := models.GetSuppliers(orgOf(c))
	if err != nil {
		respondError(c, err, "supplier")
		return
	}
	c.JSON(http.StatusOK, out)
}

func GetSupplier(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	out, err := models.GetSupplier(orgOf(c), id)
	if err != nil {
		respondError(c, err, "supplier")
		return
	}
	c.JSON(http.StatusOK, out)
}

func CreateSupplier(c *gin.Context) {
	var in models.SupplierRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.CreateSupplier(orgOf(c), in)
	if err != nil {
		respondError(c, err, "supplier")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Proveedor creado exitosamente.")
}

func UpdateSupplier(c *gin.Context) {
	id, ok := idParam(c, "id")
	if !ok {
		return
	}
	var in models.SupplierRequest
	if !bindJSON(c, &in) {
		return
	}
	out, err := models.UpdateSupplier(orgOf(c), id, in)
	if err != nil {
		respondError(c, err, "supplier")
		return
	}
	mutation(c, http.StatusOK, out.ID, "Proveedor actualizado exitosamente.")
}

var UpdateSupplierStatus = catalogStatus("suppliers", "supplier", "Estado del proveedor actualizado exitosamente.")
