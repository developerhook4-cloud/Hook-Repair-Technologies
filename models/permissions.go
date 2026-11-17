package models

import "strings"

// Permisos del sistema (Módulo.Acción).
const (
	ViewOrganizations   = "Organizations.View"
	UpdateOrganizations = "Organizations.Update"
	DeleteOrganizations = "Organizations.Delete"

	ViewUsers   = "Users.View"
	UpdateUsers = "Users.Update"
	DeleteUsers = "Users.Delete"

	ViewRoles   = "Roles.View"
	UpdateRoles = "Roles.Update"
	DeleteRoles = "Roles.Delete"

	ViewPos   = "Pos.View"
	UpdatePos = "Pos.Update"
	DeletePos = "Pos.Delete"

	ViewRepairs   = "Repairs.View"
	UpdateRepairs = "Repairs.Update"
	DeleteRepairs = "Repairs.Delete"

	ViewInventory   = "Inventory.View"
	UpdateInventory = "Inventory.Update"
	DeleteInventory = "Inventory.Delete"

	ViewCustomers   = "Customers.View"
	UpdateCustomers = "Customers.Update"
	DeleteCustomers = "Customers.Delete"

	ViewSuppliers   = "Suppliers.View"
	UpdateSuppliers = "Suppliers.Update"
	DeleteSuppliers = "Suppliers.Delete"

	ViewCatalogs   = "Catalogs.View"
	UpdateCatalogs = "Catalogs.Update"
	DeleteCatalogs = "Catalogs.Delete"

	ViewDevices   = "Devices.View"
	UpdateDevices = "Devices.Update"
	DeleteDevices = "Devices.Delete"

	ViewServices   = "Services.View"
	UpdateServices = "Services.Update"
	DeleteServices = "Services.Delete"

	ViewRepairStatus   = "RepairStatus.View"
	UpdateRepairStatus = "RepairStatus.Update"
	DeleteRepairStatus = "RepairStatus.Delete"

	ViewReports   = "Reports.View"
	UpdateReports = "Reports.Update"
	DeleteReports = "Reports.Delete"
)

// AllPermissions en el mismo orden en que se declaran (el frontend saca los
// módulos de aquí, en este orden).
var AllPermissions = []string{
	ViewOrganizations, UpdateOrganizations, DeleteOrganizations,
	ViewUsers, UpdateUsers, DeleteUsers,
	ViewRoles, UpdateRoles, DeleteRoles,
	ViewPos, UpdatePos, DeletePos,
	ViewRepairs, UpdateRepairs, DeleteRepairs,
	ViewInventory, UpdateInventory, DeleteInventory,
	ViewCustomers, UpdateCustomers, DeleteCustomers,
	ViewSuppliers, UpdateSuppliers, DeleteSuppliers,
	ViewCatalogs, UpdateCatalogs, DeleteCatalogs,
	ViewDevices, UpdateDevices, DeleteDevices,
	ViewServices, UpdateServices, DeleteServices,
	ViewRepairStatus, UpdateRepairStatus, DeleteRepairStatus,
	ViewReports, UpdateReports, DeleteReports,
}

// CanonicalPermission devuelve el permiso con su escritura oficial, o "" si no existe.
func CanonicalPermission(p string) string {
	p = strings.TrimSpace(p)
	for _, valid := range AllPermissions {
		if strings.EqualFold(valid, p) {
			return valid
		}
	}
	return ""
}

// HasPermission revisa una lista de permisos (sin distinguir mayúsculas).
func HasPermission(perms []string, required string) bool {
	for _, p := range perms {
		if p == "*" || strings.EqualFold(p, required) {
			return true
		}
	}
	return false
}

// defaultRole es un rol que se crea solo con cada organización nueva.
type defaultRole struct {
	Name        string
	Permissions []string
}

// DefaultRoles — roles que se crean con cada organización nueva.
var DefaultRoles = []defaultRole{
	{"Administrador", []string{
		ViewUsers, UpdateUsers, DeleteUsers,
		ViewRoles, UpdateRoles, DeleteRoles,
		ViewPos, UpdatePos, DeletePos,
		ViewRepairs, UpdateRepairs, DeleteRepairs,
		ViewInventory, UpdateInventory, DeleteInventory,
		ViewCustomers, UpdateCustomers, DeleteCustomers,
		ViewSuppliers, UpdateSuppliers, DeleteSuppliers,
		ViewCatalogs, UpdateCatalogs, DeleteCatalogs,
		ViewDevices, UpdateDevices, DeleteDevices,
		ViewServices, UpdateServices, DeleteServices,
		ViewRepairStatus, UpdateRepairStatus, DeleteRepairStatus,
		ViewReports, UpdateReports, DeleteReports,
	}},
	{"Gerente", []string{
		ViewUsers,
		ViewPos, UpdatePos, DeletePos,
		ViewRepairs, UpdateRepairs, DeleteRepairs,
		ViewInventory, UpdateInventory, DeleteInventory,
		ViewCustomers, UpdateCustomers, DeleteCustomers,
		ViewSuppliers, UpdateSuppliers, DeleteSuppliers,
		ViewCatalogs, UpdateCatalogs, DeleteCatalogs,
		ViewDevices, UpdateDevices, DeleteDevices,
		ViewServices, UpdateServices, DeleteServices,
		ViewRepairStatus, UpdateRepairStatus, DeleteRepairStatus,
		ViewReports, UpdateReports, DeleteReports,
	}},
	{"Técnico", []string{
		ViewRepairs, UpdateRepairs,
		ViewInventory, UpdateInventory,
		ViewDevices, UpdateDevices,
		ViewServices, UpdateServices,
		ViewCustomers,
	}},
	{"Cajero", []string{
		ViewPos, UpdatePos,
		ViewRepairs, UpdateRepairs,
		ViewInventory, UpdateInventory,
		ViewCustomers, UpdateCustomers,
	}},
}

// platformModule — prefijo de los permisos de administración de plataforma.
const platformModule = "Organizations."

// IsPlatformPermission dice si el permiso es de Organizations.* (solo la
// organización de plataforma puede tener o dar estos permisos).
func IsPlatformPermission(p string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(p)), strings.ToLower(platformModule))
}

// WithoutPlatformPermissions devuelve la lista sin los permisos Organizations.*.
func WithoutPlatformPermissions(perms []string) []string {
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		if !IsPlatformPermission(p) {
			out = append(out, p)
		}
	}
	return out
}
