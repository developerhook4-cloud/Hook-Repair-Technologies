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
    const cls =
      s === 'agotado' ? 'bg-error-subtle text-error-primary' : s === 'bajo stock' ? 'bg-warning-subtle text-warning-darker' : 'bg-success-subtle text-success-darker';
    return `<span class="px-4 py-1.5 rounded-full label-small inline-block font-bold whitespace-nowrap ${cls}">${esc(status)}</span>`;
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
      <div class="flex flex-col gap-1.5">
        <label class="label-normal font-semibold text-dark-1">${esc(label)}</label>
        <input type="number" name="${name}" value="${esc(value)}" step="any" min="0" ${extra}
               class="w-full border border-gray-300 rounded-[14px] px-4 py-2 focus:outline-none focus:border-brand-primary body-normal text-dark-1 disabled:bg-gray-50 disabled:text-gray-400" />
      </div>`;
    const ddf = (label, id, required, quick) => `
      <div class="flex flex-col gap-1.5">
        <div class="flex items-center gap-2">
          <label class="label-normal font-semibold text-dark-1">${esc(label)}${required ? ' <span class="text-brand-primary">*</span>' : ''}</label>
          ${quick ? ui.quickAddBtn(quick[0], quick[1]) : ''}
        </div>
        <div id="${id}"></div>
      </div>`;
    const canCatalogs = Hook.can('Catalogs.Update');
    const canSuppliers = Hook.can('Suppliers.Update');

    const m = ui.modal(`
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-5xl flex flex-col max-h-[90vh] border border-gray-100 overflow-hidden animate-fade-in">
        <div class="p-6 md:px-8 md:pt-8 pb-4 flex justify-between items-start shrink-0 border-b border-gray-100">
          <h2 class="heading-5 text-dark-1">${editing ? 'Editar producto' : 'Crear producto'}</h2>
          <button type="button" data-close class="text-gray-400 hover:text-dark-1 transition-colors">${ui.icon('xmark', 'text-xl')}</button>
        </div>
        <div class="p-6 md:p-8 overflow-y-auto flex-1 bg-white" data-form>
          <div class="flex flex-col md:flex-row gap-10">
            <div class="flex-1 flex flex-col gap-5">
              <h3 class="label-normal font-bold text-dark-1">Información General</h3>
              <div class="flex flex-col gap-1.5">
                <label class="label-normal font-semibold text-dark-1">Nombre del producto <span class="text-brand-primary">*</span></label>
                <input type="text" name="name" maxlength="250" value="${esc(p.name || '')}" placeholder="Ej. Pantalla OLED iPhone 13"
                       class="w-full border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal text-dark-1" />
              </div>
              <div class="grid grid-cols-2 gap-4">${ddf('Categoría', 'ddCat', true, canCatalogs && ['category', 'Nueva'])}${ddf('Marca', 'ddBrand', true, canCatalogs && ['brand', 'Nueva'])}</div>
              <div class="grid grid-cols-2 gap-4">${ddf('Dispositivo asociado', 'ddModel', false, canCatalogs && ['model', 'Nuevo'])}${ddf('Condición', 'ddCond')}</div>
              <div class="flex flex-col gap-1.5">
                <label class="label-normal font-semibold text-dark-1">Modelos compatibles</label>
                <div data-compatible class="flex flex-wrap gap-2 min-h-11 border border-gray-300 rounded-[14px] px-3 py-2"></div>
              </div>
              <div class="grid grid-cols-2 gap-4">
                ${ddf('Proveedor', 'ddSupplier', false, canSuppliers && ['supplier', 'Nuevo'])}
                <div class="flex flex-col gap-1.5">
                  <label class="label-normal font-semibold text-dark-1">UPC</label>
                  <input type="text" value="${esc(p.upc || '')}" placeholder="Se genera al guardar" readonly
                         class="w-full border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none body-normal text-dark-1 bg-gray-50" />
                </div>
              </div>
            </div>

            <div class="flex-1 flex flex-col gap-5 md:border-l border-gray-100 md:pl-10">
              <h3 class="label-normal font-bold text-dark-1">Precios e Inventario</h3>
              <div class="grid grid-cols-3 gap-3">
                ${num('Costo ($)', 'price', editing ? p.price : 0)}
                ${num('P. General ($)', 'generalSalePrice', editing ? p.generalSalePrice : 0)}
                ${num('P. Distrib. ($)', 'distributorSalePrice', editing ? p.distributorSalePrice : 0)}
              </div>
              <div class="grid grid-cols-3 gap-3">
                ${num(editing ? 'Stock actual' : 'Stock Inicial', 'inStock', editing ? p.inStock : 0, editing ? 'disabled title="El stock cambia con los movimientos de inventario"' : '')}
                ${num('Warning Stock', 'stockWarning', editing ? p.stockWarning : 0)}
                ${num('P. Reorden', 'minimumReorderPoint', editing ? p.minimumReorderPoint : 0)}
              </div>
              <div class="flex flex-col gap-3 mt-2">
                <label class="flex items-center gap-2 ${editing ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'} label-normal text-dark-1">
                  <input type="checkbox" name="isSerialized" ${p.isSerialized ? 'checked' : ''} ${editing ? 'disabled' : ''}
                         class="w-4 h-4 rounded border-gray-300 text-brand-primary focus:ring-brand-primary accent-brand-primary cursor-pointer" />
                  Inventario Serializado (Requerir IMEI/Serie)
                </label>
                <div data-serials class="bg-gray-50 p-4 rounded-2xl border border-gray-200 mt-2 max-h-40 overflow-y-auto scroll-smooth" hidden></div>
              </div>
              ${editing ? '<p class="label-small text-gray-400">El stock se ajusta desde Inventario → Movimientos.</p>' : ''}
            </div>
          </div>
        </div>
        <div class="p-6 md:px-8 bg-gray-50 flex justify-end gap-3 border-t border-gray-100 shrink-0">
          <button type="button" data-close class="px-5 py-2.5 rounded-lg bg-gray-200 text-dark-2 label-normal font-bold hover:bg-gray-300 transition-colors">Cancelar</button>
          <button type="button" data-save disabled class="px-5 py-2.5 rounded-lg bg-brand-primary text-white label-normal font-bold hover:opacity-90 transition-opacity shadow-sm disabled:opacity-50 disabled:cursor-not-allowed">${editing ? 'Guardar cambios' : 'Crear producto'}</button>
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
              return `<button type="button" data-model="${x.id}" class="px-3 py-1 rounded-full label-small border transition-colors ${
                on ? 'bg-brand-primary text-white border-brand-primary' : 'bg-white text-dark-2 border-gray-200 hover:border-brand-primary'
              }">${esc(x.name)}</button>`;
            })
            .join('')
        : `<span class="body-normal text-gray-400 py-1">${cur.brandId ? 'Esta marca no tiene más modelos.' : 'Selecciona una marca para ver sus modelos.'}</span>`;
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
        serialBox.innerHTML = `<p class="label-small text-dark-2 mb-2">IMEI / Series registrados:</p>
          <div class="grid grid-cols-2 gap-2 body-small text-dark-1">${serials.map((s, i) => `<div><span class="text-gray-500">#${i + 1}:</span> ${esc(s)}</div>`).join('')}</div>`;
        return;
      }
      serialBox.hidden = !(serialized && stock > 0);
      if (serialBox.hidden) return;
      serials = serials.slice(0, stock);
      while (serials.length < stock) serials.push('');
      serialBox.innerHTML = `
        <p class="label-small text-dark-2 mb-3">Ingresa los ${stock} IMEI / Series para el stock actual:</p>
        <div class="grid grid-cols-2 gap-3">
          ${serials
            .map(
              (s, i) => `<div class="flex flex-col gap-1">
                <label class="label-small text-gray-500">Unidad ${i + 1}</label>
                <input type="text" data-serial="${i}" value="${esc(s)}" placeholder="IMEI o Serie" class="w-full border border-gray-300 rounded-[10px] px-3 py-1.5 focus:outline-none focus:border-brand-primary body-small text-dark-1 bg-white" />
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
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-162.5 flex flex-col border border-gray-100 overflow-visible animate-fade-in">
        <div class="p-6 md:p-8 pb-4 flex justify-between items-start">
          <h2 class="heading-5 text-dark-1">Filtros de inventario</h2>
          <button type="button" data-close class="text-gray-400 hover:text-dark-1 transition-colors">${ui.icon('xmark', 'text-xl')}</button>
        </div>
        <div class="px-6 md:px-8 pb-6">
          <div class="grid grid-cols-2 gap-4" data-form>
            ${ui.field({ label: 'Nombre', name: 'name', value: temp.name, placeholder: 'Introducir nombre' })}
            ${ui.ddField({ label: 'Categoría', id: 'fCat' })}
            ${ui.ddField({ label: 'Marca', id: 'fBrand' })}
            ${ui.field({ label: 'Modelo', name: 'model', value: temp.model, placeholder: 'Buscar modelo' })}
            ${ui.ddField({ label: 'Distribuidor', id: 'fSup' })}
          </div>
        </div>
        <div class="p-6 md:px-8 bg-gray-50 flex justify-end gap-3 border-t border-gray-100 rounded-b-2xl">
          <button type="button" data-clear class="px-5 py-2.5 rounded-lg bg-gray-200 text-dark-2 label-normal font-bold hover:bg-gray-300 transition-colors">Limpiar</button>
          <button type="button" data-apply class="px-5 py-2.5 rounded-lg bg-brand-primary text-white label-normal font-bold hover:opacity-90 transition-opacity shadow-sm">Aplicar filtros</button>
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
      `<div><span class="label-small text-gray-500 block">${esc(label)}</span><span class="body-normal font-semibold text-dark-1 break-words">${esc(value)}</span></div>`;
    ui.modal(`
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-2xl flex flex-col max-h-[85vh] overflow-hidden border border-gray-100 animate-fade-in">
        <div class="p-6 md:px-8 md:pt-8 pb-4 flex justify-between items-start shrink-0 border-b border-gray-100">
          <div class="flex items-center gap-3"><h2 class="heading-5 text-dark-1">Detalles del producto</h2>${statusPill(p.status)}</div>
          <button type="button" data-close class="text-gray-400 hover:text-dark-1 transition-colors">${ui.icon('xmark', 'text-xl')}</button>
        </div>
        <div class="p-6 md:px-8 overflow-y-auto flex-1 flex flex-col gap-6">
          <div>
            <h3 class="label-normal font-bold text-dark-1 mb-3">Información Principal</h3>
            <div class="grid grid-cols-2 md:grid-cols-3 gap-4 bg-gray-50 p-4 rounded-2xl border border-gray-100">
              ${item('ID', p.productNumber)}${item('UPC', p.upc || 'N/A')}${item('Nombre', p.name)}
              ${item('Categoría', p.categoryName || 'N/A')}${item('Marca', p.brandName || 'N/A')}${item('Modelos', (p.modelNames || []).join(', ') || 'N/A')}
              ${item('Proveedor', p.supplierName || 'N/A')}${item('Condición', p.condition || 'N/A')}${item('Dispositivo Asociado', p.modelName || 'N/A')}
            </div>
          </div>
          <div>
            <h3 class="label-normal font-bold text-dark-1 mb-3">Inventario y Precios</h3>
            <div class="grid grid-cols-2 md:grid-cols-3 gap-4 bg-gray-50 p-4 rounded-2xl border border-gray-100">
              ${item('Stock Actual', p.inStock)}${item('Warning Stock', p.stockWarning)}${item('Punto Reorden', p.minimumReorderPoint)}
              ${item('Costo', ui.money(p.price))}${item('Precio General', ui.money(p.generalSalePrice))}${item('Precio Distribuidor', ui.money(p.distributorSalePrice))}
            </div>
          </div>
          ${
            p.isSerialized
              ? `<div>
                   <h3 class="label-normal font-bold text-dark-1 mb-3">Números de Serie / IMEI</h3>
                   ${
                     (p.serialNumbers || []).length
                       ? `<div class="bg-gray-50 p-4 rounded-2xl border border-gray-100 max-h-37.5 overflow-y-auto scroll-smooth"><div class="grid grid-cols-2 gap-2">
                            ${p.serialNumbers.map((s, i) => `<div class="body-normal text-dark-1"><span class="label-small text-gray-500">#${i + 1}:</span> ${esc(s || 'Sin registrar')}</div>`).join('')}
                          </div></div>`
                       : '<p class="body-normal text-gray-400 italic">No hay números de serie registrados para este producto.</p>'
                   }
                 </div>`
              : ''
          }
        </div>
        <div class="p-6 bg-gray-50 flex justify-end border-t border-gray-100 shrink-0">
          <button type="button" data-close class="px-6 py-2.5 rounded-lg bg-gray-200 text-dark-2 label-normal font-bold hover:bg-gray-300 transition-colors">Cerrar</button>
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