/* Inventario → Órdenes de compra (/api/:org/purchase-orders y /goods-receipts). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('purchase-orders');
  const canEdit = Hook.can('Inventory.Update');

  const STATUS = {
    pending: ['Pendiente', 'po-status--pending'],
    waiting: ['En espera', 'po-status--waiting'],
    inprogress: ['En progreso', 'po-status--progress'],
    receivedpartially: ['Recibido parcialmente', 'po-status--partial'],
    received: ['Recibido', 'po-status--received'],
    inrma: ['En RMA', 'po-status--rma'],
    canceled: ['Cancelado', 'po-status--canceled'],
    cancelled: ['Cancelado', 'po-status--canceled'],
  };
  const RECEIVABLE = ['pending', 'waiting', 'inprogress', 'receivedpartially'];
  const PAYMENT = { unpaid: 'Sin pagar', paid: 'Pagado' };
  const statusOf = (s) => STATUS[ui.norm(s)] || [s, 'po-status--pending'];
  const statusPill = (s) => `<span class="label-small pill pill--bold po-status ${statusOf(s)[1]}">${esc(statusOf(s)[0])}</span>`;
  const paymentLabel = (p) => PAYMENT[ui.norm(p)] || p;
  const PAYMENT_ITEMS = [
    { value: 'Unpaid', label: 'Sin pagar' },
    { value: 'Paid', label: 'Pagado' },
  ];
  const NEW_STATUS_ITEMS = [
    { value: 'Pending', label: 'Pendiente' },
    { value: 'Waiting', label: 'En espera' },
    { value: 'InProgress', label: 'En progreso' },
  ];
  // Estados que se pueden elegir a mano en una orden existente
  const MANUAL_STATUS_ITEMS = NEW_STATUS_ITEMS.concat([{ value: 'Canceled', label: 'Cancelado' }]);

  let products = [];
  let suppliers = [];

  const state = ui.listState({
    perPage: 5,
    search: (o, t) => String(o.purchaseOrderNumber).includes(t) || ui.norm(o.supplierName).includes(t) || ui.norm(o.userName).includes(t),
    sortValue: (o, k) => ({ id: o.purchaseOrderNumber, date: o.createdAtUtc, user: o.userName, paymentStatus: paymentLabel(o.paymentStatus) }[k] ?? o[k]),
    render,
  });

  const table = ui.table(document.getElementById('table'), {
    state,
    emptyText: 'No se encontraron órdenes de compra.',
    columns: [
      { key: 'purchaseOrderNumber', label: 'ID', width: 90, sort: 'id' },
      { key: 'createdAtUtc', label: 'Fecha de creación', width: 180, sort: 'date', render: (o) => esc(ui.date(o.createdAtUtc)), text: (o) => ui.date(o.createdAtUtc) },
      { key: 'supplierName', label: 'Proveedor', width: 180, sort: 'supplierName', title: (o) => o.supplierName },
      { key: 'userName', label: 'Usuario', width: 160, sort: 'user' },
      { key: 'paymentStatus', label: 'Estado del pago', width: 160, sort: 'paymentStatus', render: (o) => esc(paymentLabel(o.paymentStatus)) },
      { key: 'status', label: 'Estado de la orden', width: 200, render: (o) => statusPill(o.status), text: (o) => statusOf(o.status)[0] },
      { key: 'total', label: 'Total', width: 140, sort: 'total', render: (o) => ui.money(o.total), text: (o) => ui.money(o.total) },
    ],
    actions: () => ui.actionBtn('details', 'eye', 'Ver detalles'),
    onAction: (_, o) => openDetails(o),
  });

  function render() {
    table.render(state.pageRows);
    ui.pagination(document.getElementById('pager'), { page: state.page, total: state.totalPages, onChange: (p) => state.goTo(p), layout: 'center', goTo: false });
  }

  async function load() {
    try {
      const [orders, prods, sups] = await Promise.all([Hook.get(endpoint), Hook.get(Hook.org('products')), Hook.get(Hook.org('suppliers')).catch(() => [])]);
      products = prods.filter((p) => p.active);
      suppliers = sups.filter((s) => s.active);
      state.set(orders);
    } catch (err) {
      ui.error(err, 'No se pudieron cargar las órdenes de compra.');
    }
  }

  // ── Crear orden ─────────────────────────────────────────────────────
  function openCreate() {
    const order = { supplierId: null, lines: [], status: 'Pending', paymentStatus: 'Unpaid' };
    const today = new Date().toLocaleDateString('en-CA');
    const m = ui.modal(`
      <div class="modal-card modal-card--full modal-card--clip">
        <div class="modal-top modal-top--bordered">
          <h2 class="heading-5">Crear orden de compra</h2>
          <button type="button" data-close class="modal-close" aria-label="Cerrar">${ui.icon('xmark')}</button>
        </div>
        <div class="modal-body">
          <div class="po-form">
            <div class="po-form__general">
              <h3 class="label-normal section-label">Datos Generales</h3>
              <div class="field">
                <div class="field__label-row">
                  <label class="label-normal field__label">Proveedor <span class="req">*</span></label>
                  ${Hook.can('Suppliers.Update') ? ui.quickAddBtn('supplier', 'Nuevo') : ''}
                </div>
                <div id="ddSupplier"></div>
              </div>
              <div class="form-grid form-grid--2-always">
                ${ui.field({ label: 'Fecha', name: 'date', type: 'date', value: today, readonly: true })}
                ${ui.field({ label: 'Usuario', name: 'user', value: (Hook.user && Hook.user.name) || '', readonly: true })}
              </div>
              <div class="form-grid form-grid--2-always">
                ${ui.ddField({ label: 'Estado pago', id: 'ddPay' })}
                ${ui.ddField({ label: 'Estado orden', id: 'ddStatus' })}
              </div>
              <p class="label-small field__hint">El costo de cada producto es su costo actual. "Recibido" se marca solo al recibir la mercancía.</p>
            </div>
            <div class="po-form__lines">
              <div class="po-form__lines-head">
                <h3 class="label-normal section-label">Productos de la orden</h3>
                <button type="button" data-add-line class="btn btn--outline po-form__add">${ui.icon('plus')} Añadir producto</button>
              </div>
              <div data-lines class="po-lines"></div>
              <div class="po-total">
                <div class="po-total__box">
                  <span class="label-normal po-total__label">Gran Total:</span>
                  <span data-total class="heading-5">$0.00</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="modal-footer modal-footer--soft">
          <button type="button" data-close class="btn btn--muted">Cancelar</button>
          <button type="button" data-save disabled class="btn btn--primary">Guardar orden</button>
        </div>
      </div>`);

    const linesEl = m.el.querySelector('[data-lines]');
    const totalEl = m.el.querySelector('[data-total]');
    const save = m.el.querySelector('[data-save]');

    const productById = (id) => products.find((p) => p.id === id);
    const lineTotal = (l) => {
      const p = productById(l.productId);
      return p ? p.price * (l.quantity || 0) : 0;
    };

    function sync() {
      const total = order.lines.reduce((a, l) => a + lineTotal(l), 0);
      totalEl.textContent = ui.money(total);
      const ids = order.lines.map((l) => l.productId);
      const dupes = ids.some((id, i) => id && ids.indexOf(id) !== i);
      save.disabled = !order.supplierId || !order.lines.length || order.lines.some((l) => !l.productId || !(l.quantity >= 1)) || dupes;
    }

    function paintLines() {
      if (!order.lines.length) {
        linesEl.innerHTML = `
          <div class="po-lines__empty">
            <p class="body-normal">No hay productos en esta orden.</p>
            <button type="button" data-add-line class="btn btn--primary po-form__add">Agregar el primer producto</button>
          </div>`;
        sync();
        return;
      }
      linesEl.innerHTML = order.lines
        .map(
          (l, i) => `
          <div data-line="${i}" class="po-line">
            <button type="button" data-remove="${i}" title="Quitar" class="po-line__remove">${ui.icon('xmark')}</button>
            <div class="po-line__cell po-line__cell--product">
              <label class="label-small detail-label">Producto</label>
              <div data-dd-line="${i}"></div>
            </div>
            <div class="po-line__cell po-line__cell--qty">
              <label class="label-small detail-label">Cantidad</label>
              <input type="number" min="1" step="1" data-qty="${i}" value="${l.quantity}" class="input input--tight" />
            </div>
            <div class="po-line__cell po-line__cell--cost">
              <label class="label-small detail-label">Costo Un. ($)</label>
              <input type="text" readonly value="${ui.number2(productById(l.productId) ? productById(l.productId).price : 0)}" class="input input--tight input--muted" />
            </div>
            <div class="po-line__cell po-line__cell--total">
              <label class="label-small detail-label">Total Línea</label>
              <input type="text" readonly data-line-total="${i}" value="${ui.money(lineTotal(l))}" class="input input--tight input--muted input--bold" />
            </div>
          </div>`
        )
        .join('');
      order.lines.forEach((l, i) => {
        ui.dropdown(linesEl.querySelector(`[data-dd-line="${i}"]`), {
          items: products.map((p) => ({ value: p.id, label: p.name })),
          value: l.productId,
          placeholder: 'Selecciona un producto',
          onChange: (v) => {
            l.productId = v;
            paintLines();
          },
        });
      });
      sync();
    }

    function addLine() {
      order.lines.push({ productId: null, quantity: 1 });
      paintLines();
    }

    m.el.addEventListener('click', (e) => {
      if (e.target.closest('[data-add-line]')) addLine();
      const rm = e.target.closest('[data-remove]');
      if (rm) {
        order.lines.splice(Number(rm.dataset.remove), 1);
        paintLines();
      }
    });
    linesEl.addEventListener('input', (e) => {
      const i = e.target.dataset.qty;
      if (i === undefined) return;
      const l = order.lines[Number(i)];
      l.quantity = Math.max(0, parseInt(e.target.value, 10) || 0);
      linesEl.querySelector(`[data-line-total="${i}"]`).value = ui.money(lineTotal(l));
      sync();
    });

    const ddSupplier = ui.dropdown(m.el.querySelector('#ddSupplier'), {
      items: suppliers.map((s) => ({ value: s.id, label: s.name })),
      placeholder: suppliers.length ? 'Seleccionar proveedor' : 'No hay proveedores, crea uno',
      onChange: (v) => ((order.supplierId = v), sync()),
    });
    ui.dropdown(m.el.querySelector('#ddPay'), { items: PAYMENT_ITEMS, value: order.paymentStatus, onChange: (v) => (order.paymentStatus = v || 'Unpaid') });
    ui.dropdown(m.el.querySelector('#ddStatus'), { items: NEW_STATUS_ITEMS, value: order.status, onChange: (v) => (order.status = v || 'Pending') });
    m.el.addEventListener('click', (e) => {
      if (!e.target.closest('[data-quick-add="supplier"]')) return;
      ui.quickAdd({
        title: 'Nuevo proveedor',
        fields: [
          { name: 'name', label: 'Nombre', placeholder: 'Ej. Tech Source', required: true },
          { name: 'phone1', label: 'Teléfono', placeholder: '662 481 9204', required: true, phone: true },
          { name: 'email', label: 'Email', type: 'email', placeholder: 'ejemplo@proveedor.com', required: true },
        ],
        validate: (v) => (!ui.isPhone(v.phone1) ? 'El teléfono solo puede tener números (máximo 15 dígitos).' : !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(v.email) ? 'El email no es válido.' : ''),
        save: async (v) => ({ id: (await Hook.post(Hook.org('suppliers'), v)).id, name: v.name, active: true }),
        onCreated: (rec) => {
          suppliers.push(rec);
          ddSupplier.setItems(suppliers.map((s) => ({ value: s.id, label: s.name })));
          ddSupplier.setPlaceholder('Seleccionar proveedor');
          ddSupplier.setValue(rec.id, false);
        },
      });
    });
    addLine();

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        try {
          await Hook.post(endpoint, {
            supplierId: order.supplierId,
            status: order.status,
            paymentStatus: order.paymentStatus,
            products: order.lines.map((l) => ({ productId: l.productId, quantity: l.quantity })),
          });
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, 'Error al crear la orden de compra');
        }
      })
    );
  }

  // ── Detalle (formato de recibo) ─────────────────────────────────────
  function openDetails(order) {
    const receivable = canEdit && RECEIVABLE.includes(ui.norm(order.status));
    const lines = order.lines || [];
    const m = ui.modal(
      `
      <div class="modal-card modal-card--xl modal-card--clip receipt">
        <button type="button" data-close class="modal-close receipt__close" aria-label="Cerrar">${ui.icon('xmark')}</button>
        <div class="receipt__body">
          <div class="receipt__head">
            <div class="receipt__title">
              <h2 class="heading-4">Detalles de la compra</h2>
              <span class="heading-5">#${esc(order.purchaseOrderNumber)}</span>
            </div>
            <div class="receipt__date">${esc(ui.date(order.createdAtUtc))}</div>
          </div>
          <div class="body-normal receipt__meta">
            <span>Proveedor: <strong>${esc(order.supplierName)}</strong></span>
            <span class="sep">|</span>
            <span>Pago: <strong>${esc(paymentLabel(order.paymentStatus))}</strong></span>
            <span class="sep">|</span>
            ${statusPill(order.status)}
          </div>
          ${
            canEdit
              ? `<div class="receipt__status">
                   ${ui.ddField({ label: 'Estado del pago', id: 'ddEditPay' })}
                   ${ui.ddField({ label: 'Estado de la orden', id: 'ddEditStatus' })}
                   <button type="button" data-save-status disabled class="btn btn--primary">Guardar estado</button>
                 </div>`
              : ''
          }
          <div class="receipt__table">
            <div class="label-normal receipt__row receipt__row--head">
              <div class="receipt__c1">Descripción</div><div class="receipt__c2">Cantidad</div><div class="receipt__c3">Precio</div><div class="receipt__c4">Importe</div>
            </div>
            <div class="receipt__lines">
              ${
                lines.length
                  ? lines
                      .map(
                        (l) => `<div class="body-normal receipt__row">
                          <div class="receipt__c1" title="${esc(l.productName)}">${esc(l.productName)}</div>
                          <div class="receipt__c2">${esc(l.quantity)}</div>
                          <div class="receipt__c3">${ui.number2(l.unitCost)}</div>
                          <div class="receipt__c4">${ui.number2(l.lineTotal)}</div>
                        </div>`
                      )
                      .join('')
                  : '<div class="body-normal receipt__empty">No hay productos registrados</div>'
              }
            </div>
            <div class="receipt__total">
              <span class="label-large">Total a pagar:</span>
              <span class="receipt__total-amount">${ui.money(order.total)}</span>
            </div>
          </div>
          ${
            receivable
              ? `<div class="receipt__receive">
                   <p class="body-normal">¿Ya llegó la mercancía? Se suma al stock lo pendiente de cada producto.</p>
                   <button type="button" data-receive class="btn btn--success">${ui.icon('box-open')} Recibir mercancía</button>
                 </div>`
              : ''
          }
        </div>
      </div>`,
      { overlay: 'modal-overlay--receipt' }
    );
    const saveStatus = m.el.querySelector('[data-save-status]');
    if (saveStatus) {
      const edit = { status: order.status, paymentStatus: order.paymentStatus };
      const locked = ['received', 'receivedpartially', 'inrma'].includes(ui.norm(order.status));
      const syncStatus = () => (saveStatus.disabled = edit.status === order.status && edit.paymentStatus === order.paymentStatus);
      ui.dropdown(m.el.querySelector('#ddEditPay'), { items: PAYMENT_ITEMS, value: edit.paymentStatus, onChange: (v) => ((edit.paymentStatus = v), syncStatus()) });
      const ddEditStatus = ui.dropdown(m.el.querySelector('#ddEditStatus'), {
        items: locked ? [{ value: order.status, label: statusOf(order.status)[0] }] : MANUAL_STATUS_ITEMS,
        value: edit.status,
        onChange: (v) => ((edit.status = v), syncStatus()),
      });
      ddEditStatus.setDisabled(locked);
      saveStatus.addEventListener('click', () =>
        ui.busy(saveStatus, async () => {
          try {
            await Hook.patch(`${endpoint}/${order.id}`, edit);
            m.close();
            ui.toast('Cambios guardados');
            load();
          } catch (err) {
            ui.error(err, 'No se pudo cambiar el estado.');
          }
        })
      );
    }
    const receive = m.el.querySelector('[data-receive]');
    if (receive) {
      receive.addEventListener('click', () =>
        ui.busy(receive, async () => {
          try {
            await Hook.post(Hook.org('goods-receipts'), { purchaseOrderId: order.id, lines: null, notes: null });
            m.close();
            ui.toast('Recepción registrada');
            load();
          } catch (err) {
            ui.error(err, 'No se pudo registrar la recepción.');
          }
        })
      );
    }
  }

  function exportCSV() {
    ui.downloadCSV('ordenes-de-compra.csv', [
      { label: 'ID', value: (o) => o.purchaseOrderNumber },
      { label: 'Fecha', value: (o) => ui.date(o.createdAtUtc) },
      { label: 'Proveedor', value: (o) => o.supplierName },
      { label: 'Usuario', value: (o) => o.userName },
      { label: 'Estado del pago', value: (o) => paymentLabel(o.paymentStatus) },
      { label: 'Estado de la orden', value: (o) => statusOf(o.status)[0] },
      { label: 'Total', value: (o) => o.total },
    ], state.processed);
  }

  const addBtn = document.getElementById('addBtn');
  if (addBtn) addBtn.addEventListener('click', openCreate);
  document.getElementById('exportBtn').addEventListener('click', exportCSV);
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();
