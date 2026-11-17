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
    const cls = t === 'Entry' ? 'pill--success' : t === 'Exit' ? 'pill--danger' : 'pill--warning';
    return `<span class="label-small pill pill--bold ${cls}">${esc(typeLabel(t))}</span>`;
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
    const cur = { productId: null, type: null, serials: [], picked: new Set() };
    const m = ui.modal(`
      <div class="modal-card modal-card--lg modal-card--visible">
        <div class="modal-body">
          ${ui.modalHeader('Nuevo movimiento')}
          <div class="form-grid form-grid--roomy" data-form>
            ${ui.ddField({ label: 'Producto', id: 'ddProduct', required: true })}
            ${ui.ddField({ label: 'Tipo de movimiento', id: 'ddType', required: true })}
            <div class="field">
              <label class="label-normal field__label">Cantidad <span class="req">*</span></label>
              <input type="number" name="quantity" step="1" value="" placeholder="0"
                     class="input input--strong" />
              <span data-hint class="label-small field__hint"></span>
            </div>
            <div class="field field--span" data-serial-box hidden></div>
            <div class="field">
              <label class="label-normal field__label">Stock</label>
              <div data-preview class="body-normal stock-preview">—</div>
            </div>
            <div class="field field--span">
              <label class="label-normal field__label">Motivo (Opcional)</label>
              <textarea name="reason" rows="2" maxlength="500" placeholder="Ej. Conteo físico, merma, devolución..."
                        class="textarea"></textarea>
            </div>
          </div>
        </div>
        <div class="modal-footer modal-footer--soft">
          <button type="button" data-close class="btn btn--muted">Cancelar</button>
          <button type="button" data-save disabled class="btn btn--primary">Crear movimiento</button>
        </div>
      </div>`);
    const form = m.el.querySelector('[data-form]');
    const save = m.el.querySelector('[data-save]');
    const qty = form.querySelector('[name=quantity]');
    const hint = form.querySelector('[data-hint]');
    const preview = form.querySelector('[data-preview]');

    const serialBox = form.querySelector('[data-serial-box]');
    // Producto serializado: la entrada captura un IMEI por pieza; la salida elige IMEI disponibles (KAN-137).
    function paintSerials() {
      const product = products.find((p) => p.id === cur.productId);
      const q = parseInt(qty.value, 10);
      serialBox.hidden = !(product && product.isSerialized && (cur.type === 'Entry' || cur.type === 'Exit'));
      if (serialBox.hidden) return;
      if (cur.type === 'Entry') {
        const n = !isNaN(q) && q > 0 ? Math.min(q, 200) : 0;
        cur.serials.length = n;
        for (let i = 0; i < n; i++) if (cur.serials[i] === undefined) cur.serials[i] = '';
        serialBox.innerHTML = `<div class="serial-box">
          <p class="label-small serial-box__title">${n ? `Ingresa los ${n} IMEI / Series de las piezas que entran:` : 'Escribe la cantidad para capturar los IMEI.'}</p>
          <div class="serial-box__grid serial-box__grid--inputs">
            ${cur.serials.map((s, i) => `<div class="serial-box__unit">
              <label class="label-small detail-label">Unidad ${i + 1}</label>
              <input type="text" data-serial="${i}" value="${esc(s)}" maxlength="100" placeholder="IMEI o Serie" class="body-small serial-box__input" />
            </div>`).join('')}
          </div>
        </div>`;
      } else {
        const list = product.serialNumbers || [];
        serialBox.innerHTML = `<div class="serial-box">
          <p class="label-small serial-box__title">${list.length ? 'Elige los IMEI / Series que salen (la cantidad es lo que elijas):' : 'Este producto no tiene IMEI disponibles.'}</p>
          <div class="serial-box__grid">
            ${list.map((s) => `<label class="check"><input type="checkbox" data-pick value="${esc(s)}" ${cur.picked.has(s) ? 'checked' : ''} /> <span>${esc(s)}</span></label>`).join('')}
          </div>
        </div>`;
      }
    }
    serialBox.addEventListener('input', (e) => {
      if (e.target.dataset.serial !== undefined) cur.serials[Number(e.target.dataset.serial)] = e.target.value;
      sync(false);
    });
    serialBox.addEventListener('change', (e) => {
      if (e.target.dataset.pick === undefined) return;
      if (e.target.checked) cur.picked.add(e.target.value);
      else cur.picked.delete(e.target.value);
      if (cur.type === 'Exit') qty.value = cur.picked.size || '';
      sync(false);
    });

    function sync(repaint = true) {
      const product = products.find((p) => p.id === cur.productId);
      const serialized = !!(product && product.isSerialized);
      qty.readOnly = serialized && cur.type === 'Exit';
      if (repaint) paintSerials();
      const q = parseInt(qty.value, 10);
      hint.textContent = cur.type === 'Adjustment' ? 'En ajustes usa negativo para restar (ej. -2).' : cur.type ? 'Debe ser mayor a cero.' : '';
      let ok = !!product && !!cur.type && !isNaN(q) && (cur.type === 'Adjustment' ? q !== 0 : q > 0);
      if (serialized && cur.type === 'Adjustment') {
        ok = false;
        hint.textContent = 'Los productos serializados no admiten ajustes: usa entrada o salida con IMEI.';
      } else if (serialized && cur.type === 'Entry') {
        const vals = cur.serials.map((v) => v.trim().toLowerCase());
        if (vals.some((v) => !v) || new Set(vals).size !== vals.length) ok = false;
      } else if (serialized && cur.type === 'Exit') {
        if (cur.picked.size === 0) ok = false;
      }
      if (product) {
        const delta = isNaN(q) ? 0 : cur.type === 'Exit' ? -q : q;
        const next = product.inStock + (cur.type ? delta : 0);
        preview.innerHTML = `${product.inStock} → <strong class="${next < 0 ? 'text-error' : 'text-strong'}">${next}</strong>`;
        if (next < 0) ok = false;
      } else {
        preview.textContent = '—';
      }
      save.disabled = !ok;
    }

    ui.dropdown(m.el.querySelector('#ddProduct'), {
      items: products.map((p) => ({ value: p.id, label: `${p.name} (${p.inStock})${p.isSerialized ? ' · Serializado (IMEI)' : ''}` })),
      placeholder: products.length ? 'Seleccionar producto' : 'No hay productos',
      onChange: (v) => ((cur.productId = v), (cur.serials = []), cur.picked.clear(), sync()),
    });
    if (!products.length) {
      form.insertAdjacentHTML(
        'afterbegin',
        `<div class="body-normal notice notice--warning span-all">
           ${ui.icon('circle-info')}
           <span>Todavía no hay productos. Crea uno en <a href="/inventory/products">Inventario → Productos</a> para poder registrar movimientos.</span>
         </div>`
      );
    }
    ui.dropdown(m.el.querySelector('#ddType'), { items: TYPES, onChange: (v) => ((cur.type = v), (cur.serials = []), cur.picked.clear(), sync()) });
    qty.addEventListener('input', sync);

    function serialsToSend() {
      const product = products.find((p) => p.id === cur.productId);
      if (!product || !product.isSerialized) return undefined;
      if (cur.type === 'Entry') return cur.serials.map((v) => v.trim());
      if (cur.type === 'Exit') return [...cur.picked];
      return undefined;
    }

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        try {
          await Hook.post(endpoint, {
            productId: cur.productId,
            type: cur.type,
            quantity: parseInt(qty.value, 10),
            serials: serialsToSend(),
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
    const item = (label, value) => `<div><span class="label-small detail-label">${esc(label)}</span><span class="body-normal detail-value">${esc(value)}</span></div>`;
    ui.modal(`
      <div class="modal-card modal-card--lg modal-card--tall modal-card--clip">
        <div class="modal-top modal-top--bordered">
          <div class="modal-top__title"><h2 class="heading-5">Detalles del movimiento</h2>${typePill(mv.type)}</div>
          <button type="button" data-close class="modal-close" aria-label="Cerrar">${ui.icon('xmark')}</button>
        </div>
        <div class="modal-scroll modal-scroll--stack">
          <div>
            <h3 class="label-normal section-label section-label--spaced">Información Principal</h3>
            <div class="detail-grid detail-grid--2">
              ${item('ID', mv.id)}${item('Producto', mv.productName)}${item('Fecha', ui.dateTime(mv.createdAtUtc))}${item('Motivo', mv.reason || 'N/A')}
            </div>
          </div>
          <div>
            <h3 class="label-normal section-label section-label--spaced">Inventario</h3>
            <div class="detail-grid detail-grid--3">
              ${item('Cantidad', mv.quantity)}${item('Cantidad anterior', mv.previousStock)}${item('Cantidad nueva', mv.currentStock)}
            </div>
          </div>
        </div>
        <div class="modal-footer modal-footer--soft">
          <button type="button" data-close class="btn btn--muted">Cerrar</button>
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
