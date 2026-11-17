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
	// Organización de plataforma: la del superadmin raíz (admin@hookrepair.com);
	// si no existe ese usuario, la organización de menor id.
	if !columnExists("organizations", "is_platform") {
		mustExec("ALTER TABLE organizations ADD COLUMN is_platform TINYINT(1) NOT NULL DEFAULT 0")
		markPlatformOrganization()
	}
	// Versión de sesiones del usuario (KAN-143): subirla invalida las cookies ya emitidas.
	if !columnExists("users", "session_version") {
		mustExec("ALTER TABLE users ADD COLUMN session_version INT NOT NULL DEFAULT 0")
	}
	// Series (IMEI) de productos eliminados (KAN-144): también llevan deleted_at, y las
	// de productos que ya estaban eliminados se marcan una sola vez al agregar la columna.
	if !columnExists("product_serials", "deleted_at") {
		mustExec("ALTER TABLE product_serials ADD COLUMN deleted_at DATETIME NULL")
		mustExec(`UPDATE product_serials s JOIN products p ON p.id = s.product_id
			SET s.deleted_at = p.deleted_at WHERE p.deleted_at IS NOT NULL`)
	}
	// UNIQUE que ignora lo eliminado (permite volver a usar el nombre / email / IMEI).
	live := []struct{ table, col, src, typ, index, cols string }{
		{"users", "email_live", "email", "VARCHAR(255)", "uq_users_email", "email_live"},
		{"categories", "name_live", "name", "VARCHAR(150)", "uq_categories_name", "organization_id, name_live"},
		{"brands", "name_live", "name", "VARCHAR(150)", "uq_brands_name", "organization_id, name_live"},
		{"device_models", "name_live", "name", "VARCHAR(150)", "uq_device_models_name", "organization_id, brand_id, name_live"},
		{"product_serials", "imei_live", "imei", "VARCHAR(100)", "uq_product_serials_imei", "organization_id, imei_live"},
	}
	for _, l := range live {
		if columnExists(l.table, l.col) {
			continue
		}
		mustExec("ALTER TABLE " + l.table + " ADD COLUMN " + l.col + " " + l.typ +
			" GENERATED ALWAYS AS (IF(deleted_at IS NULL, " + l.src + ", NULL)) STORED")
		mustExec("ALTER TABLE " + l.table + " DROP INDEX " + l.index + ", ADD UNIQUE KEY " + l.index + " (" + l.cols + ")")
	}
	// Teléfonos múltiples (KAN-101): la tabla phones la crea schema.sql.
	migrateLegacyPhones()
}

// Migrate vuelve a correr los pasos de migrate (idempotentes); lo usan las pruebas.
func Migrate() { migrate() }

// migrateLegacyPhones copia phone1 / phone2 a la tabla phones (KAN-101) para las
// organizaciones, clientes y proveedores que aún no tienen filas ahí: phone1 es
// el principal (fijo, posición 0) y phone2 el segundo (celular; en proveedores
// fijo). Es idempotente: NOT EXISTS evita duplicar. Los eliminados se omiten.
func migrateLegacyPhones() {
	for _, o := range []struct{ table, owner, orgCol, kind2, live string }{
		{"organizations", "organization", "t.id", "celular", ""},
		{"customers", "customer", "t.organization_id", "celular", " AND t.deleted_at IS NULL"},
		{"suppliers", "supplier", "t.organization_id", "fijo", " AND t.deleted_at IS NULL"},
	} {
		missing := ` AND NOT EXISTS (SELECT 1 FROM phones p WHERE p.owner_type = '` + o.owner + `' AND p.owner_id = t.id)` + o.live
		res, err := DB.Exec(`INSERT INTO phones (organization_id, owner_type, owner_id, kind, number, is_primary, position)
			SELECT ` + o.orgCol + `, '` + o.owner + `', t.id, 'fijo', t.phone1, 1, 0 FROM ` + o.table + ` t
			WHERE COALESCE(t.phone1, '') <> ''` + missing + `
			UNION ALL
			SELECT ` + o.orgCol + `, '` + o.owner + `', t.id, '` + o.kind2 + `', t.phone2, IF(COALESCE(t.phone1, '') = '', 1, 0), IF(COALESCE(t.phone1, '') = '', 0, 1)
			FROM ` + o.table + ` t WHERE COALESCE(t.phone2, '') <> ''` + missing)
		if err != nil {
			log.Fatalf("Error copiando los teléfonos de %s: %v", o.table, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			log.Printf("Migración aplicada: %d teléfonos copiados de %s a phones", n, o.table)
		}
	}
}

// markPlatformOrganization marca como de plataforma la organización de
// admin@hookrepair.com y, si no hay ninguna marcada, la de menor id.
func markPlatformOrganization() {
	mustExec(`UPDATE organizations SET is_platform = 1
		WHERE id IN (SELECT organization_id FROM users WHERE email = 'admin@hookrepair.com' AND deleted_at IS NULL)`)
	var marked int
	if err := DB.QueryRow(`SELECT COUNT(*) FROM organizations WHERE is_platform = 1`).Scan(&marked); err != nil {
		log.Fatalf("Error revisando la organización de plataforma: %v", err)
	}
	if marked == 0 {
		mustExec(`UPDATE organizations SET is_platform = 1 ORDER BY id LIMIT 1`)
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
