// Command sepomex-import carga el catálogo SEPOMEX de códigos postales
// (KAN-102) en la base configurada por las mismas variables que la app
// (MYSQL_PUBLIC_URL, MYSQL_URL o MYSQLHOST/MYSQLPORT/...). Sirve para llenar
// la base de Railway sin subir el archivo al repositorio:
//
//	MYSQL_PUBLIC_URL=mysql://... go run ./cmd/sepomex-import db/data/sepomex.tsv.gz
//
// Si postal_codes ya tiene filas no hace nada.
package main

import (
	"log"
	"os"
	"time"

	"hook-repair/db"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("uso: go run ./cmd/sepomex-import <sepomex.tsv.gz>")
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	db.Connect()
	defer db.DB.Close()
	db.EnsureSchema()

	start := time.Now()
	n, err := db.LoadPostalFile(f)
	if err != nil {
		log.Fatalf("no se pudo cargar el catálogo: %v", err)
	}
	if n == 0 {
		log.Print("postal_codes ya tenía datos; no se cargó nada.")
		return
	}
	log.Printf("Catálogo cargado: %d filas en %s.", n, time.Since(start).Round(time.Millisecond))
}
