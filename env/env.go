// Package env — datos del entorno de ejecución compartidos por varios paquetes.
package env

import (
	"os"
	"strings"
)

// IsProduction dice si la app corre en producción: Railway pone
// RAILWAY_ENVIRONMENT_NAME / RAILWAY_ENVIRONMENT, y APP_ENV=production lo
// permite marcar a mano en cualquier otro servidor.
func IsProduction() bool {
	return os.Getenv("RAILWAY_ENVIRONMENT_NAME") != "" ||
		os.Getenv("RAILWAY_ENVIRONMENT") != "" ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
}
