/* Inventario → Productos (/api/:org/products). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('products');
  const canEdit = Hook.can('Inventory.Update');
  const canDelete = Hook.can('Inventory.Delete');
  const CONDITIONS = ['Nuevo', 'Reacondicionado', 'Usado'];

  let categories = [];
  let brands = [];
  let models = [];
  let suppliers = [];
  let filters = { name: '', category: '', brand: '', model: '', supplier: '' };

  const statusPill = (status) => {
    const s = ui.norm(status);
    const cls = s === 'agotado' ? 'pill--danger' : s === 'bajo stock' ? 'pill--warning' : 'pill--success';
    return `<span class="label-small pill pill--bold ${cls}">${esc(status)}</span>`;
  };

  const state = ui.listState({
    perPage: 10,
    search: (p, t) =>
      String(p.productNumber).includes(t) ||
      ui.norm(p.upc).includes(t) ||
      ui.norm(p.name).includes(t) ||
      ui.norm(p.categoryName).includes(t) ||
      ui.norm(p.brandName).includes(t) ||
      (p.modelNames || []).some((m) => ui.norm(m).includes(t)) ||
      ui.norm(p.supplierName).includes(t) ||
      (p.serialNumbers || []).some((s) => ui.norm(s).includes(t)),
    filters: (p) =>
      (!filters.name || ui.norm(p.name).includes(filters.name.toLowerCase())) &&
      (!filters.category || p.categoryId === filters.category) &&
      (!filters.brand || p.brandId === filters.brand) &&
      (!filters.model || (p.modelNames || []).some((m) => ui.norm(m).includes(filters.model.toLowerCase()))) &&
      (!filters.supplier || p.supplierId === filters.supplier),
    sortValue: (p, k) =>
      ({ id: p.productNumber, category: p.categoryName, brand: p.brandName, models: p.modelName, stock: p.inStock, status: p.status }[k] ?? p[k]),
    render,
  });

  const actions = (p) =>
    (canEdit ? ui.actionBtn('edit', 'pen', 'Editar') : '') +
    (canDelete ? ui.actionBtn('delete', 'trash-can', 'Eliminar') : '') +
    ui.actionBtn('details', 'eye', 'Ver detalles');

  const table = ui.table(document.getElementById('table'), {
    state,
    emptyText: 'No se encontraron productos.',
    columns: [
      { key: 'productNumber', label: 'ID', width: 80, sort: 'id' },
      { key: 'upc', label: 'UPC', width: 170, sort: 'upc' },
      { key: 'categoryName', label: 'Categoría', width: 150, sort: 'category', render: (p) => esc(p.categoryName || '-') },
      { key: 'brandName', label: 'Marca', width: 130, sort: 'brand', render: (p) => esc(p.brandName || '-') },
      { key: 'modelName', label: 'Modelo', width: 130, sort: 'models', render: (p) => esc((p.modelNames || []).join(', ') || '-'), title: (p) => (p.modelNames || []).join(', ') },
      { key: 'name', label: 'Nombre', width: 220, sort: 'name', title: (p) => p.name },
      { key: 'status', label: 'Estado', width: 150, sort: 'status', render: (p) => statusPill(p.status) },
      { key: 'inStock', label: 'En stock', width: 120, sort: 'stock' },
    ],
    actions,
    onAction: (action, p) => (action === 'edit' ? openModal(p) : action === 'delete' ? openDelete(p) : openDetails(p)),
  });

  function render() {
    table.render(state.pageRows);
    ui.pagination(document.getElementById('pager'), { page: state.page, total: state.totalPages, onChange: (p) => state.goTo(p), layout: 'center' });
    const n = Object.values(filters).filter(Boolean).length;
    const badge = document.getElementById('filterCount');
    badge.hidden = !n;
    badge.textContent = n;
  }

  async function load() {
    try {
      const [products, cats, brs, mds, sups] = await Promise.all([
        Hook.get(endpoint),
        Hook.get(Hook.org('categories')).catch(() => []),
        Hook.get(Hook.org('brands')).catch(() => []),
        Hook.get(Hook.org('models')).catch(() => []),
        Hook.get(Hook.org('suppliers')).catch(() => []),
      ]);
      categories = cats.filter((c) => c.active);
      brands = brs.filter((b) => b.active);
      models = mds.filter((m) => m.active);
      suppliers = sups.filter((s) => s.active);
      state.set(products.filter((p) => p.active));
    } catch (err) {
      ui.error(err, 'No se pudieron cargar los productos.');
    }
  }

  const items = (arr) => arr.map((x) => ({ value: x.id, label: x.name }));

  // ── Crear / editar ──────────────────────────────────────────────────
  function openModal(product) {
    const editing = !!product;
    const p = product || {};
    const cur = {
      categoryId: p.categoryId || null,
      brandId: p.brandId || null,
      modelId: p.modelId || null,
      supplierId: p.supplierId || null,
      condition: p.condition || null,
      compatible: new Set((p.modelIds || []).filter((id) => id !== p.modelId)),
    };
    const num = (label, name, value, extra = '') => `
      <div class="field">
        <label class="label-normal field__label">${esc(label)}</label>
        <input type="number" name="${name}" value="${esc(value)}" step="any" min="0" ${extra}
               class="input input--compact" />
      </div>`;
    const ddf = (label, id, required, quick) => `
      <div class="field">
        <div class="field__label-row">
          <label class="label-normal field__label">${esc(label)}${required ? ' <span class="req">*</span>' : ''}</label>
          ${quick ? ui.quickAddBtn(quick[0], quick[1]) : ''}
        </div>
        <div id="${id}"></div>
      </div>`;
    const canCatalogs = Hook.can('Catalogs.Update');
    const canSuppliers = Hook.can('Suppliers.Update');

    const m = ui.modal(`
      <div class="modal-card modal-card--3xl modal-card--clip">
        <div class="modal-top modal-top--bordered">
          <h2 class="heading-5">${editing ? 'Editar producto' : 'Crear producto'}</h2>
          <button type="button" data-close class="modal-close" aria-label="Cerrar">${ui.icon('xmark')}</button>
        </div>
        <div class="modal-body" data-form>
          <div class="product-form">
            <div class="product-form__col">
              <h3 class="label-normal section-label">Información General</h3>
              <div class="field">
                <label class="label-normal field__label">Nombre del producto <span class="req">*</span></label>
                <input type="text" name="name" maxlength="250" value="${esc(p.name || '')}" placeholder="Ej. Pantalla OLED iPhone 13"
                       class="input" />
              </div>
              <div class="form-grid form-grid--2-always">${ddf('Categoría', 'ddCat', true, canCatalogs && ['category', 'Nueva'])}${ddf('Marca', 'ddBrand', true, canCatalogs && ['brand', 'Nueva'])}</div>
              <div class="form-grid form-grid--2-always">${ddf('Dispositivo asociado', 'ddModel', false, canCatalogs && ['model', 'Nuevo'])}${ddf('Condición', 'ddCond')}</div>
              <div class="field">
                <label class="label-normal field__label">Modelos compatibles</label>
                <div data-compatible class="chip-box"></div>
              </div>
              <div class="form-grid form-grid--2-always">
                ${ddf('Proveedor', 'ddSupplier', false, canSuppliers && ['supplier', 'Nuevo'])}
                <div class="field">
                  <label class="label-normal field__label">UPC</label>
                  <input type="text" value="${esc(p.upc || '')}" placeholder="Se genera al guardar" readonly
                         class="input" />
                </div>
              </div>
            </div>

            <div class="product-form__col product-form__col--side">
              <h3 class="label-normal section-label">Precios e Inventario</h3>
              <div class="product-form__trio">
                ${num('Costo ($)', 'price', editing ? p.price : 0)}
                ${num('P. General ($)', 'generalSalePrice', editing ? p.generalSalePrice : 0)}
                ${num('P. Distrib. ($)', 'distributorSalePrice', editing ? p.distributorSalePrice : 0)}
              </div>
              <div class="product-form__trio">
                ${num(editing ? 'Stock actual' : 'Stock Inicial', 'inStock', editing ? p.inStock : 0, editing ? 'disabled title="El stock cambia con los movimientos de inventario"' : '')}
                ${num('Warning Stock', 'stockWarning', editing ? p.stockWarning : 0)}
                ${num('P. Reorden', 'minimumReorderPoint', editing ? p.minimumReorderPoint : 0)}
              </div>
              <div class="product-form__serial">
                <label class="label-normal check${editing ? ' is-disabled' : ''}">
                  <input type="checkbox" name="isSerialized" ${p.isSerialized ? 'checked' : ''} ${editing ? 'disabled' : ''}
                         class="checkbox" />
                  Inventario Serializado (Requerir IMEI/Serie)
                </label>
                <div data-serials class="serial-box" hidden></div>
              </div>
              ${editing ? '<p class="label-small field__hint">El stock se ajusta desde Inventario → Movimientos.</p>' : ''}
            </div>
          </div>
        </div>
        <div class="modal-footer modal-footer--soft">
          <button type="button" data-close class="btn btn--muted">Cancelar</button>
          <button type="button" data-save disabled class="btn btn--primary">${editing ? 'Guardar cambios' : 'Crear producto'}</button>
        </div>
      </div>`);

    const form = m.el.querySelector('[data-form]');
    const save = m.el.querySelector('[data-save]');
    const serialBox = m.el.querySelector('[data-serials]');
    const compatBox = m.el.querySelector('[data-compatible]');
    let serials = (p.serialNumbers || []).slice();

    const valid = () => form.querySelector('[name=name]').value.trim() !== '' && !!cur.categoryId && !!cur.brandId;
    const sync = () => (save.disabled = !valid());

    const brandModels = () => models.filter((x) => !cur.brandId || x.brandId === cur.brandId);
    function paintCompatible() {
      const list = brandModels().filter((x) => x.id !== cur.modelId);
      compatBox.innerHTML = list.length
        ? list
            .map((x) => {
              const on = cur.compatible.has(x.id);
              return `<button type="button" data-model="${x.id}" class="label-small chip${on ? ' is-on' : ''}">${esc(x.name)}</button>`;
            })
            .join('')
        : `<span class="body-normal chip-box__empty">${cur.brandId ? 'Esta marca no tiene más modelos.' : 'Selecciona una marca para ver sus modelos.'}</span>`;
    }
    compatBox.addEventListener('click', (e) => {
      const b = e.target.closest('[data-model]');
      if (!b) return;
      const id = b.dataset.model;
      cur.compatible.has(id) ? cur.compatible.delete(id) : cur.compatible.add(id);
      paintCompatible();
    });

    const ddCat = ui.dropdown(m.el.querySelector('#ddCat'), { items: items(categories), value: cur.categoryId, onChange: (v) => ((cur.categoryId = v), sync()) });
    const ddModel = ui.dropdown(m.el.querySelector('#ddModel'), {
      items: items(brandModels()),
      value: cur.modelId,
      none: 'Ninguno',
      onChange: (v) => {
        cur.modelId = v;
        cur.compatible.delete(v);
        paintCompatible();
      },
    });
    const ddBrand = ui.dropdown(m.el.querySelector('#ddBrand'), {
      items: items(brands),
      value: cur.brandId,
      onChange: (v) => {
        cur.brandId = v;
        if (cur.modelId && !brandModels().some((x) => x.id === cur.modelId)) cur.modelId = null;
        [...cur.compatible].forEach((id) => !brandModels().some((x) => x.id === id) && cur.compatible.delete(id));
        ddModel.setItems(items(brandModels()));
        ddModel.setValue(cur.modelId);
        paintCompatible();
        sync();
      },
    });
    ui.dropdown(m.el.querySelector('#ddCond'), { items: CONDITIONS.map((c) => ({ value: c, label: c })), value: cur.condition, none: 'Sin especificar', onChange: (v) => (cur.condition = v) });
    const ddSupplier = ui.dropdown(m.el.querySelector('#ddSupplier'), { items: items(suppliers), value: cur.supplierId, none: 'Ninguno', up: true, onChange: (v) => (cur.supplierId = v) });
    paintCompatible();

    // Alta rápida de categoría, marca, modelo y proveedor sin salir del formulario
    m.el.addEventListener('click', (e) => {
      const b = e.target.closest('[data-quick-add]');
      if (!b) return;
      const kind = b.dataset.quickAdd;
      if (kind === 'category') {
        ui.quickAdd({
          title: 'Nueva categoría',
          fields: [{ name: 'name', label: 'Nombre', placeholder: 'Ej. Pantallas', required: true }],
          save: async (v) => ({ id: (await Hook.post(Hook.org('categories'), { name: v.name, icon: '' })).id, name: v.name, active: true }),
          onCreated: (rec) => {
            categories.push(rec);
            ddCat.setItems(items(categories));
            ddCat.setValue(rec.id, false);
          },
        });
      } else if (kind === 'brand') {
        ui.quickAdd({
          title: 'Nueva marca',
          fields: [{ name: 'name', label: 'Nombre', placeholder: 'Ej. Apple', required: true }],
          save: async (v) => ({ id: (await Hook.post(Hook.org('brands'), { name: v.name })).id, name: v.name, active: true }),
          onCreated: (rec) => {
            brands.push(rec);
            ddBrand.setItems(items(brands));
            ddBrand.setValue(rec.id, false);
          },
        });
      } else if (kind === 'model') {
        const brand = brands.find((x) => x.id === cur.brandId);
        if (!brand) return ui.toast('Primero selecciona la marca del dispositivo.', 'error');
        ui.quickAdd({
          title: 'Nuevo dispositivo',
          note: `Marca: ${brand.name}`,
          fields: [{ name: 'name', label: 'Modelo', placeholder: 'Ej. iPhone 16', required: true }],
          save: async (v) => ({
            id: (await Hook.post(Hook.org('models'), { name: v.name, brandId: brand.id })).id,
            name: v.name,
            brandId: brand.id,
            brandName: brand.name,
            active: true,
          }),
          onCreated: (rec) => {
            models.push(rec);
            ddModel.setItems(items(brandModels()));
            ddModel.setValue(rec.id, false);
          },
        });
      } else if (kind === 'supplier') {
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
            ddSupplier.setItems(items(suppliers));
            ddSupplier.setValue(rec.id, false);
          },
        });
      }
    });

    // IMEI / series: un campo por cada pieza del stock inicial
    function paintSerials() {
      const serialized = form.querySelector('[name=isSerialized]').checked;
      const stock = Math.max(0, parseInt(form.querySelector('[name=inStock]').value || '0', 10) || 0);
      if (editing) {
        serialBox.hidden = !(p.isSerialized && serials.length);
        serialBox.innerHTML = `<p class="label-small serial-box__title">IMEI / Series registrados:</p>
          <div class="body-small serial-box__grid">${serials.map((s, i) => `<div><span class="serial-num">#${i + 1}:</span> ${esc(s)}</div>`).join('')}</div>`;
        return;
      }
      serialBox.hidden = !(serialized && stock > 0);
      if (serialBox.hidden) return;
      serials = serials.slice(0, stock);
      while (serials.length < stock) serials.push('');
      serialBox.innerHTML = `
        <p class="label-small serial-box__title">Ingresa los ${stock} IMEI / Series para el stock actual:</p>
        <div class="serial-box__grid serial-box__grid--inputs">
          ${serials
            .map(
              (s, i) => `<div class="serial-box__unit">
                <label class="label-small detail-label">Unidad ${i + 1}</label>
                <input type="text" data-serial="${i}" value="${esc(s)}" placeholder="IMEI o Serie" class="body-small serial-box__input" />
              </div>`
            )
            .join('')}
        </div>`;
    }
    serialBox.addEventListener('input', (e) => {
      const i = e.target.dataset.serial;
      if (i !== undefined) serials[Number(i)] = e.target.value;
    });
    form.querySelector('[name=isSerialized]').addEventListener('change', paintSerials);
    form.querySelector('[name=inStock]').addEventListener('change', paintSerials);
    form.addEventListener('input', (e) => !e.target.dataset.serial && sync());
    paintSerials();
    sync();

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        const f = ui.formValues(form);
        const numOr0 = (v) => (v === '' || isNaN(Number(v)) ? 0 : Number(v));
        const payload = {
          name: f.name.trim(),
          categoryId: cur.categoryId,
          brandId: cur.brandId,
          modelId: cur.modelId,
          modelIds: [cur.modelId, ...cur.compatible].filter(Boolean),
          supplierId: cur.supplierId,
          condition: cur.condition,
          inStock: Math.trunc(numOr0(f.inStock)),
          stockWarning: Math.trunc(numOr0(f.stockWarning)),
          minimumReorderPoint: Math.trunc(numOr0(f.minimumReorderPoint)),
          price: Math.round(numOr0(f.price) * 100) / 100,
          generalSalePrice: Math.round(numOr0(f.generalSalePrice) * 100) / 100,
          distributorSalePrice: Math.round(numOr0(f.distributorSalePrice) * 100) / 100,
          isSerialized: editing ? p.isSerialized : f.isSerialized,
          serialNumbers: editing ? [] : serials.map((s) => s.trim()).filter(Boolean),
          active: true,
        };
        try {
          if (editing) await Hook.put(`${endpoint}/${p.id}`, payload);
          else await Hook.post(endpoint, payload);
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, editing ? 'Error al editar producto' : 'Error al crear producto');
        }
      })
    );
  }

  // ── Filtros ─────────────────────────────────────────────────────────
  function openFilters() {
    const temp = Object.assign({}, filters);
    const m = ui.modal(`
      <div class="modal-card modal-card--filters modal-card--visible">
        <div class="modal-top">
          <h2 class="heading-5">Filtros de inventario</h2>
          <button type="button" data-close class="modal-close" aria-label="Cerrar">${ui.icon('xmark')}</button>
        </div>
        <div class="modal-scroll modal-scroll--visible">
          <div class="form-grid form-grid--2-always" data-form>
            ${ui.field({ label: 'Nombre', name: 'name', value: temp.name, placeholder: 'Introducir nombre' })}
            ${ui.ddField({ label: 'Categoría', id: 'fCat' })}
            ${ui.ddField({ label: 'Marca', id: 'fBrand' })}
            ${ui.field({ label: 'Modelo', name: 'model', value: temp.model, placeholder: 'Buscar modelo' })}
            ${ui.ddField({ label: 'Distribuidor', id: 'fSup' })}
          </div>
        </div>
        <div class="modal-footer modal-footer--soft">
          <button type="button" data-clear class="btn btn--muted">Limpiar</button>
          <button type="button" data-apply class="btn btn--primary">Aplicar filtros</button>
        </div>
      </div>`);
    const dCat = ui.dropdown(m.el.querySelector('#fCat'), { items: items(categories), value: temp.category || null, none: 'Todas', onChange: (v) => (temp.category = v || '') });
    const dBrand = ui.dropdown(m.el.querySelector('#fBrand'), { items: items(brands), value: temp.brand || null, none: 'Todas', placeholder: 'Seleccionar marca', onChange: (v) => (temp.brand = v || '') });
    const dSup = ui.dropdown(m.el.querySelector('#fSup'), { items: items(suppliers), value: temp.supplier || null, none: 'Todos', placeholder: 'Seleccionar distribuidor', up: true, onChange: (v) => (temp.supplier = v || '') });
    const form = m.el.querySelector('[data-form]');
    m.el.querySelector('[data-clear]').addEventListener('click', () => {
      Object.assign(temp, { name: '', category: '', brand: '', model: '', supplier: '' });
      form.querySelector('[name=name]').value = '';
      form.querySelector('[name=model]').value = '';
      [dCat, dBrand, dSup].forEach((d) => d.setValue(null));
    });
    m.el.querySelector('[data-apply]').addEventListener('click', () => {
      const f = ui.formValues(form);
      filters = Object.assign(temp, { name: f.name.trim(), model: f.model.trim() });
      state.page = 1;
      m.close();
      render();
    });
  }

  // ── Detalles ────────────────────────────────────────────────────────
  function openDetails(p) {
    const item = (label, value) =>
      `<div><span class="label-small detail-label">${esc(label)}</span><span class="body-normal detail-value">${esc(value)}</span></div>`;
    ui.modal(`
      <div class="modal-card modal-card--lg modal-card--tall modal-card--clip">
        <div class="modal-top modal-top--bordered">
          <div class="modal-top__title"><h2 class="heading-5">Detalles del producto</h2>${statusPill(p.status)}</div>
          <button type="button" data-close class="modal-close" aria-label="Cerrar">${ui.icon('xmark')}</button>
        </div>
        <div class="modal-scroll modal-scroll--stack">
          <div>
            <h3 class="label-normal section-label section-label--spaced">Información Principal</h3>
            <div class="detail-grid">
              ${item('ID', p.productNumber)}${item('UPC', p.upc || 'N/A')}${item('Nombre', p.name)}
              ${item('Categoría', p.categoryName || 'N/A')}${item('Marca', p.brandName || 'N/A')}${item('Modelos', (p.modelNames || []).join(', ') || 'N/A')}
              ${item('Proveedor', p.supplierName || 'N/A')}${item('Condición', p.condition || 'N/A')}${item('Dispositivo Asociado', p.modelName || 'N/A')}
            </div>
          </div>
          <div>
            <h3 class="label-normal section-label section-label--spaced">Inventario y Precios</h3>
            <div class="detail-grid">
              ${item('Stock Actual', p.inStock)}${item('Warning Stock', p.stockWarning)}${item('Punto Reorden', p.minimumReorderPoint)}
              ${item('Costo', ui.money(p.price))}${item('Precio General', ui.money(p.generalSalePrice))}${item('Precio Distribuidor', ui.money(p.distributorSalePrice))}
            </div>
          </div>
          ${
            p.isSerialized
              ? `<div>
                   <h3 class="label-normal section-label section-label--spaced">Números de Serie / IMEI</h3>
                   ${
                     (p.serialNumbers || []).length
                       ? `<div class="serial-box serial-box--detail"><div class="serial-box__grid">
                            ${p.serialNumbers.map((s, i) => `<div class="body-normal"><span class="label-small serial-num">#${i + 1}:</span> ${esc(s || 'Sin registrar')}</div>`).join('')}
                          </div></div>`
                       : '<p class="body-normal text-muted text-italic">No hay números de serie registrados para este producto.</p>'
                   }
                 </div>`
              : ''
          }
        </div>
        <div class="modal-footer modal-footer--soft">
          <button type="button" data-close class="btn btn--muted">Cerrar</button>
        </div>
      </div>`);
  }

  function openDelete(p) {
    ui.confirmDelete({
      title: 'Eliminar producto',
      message: '¿Está seguro de que desea eliminar el producto',
      name: p.name,
      onConfirm: async () => {
        await Hook.del(`${endpoint}/${p.id}`);
        state.rows = state.rows.filter((r) => r.id !== p.id);
        state.refresh();
        ui.toast('Cambios guardados');
      },
    });
  }

  function exportCSV() {
    ui.downloadCSV('productos.csv', [
      { label: 'ID', value: (p) => p.productNumber },
      { label: 'UPC', value: (p) => p.upc },
      { label: 'Nombre', value: (p) => p.name },
      { label: 'Categoría', value: (p) => p.categoryName },
      { label: 'Marca', value: (p) => p.brandName },
      { label: 'Modelos', value: (p) => (p.modelNames || []).join(' / ') },
      { label: 'Proveedor', value: (p) => p.supplierName },
      { label: 'Condición', value: (p) => p.condition },
      { label: 'Estado', value: (p) => p.status },
      { label: 'En stock', value: (p) => p.inStock },
      { label: 'Costo', value: (p) => p.price },
      { label: 'Precio general', value: (p) => p.generalSalePrice },
      { label: 'Precio distribuidor', value: (p) => p.distributorSalePrice },
    ], state.processed);
  }

  const addBtn = document.getElementById('addBtn');
  if (addBtn) addBtn.addEventListener('click', () => openModal(null));
  document.getElementById('filterBtn').addEventListener('click', openFilters);
  document.getElementById('exportBtn').addEventListener('click', exportCSV);
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();
