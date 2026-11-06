package handlers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"hook-repair/models"
)

// SidebarSub — opción dentro de un grupo del menú lateral.
type SidebarSub struct {
	Label      string
	Route      string
	Permission string
	Active     bool
}

// SidebarItem — opción del menú lateral (con o sin subopciones).
type SidebarItem struct {
	Label    string
	Icon     string // nombre del ícono de Font Awesome sin "fa-" (p. ej. "building")
	Route    string
	Subs     []SidebarSub
	Active   bool
	Expanded bool
}

// Sidebar — menú lateral de Configuración o Inventario.
type Sidebar struct {
	Title string
	Icon  string
	Items []SidebarItem
}

// PageData — lo que reciben todos los templates de páginas.
type PageData struct {
	Title       string
	User        *models.SessionUser
	SessionJSON template.JS
	Sidebar     *Sidebar
	Script      string   // JS propio de la página (/static/js/...)
	Styles      []string // CSS propio de la página (/static/css/...)
	Heading     string   // título para "Próximamente"
	Description string   // texto corto bajo el título de la página
	Path        string
}

// Can — para usar en los templates: {{if .Can "Inventory.View"}}.
func (p PageData) Can(permission string) bool { return p.User.Can(permission) }

// settingsMenu — menú lateral de Configuración.
func settingsMenu() []SidebarItem {
	return []SidebarItem{
		{Label: "Perfil", Icon: "circle-user", Route: "/settings/profile"},
		{Label: "Organización", Icon: "building", Subs: []SidebarSub{
			{Label: "Información", Route: "/settings/organization"},
			{Label: "Empleados", Route: "/settings/organization/users", Permission: models.ViewUsers},
			{Label: "Roles y permisos", Route: "/settings/organization/roles", Permission: models.ViewRoles},
		}},
		{Label: "Terceros", Icon: "users", Subs: []SidebarSub{
			{Label: "Clientes", Route: "/settings/crm/clients", Permission: models.ViewCustomers},
			{Label: "Proveedores", Route: "/settings/crm/suppliers", Permission: models.ViewSuppliers},
		}},
		{Label: "Inventario", Icon: "boxes-stacked", Subs: []SidebarSub{
			{Label: "Categorías", Route: "/settings/inventory/categories", Permission: models.ViewCatalogs},
			{Label: "Marcas", Route: "/settings/inventory/brands", Permission: models.ViewDevices},
			{Label: "Modelos", Route: "/settings/inventory/models", Permission: models.ViewDevices},
		}},
		{Label: "Reparaciones", Icon: "wrench", Subs: []SidebarSub{
			{Label: "Servicios", Route: "/settings/repairs/services", Permission: models.ViewServices},
			{Label: "Estados", Route: "/settings/repairs/statuses"},
		}},
		{Label: "Cajas", Icon: "cash-register", Route: "/settings/registers"},
	}
}

// inventoryMenu — menú lateral de Inventario.
func inventoryMenu() []SidebarItem {
	return []SidebarItem{
		{Label: "Productos", Icon: "box-open", Route: "/inventory/products"},
		{Label: "Movimientos", Icon: "left-right", Route: "/inventory/movements"},
		{Label: "Órdenes de compra", Icon: "cart-shopping", Route: "/inventory/purchase-orders"},
		{Label: "GRN", Icon: "file-invoice", Route: "/inventory/grn"},
		{Label: "RMA", Icon: "handshake", Route: "/inventory/rma"},
		{Label: "Órdenes especiales", Icon: "star", Route: "/inventory/special-orders"},
		{Label: "Bundles", Icon: "layer-group", Route: "/inventory/bundles"},
		{Label: "Conteo de inventario", Icon: "clipboard-list", Route: "/inventory/stock-count"},
	}
}

// buildSidebar marca la opción activa según la ruta y esconde las subopciones
// a las que el usuario no tiene permiso (un grupo sin subopciones se esconde).
func buildSidebar(title, icon string, items []SidebarItem, path string, u *models.SessionUser) *Sidebar {
	out := make([]SidebarItem, 0, len(items))
	for _, it := range items {
		if len(it.Subs) > 0 {
			subs := make([]SidebarSub, 0, len(it.Subs))
			for _, s := range it.Subs {
				if s.Permission != "" && !u.Can(s.Permission) {
					continue
				}
				s.Active = path == s.Route
				if s.Active {
					it.Active, it.Expanded = true, true
				}
				subs = append(subs, s)
			}
			if len(subs) == 0 {
				continue
			}
			it.Subs = subs
		} else {
			it.Active = path == it.Route || strings.HasPrefix(path, it.Route+"/")
		}
		out = append(out, it)
	}
	return &Sidebar{Title: title, Icon: icon, Items: out}
}

func sessionJSON(u *models.SessionUser) template.JS {
	if u == nil {
		return template.JS("null")
	}
	b, _ := json.Marshal(gin.H{"user": u})
	return template.JS(b)
}

