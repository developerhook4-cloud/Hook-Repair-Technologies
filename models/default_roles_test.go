package models

import "testing"

func TestDefaultRolesIncludeRequiredReadPermissions(t *testing.T) {
	roles := map[string][]string{}

	for _, role := range DefaultRoles {
		roles[role.Name] = role.Permissions
	}

	checks := []struct {
		role       string
		permission string
	}{
		{"Gerente", ViewRoles},
		{"Técnico", ViewCatalogs},
		{"Técnico", ViewSuppliers},
		{"Cajero", ViewCatalogs},
		{"Cajero", ViewSuppliers},
	}

	for _, check := range checks {
		if !HasPermission(roles[check.role], check.permission) {
			t.Errorf(
				"el rol %q debe incluir el permiso %q",
				check.role,
				check.permission,
			)
		}
	}
}

func TestMergeDefaultPermissionsPreservesExistingAndAddsMissing(t *testing.T) {
	current := []string{
		ViewInventory,
		"Custom.View",
	}

	defaults := []string{
		ViewInventory,
		ViewCatalogs,
	}

	merged := mergeDefaultPermissions(current, defaults)

	if len(merged) != 3 {
		t.Fatalf(
			"se esperaban 3 permisos después de fusionar, se recibieron %d",
			len(merged),
		)
	}

	for _, permission := range []string{
		ViewInventory,
		"Custom.View",
		ViewCatalogs,
	} {
		if !HasPermission(merged, permission) {
			t.Errorf(
				"el permiso %q debería conservarse o agregarse",
				permission,
			)
		}
	}
}
