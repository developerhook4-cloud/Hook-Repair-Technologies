package db

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// DB es la conexión global a MySQL. Se inicializa con Connect() en main.go.
var DB *sql.DB

// Connect abre la conexión a MySQL usando las variables de entorno de Railway.
// Soporta dos formas, según cómo las tengas copiadas desde Railway:
//  1. MYSQL_URL="mysql://usuario:password@host:puerto/nombre_db"
//  2. Variables sueltas: MYSQLHOST, MYSQLPORT, MYSQLUSER, MYSQLPASSWORD, MYSQLDATABASE
func Connect() {
	dsn := buildDSN()

	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Error abriendo conexión a MySQL: %v", err)
	}

	DB.SetMaxOpenConns(10)
	DB.SetMaxIdleConns(5)

	if err := DB.Ping(); err != nil {
		log.Fatalf("No se pudo conectar a MySQL (Railway): %v", err)
	}

	log.Println("Conectado a MySQL (Railway) correctamente.")
}

// dsnParams: fechas como time.Time, utf8mb4 y todo en UTC (las fechas se
// guardan en UTC y el navegador las muestra en la hora local).
// clientFoundRows: un UPDATE que no cambia nada igual cuenta la fila
// encontrada (así "0 filas" siempre quiere decir "no existe").
const dsnParams = "parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=UTC&clientFoundRows=true"

func buildDSN() string {
	// MYSQL_PUBLIC_URL: host público, funciona desde tu computadora local.
	// MYSQL_URL: host interno (mysql.railway.internal), solo funciona
	// cuando tu app corre DENTRO de Railway. Por eso probamos primero la
	// pública y solo caemos a la interna si no existe.
	if raw := os.Getenv("MYSQL_PUBLIC_URL"); raw != "" {
		return parseMySQLURL(raw)
	}
	if raw := os.Getenv("MYSQL_URL"); raw != "" {
		return parseMySQLURL(raw)
	}

	host := os.Getenv("MYSQLHOST")
	port := os.Getenv("MYSQLPORT")
	user := os.Getenv("MYSQLUSER")
	pass := os.Getenv("MYSQLPASSWORD")
	name := os.Getenv("MYSQLDATABASE")

	if host == "" {
		log.Fatal("Faltan variables de entorno de MySQL: define MYSQL_URL o MYSQLHOST/MYSQLPORT/MYSQLUSER/MYSQLPASSWORD/MYSQLDATABASE")
	}
	if port == "" {
		port = "3306"
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", user, pass, host, port, name, dsnParams)
}

// parseMySQLURL convierte una URL tipo mysql://user:pass@host:port/dbname
// (como la que da Railway) al formato DSN que espera el driver de Go.
func parseMySQLURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		log.Fatalf("MYSQL_URL inválida: %v", err)
	}

	user := u.User.Username()
	pass, _ := u.User.Password()
	dbname := strings.TrimPrefix(u.Path, "/")
	host := u.Host
	if u.Port() == "" {
		host += ":3306"
	}

	return fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", user, pass, host, dbname, dsnParams)
}

// ConnectDSN abre la conexión a partir de un DSN explícito
// ("usuario:password@tcp(host:puerto)/nombre_db"); si no trae parámetros agrega
// los mismos que Connect. Devuelve el error en vez de terminar el programa
// (lo usan las pruebas de integración).
func ConnectDSN(dsn string) error {
	if !strings.Contains(dsn, "?") {
		dsn += "?" + dsnParams
	}
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	if err := conn.Ping(); err != nil {
		conn.Close()
		return err
	}
	DB = conn
	return nil
}
