/* Inventario → Órdenes de compra (/api/:org/purchase-orders y /goods-receipts). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('purchase-orders');
  const canEdit = Hook.can('Inventory.Update');

  const STATUS = {
    pending: ['Pendiente', 'bg-gray-100 text-gray-600 border-gray-200'],
    waiting: ['En espera', 'bg-blue-100 text-blue-700 border-blue-200'],
    inprogress: ['En progreso', 'bg-[#FFE8A1] text-[#A67C00] border-[#F2D172]'],
    receivedpartially: ['Recibido parcialmente', 'bg-purple-100 text-purple-700 border-purple-200'],
    received: ['Recibido', 'bg-green-100 text-green-700 border-green-200'],
    inrma: ['En RMA', 'bg-[#FFE4B5] text-[#D2691E] border-[#FFDAB9]'],
    canceled: ['Cancelado', 'bg-red-100 text-red-700 border-red-200'],
    cancelled: ['Cancelado', 'bg-red-100 text-red-700 border-red-200'],
  };
  const RECEIVABLE = ['pending', 'waiting', 'inprogress', 'receivedpartially'];
  const PAYMENT = { unpaid: 'Sin pagar', paid: 'Pagado' };
  const statusOf = (s) => STATUS[ui.norm(s)] || [s, 'bg-gray-100 text-gray-600 border-gray-200'];
  const statusPill = (s) => `<span class="px-4 py-1.5 rounded-full label-small inline-block font-bold whitespace-nowrap border ${statusOf(s)[1]}">${esc(statusOf(s)[0])}</span>`;
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
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-6xl flex flex-col max-h-[90vh] border border-gray-100 overflow-hidden animate-fade-in">
        <div class="p-6 md:px-8 md:pt-8 pb-4 flex justify-between items-start shrink-0 border-b border-gray-100">
          <h2 class="heading-5 text-dark-1">Crear orden de compra</h2>
          <button type="button" data-close class="text-gray-400 hover:text-dark-1 transition-colors">${ui.icon('xmark', 'text-xl')}</button>
        </div>
        <div class="p-6 md:p-8 overflow-y-auto flex-1 bg-white">
          <div class="flex flex-col lg:flex-row gap-10">
            <div class="lg:w-[34%] flex flex-col gap-5">
              <h3 class="label-normal font-bold text-dark-1">Datos Generales</h3>
              <div class="flex flex-col gap-1.5">
                <div class="flex items-center gap-2">
                  <label class="label-normal text-dark-1">Proveedor <span class="text-brand-primary">*</span></label>
                  ${Hook.can('Suppliers.Update') ? ui.quickAddBtn('supplier', 'Nuevo') : ''}
                </div>
                <div id="ddSupplier"></div>
              </div>
              <div class="grid grid-cols-2 gap-4">
                ${ui.field({ label: 'Fecha', name: 'date', type: 'date', value: today, readonly: true })}
                ${ui.field({ label: 'Usuario', name: 'user', value: (Hook.user && Hook.user.name) || '', readonly: true })}
              </div>
              <div class="grid grid-cols-2 gap-4">
                ${ui.ddField({ label: 'Estado pago', id: 'ddPay' })}
                ${ui.ddField({ label: 'Estado orden', id: 'ddStatus' })}
              </div>
              <p class="label-small text-gray-400">El costo de cada producto es su costo actual. "Recibido" se marca solo al recibir la mercancía.</p>
            </div>
            <div class="flex-1 flex flex-col gap-4 lg:border-l border-gray-100 lg:pl-10">
              <div class="flex items-center justify-between">
                <h3 class="label-normal font-bold text-dark-1">Productos de la orden</h3>
                <button type="button" data-add-line class="px-4 py-2 rounded-[12px] bg-gray-50 border border-gray-200 text-dark-2 label-normal font-semibold hover:bg-gray-100 transition-colors flex items-center gap-2">${ui.icon('plus', 'text-xs')} Añadir producto</button>
              </div>
              <div data-lines class="flex flex-col gap-3"></div>
              <div class="flex justify-end">
                <div class="bg-gray-50 border border-gray-100 rounded-[14px] px-6 py-3 flex items-center gap-4">
                  <span class="label-normal text-dark-2">Gran Total:</span>
                  <span data-total class="heading-5 text-dark-1">$0.00</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div class="p-6 md:px-8 bg-gray-50 flex justify-end gap-3 border-t border-gray-100 shrink-0">
          <button type="button" data-close class="px-5 py-2.5 rounded-lg bg-gray-200 text-dark-2 label-normal font-bold hover:bg-gray-300 transition-colors">Cancelar</button>
          <button type="button" data-save disabled class="px-5 py-2.5 rounded-lg bg-brand-primary text-white label-normal font-bold hover:opacity-90 transition-opacity shadow-sm disabled:opacity-50 disabled:cursor-not-allowed">Guardar orden</button>
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
          <div class="border-2 border-dashed border-gray-200 rounded-[20px] p-8 flex flex-col items-center justify-center gap-3 text-gray-400">
            <p class="body-normal">No hay productos en esta orden.</p>
            <button type="button" data-add-line class="px-4 py-2 rounded-[12px] bg-brand-primary text-white label-normal font-bold hover:opacity-90">Agregar el primer producto</button>
          </div>`;
        sync();
        return;
      }
      linesEl.innerHTML = order.lines
        .map(
          (l, i) => `
          <div data-line="${i}" class="bg-white border border-gray-200 p-4 rounded-[20px] flex flex-col sm:flex-row items-end gap-4 shadow-sm relative group">
            <button type="button" data-remove="${i}" title="Quitar" class="absolute top-2 right-2 text-gray-300 hover:text-red-500 hover:bg-red-50 w-8 h-8 rounded-lg flex items-center justify-center transition-colors z-20">${ui.icon('xmark')}</button>
            <div class="w-full sm:w-[40%] flex flex-col gap-1.5 relative">
              <label class="label-small text-gray-500 font-semibold">Producto</label>
              <div data-dd-line="${i}"></div>
            </div>
            <div class="w-full sm:w-[15%] flex flex-col gap-1.5">
              <label class="label-small text-gray-500 font-semibold">Cantidad</label>
              <input type="number" min="1" step="1" data-qty="${i}" value="${l.quantity}" class="w-full border border-gray-300 rounded-[14px] px-3 py-2.5 focus:outline-none focus:border-brand-primary body-normal text-dark-1" />
            </div>
            <div class="w-full sm:w-[20%] flex flex-col gap-1.5">
              <label class="label-small text-gray-500 font-semibold">Costo Un. ($)</label>
              <input type="text" readonly value="${ui.number2(productById(l.productId) ? productById(l.productId).price : 0)}" class="w-full border border-gray-200 rounded-[14px] px-3 py-2.5 body-normal text-dark-2 bg-gray-50" />
            </div>
            <div class="w-full sm:w-[25%] flex flex-col gap-1.5">
              <label class="label-small text-gray-500 font-semibold">Total Línea</label>
              <input type="text" readonly data-line-total="${i}" value="${ui.money(lineTotal(l))}" class="w-full border border-gray-200 rounded-[14px] px-3 py-2.5 body-normal font-bold text-dark-1 bg-gray-50" />
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
      <div class="bg-white rounded-[20px] shadow-2xl w-full max-w-3xl flex flex-col relative border border-gray-100 overflow-hidden animate-fade-in max-h-[90vh]">
        <button type="button" data-close class="absolute top-4 right-4 text-gray-400 hover:text-dark-1 transition-colors z-10 p-1">${ui.icon('xmark', 'text-xl')}</button>
        <div class="p-8 md:p-10 flex-1 bg-white overflow-y-auto">
          <div class="flex flex-col sm:flex-row justify-between sm:items-center mb-8 border-b border-gray-100 pb-6 pr-4 sm:pr-8 gap-2">
            <div class="flex items-baseline gap-2 flex-wrap">
              <h2 class="heading-4 font-bold text-dark-1">Detalles de la compra</h2>
              <span class="heading-5 font-bold text-dark-1">#${esc(order.purchaseOrderNumber)}</span>
            </div>
            <div class="text-xs font-bold text-dark-1 uppercase tracking-wide">${esc(ui.date(order.createdAtUtc))}</div>
          </div>
          <div class="flex flex-wrap items-center gap-3 mb-6 body-normal text-dark-2">
            <span>Proveedor: <strong class="text-dark-1">${esc(order.supplierName)}</strong></span>
            <span class="text-gray-300">|</span>
            <span>Pago: <strong class="text-dark-1">${esc(paymentLabel(order.paymentStatus))}</strong></span>
            <span class="text-gray-300">|</span>
            ${statusPill(order.status)}
          </div>
          ${
            canEdit
              ? `<div class="grid grid-cols-1 sm:grid-cols-3 gap-4 items-end mb-8 bg-gray-50 border border-gray-100 rounded-2xl p-4">
                   ${ui.ddField({ label: 'Estado del pago', id: 'ddEditPay' })}
                   ${ui.ddField({ label: 'Estado de la orden', id: 'ddEditStatus' })}
                   <button type="button" data-save-status disabled class="px-5 py-2.5 rounded-lg bg-brand-primary text-white label-normal font-bold hover:opacity-90 transition-opacity shadow-sm disabled:opacity-50 disabled:cursor-not-allowed">Guardar estado</button>
                 </div>`
              : ''
          }
          <div class="w-full">
            <div class="bg-brand-primary text-white px-6 py-3 rounded-[10px] flex label-normal font-bold mb-4">
              <div class="w-[45%]">Descripción</div><div class="w-[15%] text-center">Cantidad</div><div class="w-[20%] text-center">Precio</div><div class="w-[20%] text-right">Importe</div>
            </div>
            <div class="flex flex-col px-6 gap-3 min-h-37.5 mb-4">
              ${
                lines.length
                  ? lines
                      .map(
                        (l) => `<div class="flex body-normal text-dark-1">
                          <div class="w-[45%] pr-4 truncate" title="${esc(l.productName)}">${esc(l.productName)}</div>
                          <div class="w-[15%] text-center">${esc(l.quantity)}</div>
                          <div class="w-[20%] text-center">${ui.number2(l.unitCost)}</div>
                          <div class="w-[20%] text-right">${ui.number2(l.lineTotal)}</div>
                        </div>`
                      )
                      .join('')
                  : '<div class="text-center py-4 text-gray-400 italic body-normal">No hay productos registrados</div>'
              }
            </div>
            <div class="bg-brand-primary text-white px-6 py-4 rounded-[10px] flex justify-end items-center gap-4">
              <span class="label-large font-bold">Total a pagar:</span>
              <span class="text-2xl font-bold">${ui.money(order.total)}</span>
            </div>
          </div>
          ${
            receivable
              ? `<div class="mt-6 flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-t border-gray-100 pt-6">
                   <p class="body-normal text-dark-2">¿Ya llegó la mercancía? Se suma al stock lo pendiente de cada producto.</p>
                   <button type="button" data-receive class="px-5 py-2.5 rounded-lg bg-success-primary text-white label-normal font-bold hover:bg-success-darker transition-colors shadow-sm disabled:opacity-50 whitespace-nowrap">${ui.icon('box-open')} Recibir mercancía</button>
                 </div>`
              : ''
          }
        </div>
      </div>`,
      { z: 'z-70', overlayClass: 'bg-gray-500/30' }
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