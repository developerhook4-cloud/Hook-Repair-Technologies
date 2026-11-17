package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"hook-repair/auth"
	"hook-repair/db"
	h "hook-repair/handlers"
	"hook-repair/models"
)

// ⚠️ Ajusta "hook-repair" en los imports de arriba si cambias el nombre del
// módulo en go.mod (primera línea: "module xxxxx").

func loadTemplates() *template.Template {
	funcs := template.FuncMap{
		"lower":     strings.ToLower,
		"hasPrefix": strings.HasPrefix,
		// initials: "Super Admin" → "SA" (avatar de la barra de usuario)
		"initials": func(name string) string {
			out := ""
			for _, w := range strings.Fields(name) {
				for _, r := range w {
					out += strings.ToUpper(string(r))
					break
				}
				if len([]rune(out)) == 2 {
					break
				}
			}
			if out == "" {
				return "?"
			}
			return out
		},
		// dict arma un mapa para pasarle varios valores a un template:
		// {{template "input" dict "Label" "Nombre" "Name" "name"}}
		"dict": func(kv ...any) map[string]any {
			m := make(map[string]any, len(kv)/2)
			for i := 0; i+1 < len(kv); i += 2 {
				if k, ok := kv[i].(string); ok {
					m[k] = kv[i+1]
				}
			}
			return m
		},
	}
	tmpl := template.New("base").Funcs(funcs)
	for _, dir := range []string{"partials", "auth", "dashboard", "pages", "organizations", "settings", "inventory"} {
		tmpl = template.Must(tmpl.ParseGlob("templates/" + dir + "/*.html"))
	}
	return tmpl
}