// pageStyles — CSS de cada template (static/css/ sigue la misma estructura
// que templates/: un archivo por template).
var pageStyles = map[string]string{
	"login":                     "/static/css/auth/login.css",
	"dashboard":                 "/static/css/dashboard/dashboard.css",
	"organizations":             "/static/css/organizations/organizations.css",
	"inventory-products":        "/static/css/inventory/inventory.css",
	"inventory-movements":       "/static/css/inventory/inventory.css",
	"inventory-purchase-orders": "/static/css/inventory/inventory.css",
	"settings-profile":          "/static/css/settings/profile.css",
	"settings-organization":     "/static/css/settings/organization.css",
	"settings-roles":            "/static/css/settings/roles.css",
	"settings-categories":       "/static/css/settings/catalogs.css",
	"settings-brands":           "/static/css/settings/catalogs.css",
	"settings-models":           "/static/css/settings/catalogs.css",
	"settings-services":         "/static/css/settings/tables.css",
	"settings-clients":          "/static/css/settings/tables.css",
	"settings-suppliers":        "/static/css/settings/tables.css",
	"settings-users":            "/static/css/settings/tables.css",
	"coming-soon":               "/static/css/pages/coming-soon.css",
	"coming-soon-settings":      "/static/css/pages/coming-soon.css",
	"coming-soon-inventory":     "/static/css/pages/coming-soon.css",
}

// pageDescriptions — texto bajo el título de cada página.
var pageDescriptions = map[string]string{
	"dashboard":                 "Lo más importante del taller de un vistazo.",
	"organizations":             "Los talleres que usan Hook: sus datos y sus usuarios.",
	"inventory-products":        "El catálogo del inventario: precios, existencias y proveedores.",
	"inventory-movements":       "Entradas, salidas y ajustes de stock de cada producto.",
	"inventory-purchase-orders": "Pedidos a proveedores y la recepción de la mercancía.",
	"settings-profile":          "Tu nombre, correo y contraseña.",
	"settings-organization":     "Datos de contacto y ubicación del taller.",
	"settings-users":            "Las personas que entran al sistema y su rol.",
	"settings-roles":            "Qué puede ver y hacer cada rol.",
	"settings-clients":          "Clientes públicos y distribuidores, con sus datos de facturación.",
	"settings-suppliers":        "A quién le compras refacciones y equipos.",
	"settings-categories":       "Cómo se agrupan los productos y las reparaciones.",
	"settings-brands":           "Marcas de los dispositivos que reparas o vendes.",
	"settings-models":           "Modelos de cada marca.",
	"settings-services":         "Las reparaciones que ofreces y sus precios.",
}

func newPage(c *gin.Context, tmpl, title, script string) PageData {
	u := currentUser(c)
	p := PageData{Title: title, User: u, SessionJSON: sessionJSON(u), Script: script, Path: c.Request.URL.Path, Description: pageDescriptions[tmpl]}
	if css, ok := pageStyles[tmpl]; ok {
		p.Styles = []string{css}
	}
	return p
}

// Page renderiza una página sin menú lateral.
func Page(tmpl, title, script string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, tmpl, newPage(c, tmpl, title, script))
	}
}

// SettingsPage renderiza una página dentro del layout de Configuración.
func SettingsPage(tmpl, title, script string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := newPage(c, tmpl, title, script)
		p.Sidebar = buildSidebar("Configuración", "gear", settingsMenu(), p.Path, p.User)
		c.HTML(http.StatusOK, tmpl, p)
	}
}

// InventoryPage renderiza una página dentro del layout de Inventario.
func InventoryPage(tmpl, title, script string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := newPage(c, tmpl, title, script)
		p.Sidebar = buildSidebar("Inventario", "boxes-stacked", inventoryMenu(), p.Path, p.User)
		c.HTML(http.StatusOK, tmpl, p)
	}
}

// ComingSoon — secciones que todavía no existen (Punto de venta, Reparaciones,
// Reportes, y opciones del menú lateral sin página). layout: "", "settings" o "inventory".
func ComingSoon(heading, layout string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tmpl := "coming-soon"
		if layout != "" {
			tmpl = "coming-soon-" + layout
		}
		p := newPage(c, tmpl, heading, "")
		p.Heading = heading
		p.Description = "Esta sección está en construcción."
		switch layout {
		case "settings":
			p.Sidebar = buildSidebar("Configuración", "gear", settingsMenu(), p.Path, p.User)
		case "inventory":
			p.Sidebar = buildSidebar("Inventario", "boxes-stacked", inventoryMenu(), p.Path, p.User)
		}
		c.HTML(http.StatusOK, tmpl, p)
	}
}

// LoginPage — GET /login (si ya hay sesión, al dashboard).
func LoginPage(c *gin.Context) {
	if currentUser(c) != nil {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	c.HTML(http.StatusOK, "login", PageData{Title: "Login", SessionJSON: template.JS("null"), Styles: []string{pageStyles["login"]}})
}

// Home — GET /
func Home(c *gin.Context) {
	if currentUser(c) != nil {
		c.Redirect(http.StatusFound, "/dashboard")
		return
	}
	c.Redirect(http.StatusFound, "/login")
}

// Redirect — atajo para rutas que solo redirigen.
func Redirect(to string) gin.HandlerFunc {
	return func(c *gin.Context) { c.Redirect(http.StatusFound, to) }
}
