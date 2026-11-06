package db

import (
	_ "embed"
	"log"
	"strings"
)

//go:embed schema.sql
var schemaSQL string

// EnsureSchema crea las tablas que falten (db/schema.sql). Todo es
// CREATE TABLE IF NOT EXISTS: en una base que ya las tiene no toca nada.
func EnsureSchema() {
	for _, stmt := range splitSQL(schemaSQL) {
		if _, err := DB.Exec(stmt); err != nil {
			log.Fatalf("Error preparando la base de datos: %v\n%s", err, stmt)
		}
	}
	migrate()
	log.Println("Tablas de MySQL verificadas.")
}

// migrate pone al día una base creada con una versión anterior del esquema
// (agrega columnas nuevas). Cada paso revisa antes si ya está aplicado.
func migrate() {
	for _, t := range []string{"roles", "users", "categories", "brands", "device_models",
		"repair_services", "customers", "suppliers", "products"} {
		if !columnExists(t, "deleted_at") {
			mustExec("ALTER TABLE " + t + " ADD COLUMN deleted_at DATETIME NULL")
		}
	}
	// UNIQUE que ignora lo eliminado (permite volver a usar el nombre / email).
	live := []struct{ table, col, src, typ, index, cols string }{
		{"users", "email_live", "email", "VARCHAR(255)", "uq_users_email", "email_live"},
		{"categories", "name_live", "name", "VARCHAR(150)", "uq_categories_name", "organization_id, name_live"},
		{"brands", "name_live", "name", "VARCHAR(150)", "uq_brands_name", "organization_id, name_live"},
		{"device_models", "name_live", "name", "VARCHAR(150)", "uq_device_models_name", "organization_id, brand_id, name_live"},
	}
	for _, l := range live {
		if columnExists(l.table, l.col) {
			continue
		}
		mustExec("ALTER TABLE " + l.table + " ADD COLUMN " + l.col + " " + l.typ +
			" GENERATED ALWAYS AS (IF(deleted_at IS NULL, " + l.src + ", NULL)) STORED")
		mustExec("ALTER TABLE " + l.table + " DROP INDEX " + l.index + ", ADD UNIQUE KEY " + l.index + " (" + l.cols + ")")
	}
}

func columnExists(table, column string) bool {
	var n int
	err := DB.QueryRow(`SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?`, table, column).Scan(&n)
	if err != nil {
		log.Fatalf("Error revisando la columna %s.%s: %v", table, column, err)
	}
	return n > 0
}

func mustExec(stmt string) {
	if _, err := DB.Exec(stmt); err != nil {
		log.Fatalf("Error actualizando la base de datos: %v\n%s", err, stmt)
	}
	log.Printf("Migración aplicada: %s", stmt)
}

// splitSQL quita los comentarios "-- ..." y separa por ";".
func splitSQL(src string) []string {
	var b strings.Builder
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	out := []string{}
	for _, s := range strings.Split(b.String(), ";") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
