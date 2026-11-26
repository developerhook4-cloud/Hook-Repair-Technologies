package db

import (
	"bufio"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// El catálogo SEPOMEX NO va en el repositorio: sus términos prohíben
// distribuirlo a terceros. Se descarga de Correos de México, se convierte con
// scripts/sepomex/convert.py y se carga desde un archivo local (SEPOMEX_FILE)
// o directo a la base con `go run ./cmd/sepomex-import`; ver db/data/README.md.
const defaultSepomexFile = "db/data/sepomex.tsv.gz"

const (
	postalBatch   = 1000 // filas por INSERT multi-fila
	postalColumns = 6
	postalLock    = "hook_repair_postal_load"
)

// EnsurePostalCatalog llena postal_codes desde el archivo de SEPOMEX_FILE
// (por omisión db/data/sepomex.tsv.gz) si la tabla está vacía (KAN-102).
// Pensada para correr en una goroutine al arrancar: si el archivo no existe o
// la carga falla solo lo registra, y la API de códigos postales responde 503
// mientras la tabla siga vacía.
func EnsurePostalCatalog() {
	path := strings.TrimSpace(os.Getenv("SEPOMEX_FILE"))
	if path == "" {
		path = defaultSepomexFile
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("Catálogo de códigos postales: no existe %s; si la tabla está vacía, cárgalo con `go run ./cmd/sepomex-import` (ver db/data/README.md).", path)
			return
		}
		log.Printf("Catálogo de códigos postales: no se pudo abrir %s: %v", path, err)
		return
	}
	defer f.Close()
	start := time.Now()
	n, err := LoadPostalFile(f)
	switch {
	case err != nil:
		log.Printf("Catálogo de códigos postales: no se pudo cargar: %v", err)
	case n > 0:
		log.Printf("Catálogo de códigos postales cargado: %d filas en %s.", n, time.Since(start).Round(time.Millisecond))
	}
}

// LoadPostalFile carga el catálogo desde un TSV, comprimido con gzip o no
// (se detecta por la firma del archivo).
func LoadPostalFile(r io.Reader) (int, error) {
	br := bufio.NewReader(r)
	if magic, err := br.Peek(2); err == nil && magic[0] == 0x1f && magic[1] == 0x8b {
		zr, err := gzip.NewReader(br)
		if err != nil {
			return 0, fmt.Errorf("gzip inválido: %w", err)
		}
		defer zr.Close()
		return LoadPostalCatalog(zr)
	}
	return LoadPostalCatalog(br)
}

// LoadPostalCatalog lee el TSV (cp, colonia, tipo, municipio, ciudad, estado)
// y lo inserta en postal_codes en lotes dentro de una sola transacción. Si la
// tabla ya tiene filas no hace nada (devuelve 0), así que es idempotente. Un
// candado de MySQL evita que dos instancias que arrancan a la vez la llenen
// dos veces.
func LoadPostalCatalog(r io.Reader) (int, error) {
	ctx := context.Background()
	// El candado vive en la sesión: se usa una conexión dedicada.
	conn, err := DB.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, `SELECT GET_LOCK(?, 0)`, postalLock).Scan(&got); err != nil {
		return 0, err
	}
	if got.Int64 != 1 {
		return 0, nil // otra instancia la está cargando
	}
	defer conn.ExecContext(ctx, `SELECT RELEASE_LOCK(?)`, postalLock)

	var one int
	err = conn.QueryRowContext(ctx, `SELECT 1 FROM postal_codes LIMIT 1`).Scan(&one)
	if err == nil {
		return 0, nil // ya está cargado
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	total := 0
	batch := make([]any, 0, postalBatch*postalColumns)
	flush := func() error {
		rows := len(batch) / postalColumns
		if rows == 0 {
			return nil
		}
		q := "INSERT INTO postal_codes (cp, colonia, tipo_asentamiento, municipio, ciudad, estado) VALUES " +
			strings.TrimSuffix(strings.Repeat("(?,?,?,?,?,?),", rows), ",")
		if _, err := tx.ExecContext(ctx, q, batch...); err != nil {
			return err
		}
		total += rows
		batch = batch[:0]
		return nil
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		f := strings.Split(strings.TrimRight(sc.Text(), "\r"), "\t")
		if len(f) != postalColumns || len(f[0]) != 5 {
			if line == 1 && f[0] == "cp" {
				continue // encabezado
			}
			return 0, fmt.Errorf("línea %d inválida en el catálogo", line)
		}
		for _, v := range f {
			batch = append(batch, v)
		}
		if len(batch) == cap(batch) {
			if err := flush(); err != nil {
				return 0, err
			}
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	if err := flush(); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}