func main() {
	// .env es opcional: en tu computadora lo usas para las credenciales de
	// Railway; en producción las variables ya vienen en el entorno.
	if err := godotenv.Load(); err != nil {
		log.Println("No se encontró .env — usando variables de entorno del sistema.")
	}

	db.Connect()
	defer db.DB.Close()

	// Crea esquemas/tablas si no existen (en la base actual no toca nada)
	// y el super administrador inicial si todavía no existe.
	db.EnsureSchema()
	models.SeedRootAdmin()
	models.EnsureDefaultRoles()

	auth.InitStore()

	router := setupRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Hook Repair escuchando en :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// setupRouter arma el router con todas las rutas (separado de main para poder
// probarlo con httptest). Requiere que db y auth ya estén inicializados.
func setupRouter() *gin.Engine {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	// Railway (y cualquier proxy) termina el HTTPS y reenvía la IP original.
	_ = router.SetTrustedProxies(h.TrustedProxies)
	// SameOrigin (anti-CSRF) va antes de LoadUser: rechaza sin tocar la base.
	router.Use(h.SecurityHeaders(), h.SameOrigin(), h.LoadUser())

	router.SetHTMLTemplate(loadTemplates())
	router.Static("/static", "./static")
	router.StaticFile("/favicon.ico", "./static/img/favicon.ico")

	router.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// ── Páginas ────────────────────────────────────────────────────────
	router.GET("/", h.Home)
	router.GET("/login", h.LoginPage)
	// GET /logout no cierra sesión (CSRF): solo redirige. Salir es POST /logout.
	router.GET("/logout", h.LogoutRedirect)
	router.POST("/logout", h.LogoutPage)

	pages := router.Group("/", h.RequirePage())
	{
		pages.GET("/dashboard", h.Page("dashboard", "Dashboard", ""))
		pages.GET("/organizations", h.RequirePagePlatform(), h.RequirePagePermission(models.ViewOrganizations),
			h.Page("organizations", "Organizaciones", "/static/js/organizations/organizations.js"))
		pages.GET("/point-of-sale", h.ComingSoon("Punto de venta", ""))
		pages.GET("/repairs", h.ComingSoon("Reparaciones", ""))
		pages.GET("/reports", h.ComingSoon("Reportes", ""))

		// Inventario
		pages.GET("/inventory", h.Redirect("/inventory/products"))
		inv := pages.Group("/inventory", h.RequirePagePermission(models.ViewInventory))
		inv.GET("/products", h.InventoryPage("inventory-products", "Productos", "/static/js/inventory/products.js"))
		inv.GET("/movements", h.InventoryPage("inventory-movements", "Movimientos", "/static/js/inventory/movements.js"))
		inv.GET("/purchase-orders", h.InventoryPage("inventory-purchase-orders", "Órdenes de compra", "/static/js/inventory/purchase-orders.js"))
		inv.GET("/grn", h.ComingSoon("GRN", "inventory"))
		inv.GET("/rma", h.ComingSoon("RMA", "inventory"))
		inv.GET("/special-orders", h.ComingSoon("Órdenes especiales", "inventory"))
		inv.GET("/bundles", h.ComingSoon("Bundles", "inventory"))
		inv.GET("/stock-count", h.ComingSoon("Conteo de inventario", "inventory"))

		// Configuración
		pages.GET("/settings", h.Redirect("/settings/organization"))
		st := pages.Group("/settings")
		st.GET("/profile", h.SettingsPage("settings-profile", "Perfil", "/static/js/settings/profile.js"))
		st.GET("/organization", h.SettingsPage("settings-organization", "Organización", "/static/js/settings/organization.js"))
		st.GET("/organization/users", h.RequirePagePermission(models.ViewUsers),
			h.SettingsPage("settings-users", "Empleados", "/static/js/settings/users.js"))
		st.GET("/organization/roles", h.RequirePagePermission(models.ViewRoles),
			h.SettingsPage("settings-roles", "Roles y permisos", "/static/js/settings/roles.js"))
		st.GET("/crm/clients", h.RequirePagePermission(models.ViewCustomers),
			h.SettingsPage("settings-clients", "Clientes", "/static/js/settings/clients.js"))
		st.GET("/crm/suppliers", h.RequirePagePermission(models.ViewSuppliers),
			h.SettingsPage("settings-suppliers", "Proveedores", "/static/js/settings/suppliers.js"))
		st.GET("/inventory/categories", h.RequirePagePermission(models.ViewCatalogs),
			h.SettingsPage("settings-categories", "Categorías", "/static/js/settings/categories.js"))
		st.GET("/inventory/brands", h.RequirePagePermission(models.ViewDevices),
			h.SettingsPage("settings-brands", "Marcas", "/static/js/settings/brands.js"))
		st.GET("/inventory/models", h.RequirePagePermission(models.ViewDevices),
			h.SettingsPage("settings-models", "Modelos", "/static/js/settings/models.js"))
		st.GET("/repairs/services", h.RequirePagePermission(models.ViewServices),
			h.SettingsPage("settings-services", "Servicios", "/static/js/settings/services.js"))
		st.GET("/repairs/statuses", h.ComingSoon("Estados de reparación", "settings"))
		st.GET("/registers", h.ComingSoon("Cajas", "settings"))
	}

	// ── API de autenticación ───────────────────────────────────────────
	authAPI := router.Group("/api/auth")
	authAPI.POST("/login", h.Login)
	authAPI.POST("/logout", h.Logout)
	authAPI.GET("/me", h.Me)
	authAPI.POST("/refresh-token", h.Me) // compatibilidad con el frontend anterior

	api := router.Group("/api", h.RequireAPIAuth())

	// Perfil del usuario con sesión.
	api.PUT("/users/me", h.UpdateProfile)

	// Organizaciones (plataforma). "mine" va antes que :id. GET /mine lo puede
	// leer cualquier usuario con sesión; PUT /mine solo los administradores.
	api.GET("/organizations/mine", h.GetMyOrganization)
	api.PUT("/organizations/mine", h.RequireOrgAdmin(), h.UpdateMyOrganization)
	// Solo la organización de plataforma (is_platform) puede usar estas rutas,
	// además del permiso Organizations.* de cada una.
	orgs := api.Group("/organizations", h.RequirePlatform())
	orgs.GET("", h.RequirePermission(models.ViewOrganizations), h.ListOrganizations)
	orgs.POST("", h.RequirePermission(models.UpdateOrganizations), h.CreateOrganization)
	orgs.GET("/:id", h.RequirePermission(models.ViewOrganizations), h.GetOrganization)
	orgs.PUT("/:id", h.RequirePermission(models.UpdateOrganizations), h.UpdateOrganization)
	orgs.PATCH("/:id/status", h.RequirePermission(models.DeleteOrganizations), h.UpdateOrganizationStatus)
	orgs.GET("/:id/users", h.RequirePermission(models.ViewOrganizations), h.ListOrganizationUsers)
	orgs.GET("/:id/roles", h.RequirePermission(models.ViewOrganizations), h.ListOrganizationRoles)
	orgs.POST("/:id/users", h.RequirePermission(models.UpdateOrganizations), h.CreateOrganizationUser)
	orgs.PUT("/:id/users/:userId", h.RequirePermission(models.UpdateOrganizations), h.UpdateOrganizationUser)
	orgs.PATCH("/:id/users/:userId/status", h.RequirePermission(models.DeleteOrganizations), h.UpdateOrganizationUserStatus)
	orgs.DELETE("/:id/users/:userId", h.RequirePermission(models.DeleteOrganizations), h.DeleteOrganizationUser)

	// Todo lo de una organización: /api/:orgId/...
	tenant := api.Group("/:orgId", h.RequireTenant())

	crud := func(prefix, view, update, del string, list, get, create, edit, status, remove gin.HandlerFunc) {
		g := tenant.Group(prefix)
		g.GET("", h.RequirePermission(view), list)
		g.GET("/:id", h.RequirePermission(view), get)
		g.POST("", h.RequirePermission(update), create)
		g.PUT("/:id", h.RequirePermission(update), edit)
		g.PATCH("/:id/status", h.RequirePermission(del), status)
		g.DELETE("/:id", h.RequirePermission(del), remove)
	}

	tenant.GET("/roles/permissions", h.RequirePermission(models.ViewRoles), h.ListPermissions)
	crud("/roles", models.ViewRoles, models.UpdateRoles, models.DeleteRoles,
		h.ListRoles, h.GetRole, h.CreateRole, h.UpdateRole, h.UpdateRoleStatus, h.DeleteRole)
	crud("/users", models.ViewUsers, models.UpdateUsers, models.DeleteUsers,
		h.ListUsers, h.GetUser, h.CreateUser, h.UpdateUser, h.UpdateUserStatus, h.DeleteUser)
	crud("/categories", models.ViewCatalogs, models.UpdateCatalogs, models.DeleteCatalogs,
		h.ListCategories, h.GetCategory, h.CreateCategory, h.UpdateCategory, h.UpdateCategoryStatus, h.DeleteCategory)
	crud("/brands", models.ViewCatalogs, models.UpdateCatalogs, models.DeleteCatalogs,
		h.ListBrands, h.GetBrand, h.CreateBrand, h.UpdateBrand, h.UpdateBrandStatus, h.DeleteBrand)
	crud("/models", models.ViewCatalogs, models.UpdateCatalogs, models.DeleteCatalogs,
		h.ListModels, h.GetModel, h.CreateModel, h.UpdateModel, h.UpdateModelStatus, h.DeleteModel)
	crud("/services", models.ViewServices, models.UpdateServices, models.DeleteServices,
		h.ListServices, h.GetService, h.CreateService, h.UpdateService, h.UpdateServiceStatus, h.DeleteService)
	crud("/clients", models.ViewCustomers, models.UpdateCustomers, models.DeleteCustomers,
		h.ListClients, h.GetClient, h.CreateClient, h.UpdateClient, h.UpdateClientStatus, h.DeleteClient)
	crud("/suppliers", models.ViewSuppliers, models.UpdateSuppliers, models.DeleteSuppliers,
		h.ListSuppliers, h.GetSupplier, h.CreateSupplier, h.UpdateSupplier, h.UpdateSupplierStatus, h.DeleteSupplier)
	crud("/products", models.ViewInventory, models.UpdateInventory, models.DeleteInventory,
		h.ListProducts, h.GetProduct, h.CreateProduct, h.UpdateProduct, h.UpdateProductStatus, h.DeleteProduct)

	mv := tenant.Group("/inventory/movements")
	mv.GET("", h.RequirePermission(models.ViewInventory), h.ListMovements)
	mv.GET("/:id", h.RequirePermission(models.ViewInventory), h.GetMovement)
	mv.POST("", h.RequirePermission(models.UpdateInventory), h.CreateMovement)

	po := tenant.Group("/purchase-orders")
	po.GET("", h.RequirePermission(models.ViewInventory), h.ListPurchaseOrders)
	po.GET("/:id", h.RequirePermission(models.ViewInventory), h.GetPurchaseOrder)
	po.POST("", h.RequirePermission(models.UpdateInventory), h.CreatePurchaseOrder)
	po.PATCH("/:id", h.RequirePermission(models.UpdateInventory), h.UpdatePurchaseOrderStatus)

	gr := tenant.Group("/goods-receipts")
	gr.GET("", h.RequirePermission(models.ViewInventory), h.ListGoodsReceipts)
	gr.GET("/:id", h.RequirePermission(models.ViewInventory), h.GetGoodsReceipt)
	gr.POST("", h.RequirePermission(models.UpdateInventory), h.CreateGoodsReceipt)

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"title": "Not found", "status": 404, "detail": "La ruta no existe."})
			return
		}
		c.Redirect(http.StatusFound, "/")
	})

	return router
}
