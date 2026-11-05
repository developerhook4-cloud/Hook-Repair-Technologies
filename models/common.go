package models

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"

	"hook-repair/db"
)

// ⚠️ Ajusta "hook-repair" en los imports si cambias el nombre del módulo en go.mod.
//
// Los ids son BIGINT AUTO_INCREMENT en MySQL, pero en Go y en el JSON viajan
// como texto ("12"): así el JS los compara siempre igual sin importar si
// vienen de la API o de un <select>.

// ── Errores que los handlers convierten en respuestas HTTP ─────────────

// ErrNotFound → 404.
var ErrNotFound = errors.New("not found")

// ValidationError → 400 con el mensaje tal cual.
type ValidationError struct{ Msg string }

func (e *ValidationError) Error() string { return e.Msg }

// ConflictError → 409 con el mensaje tal cual.
type ConflictError struct{ Msg string }

func (e *ConflictError) Error() string { return e.Msg }

// Invalid arma un ValidationError con formato.
func Invalid(format string, a ...any) error {
	return &ValidationError{Msg: fmt.Sprintf(format, a...)}
}

// Conflict arma un ConflictError con formato.
func Conflict(format string, a ...any) error {
	return &ConflictError{Msg: fmt.Sprintf(format, a...)}
}

// IsUniqueViolation dice si el error de MySQL es por un UNIQUE repetido (1062).
func IsUniqueViolation(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// ── Ids ────────────────────────────────────────────────────────────────

// IsID valida que el texto sea un id (entero positivo).
func IsID(s string) bool {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return err == nil && n > 0
}

// idStr convierte el id de un INSERT a texto.
func idStr(res sql.Result) (string, error) {
	n, err := res.LastInsertId()
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(n, 10), nil
}

// execer — *sql.DB o *sql.Tx.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// insert ejecuta un INSERT y devuelve el id nuevo.
func insert(ex execer, query string, args ...any) (string, error) {
	res, err := ex.Exec(query, args...)
	if err != nil {
		return "", err
	}
	return idStr(res)
}

// placeholders arma "?,?,?" para un IN (...).
func placeholders(n int) string {
	if n <= 0 {
		return "NULL"
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// anySlice convierte []string en []any para pasarlo como argumentos.
func anySlice(in []string) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// ── Helpers de valores ─────────────────────────────────────────────────

// trimOrNil devuelve nil (NULL en la base) si el texto viene vacío.
func trimOrNil(s *string) any {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return t
}

// idOrNil devuelve nil si el id viene vacío.
func idOrNil(s *string) any {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return strings.TrimSpace(*s)
}

// nullStr convierte un NullString en *string (nil → null en el JSON).
func nullStr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

// strOrEmpty convierte un NullString en string ("" si es NULL).
func strOrEmpty(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

// jsonList lee una columna JSON con un arreglo de textos.
func jsonList(ns sql.NullString) []string {
	out := []string{}
	if !ns.Valid || ns.String == "" || ns.String == "null" {
		return out
	}
	_ = json.Unmarshal([]byte(ns.String), &out)
	return out
}

// toJSON guarda un arreglo de textos en una columna JSON.
func toJSON(list []string) string {
	if list == nil {
		list = []string{}
	}
	b, _ := json.Marshal(list)
	return string(b)
}

// has2Decimals valida que un precio no tenga más de 2 decimales.
func has2Decimals(v float64) bool {
	return math.Abs(math.Round(v*100)-v*100) < 1e-6
}

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// IsEmail valida el formato básico de un correo.
func IsEmail(s string) bool { return emailPattern.MatchString(strings.TrimSpace(s)) }

// runeLen cuenta caracteres (no bytes).
func runeLen(s string) int { return len([]rune(s)) }

// withTx corre fn dentro de una transacción.
func withTx(fn func(tx *sql.Tx) error) error {
	tx, err := db.DB.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
