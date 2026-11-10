package models

import (
	"database/sql"
	"errors"
	"fmt"

	"hook-repair/db"
)

// ── Escalada de privilegios ────────────────────────────────────────────
//
// Regla: quien actúa solo puede otorgar o tocar lo que él mismo tiene.
//   - Un rol solo se crea/edita con permisos que el actor tiene, y solo si los
//     permisos ACTUALES del rol también los tiene (no se edita un rol más
//     poderoso que uno).
//   - A un usuario solo se le asigna un rol cuyos permisos el actor tiene, y
//     solo se edita, desactiva o elimina si los permisos de SU rol actual
//     también los tiene el actor.
// "*" cuenta como "tiene todo" (ver HasPermission).

// ForbiddenError → 403 con el mensaje tal cual.
type ForbiddenError struct{ Msg string }

func (e *ForbiddenError) Error() string { return e.Msg }

// Forbidden arma un ForbiddenError con formato.
func Forbidden(format string, a ...any) error {
	return &ForbiddenError{Msg: fmt.Sprintf(format, a...)}
}

var (
	errCannotGrant = Forbidden("No puedes otorgar permisos que tú no tienes.")
	errCannotTouch = Forbidden("No puedes modificar un rol ni a un usuario con más permisos que los tuyos.")
)

// IsPlatformOrganization dice si la organización es la de plataforma.
func IsPlatformOrganization(orgID string) (bool, error) {
	var platform bool
	err := db.DB.QueryRow(`SELECT is_platform FROM organizations WHERE id = ?`, orgID).Scan(&platform)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	return platform, err
}

// effectivePermissions son los permisos de un rol que de verdad valen en esa
// organización: fuera de la plataforma, Organizations.* está inerte.
func effectivePermissions(orgID string, perms []string) ([]string, error) {
	platform, err := IsPlatformOrganization(orgID)
	if err != nil {
		return nil, err
	}
	if platform {
		return perms, nil
	}
	return WithoutPlatformPermissions(perms), nil
}

// ensureCanGrant — el actor debe tener todos los permisos de la lista.
func ensureCanGrant(actor *SessionUser, perms []string) error {
	if actor == nil || !actor.CanAll(perms) {
		return errCannotGrant
	}
	return nil
}

// ensureCanTouchRole — el actor debe tener todos los permisos vigentes del rol.
func ensureCanTouchRole(actor *SessionUser, orgID string, role *Role) error {
	perms, err := effectivePermissions(orgID, role.Permissions)
	if err != nil {
		return err
	}
	if actor == nil || !actor.CanAll(perms) {
		return errCannotTouch
	}
	return nil
}

// ensureCanTouchUser — el actor debe tener todos los permisos del rol actual
// del usuario (si el rol ya no existe, no hay nada que proteger).
func ensureCanTouchUser(actor *SessionUser, orgID string, u *User) error {
	role, err := GetRole(orgID, u.RoleID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return ensureCanTouchRole(actor, orgID, role)
}

// ensureNoPlatformPermissions rechaza Organizations.* en roles de una
// organización que no es de plataforma.
func ensureNoPlatformPermissions(orgID string, perms []string) error {
	platform, err := IsPlatformOrganization(orgID)
	if err != nil {
		return err
	}
	if !platform && len(WithoutPlatformPermissions(perms)) != len(perms) {
		return Invalid("Los permisos de Organizaciones solo existen en la organización de plataforma y no se pueden asignar a este rol.")
	}
	return nil
}
