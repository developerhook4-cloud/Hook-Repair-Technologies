/* Inventario → Movimientos (/api/:org/inventory/movements). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('inventory/movements');

  const TYPES = [
    { value: 'Entry', label: 'Entrada' },
    { value: 'Exit', label: 'Salida' },
    { value: 'Adjustment', label: 'Ajuste' },
  ];
  const typeLabel = (t) => (TYPES.find((x) => x.value === t) || { label: t }).label;
  const typePill = (t) => {
    const cls =
      t === 'Entry' ? 'bg-success-subtle text-success-darker' : t === 'Exit' ? 'bg-error-subtle text-error-primary' : 'bg-warning-subtle text-warning-darker';
    return `<span class="px-4 py-1.5 rounded-full label-small inline-block font-bold whitespace-nowrap ${cls}">${esc(typeLabel(t))}</span>`;
  };
  let products = [];

  const state = ui.listState({
    perPage: 10,
    search: (m, t) => ui.norm(m.productName).includes(t) || ui.norm(typeLabel(m.type)).includes(t) || ui.norm(m.reason).includes(t) || ui.norm(ui.date(m.createdAtUtc)).includes(t),
    sortValue: (m, k) => (k === 'product' ? m.productName : k === 'date' ? m.createdAtUtc : k === 'type' ? typeLabel(m.type) : m[k]),
    render,
  });

  const table = ui.table(document.getElementById('table'), {
    state,
    emptyText: 'No se encontraron movimientos.',
    columns: [
      { key: 'productName', label: 'Producto', width: 220, sort: 'product', title: (m) => m.productName },
      { key: 'quantity', label: 'Cantidad', width: 110, sort: 'quantity' },
      { key: 'type', label: 'Tipo de movimiento', width: 170, sort: 'type', render: (m) => typePill(m.type), text: (m) => typeLabel(m.type) },
      { key: 'previousStock', label: 'Cantidad anterior', width: 160, sort: 'previousStock' },
      { key: 'currentStock', label: 'Nueva cantidad', width: 150, sort: 'currentStock' },
      { key: 'createdAtUtc', label: 'Fecha', width: 190, sort: 'date', render: (m) => esc(ui.dateTime(m.createdAtUtc)), text: (m) => ui.dateTime(m.createdAtUtc) },
    ],
    actions: () => ui.actionBtn('details', 'eye', 'Ver detalles'),
    onAction: (_, m) => openDetails(m),
  });

  function render() {
    table.render(state.pageRows);
    ui.pagination(document.getElementById('pager'), { page: state.page, total: state.totalPages, onChange: (p) => state.goTo(p) });
  }

  async function load() {
    try {
      const [movements, prods] = await Promise.all([Hook.get(endpoint), Hook.get(Hook.org('products'))]);
      products = prods.filter((p) => p.active);
      state.set(movements);
    } catch (err) {
      ui.error(err, 'No se pudieron cargar los movimientos.');
    }
  }

  function openCreate() {
    const cur = { productId: null, type: null };
    const m = ui.modal(`
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-2xl flex flex-col border border-gray-100 overflow-visible animate-fade-in">
        <div class="p-6 md:p-8 pb-4">
          ${ui.modalHeader('Nuevo movimiento')}
          <div class="grid grid-cols-1 md:grid-cols-2 gap-5" data-form>
            ${ui.ddField({ label: 'Producto', id: 'ddProduct', required: true })}
            ${ui.ddField({ label: 'Tipo de movimiento', id: 'ddType', required: true })}
            <div class="flex flex-col gap-1.5">
              <label class="label-normal font-semibold text-dark-1">Cantidad <span class="text-brand-primary">*</span></label>
              <input type="number" name="quantity" step="1" value="" placeholder="0"
                     class="w-full border border-gray-300 rounded-[14px] px-4 py-2 focus:outline-none focus:border-brand-primary body-normal font-bold text-dark-1 bg-white" />
              <span data-hint class="label-small text-gray-400"></span>
            </div>
            <div class="flex flex-col gap-1.5">
              <label class="label-normal font-semibold text-dark-1">Stock</label>
              <div data-preview class="w-full border border-gray-200 bg-gray-50 rounded-[14px] px-4 py-2 body-normal text-dark-2">—</div>
            </div>
            <div class="flex flex-col gap-1.5 md:col-span-2">
              <label class="label-normal font-semibold text-dark-1">Motivo (Opcional)</label>
              <textarea name="reason" rows="2" maxlength="500" placeholder="Ej. Conteo físico, merma, devolución..."
                        class="w-full border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal text-dark-1 resize-none"></textarea>
            </div>
          </div>
        </div>
        <div class="p-6 md:px-8 bg-gray-50 flex justify-end gap-3 mt-4 border-t border-gray-100 rounded-b-2xl">
          <button type="button" data-close class="px-5 py-2.5 rounded-lg bg-gray-200 text-dark-2 label-normal font-bold hover:bg-gray-300 transition-colors">Cancelar</button>
          <button type="button" data-save disabled class="px-5 py-2.5 rounded-lg bg-brand-primary text-white label-normal font-bold hover:opacity-90 transition-opacity shadow-sm disabled:opacity-50 disabled:cursor-not-allowed">Crear movimiento</button>
        </div>
      </div>`);
    const form = m.el.querySelector('[data-form]');
    const save = m.el.querySelector('[data-save]');
    const qty = form.querySelector('[name=quantity]');
    const hint = form.querySelector('[data-hint]');
    const preview = form.querySelector('[data-preview]');

    function sync() {
      const q = parseInt(qty.value, 10);
      const product = products.find((p) => p.id === cur.productId);
      hint.textContent = cur.type === 'Adjustment' ? 'En ajustes usa negativo para restar (ej. -2).' : cur.type ? 'Debe ser mayor a cero.' : '';
      let ok = !!product && !!cur.type && !isNaN(q) && (cur.type === 'Adjustment' ? q !== 0 : q > 0);
      if (product) {
        const delta = isNaN(q) ? 0 : cur.type === 'Exit' ? -q : q;
        const next = product.inStock + (cur.type ? delta : 0);
        preview.innerHTML = `${product.inStock} → <strong class="${next < 0 ? 'text-error-primary' : 'text-dark-1'}">${next}</strong>`;
        if (next < 0) ok = false;
      } else {
        preview.textContent = '—';
      }
      save.disabled = !ok;
    }

    ui.dropdown(m.el.querySelector('#ddProduct'), {
      items: products.map((p) => ({ value: p.id, label: `${p.name} (${p.inStock})` })),
      placeholder: products.length ? 'Seleccionar producto' : 'No hay productos',
      onChange: (v) => ((cur.productId = v), sync()),
    });
    if (!products.length) {
      form.insertAdjacentHTML(
        'afterbegin',
        `<div class="md:col-span-2 flex items-center gap-3 bg-warning-subtle text-warning-darker rounded-[14px] px-4 py-3 body-normal">
           ${ui.icon('circle-info')}
           <span>Todavía no hay productos. Crea uno en <a href="/inventory/products" class="font-bold underline">Inventario → Productos</a> para poder registrar movimientos.</span>
         </div>`
      );
    }
    ui.dropdown(m.el.querySelector('#ddType'), { items: TYPES, onChange: (v) => ((cur.type = v), sync()) });
    qty.addEventListener('input', sync);

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        try {
          await Hook.post(endpoint, {
            productId: cur.productId,
            type: cur.type,
            quantity: parseInt(qty.value, 10),
            reason: form.querySelector('[name=reason]').value.trim() || null,
          });
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, 'Error al crear movimiento');
        }
      })
    );
  }

  function openDetails(mv) {
    const item = (label, value) => `<div><span class="label-small text-gray-500 block">${esc(label)}</span><span class="body-normal font-semibold text-dark-1 break-words">${esc(value)}</span></div>`;
    ui.modal(`
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-2xl flex flex-col max-h-[85vh] overflow-hidden border border-gray-100 animate-fade-in">
        <div class="p-6 md:px-8 md:pt-8 pb-4 flex justify-between items-start shrink-0 border-b border-gray-100">
          <div class="flex items-center gap-3"><h2 class="heading-5 text-dark-1">Detalles del movimiento</h2>${typePill(mv.type)}</div>
          <button type="button" data-close class="text-gray-400 hover:text-dark-1 transition-colors">${ui.icon('xmark', 'text-xl')}</button>
        </div>
        <div class="p-6 md:px-8 overflow-y-auto flex-1 flex flex-col gap-6">
          <div>
            <h3 class="label-normal font-bold text-dark-1 mb-3">Información Principal</h3>
            <div class="grid grid-cols-2 gap-4 bg-gray-50 p-4 rounded-2xl border border-gray-100">
              ${item('ID', mv.id)}${item('Producto', mv.productName)}${item('Fecha', ui.dateTime(mv.createdAtUtc))}${item('Motivo', mv.reason || 'N/A')}
            </div>
          </div>
          <div>
            <h3 class="label-normal font-bold text-dark-1 mb-3">Inventario</h3>
            <div class="grid grid-cols-3 gap-4 bg-gray-50 p-4 rounded-2xl border border-gray-100">
              ${item('Cantidad', mv.quantity)}${item('Cantidad anterior', mv.previousStock)}${item('Cantidad nueva', mv.currentStock)}
            </div>
          </div>
        </div>
        <div class="p-6 bg-gray-50 flex justify-end border-t border-gray-100 shrink-0">
          <button type="button" data-close class="px-6 py-2.5 rounded-lg bg-gray-200 text-dark-2 label-normal font-bold hover:bg-gray-300 transition-colors">Cerrar</button>
        </div>
      </div>`);
  }

  function exportCSV() {
    ui.downloadCSV('movimientos.csv', [
      { label: 'Fecha', value: (m) => ui.dateTime(m.createdAtUtc) },
      { label: 'Producto', value: (m) => m.productName },
      { label: 'Tipo', value: (m) => typeLabel(m.type) },
      { label: 'Cantidad', value: (m) => m.quantity },
      { label: 'Cantidad anterior', value: (m) => m.previousStock },
      { label: 'Nueva cantidad', value: (m) => m.currentStock },
      { label: 'Motivo', value: (m) => m.reason },
    ], state.processed);
  }

  const addBtn = document.getElementById('addBtn');
  if (addBtn) addBtn.addEventListener('click', openCreate);
  document.getElementById('exportBtn').addEventListener('click', exportCSV);
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();