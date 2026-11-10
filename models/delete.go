package models

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"unicode"

	"hook-repair/db"
)

// ── Eliminar ───────────────────────────────────────────────────────────
//
// "Eliminar" no borra la fila: llena deleted_at y la desactiva. Así lo que ya
// la usa (órdenes, movimientos, productos…) conserva su historial, pero deja de
// aparecer en las listas y su nombre se puede volver a usar.

// deletableTables — catálogos que se eliminan con DeleteCatalogRecord.
var deletableTables = map[string]string{
	"categories": "categories",
	"brands":     "brands",
	"models":     "device_models",
	"services":   "repair_services",
	"clients":    "customers",
	"suppliers":  "suppliers",
	"products":   "products",
}

// DeleteCatalogRecord elimina un registro de uno de los catálogos de arriba.
func DeleteCatalogRecord(kind, orgID, id string) error {
	table, ok := deletableTables[kind]
	if !ok {
		return errors.New("catálogo desconocido: " + kind)
	}
	return withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE `+table+` SET deleted_at = UTC_TIMESTAMP(), active = 0
			WHERE organization_id = ? AND id = ? AND deleted_at IS NULL`, orgID, id)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
		// Los modelos de una marca eliminada se van con ella.
		if kind == "brands" {
			_, err = tx.Exec(`UPDATE device_models SET deleted_at = UTC_TIMESTAMP(), active = 0
				WHERE organization_id = ? AND brand_id = ? AND deleted_at IS NULL`, orgID, id)
		}
		return err
	})
}

// DeleteUser elimina un usuario (no puede ser el último administrador).
func DeleteUser(actor *SessionUser, orgID, userID string) error {
	u, err := GetUser(orgID, userID)
	if err != nil {
		return err
	}
	if err := ensureCanTouchUser(actor, orgID, u); err != nil {
		return err
	}
	if err := ensureAdminRemains(orgID, userID, ""); err != nil {
		return err
	}
	// Eliminar también invalida sus sesiones (KAN-143).
	_, err = db.DB.Exec(`UPDATE users SET deleted_at = UTC_TIMESTAMP(), active = 0, session_version = session_version + 1
		WHERE organization_id = ? AND id = ? AND deleted_at IS NULL`, orgID, userID)
	return err
}

// DeleteRole elimina un rol que ya no tenga usuarios.
func DeleteRole(actor *SessionUser, orgID, roleID string) error {
	role, err := GetRole(orgID, roleID)
	if err != nil {
		return err
	}
	if err := ensureCanTouchRole(actor, orgID, role); err != nil {
		return err
	}
	var users int
	if err := db.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE organization_id = ? AND role_id = ? AND deleted_at IS NULL`,
		orgID, roleID).Scan(&users); err != nil {
		return err
	}
	if users > 0 {
		return Conflict("No se puede eliminar el rol porque tiene %d usuario(s) asignado(s). Cámbialos de rol primero.", users)
	}
	_, err = db.DB.Exec(`UPDATE roles SET deleted_at = UTC_TIMESTAMP(), active = 0
		WHERE organization_id = ? AND id = ? AND deleted_at IS NULL`, orgID, roleID)
	return err
}

// ── Siempre debe quedar un administrador ───────────────────────────────

// IsAdminRole — un rol "administrador" es el que puede administrar usuarios y roles.
func IsAdminRole(perms []string) bool {
	return HasPermission(perms, UpdateUsers) && HasPermission(perms, UpdateRoles)
}

// countAdmins cuenta los usuarios activos con rol de administrador, sin contar
// al usuario exceptUserID ni a los que tienen el rol exceptRoleID.
func countAdmins(orgID, exceptUserID, exceptRoleID string) (int, error) {
	rows, err := db.DB.Query(`
		SELECT u.id, u.role_id, r.permissions
		FROM users u JOIN roles r ON r.id = u.role_id
		WHERE u.organization_id = ? AND u.active = 1 AND u.deleted_at IS NULL
		  AND r.active = 1 AND r.deleted_at IS NULL`, orgID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var uid, rid string
		var perms sql.NullString
		if err := rows.Scan(&uid, &rid, &perms); err != nil {
			return 0, err
		}
		if uid == exceptUserID || rid == exceptRoleID {
			continue
		}
		if IsAdminRole(jsonList(perms)) {
			n++
		}
	}
	return n, rows.Err()
}

var errLastAdmin = Conflict("Debe quedar al menos un administrador activo en la organización. Crea o asigna otro administrador antes de hacer este cambio.")

// ensureAdminRemains revisa que, si el usuario (o el rol) deja de ser
// administrador, quede al menos otro.
func ensureAdminRemains(orgID, userID, roleID string) error {
	n, err := countAdmins(orgID, userID, roleID)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	// No queda otro: solo es problema si el afectado sí era administrador.
	affected, err := countAdmins(orgID, "", "")
	if err != nil {
		return err
	}
	if affected > 0 {
		return errLastAdmin
	}
	return nil
}

// ── Validaciones compartidas ───────────────────────────────────────────

// ValidatePassword — mínimo 8 caracteres, con al menos una letra y un número.
func ValidatePassword(p string) error {
	if runeLen(p) < 8 {
		return Invalid("La contraseña debe tener al menos 8 caracteres.")
	}
	if len(p) > 72 {
		return Invalid("La contraseña no puede pasar de 72 caracteres.")
	}
	var letter, digit bool
	for _, r := range p {
		switch {
		case unicode.IsLetter(r):
			letter = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	if !letter || !digit {
		return Invalid("La contraseña debe tener al menos una letra y un número.")
	}
	return nil
}

var phoneChars = regexp.MustCompile(`^[0-9+\-\s().]+$`)

// validPhone — solo números (se permiten +, espacios, guiones y paréntesis) y
// como máximo 15 dígitos (largo máximo internacional).
func validPhone(label, phone string, required bool) error {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		if required {
			return Invalid("El %s es obligatorio.", label)
		}
		return nil
	}
	if !phoneChars.MatchString(phone) || strings.Contains(phone[1:], "+") {
		return Invalid("El %s solo puede tener números (el + solo va al inicio).", label)
	}
	digits := 0
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	if digits == 0 || digits > 15 {
		return Invalid("El %s debe tener entre 1 y 15 dígitos.", label)
	}
	return nil
}

// normalizePhone valida el teléfono (validPhone) y lo deja solo con dígitos,
// con "+" al inicio si lo traía: "(662) 123-4567" → "6621234567". Así siempre
// cabe en las columnas phone1 / phone2 (máximo 16 caracteres) y se guarda igual
// sin importar cómo lo escribió quien lo capturó (KAN-145).
func normalizePhone(label, phone string, required bool) (string, error) {
	phone = strings.TrimSpace(phone)
	if err := validPhone(label, phone, required); err != nil {
		return "", err
	}
	var b strings.Builder
	for i, r := range phone {
		if (r >= '0' && r <= '9') || (i == 0 && r == '+') {
			b.WriteRune(r)
		}
	}
	return b.String(), nil
}

// rfcPattern — RFC de persona moral (12) o física (13), formato del SAT.
var rfcPattern = regexp.MustCompile(`^[A-ZÑ&]{3,4}[0-9]{6}[A-Z0-9]{3}$`)

// ValidRFC dice si el RFC tiene el formato del SAT.
func ValidRFC(rfc string) bool {
	return rfcPattern.MatchString(strings.ToUpper(strings.TrimSpace(rfc)))
}
