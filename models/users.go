package models

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"hook-repair/db"
)

// bcryptCost — factor de trabajo de bcrypt.
const bcryptCost = 12

// HashPassword genera el hash bcrypt de una contraseña.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	return string(b), err
}

// CheckPassword compara la contraseña con el hash.
func CheckPassword(plain, hash string) bool {
	if strings.TrimSpace(hash) == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// User — fila de users con su rol. PasswordHash nunca sale en el JSON.
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	RoleID   string `json:"roleId"`
	RoleName string `json:"roleName"`
	Role     string `json:"role"` // alias de roleName (tablas del frontend)
	Active   bool   `json:"active"`
	IsActive bool   `json:"isActive"` // alias de active (vista de organizaciones)
}

const userSelect = `
	SELECT u.id, u.name, u.email, u.role_id, COALESCE(r.name, 'Sin Rol'), u.active
	FROM users u LEFT JOIN roles r ON r.id = u.role_id`

func scanUser(sc interface{ Scan(...any) error }) (*User, error) {
	var u User
	if err := sc.Scan(&u.ID, &u.Name, &u.Email, &u.RoleID, &u.RoleName, &u.Active); err != nil {
		return nil, err
	}
	u.Role, u.IsActive = u.RoleName, u.Active
	return &u, nil
}

