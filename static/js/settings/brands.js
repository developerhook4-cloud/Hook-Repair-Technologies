/* Configuración → Inventario → Marcas (/api/:org/brands). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('brands');

  const state = ui.listState({
    perPage: 5,
    search: (r, t) => ui.norm(r.name).includes(t),
    render,
  });

  const list = ui.cardList(document.getElementById('list'), {
    emptyText: 'No se encontraron marcas.',
    rowHtml: (b, last) => `
      <div class="card-row">
        <h4 class="label-normal card-row__title">${esc(b.name)}</h4>
        ${ui.cardButtons()}
      </div>`,
    onAction: (action, b) => (action === 'edit' ? openModal(b) : openDelete(b)),
  });

  function render() {
    list.render(state.pageRows);
    ui.pagination(document.getElementById('pager'), { page: state.page, total: state.totalPages, onChange: (p) => state.goTo(p) });
  }

  async function load() {
    try {
      const rows = await Hook.get(endpoint);
      state.set(rows.filter((b) => b.active));
    } catch (err) {
      ui.error(err, 'No se pudieron cargar las marcas.');
    }
  }

  function openModal(brand) {
    const editing = !!brand;
    const m = ui.modal(`
      <div class="modal-card modal-card--2xl catalog-modal">
        <div class="catalog-modal__body">
          ${ui.modalHeader(editing ? 'Editar marca' : 'Agregar nueva marca')}
          <div class="catalog-modal__field">
            <label class="label-normal field__label">Nombre de la marca <span class="req">*</span></label>
            <input type="text" name="name" value="${esc(editing ? brand.name : '')}" placeholder="Introduzca el nombre de la marca" maxlength="150"
                   class="input catalog-modal__input" />
          </div>
        </div>
        <div class="modal-footer">
          <button type="button" data-close class="btn btn--soft">Cancelar</button>
          <button type="button" data-save disabled class="btn btn--primary">${editing ? 'Guardar cambios' : 'Crear marca'}</button>
        </div>
      </div>`);
    const name = m.el.querySelector('[name=name]');
    const save = m.el.querySelector('[data-save]');
    const sync = () => (save.disabled = !name.value.trim());
    name.addEventListener('input', sync);
    name.addEventListener('keydown', (e) => e.key === 'Enter' && !save.disabled && save.click());
    sync();

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        try {
          if (editing) await Hook.put(`${endpoint}/${brand.id}`, { name: name.value.trim() });
          else await Hook.post(endpoint, { name: name.value.trim() });
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, editing ? 'Error al actualizar marca' : 'Error al crear marca');
        }
      })
    );
  }

  function openDelete(brand) {
    ui.confirmDelete({
      title: 'Eliminar marca',
      message: '¿Está seguro de que desea eliminar la marca',
      name: brand.name,
      onConfirm: async () => {
        await Hook.del(`${endpoint}/${brand.id}`);
        state.rows = state.rows.filter((r) => r.id !== brand.id);
        state.refresh();
        ui.toast('Cambios guardados');
      },
    });
  }

  document.getElementById('addBtn').addEventListener('click', () => openModal(null));
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();
