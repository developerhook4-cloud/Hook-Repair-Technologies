/* Dashboard — GET /api/:orgId/dashboard/summary. */
(function () {
  'use strict';

  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('dashboard/summary');
  const root = document.getElementById('dashboard');

  const movementLabels = {
    Entry: 'Entrada',
    Exit: 'Salida',
    Adjustment: 'Ajuste',
  };

  function movementType(type) {
    return movementLabels[type] || type;
  }

  function movementClass(type) {
    if (type === 'Entry') return 'dashboard__movement-type--entry';
    if (type === 'Exit') return 'dashboard__movement-type--exit';
    return 'dashboard__movement-type--adjustment';
  }

  function lowStockContent(products) {
    if (!products.length) {
      return `
        <div class="dashboard__empty-panel">
          <i class="fa-solid fa-circle-check"></i>
          <p>No hay productos con stock bajo.</p>
        </div>`;
    }

    const rows = products
      .map(
        (product) => `
          <tr>
            <td>${esc(product.name)}</td>
            <td>${product.inStock}</td>
            <td>${product.stockWarning}</td>
            <td>${product.minimumReorderPoint}</td>
          </tr>`,
      )
      .join('');

    return `
      <div class="dashboard__table-wrap">
        <table class="dashboard__table">
          <thead>
            <tr>
              <th>Producto</th>
              <th>Existencia</th>
              <th>Alerta</th>
              <th>Punto de reorden</th>
            </tr>
          </thead>
          <tbody>${rows}</tbody>
        </table>
      </div>`;
  }

  function recentMovementsContent(movements) {
    if (!movements.length) {
      return `
        <div class="dashboard__empty-panel">
          <i class="fa-solid fa-box-open"></i>
          <p>Aún no hay movimientos de inventario.</p>
        </div>`;
    }

    return movements
      .map(
        (movement) => `
          <li class="dashboard__movement">
            <div>
              <strong>${esc(movement.productName)}</strong>
              <span>${esc(movement.reason || 'Sin motivo')}</span>
            </div>
            <div class="dashboard__movement-meta">
              <span class="dashboard__movement-type ${movementClass(movement.type)}">
                ${esc(movementType(movement.type))}
              </span>
              <span>${movement.quantity} unidades</span>
              <time>${esc(ui.dateTime(movement.createdAtUtc))}</time>
            </div>
          </li>`,
      )
      .join('');
  }

  function render(summary) {
    const hasNoData =
      summary.lowStockProductsCount === 0 &&
      summary.pendingOrdersCount === 0 &&
      summary.activeClientsCount === 0 &&
      summary.recentMovements.length === 0;

    root.innerHTML = `
      ${hasNoData
        ? `<section class="dashboard__empty">
             <div class="dashboard__empty-icon">
               <i class="fa-solid fa-chart-simple"></i>
             </div>
             <h2>Aún no hay información para mostrar</h2>
             <p>Cuando registres productos, clientes, órdenes o movimientos, aquí aparecerá el resumen del taller.</p>
           </section>`
        : ''}

      <section class="dashboard__kpis" aria-label="Indicadores principales">
        <article class="dashboard__kpi dashboard__kpi--warning">
          <div class="dashboard__kpi-icon">
            <i class="fa-solid fa-triangle-exclamation"></i>
          </div>
          <div>
            <p>Productos con stock bajo</p>
            <strong>${summary.lowStockProductsCount}</strong>
          </div>
        </article>

        <article class="dashboard__kpi dashboard__kpi--info">
          <div class="dashboard__kpi-icon">
            <i class="fa-solid fa-cart-shopping"></i>
          </div>
          <div>
            <p>Órdenes de compra pendientes</p>
            <strong>${summary.pendingOrdersCount}</strong>
          </div>
        </article>

        <article class="dashboard__kpi dashboard__kpi--success">
          <div class="dashboard__kpi-icon">
            <i class="fa-solid fa-users"></i>
          </div>
          <div>
            <p>Clientes activos</p>
            <strong>${summary.activeClientsCount}</strong>
          </div>
        </article>
      </section>

      <section class="dashboard__grid">
        <article class="page-card dashboard__panel">
          <div class="dashboard__panel-head">
            <div>
              <h2>Alertas de inventario</h2>
              <p>Productos que requieren atención.</p>
            </div>
            <a href="/inventory/products" class="dashboard__link">Ver productos</a>
          </div>
          ${lowStockContent(summary.lowStockProducts)}
        </article>

        <article class="page-card dashboard__panel">
          <div class="dashboard__panel-head">
            <div>
              <h2>Últimos movimientos</h2>
              <p>Actividad reciente de inventario.</p>
            </div>
            <a href="/inventory/movements" class="dashboard__link">Ver movimientos</a>
          </div>
          <ul class="dashboard__movements">
            ${recentMovementsContent(summary.recentMovements)}
          </ul>
        </article>
      </section>`;
  }

  async function load() {
    try {
      const summary = await Hook.get(endpoint);
      render(summary);
    } catch (error) {
      root.innerHTML = `
        <section class="dashboard__empty dashboard__empty--error">
          <div class="dashboard__empty-icon">
            <i class="fa-solid fa-circle-exclamation"></i>
          </div>
          <h2>No se pudo cargar el dashboard</h2>
          <p>Intenta recargar la página.</p>
        </section>`;
      ui.error(error, 'No se pudo cargar el dashboard.');
    }
  }

  load();
})();