// GetUsers — usuarios de una organización.
func GetUsers(orgID string) ([]User, error) {
	rows, err := db.DB.Query(userSelect+` WHERE u.organization_id = ? AND u.deleted_at IS NULL ORDER BY u.name`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// GetUser — un usuario de la organización.
func GetUser(orgID, userID string) (*User, error) {
	u, err := scanUser(db.DB.QueryRow(userSelect+` WHERE u.organization_id = ? AND u.id = ? AND u.deleted_at IS NULL`, orgID, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

// SaveUserRequest — cuerpo para crear/editar un usuario (Configuración → Empleados
// y Organizaciones → usuarios).
type SaveUserRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	RoleID   string `json:"roleId"`
	Active   *bool  `json:"active"`
}

func (r *SaveUserRequest) normalize(requirePassword bool) error {
	r.Name = strings.TrimSpace(r.Name)
	r.Email = strings.ToLower(strings.TrimSpace(r.Email))
	if r.Name == "" || runeLen(r.Name) > 150 {
		return Invalid("El nombre es obligatorio (máximo 150 caracteres).")
	}
	if r.Email == "" || !IsEmail(r.Email) || runeLen(r.Email) > 320 {
		return Invalid("El email es obligatorio y debe ser válido.")
	}
	if requirePassword && strings.TrimSpace(r.Password) == "" {
		return Invalid("La contraseña es obligatoria.")
	}
	if r.Password != "" {
		return ValidatePassword(r.Password)
	}
	return nil
}

func emailTaken(email, exceptID string) (bool, error) {
	var exists bool
	err := db.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = ? AND deleted_at IS NULL AND (? = '' OR id <> ?))`,
		email, exceptID, exceptID).Scan(&exists)
	return exists, err
}

// CreateUser crea un usuario en la organización. Si no trae rol, usa
// "Administrador" de esa organización.
func CreateUser(orgID string, r SaveUserRequest) (*User, error) {
	if err := r.normalize(true); err != nil {
		return nil, err
	}
	if r.RoleID == "" {
		admin, err := FindRoleByName(orgID, "Administrador")
		if err != nil {
			return nil, Invalid("No se encontró el rol de Administrador. Asegúrate de generar los roles de la empresa antes de crear al usuario.")
		}
		r.RoleID = admin.ID
	} else if !IsID(r.RoleID) {
		return nil, Invalid("El rol no es válido.")
	} else if _, err := GetRole(orgID, r.RoleID); err != nil {
		return nil, Invalid("El rol no pertenece a esta organización.")
	}
	if taken, err := emailTaken(r.Email, ""); err != nil {
		return nil, err
	} else if taken {
		return nil, Conflict("El correo electrónico ya está registrado por otro usuario.")
	}
	hash, err := HashPassword(r.Password)
	if err != nil {
		return nil, err
	}
	active := true
	if r.Active != nil {
		active = *r.Active
	}
	id, err := insert(db.DB, `INSERT INTO users (organization_id, role_id, name, email, password_hash, active)
		VALUES (?,?,?,?,?,?)`, orgID, r.RoleID, r.Name, r.Email, hash, active)
	if err != nil {
		if IsUniqueViolation(err) {
			return nil, Conflict("El correo electrónico ya está registrado por otro usuario.")
		}
		return nil, err
	}
	return GetUser(orgID, id)
}

// UpdateUser edita nombre, email, rol y (si viene) contraseña.
func UpdateUser(orgID, userID string, r SaveUserRequest) (*User, error) {
	if err := r.normalize(false); err != nil {
		return nil, err
	}
	current, err := GetUser(orgID, userID)
	if err != nil {
		return nil, err
	}
	roleID := current.RoleID
	if r.RoleID != "" {
		if !IsID(r.RoleID) {
			return nil, Invalid("El rol no es válido.")
		}
		if _, err := GetRole(orgID, r.RoleID); err != nil {
			return nil, Invalid("El rol no pertenece a esta organización.")
		}
		roleID = r.RoleID
	}
	if roleID != current.RoleID {
		role, err := GetRole(orgID, roleID)
		if err != nil {
			return nil, err
		}
		if !IsAdminRole(role.Permissions) {
			if err := ensureAdminRemains(orgID, userID, ""); err != nil {
				return nil, err
			}
		}
	}
	if taken, err := emailTaken(r.Email, userID); err != nil {
		return nil, err
	} else if taken {
		return nil, Conflict("El correo electrónico ya está registrado por otro usuario.")
	}
	if strings.TrimSpace(r.Password) != "" {
		hash, err := HashPassword(r.Password)
		if err != nil {
			return nil, err
		}
		_, err = db.DB.Exec(`UPDATE users SET name=?, email=?, role_id=?, password_hash=? WHERE organization_id=? AND id=?`,
			r.Name, r.Email, roleID, hash, orgID, userID)
		if err != nil {
			return nil, err
		}
	} else {
		_, err = db.DB.Exec(`UPDATE users SET name=?, email=?, role_id=? WHERE organization_id=? AND id=?`,
			r.Name, r.Email, roleID, orgID, userID)
		if err != nil {
			return nil, err
		}
	}
	return GetUser(orgID, userID)
}

// SetUserStatus activa o desactiva un usuario.
func SetUserStatus(orgID, userID string, active bool) error {
	if !active {
		if err := ensureAdminRemains(orgID, userID, ""); err != nil {
			return err
		}
	}
	res, err := db.DB.Exec(`UPDATE users SET active=? WHERE organization_id=? AND id=? AND deleted_at IS NULL`, active, orgID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateProfileRequest — "Configurar perfil" del usuario con sesión.
type UpdateProfileRequest struct {
	Name        string  `json:"name"`
	Email       string  `json:"email"`
	OldPassword *string `json:"oldPassword"`
	NewPassword *string `json:"newPassword"`
}

// UpdateProfile cambia nombre, email y (si viene) la contraseña del usuario con sesión.
func UpdateProfile(userID string, r UpdateProfileRequest) error {
	name := strings.TrimSpace(r.Name)
	email := strings.ToLower(strings.TrimSpace(r.Email))
	if name == "" || email == "" || !IsEmail(email) {
		return Invalid("Nombre y un email válido son obligatorios.")
	}
	var currentEmail, hash string
	var active bool
	err := db.DB.QueryRow(`SELECT email, password_hash, active FROM users WHERE id = ? AND deleted_at IS NULL`, userID).
		Scan(&currentEmail, &hash, &active)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !active) {
		return Invalid("Usuario no encontrado o inactivo.")
	}
	if err != nil {
		return err
	}
	if currentEmail != email {
		if taken, err := emailTaken(email, userID); err != nil {
			return err
		} else if taken {
			return Invalid("El correo electrónico ya está registrado por otro usuario.")
		}
	}
	newHash := hash
	if r.NewPassword != nil && strings.TrimSpace(*r.NewPassword) != "" {
		if r.OldPassword == nil || strings.TrimSpace(*r.OldPassword) == "" {
			return Invalid("Debes ingresar tu contraseña actual para poder cambiarla.")
		}
		if !CheckPassword(*r.OldPassword, hash) {
			return Invalid("La contraseña actual es incorrecta.")
		}
		if err := ValidatePassword(*r.NewPassword); err != nil {
			return err
		}
		if newHash, err = HashPassword(*r.NewPassword); err != nil {
			return err
		}
	}
	_, err = db.DB.Exec(`UPDATE users SET name=?, email=?, password_hash=? WHERE id=?`, name, email, newHash, userID)
	return err
}

// SessionUser — lo que la app necesita saber del usuario con sesión en cada
// petición (se vuelve a leer de la base, así un cambio de permisos o una
// desactivación aplica de inmediato).
type SessionUser struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	RoleID         string   `json:"roleId"`
	Role           string   `json:"role"`
	Permissions    []string `json:"permissions"`
	OrganizationID string   `json:"organizationId"`
}

// Can dice si el usuario tiene un permiso.
func (u *SessionUser) Can(permission string) bool {
	return u != nil && HasPermission(u.Permissions, permission)
}

// loadSessionUser trae al usuario solo si él, su rol y su organización están activos.
func loadSessionUser(where string, arg any) (*SessionUser, string, error) {
	var u SessionUser
	var hash string
	var perms sql.NullString
	var userActive, roleActive, orgActive bool
	err := db.DB.QueryRow(`
		SELECT u.id, u.name, u.email, u.role_id, r.name, r.permissions,
		       u.organization_id, u.password_hash, u.active, r.active, o.active
		FROM users u
		JOIN roles r ON r.id = u.role_id
		JOIN organizations o ON o.id = u.organization_id
		WHERE u.deleted_at IS NULL AND `+where, arg).
		Scan(&u.ID, &u.Name, &u.Email, &u.RoleID, &u.Role, &perms, &u.OrganizationID, &hash, &userActive, &roleActive, &orgActive)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	if !userActive || !roleActive || !orgActive {
		return nil, "", ErrNotFound
	}
	u.Permissions = jsonList(perms)
	return &u, hash, nil
}

// GetSessionUser por id (para la sesión).
func GetSessionUser(id string) (*SessionUser, error) {
	if !IsID(id) {
		return nil, ErrNotFound
	}
	u, _, err := loadSessionUser("u.id = ?", id)
	return u, err
}

// Authenticate valida email + contraseña. Devuelve ErrNotFound si no procede.
func Authenticate(email, password string) (*SessionUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, ErrNotFound
	}
	u, hash, err := loadSessionUser("u.email = ?", email)
	if err != nil {
		return nil, err
	}
	if !CheckPassword(password, hash) {
		return nil, ErrNotFound
	}
	return u, nil
}

// SeedRootAdmin crea la organización raíz y su super administrador si todavía
// no existen. La contraseña inicial se puede cambiar
// con SEED_ADMIN_PASSWORD antes del primer arranque.
func SeedRootAdmin() {
	const email = "admin@hookrepair.com"
	var exists bool
	if err := db.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE email = ? AND deleted_at IS NULL)`, email).Scan(&exists); err != nil {
		log.Printf("Seed: no se pudo revisar el admin: %v", err)
		return
	}
	if exists {
		return
	}
	password := os.Getenv("SEED_ADMIN_PASSWORD")
	if password == "" {
		password = "AdminPassword123"
	}
	hash, err := HashPassword(password)
	if err != nil {
		log.Printf("Seed: %v", err)
		return
	}
	err = withTx(func(tx *sql.Tx) error {
		orgID, err := insert(tx, `INSERT INTO organizations (name, email, active) VALUES ('Hook Technologies', ?, 1)`, email)
		if err != nil {
			return err
		}
		roleID, err := insert(tx, `INSERT INTO roles (organization_id, name, permissions, active) VALUES (?, 'Super Administrador', ?, 1)`,
			orgID, toJSON(AllPermissions))
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO users (organization_id, role_id, name, email, password_hash, active)
			VALUES (?, ?, 'Super Admin', ?, ?, 1)`, orgID, roleID, email, hash)
		return err
	})
	if err != nil {
		log.Printf("Seed: no se pudo crear el admin: %v", err)
		return
	}
	log.Printf("Seed: se creó %s (cambia la contraseña desde Perfil).", email)
}
