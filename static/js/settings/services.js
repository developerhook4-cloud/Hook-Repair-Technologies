/* Configuración → Reparaciones → Servicios (/api/:org/services). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('services');
  let categories = [];
  let brands = [];
  let models = [];

  const deviceName = (s) => `${s.brandName || ''} ${s.modelName || ''}`.trim() || 'General';

  const state = ui.listState({
    perPage: 5,
    search: (s, t) => ui.norm(s.name).includes(t) || ui.norm(s.categoryName).includes(t) || ui.norm(deviceName(s)).includes(t),
    sortValue: (s, k) => (k === 'deviceName' ? deviceName(s) : k === 'price' ? s.retailPrice : s[k]),
    render,
  });

  const table = ui.table(document.getElementById('table'), {
    state,
    minWidth: 80,
    emptyText: 'No se encontraron servicios.',
    columns: [
      { key: 'name', label: 'Nombre', width: 200, sort: 'name', title: (s) => s.name },
      { key: 'categoryName', label: 'Categoría', width: 150, sort: 'categoryName', render: (s) => esc(s.categoryName || '-') },
      { key: 'deviceName', label: 'Dispositivo', width: 180, sort: 'deviceName', render: (s) => esc(deviceName(s)), text: deviceName },
      { key: 'retailPrice', label: 'Precio', width: 120, sort: 'price', render: (s) => ui.money(s.retailPrice), text: (s) => ui.money(s.retailPrice) },
      { key: 'publicPrice', label: 'Precio Público', width: 140, sort: 'publicPrice', cellClass: 'font-bold text-dark-2', render: (s) => ui.money(s.publicPrice), text: (s) => ui.money(s.publicPrice) },
      { key: 'distributorPrice', label: 'Precio Distribuidor', width: 160, sort: 'distributorPrice', render: (s) => ui.money(s.distributorPrice), text: (s) => ui.money(s.distributorPrice) },
    ],
    actions: () => ui.actionBtn('edit', 'pen', 'Editar') + ui.actionBtn('delete', 'trash-can', 'Eliminar'),
    onAction: (action, s) => (action === 'edit' ? openModal(s) : action === 'delete' ? openDelete(s) : null),
  });

  function render() {
    table.render(state.pageRows);
    ui.pagination(document.getElementById('pager'), { page: state.page, total: state.totalPages, onChange: (p) => state.goTo(p), layout: 'center' });
  }

  async function load() {
    try {
      const [services, cats, brs, mds] = await Promise.all([
        Hook.get(endpoint),
        Hook.get(Hook.org('categories')),
        Hook.get(Hook.org('brands')),
        Hook.get(Hook.org('models')),
      ]);
      categories = cats.filter((c) => c.active);
      brands = brs.filter((b) => b.active);
      models = mds.filter((m) => m.active);
      state.set(services.filter((s) => s.active));
    } catch (err) {
      ui.error(err, 'No se pudieron cargar los servicios.');
    }
  }

  function openModal(service) {
    const editing = !!service;
    const cur = {
      categoryId: editing ? service.categoryId : null,
      brandId: editing ? service.brandId : null,
      modelId: editing ? service.modelId : null,
    };
    const m = ui.modal(`
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-2xl flex flex-col max-h-[85vh] overflow-hidden border border-gray-100 animate-fade-in">
        <div class="p-6 md:px-8 md:pt-8 pb-4 flex justify-between items-start shrink-0">
          <h2 class="heading-5 text-dark-1">${editing ? 'Editar servicio' : 'Crear servicio'}</h2>
          <button type="button" data-close class="text-gray-400 hover:text-dark-1 transition-colors">${ui.icon('xmark', 'text-xl')}</button>
        </div>
        <div class="px-6 md:px-8 pb-6 overflow-y-auto flex-1">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-5" data-form>
            ${ui.field({ label: 'Nombre', name: 'name', value: editing ? service.name : '', placeholder: 'Ej. Cambio de Pantalla', required: true, span: 'md:col-span-2' })}
            <div class="flex flex-col gap-1.5 md:col-span-2">
              <label class="label-normal text-dark-1">Descripción (Opcional)</label>
              <textarea name="description" rows="3" placeholder="Detalles del servicio..." class="w-full border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal text-dark-1 resize-none">${esc(editing ? service.description : '')}</textarea>
            </div>
            ${ui.ddField({ label: 'Categoría (Opcional)', id: 'ddCategory' })}
            ${ui.ddField({ label: 'Marca (Opcional)', id: 'ddBrand' })}
            ${ui.ddField({ label: 'Modelo (Opcional)', id: 'ddModel', span: 'md:col-span-2' })}
            <div class="w-full h-px bg-gray-100 md:col-span-2 my-2"></div>
            ${ui.field({ label: 'Precio', name: 'price', type: 'number', value: editing ? service.retailPrice : '', placeholder: '0.00', required: true, prefix: '$' })}
            ${ui.field({ label: 'Precio Público', name: 'publicPrice', type: 'number', value: editing ? service.publicPrice : '', placeholder: '0.00', required: true, prefix: '$' })}
            ${ui.field({ label: 'Precio Distribuidor', name: 'distributorPrice', type: 'number', value: editing ? service.distributorPrice : '', placeholder: '0.00', required: true, prefix: '$' })}
          </div>
        </div>
        <div class="p-6 md:px-8 bg-gray-50 flex justify-end gap-3 border-t border-gray-100 shrink-0">
          <button type="button" data-close class="px-5 py-2.5 rounded-lg bg-gray-200 text-dark-2 label-normal font-bold hover:bg-gray-300 transition-colors">Cancelar</button>
          <button type="button" data-save disabled class="px-5 py-2.5 rounded-lg bg-brand-primary text-white label-normal font-bold hover:opacity-90 transition-opacity shadow-sm disabled:opacity-50 disabled:cursor-not-allowed">Guardar cambios</button>
        </div>
      </div>`);

    const form = m.el.querySelector('[data-form]');
    const save = m.el.querySelector('[data-save]');
    const modelItems = () => models.filter((x) => !cur.brandId || x.brandId === cur.brandId).map((x) => ({ value: x.id, label: x.name }));

    ui.dropdown(m.el.querySelector('#ddCategory'), {
      items: categories.map((c) => ({ value: c.id, label: c.name })),
      value: cur.categoryId,
      placeholder: 'Seleccionar...',
      none: 'Ninguna',
      onChange: (v) => (cur.categoryId = v),
    });
    const ddModel = ui.dropdown(m.el.querySelector('#ddModel'), {
      items: modelItems(),
      value: cur.modelId,
      placeholder: cur.brandId ? 'Seleccionar...' : 'Seleccione una marca primero',
      none: 'Ninguno',
      onChange: (v) => (cur.modelId = v),
    });
    ddModel.setDisabled(!cur.brandId);
    ui.dropdown(m.el.querySelector('#ddBrand'), {
      items: brands.map((b) => ({ value: b.id, label: b.name })),
      value: cur.brandId,
      placeholder: 'Seleccionar...',
      none: 'Ninguna',
      onChange: (v) => {
        cur.brandId = v;
        cur.modelId = null;
        ddModel.setItems(modelItems());
        ddModel.setValue(null);
        ddModel.setPlaceholder(v ? 'Seleccionar...' : 'Seleccione una marca primero');
        ddModel.setDisabled(!v);
      },
    });

    const valid = () => {
      const f = ui.formValues(form);
      return f.name.trim() !== '' && f.price !== '' && f.publicPrice !== '' && f.distributorPrice !== '';
    };
    form.addEventListener('input', () => (save.disabled = !valid()));
    save.disabled = !valid();

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        const f = ui.formValues(form);
        const payload = {
          name: f.name.trim(),
          description: f.description.trim(),
          categoryId: cur.categoryId || null,
          brandId: cur.brandId || null,
          modelId: cur.modelId || null,
          retailPrice: Number(f.price),
          publicPrice: Number(f.publicPrice),
          distributorPrice: Number(f.distributorPrice),
        };
        try {
          if (editing) await Hook.put(`${endpoint}/${service.id}`, payload);
          else await Hook.post(endpoint, payload);
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, editing ? 'Error al actualizar servicio' : 'Error al crear servicio');
        }
      })
    );
  }

  function openDelete(service) {
    ui.confirmDelete({
      title: 'Eliminar servicio',
      message: '¿Está seguro de que desea eliminar el servicio',
      name: service.name,
      onConfirm: async () => {
        await Hook.del(`${endpoint}/${service.id}`);
        state.rows = state.rows.filter((r) => r.id !== service.id);
        state.refresh();
        ui.toast('Cambios guardados');
      },
    });
  }

  document.getElementById('addBtn').addEventListener('click', () => openModal(null));
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();