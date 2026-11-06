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
      { key: 'publicPrice', label: 'Precio Público', width: 140, sort: 'publicPrice', cellClass: 'data-table__td--price', render: (s) => ui.money(s.publicPrice), text: (s) => ui.money(s.publicPrice) },
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
      <div class="modal-card modal-card--lg modal-card--tall modal-card--clip">
        <div class="modal-top">
          <h2 class="heading-5">${editing ? 'Editar servicio' : 'Crear servicio'}</h2>
          <button type="button" data-close class="modal-close" aria-label="Cerrar">${ui.icon('xmark')}</button>
        </div>
        <div class="modal-scroll">
          <div class="form-grid form-grid--roomy" data-form>
            ${ui.field({ label: 'Nombre', name: 'name', value: editing ? service.name : '', placeholder: 'Ej. Cambio de Pantalla', required: true, span: true })}
            <div class="field field--span">
              <label class="label-normal field__label">Descripción (Opcional)</label>
              <textarea name="description" rows="3" placeholder="Detalles del servicio..." class="textarea">${esc(editing ? service.description : '')}</textarea>
            </div>
            ${ui.ddField({ label: 'Categoría (Opcional)', id: 'ddCategory' })}
            ${ui.ddField({ label: 'Marca (Opcional)', id: 'ddBrand' })}
            ${ui.ddField({ label: 'Modelo (Opcional)', id: 'ddModel', span: true })}
            <div class="form-divider"></div>
            ${ui.field({ label: 'Precio', name: 'price', type: 'number', value: editing ? service.retailPrice : '', placeholder: '0.00', required: true, prefix: '$' })}
            ${ui.field({ label: 'Precio Público', name: 'publicPrice', type: 'number', value: editing ? service.publicPrice : '', placeholder: '0.00', required: true, prefix: '$' })}
            ${ui.field({ label: 'Precio Distribuidor', name: 'distributorPrice', type: 'number', value: editing ? service.distributorPrice : '', placeholder: '0.00', required: true, prefix: '$' })}
          </div>
        </div>
        <div class="modal-footer modal-footer--soft">
          <button type="button" data-close class="btn btn--muted">Cancelar</button>
          <button type="button" data-save disabled class="btn btn--primary">Guardar cambios</button>
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
