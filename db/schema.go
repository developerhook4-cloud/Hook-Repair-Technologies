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
	log.Println("Tablas de MySQL verificadas.")
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
