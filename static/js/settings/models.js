/* Configuración → Inventario → Modelos (dispositivos) (/api/:org/models). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('models');
  let brands = [];

  const state = ui.listState({
    perPage: 5,
    search: (r, t) => ui.norm(r.name).includes(t) || ui.norm(r.brandName).includes(t),
    render,
  });

  const list = ui.cardList(document.getElementById('list'), {
    emptyText: 'No se encontraron modelos.',
    rowHtml: (m, last) => `
      <div class="card-row">
        <div class="card-row__text">
          <h4 class="label-normal card-row__title">${esc(m.name)}</h4>
          <p class="body-normal card-row__sub">${esc(m.brandName || 'Sin Marca')}</p>
        </div>
        ${ui.cardButtons()}
      </div>`,
    onAction: (action, m) => (action === 'edit' ? openModal(m) : openDelete(m)),
  });

  function render() {
    list.render(state.pageRows);
    ui.pagination(document.getElementById('pager'), { page: state.page, total: state.totalPages, onChange: (p) => state.goTo(p), layout: 'center' });
  }

  async function load() {
    try {
      const [models, brandRows] = await Promise.all([Hook.get(endpoint), Hook.get(Hook.org('brands'))]);
      brands = brandRows.filter((b) => b.active);
      state.set(models.filter((m) => m.active));
    } catch (err) {
      ui.error(err, 'No se pudieron cargar los modelos.');
    }
  }

  function openModal(model) {
    const editing = !!model;
    let brandId = editing ? model.brandId : null;
    const m = ui.modal(`
      <div class="modal-card modal-card--sm catalog-modal--visible">
        <div class="modal-body">
          ${ui.modalHeader(editing ? 'Editar dispositivo' : 'Agregar dispositivo')}
          <div class="form-stack">
            ${ui.field({ label: 'Nombre', name: 'name', value: editing ? model.name : '', placeholder: 'Ej. iPhone 16', required: true })}
            ${ui.ddField({ label: 'Marca', id: 'ddBrand', required: true })}
          </div>
        </div>
        <div class="modal-footer modal-footer--soft">
          <button type="button" data-close class="btn btn--muted">Cancelar</button>
          <button type="button" data-save disabled class="btn btn--primary">Guardar cambios</button>
        </div>
      </div>`);
    const name = m.el.querySelector('[name=name]');
    const save = m.el.querySelector('[data-save]');
    const sync = () => (save.disabled = !name.value.trim() || !brandId);
    ui.dropdown(m.el.querySelector('#ddBrand'), {
      items: brands.map((b) => ({ value: b.id, label: b.name })),
      value: brandId,
      placeholder: 'Seleccionar marca',
      up: true,
      onChange: (v) => {
        brandId = v;
        sync();
      },
    });
    name.addEventListener('input', sync);
    sync();

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        const payload = { name: name.value.trim(), brandId };
        try {
          if (editing) await Hook.put(`${endpoint}/${model.id}`, payload);
          else await Hook.post(endpoint, payload);
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, editing ? 'Error al actualizar modelo' : 'Error al crear modelo');
        }
      })
    );
  }

  function openDelete(model) {
    ui.confirmDelete({
      title: 'Eliminar modelo',
      message: '¿Está seguro de que desea eliminar el modelo',
      name: model.name,
      onConfirm: async () => {
        await Hook.del(`${endpoint}/${model.id}`);
        state.rows = state.rows.filter((r) => r.id !== model.id);
        state.refresh();
        ui.toast('Cambios guardados');
      },
    });
  }

  document.getElementById('addBtn').addEventListener('click', () => openModal(null));
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();
