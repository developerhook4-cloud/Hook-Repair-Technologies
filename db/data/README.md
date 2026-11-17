# Catálogo de códigos postales (SEPOMEX)

El catálogo **no se guarda en el repositorio**: Correos de México lo entrega "en
forma gratuita para uso particular, no estando permitida su comercialización,
total o parcial, ni su distribución a terceros bajo ningún concepto". Cada
instalación lo descarga por su cuenta.

## Obtenerlo

1. Descargar de https://www.correosdemexico.gob.mx/SSLServicios/ConsultaCP/CodigoPostal_Exportar.aspx
   (estado "Todos", formato "txt"): `CPdescargatxt.zip`.
2. Convertirlo a TSV en UTF-8 comprimido (columnas `cp`, `colonia`, `tipo`,
   `municipio`, `ciudad`, `estado`):

   ```
   python3 scripts/sepomex/convert.py CPdescargatxt.zip db/data/sepomex.tsv.gz
   ```

   `db/data/sepomex.tsv*` está en `.gitignore`.

## Cargarlo

- **Local:** al arrancar, la app carga `db/data/sepomex.tsv.gz` (o la ruta de
  `SEPOMEX_FILE`) en la tabla `postal_codes` si está vacía, en segundo plano.
- **Railway u otra base remota:** desde tu máquina, apuntando a la base:

  ```
  MYSQL_PUBLIC_URL=mysql://usuario:clave@host:puerto/base go run ./cmd/sepomex-import db/data/sepomex.tsv.gz
  ```

Mientras `postal_codes` esté vacía, `GET /api/postal-codes/:cp` responde 503 y
los formularios dejan capturar la dirección a mano. Si la tabla ya tiene datos,
ninguna de las dos rutas vuelve a cargar; para actualizar el catálogo, vaciar
la tabla (`TRUNCATE postal_codes`) y cargar de nuevo.
