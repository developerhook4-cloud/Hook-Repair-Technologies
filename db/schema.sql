-- ─────────────────────────────────────────────────────────────────────
-- Hook Repair — tablas de MySQL.
--
-- La app las crea sola al arrancar (db.EnsureSchema). Si prefieres crearlas
-- a mano, pega este archivo completo en la pestaña Query de Railway: todo
-- es CREATE TABLE IF NOT EXISTS, así que no toca tablas que ya existan.
--
-- Todos los ids son BIGINT AUTO_INCREMENT. Cada registro de catálogos,
-- terceros e inventario lleva organization_id (cada taller ve solo lo suyo).
--
-- deleted_at: "Eliminar" no borra la fila (se conserva por historial), solo
-- la marca y deja de mostrarse. Las columnas *_live valen NULL en lo eliminado,
-- así los UNIQUE permiten volver a usar el nombre o el email.
-- ─────────────────────────────────────────────────────────────────────

-- ── Organizaciones, roles y usuarios ────────────────────────────────

CREATE TABLE IF NOT EXISTS organizations (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  name       VARCHAR(200) NOT NULL,
  email      VARCHAR(200) NOT NULL,
  website    VARCHAR(500) NOT NULL DEFAULT '',
  phone1     VARCHAR(30)  NOT NULL DEFAULT '',
  phone2     VARCHAR(30)  NOT NULL DEFAULT '',
  address    VARCHAR(300) NOT NULL DEFAULT '',
  city       VARCHAR(100) NOT NULL DEFAULT '',
  state      VARCHAR(100) NOT NULL DEFAULT '',
  post_code  VARCHAR(20)  NOT NULL DEFAULT '',
  country    VARCHAR(100) NOT NULL DEFAULT '',
  active     TINYINT(1)   NOT NULL DEFAULT 1,
  -- is_platform: 1 = organización de plataforma (la de Hook): solo sus usuarios
  -- pueden usar los permisos Organizations.* y /api/organizations.
  is_platform TINYINT(1)  NOT NULL DEFAULT 0,
  created_at DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- permissions: arreglo JSON, p. ej. ["Inventory.View","Inventory.Update"]
CREATE TABLE IF NOT EXISTS roles (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id BIGINT       NOT NULL,
  name            VARCHAR(100) NOT NULL,
  permissions     JSON         NOT NULL,
  active          TINYINT(1)   NOT NULL DEFAULT 1,
  deleted_at      DATETIME     NULL,
  created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_roles_org (organization_id),
  CONSTRAINT fk_roles_org FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS users (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id BIGINT       NOT NULL,
  role_id         BIGINT       NOT NULL,
  name            VARCHAR(150) NOT NULL,
  email           VARCHAR(255) NOT NULL,
  password_hash   VARCHAR(255) NOT NULL,
  active          TINYINT(1)   NOT NULL DEFAULT 1,
  -- Versión de las sesiones: cada cookie lleva la que tenía al iniciar sesión y
  -- se invalida si cambia (logout, cambio de contraseña, desactivar, eliminar).
  session_version INT          NOT NULL DEFAULT 0,
  created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at      DATETIME     NULL,
  email_live      VARCHAR(255) GENERATED ALWAYS AS (IF(deleted_at IS NULL, email, NULL)) STORED,
  UNIQUE KEY uq_users_email (email_live),
  KEY idx_users_org (organization_id),
  CONSTRAINT fk_users_org  FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
  CONSTRAINT fk_users_role FOREIGN KEY (role_id) REFERENCES roles(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── Catálogos ───────────────────────────────────────────────────────

-- icon: clase de Font Awesome, p. ej. "fas fa-mobile-screen"
CREATE TABLE IF NOT EXISTS categories (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id BIGINT       NOT NULL,
  name            VARCHAR(150) NOT NULL,
  icon            VARCHAR(100) NOT NULL DEFAULT '',
  active          TINYINT(1)   NOT NULL DEFAULT 1,
  deleted_at      DATETIME     NULL,
  name_live       VARCHAR(150) GENERATED ALWAYS AS (IF(deleted_at IS NULL, name, NULL)) STORED,
  UNIQUE KEY uq_categories_name (organization_id, name_live)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS brands (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id BIGINT       NOT NULL,
  name            VARCHAR(150) NOT NULL,
  active          TINYINT(1)   NOT NULL DEFAULT 1,
  deleted_at      DATETIME     NULL,
  name_live       VARCHAR(150) GENERATED ALWAYS AS (IF(deleted_at IS NULL, name, NULL)) STORED,
  UNIQUE KEY uq_brands_name (organization_id, name_live)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Modelos de dispositivo (iPhone 13, Galaxy S22…)
CREATE TABLE IF NOT EXISTS device_models (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id BIGINT       NOT NULL,
  brand_id        BIGINT       NOT NULL,
  name            VARCHAR(150) NOT NULL,
  active          TINYINT(1)   NOT NULL DEFAULT 1,
  deleted_at      DATETIME     NULL,
  name_live       VARCHAR(150) GENERATED ALWAYS AS (IF(deleted_at IS NULL, name, NULL)) STORED,
  UNIQUE KEY uq_device_models_name (organization_id, brand_id, name_live),
  CONSTRAINT fk_device_models_brand FOREIGN KEY (brand_id) REFERENCES brands(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Servicios de reparación (cambio de pantalla, batería…)
CREATE TABLE IF NOT EXISTS repair_services (
  id                BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id   BIGINT        NOT NULL,
  name              VARCHAR(200)  NOT NULL,
  description       VARCHAR(2000) NOT NULL DEFAULT '',
  category_id       BIGINT        NULL,
  brand_id          BIGINT        NULL,
  model_id          BIGINT        NULL,
  retail_price      DECIMAL(18,2) NOT NULL DEFAULT 0,
  public_price      DECIMAL(18,2) NOT NULL DEFAULT 0,
  distributor_price DECIMAL(18,2) NOT NULL DEFAULT 0,
  active            TINYINT(1)    NOT NULL DEFAULT 1,
  deleted_at        DATETIME      NULL,
  KEY idx_repair_services_org (organization_id),
  CONSTRAINT fk_repair_services_category FOREIGN KEY (category_id) REFERENCES categories(id),
  CONSTRAINT fk_repair_services_brand    FOREIGN KEY (brand_id)    REFERENCES brands(id),
  CONSTRAINT fk_repair_services_model    FOREIGN KEY (model_id)    REFERENCES device_models(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── Terceros ────────────────────────────────────────────────────────

-- customer_type: 'public' (cliente) o 'distributor' (distribuidor)
-- authorized_people: arreglo JSON de nombres
CREATE TABLE IF NOT EXISTS customers (
  id                BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id   BIGINT       NOT NULL,
  customer_type     VARCHAR(20)  NOT NULL DEFAULT 'public',
  name              VARCHAR(200) NOT NULL,
  phone1            VARCHAR(20)  NOT NULL,
  phone2            VARCHAR(20)  NULL,
  legal_name        VARCHAR(200) NULL,
  email             VARCHAR(254) NULL,
  address           VARCHAR(500) NULL,
  city              VARCHAR(500) NULL,
  state             VARCHAR(500) NULL,
  post_code         VARCHAR(500) NULL,
  country           VARCHAR(500) NULL,
  titular_name      VARCHAR(200) NULL,
  authorized_people JSON         NULL,
  rfc               VARCHAR(13)  NULL,
  tax_regime        VARCHAR(200) NULL,
  expense_type      VARCHAR(200) NULL,
  active            TINYINT(1)   NOT NULL DEFAULT 1,
  created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at        DATETIME     NULL,
  KEY idx_customers_org (organization_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS suppliers (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id BIGINT       NOT NULL,
  name            VARCHAR(200) NOT NULL,
  phone1          VARCHAR(20)  NOT NULL,
  phone2          VARCHAR(20)  NULL,
  email           VARCHAR(254) NOT NULL,
  active          TINYINT(1)   NOT NULL DEFAULT 1,
  created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at      DATETIME     NULL,
  KEY idx_suppliers_org (organization_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ── Inventario ──────────────────────────────────────────────────────

-- upc: '4' + (1 si es serializado, 0 si no) + el id con 10 dígitos.
-- Lo pone la app justo después de insertar (MySQL no deja calcularlo con el AUTO_INCREMENT).
CREATE TABLE IF NOT EXISTS products (
  id                     BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id        BIGINT        NOT NULL,
  name                   VARCHAR(250)  NOT NULL,
  category_id            BIGINT        NULL,
  brand_id               BIGINT        NULL,
  supplier_id            BIGINT        NULL,
  upc                    VARCHAR(12)   NULL,
  image_url              VARCHAR(2048) NULL,
  item_condition         VARCHAR(100)  NULL,
  in_stock               INT           NOT NULL DEFAULT 0,
  stock_warning          INT           NOT NULL DEFAULT 0,
  reorder_point          INT           NOT NULL DEFAULT 0,
  cost_price             DECIMAL(18,2) NOT NULL DEFAULT 0,
  general_price          DECIMAL(18,2) NOT NULL DEFAULT 0,
  distributor_price      DECIMAL(18,2) NOT NULL DEFAULT 0,
  is_serialized          TINYINT(1)    NOT NULL DEFAULT 0,
  active                 TINYINT(1)    NOT NULL DEFAULT 1,
  created_at             DATETIME      NOT NULL DEFAULT CURRENT_TIMESTAMP,
  deleted_at             DATETIME      NULL,
  UNIQUE KEY uq_products_upc (upc),
  KEY idx_products_org (organization_id),
  CONSTRAINT fk_products_category FOREIGN KEY (category_id) REFERENCES categories(id),
  CONSTRAINT fk_products_brand    FOREIGN KEY (brand_id)    REFERENCES brands(id),
  CONSTRAINT fk_products_supplier FOREIGN KEY (supplier_id) REFERENCES suppliers(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Modelos compatibles de cada producto
CREATE TABLE IF NOT EXISTS product_models (
  product_id BIGINT NOT NULL,
  model_id   BIGINT NOT NULL,
  PRIMARY KEY (product_id, model_id),
  CONSTRAINT fk_product_models_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE,
  CONSTRAINT fk_product_models_model   FOREIGN KEY (model_id)   REFERENCES device_models(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- IMEI / número de serie de cada pieza de un producto serializado
CREATE TABLE IF NOT EXISTS product_serials (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id BIGINT       NOT NULL,
  product_id      BIGINT       NOT NULL,
  imei            VARCHAR(100) NOT NULL,
  status          VARCHAR(50)  NOT NULL DEFAULT 'Disponible',
  UNIQUE KEY uq_product_serials_imei (organization_id, imei),
  CONSTRAINT fk_product_serials_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- type: 'Entry' (entrada), 'Exit' (salida) o 'Adjustment' (ajuste)
CREATE TABLE IF NOT EXISTS inventory_movements (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id BIGINT       NOT NULL,
  product_id      BIGINT       NOT NULL,
  type            VARCHAR(20)  NOT NULL,
  quantity        INT          NOT NULL,
  previous_stock  INT          NOT NULL,
  current_stock   INT          NOT NULL,
  reason          VARCHAR(500) NULL,
  created_at      DATETIME     NOT NULL,
  KEY idx_inventory_movements_org (organization_id, product_id, created_at),
  CONSTRAINT fk_inventory_movements_product FOREIGN KEY (product_id) REFERENCES products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- status: Pending, Waiting, InProgress, ReceivedPartially, Received, InRma, Canceled
-- payment_status: Unpaid, Paid
CREATE TABLE IF NOT EXISTS purchase_orders (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id BIGINT        NOT NULL,
  supplier_id     BIGINT        NOT NULL,
  created_by      BIGINT        NOT NULL,
  status          VARCHAR(30)   NOT NULL DEFAULT 'Pending',
  payment_status  VARCHAR(30)   NOT NULL DEFAULT 'Unpaid',
  total           DECIMAL(18,2) NOT NULL DEFAULT 0,
  created_at      DATETIME      NOT NULL,
  KEY idx_purchase_orders_org (organization_id, created_at),
  CONSTRAINT fk_purchase_orders_supplier FOREIGN KEY (supplier_id) REFERENCES suppliers(id),
  CONSTRAINT fk_purchase_orders_user     FOREIGN KEY (created_by)  REFERENCES users(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS purchase_order_lines (
  id                BIGINT AUTO_INCREMENT PRIMARY KEY,
  purchase_order_id BIGINT        NOT NULL,
  product_id        BIGINT        NOT NULL,
  quantity          INT           NOT NULL,
  unit_cost         DECIMAL(18,2) NOT NULL DEFAULT 0,
  UNIQUE KEY uq_purchase_order_lines_product (purchase_order_id, product_id),
  CONSTRAINT fk_purchase_order_lines_order   FOREIGN KEY (purchase_order_id) REFERENCES purchase_orders(id) ON DELETE CASCADE,
  CONSTRAINT fk_purchase_order_lines_product FOREIGN KEY (product_id)        REFERENCES products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Recepciones de mercancía (GRN) de una orden de compra
CREATE TABLE IF NOT EXISTS goods_receipts (
  id                BIGINT AUTO_INCREMENT PRIMARY KEY,
  organization_id   BIGINT       NOT NULL,
  purchase_order_id BIGINT       NOT NULL,
  received_by       BIGINT       NOT NULL,
  notes             VARCHAR(500) NULL,
  created_at        DATETIME     NOT NULL,
  KEY idx_goods_receipts_org (organization_id, created_at),
  CONSTRAINT fk_goods_receipts_order FOREIGN KEY (purchase_order_id) REFERENCES purchase_orders(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS goods_receipt_lines (
  id                     BIGINT AUTO_INCREMENT PRIMARY KEY,
  goods_receipt_id       BIGINT NOT NULL,
  purchase_order_line_id BIGINT NOT NULL,
  product_id             BIGINT NOT NULL,
  ordered_quantity       INT    NOT NULL,
  received_quantity      INT    NOT NULL,
  UNIQUE KEY uq_goods_receipt_lines (goods_receipt_id, purchase_order_line_id),
  CONSTRAINT fk_goods_receipt_lines_receipt FOREIGN KEY (goods_receipt_id)       REFERENCES goods_receipts(id) ON DELETE CASCADE,
  CONSTRAINT fk_goods_receipt_lines_line    FOREIGN KEY (purchase_order_line_id) REFERENCES purchase_order_lines(id),
  CONSTRAINT fk_goods_receipt_lines_product FOREIGN KEY (product_id)             REFERENCES products(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;