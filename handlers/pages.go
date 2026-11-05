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
	Script      string // JS propio de la página (/static/js/...)
	Heading     string // título para "Próximamente"
	Path        string
}

// Can — para usar en los templates: {{if .Can "Inventory.View"}}.
func (p PageData) Can(permission string) bool { return p.User.Can(permission) }

// settingsMenu — mismo menú que features/configuration/layout.
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

// inventoryMenu — mismo menú que features/inventory/layout.
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

func newPage(c *gin.Context, title, script string) PageData {
	u := currentUser(c)
	return PageData{Title: title, User: u, SessionJSON: sessionJSON(u), Script: script, Path: c.Request.URL.Path}
}

// Page renderiza una página sin menú lateral.
func Page(tmpl, title, script string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, tmpl, newPage(c, title, script))
	}
}

// SettingsPage renderiza una página dentro del layout de Configuración.
func SettingsPage(tmpl, title, script string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := newPage(c, title, script)
		p.Sidebar = buildSidebar("Configuración", "gear", settingsMenu(), p.Path, p.User)
		c.HTML(http.StatusOK, tmpl, p)
	}
}

// InventoryPage renderiza una página dentro del layout de Inventario.
func InventoryPage(tmpl, title, script string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := newPage(c, title, script)
		p.Sidebar = buildSidebar("Inventario", "boxes-stacked", inventoryMenu(), p.Path, p.User)
		c.HTML(http.StatusOK, tmpl, p)
	}
}

// ComingSoon — secciones que todavía no existen (Punto de venta, Reparaciones,
// Reportes, y opciones del menú lateral sin página). layout: "", "settings" o "inventory".
func ComingSoon(heading, layout string) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := newPage(c, heading, "")
		p.Heading = heading
		tmpl := "coming-soon"
		switch layout {
		case "settings":
			p.Sidebar = buildSidebar("Configuración", "gear", settingsMenu(), p.Path, p.User)
			tmpl = "coming-soon-settings"
		case "inventory":
			p.Sidebar = buildSidebar("Inventario", "boxes-stacked", inventoryMenu(), p.Path, p.User)
			tmpl = "coming-soon-inventory"
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
	c.HTML(http.StatusOK, "login", PageData{Title: "Login", SessionJSON: template.JS("null")})
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
