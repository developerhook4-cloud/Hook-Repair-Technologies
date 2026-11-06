# Hook Repair (Go + HTML/CSS/JS)



## Correr en local

```bash
cp .env.example .env      # llena MYSQL_PUBLIC_URL y pon COOKIE_SECURE=false
go run .
# http://localhost:8080  →  admin@hookrepair.com / SEED_ADMIN_PASSWORD
```

Al arrancar, `db.EnsureSchema()` crea las tablas que falten (`CREATE TABLE IF NOT
EXISTS`), así que en una base vacía deja todo listo y en una con datos no toca
nada. El esquema completo está en `db/schema.sql`; también lo puedes pegar en la
pestaña **Query** de MySQL en Railway.

## Railway

- Variables del servicio: `MYSQL_URL=${{MySQL.MYSQL_URL}}`, `SESSION_SECRET`,
  `COOKIE_SECURE=true` y, la primera vez, `SEED_ADMIN_PASSWORD`.
- Railway detecta `go.mod` y compila solo. `PORT` lo pone Railway.

## Estructura

```
main.go                 rutas (páginas + API)
auth/                   sesión (gorilla/sessions)
db/                     conexión a MySQL y esquema (schema.sql)
models/                 consultas SQL por módulo (auth, catálogos, CRM, inventario)
handlers/               páginas, API y middlewares (permisos, organización)
templates/<sección>/    HTML (partials, auth, settings, inventory, organizations…)
static/js/<sección>/    JS de cada página; static/js/core/ = API, UI y layout
static/css/<sección>/    CSS de cada página (misma estructura que templates/)
```

## Base de datos

Tablas: `organizations`, `roles`, `users`, `categories`, `brands`,
`device_models`, `repair_services`, `customers`, `suppliers`, `products`,
`product_models`, `product_serials`, `inventory_movements`, `purchase_orders`,
`purchase_order_lines`, `goods_receipts`, `goods_receipt_lines`.

Los IDs son `BIGINT AUTO_INCREMENT`. El UPC de cada producto se arma al crearlo:
`4` + `1` si lleva IMEI / `0` si no + el id con 10 dígitos (ej. `400000000001`).

**Eliminar** no borra la fila: llena `deleted_at`, el registro deja de aparecer y
su nombre (o email) se puede volver a usar. Así las órdenes y movimientos viejos
conservan su historial. Si la base se creó con una versión anterior, al arrancar
la app agrega sola las columnas nuevas (`db/schema.go → migrate`).

## Reglas de validación

- Teléfonos: solo números (se aceptan `+`, espacios, guiones y paréntesis), máximo 15 dígitos.
- Contraseñas: mínimo 8 caracteres, con al menos una letra y un número.
- Clientes con "Requiere facturar": RFC (12 o 13 caracteres, formato SAT), régimen fiscal y uso de CFDI obligatorios.
- Roles: deben tener al menos un permiso; uno con usuarios asignados no se puede eliminar.
- Siempre queda al menos un administrador activo (rol con `Users.Update` y `Roles.Update`) por organización.
- Cada organización tiene los roles Administrador, Gerente, Técnico y Cajero (se crean solos al arrancar si faltan).

## Estilos

CSS escrito a mano, sin frameworks ni compilación (igual que Avante):

```
static/css/base.css              tipografías, colores (variables :root), reset y estilos de texto
static/css/partials/             navbar, menú lateral, estructura, encabezado de listas, campos
static/css/core/ui.css           componentes de static/js/core/ui.js (botones, modales, tablas…)
static/css/<sección>/<pág>.css   lo propio de cada template (auth/login.css, settings/roles.css…)
```

Cada página carga `base.css`, los `partials/`, `core/ui.css` y su propio archivo
(se asigna en `handlers/pages.go → pageStyles`). Los colores están en variables
(`--brand`, `--dark-1`, `--gray-200`…) en `base.css`.

## API

Todo bajo `/api`: `/api/auth/*`, `/api/organizations…` y, por organización,
`/api/{orgId}/roles|users|categories|brands|models|services|clients|suppliers|products|inventory/movements|purchase-orders|goods-receipts`.
Permisos con formato `Módulo.Acción` (`Inventory.View`, `Customers.Update`, …).
El login deja una cookie de sesión HttpOnly (`hook_session`).
