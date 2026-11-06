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
      <div class="py-4 px-6 flex justify-between items-center transition-colors hover:bg-gray-50 ${last ? '' : 'border-b border-gray-200'}">
        <div class="flex items-start"><h4 class="label-normal font-bold text-dark-1 mb-1">${esc(b.name)}</h4></div>
        ${ui.cardButtons('hover:text-gray-600')}
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
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-4xl flex flex-col max-h-[90vh] overflow-hidden border border-gray-100 animate-fade-in">
        <div class="p-8 pb-4">
          ${ui.modalHeader(editing ? 'Editar marca' : 'Agregar nueva marca')}
          <div class="flex flex-col mb-6">
            <label class="label-normal font-semibold text-dark-1 mb-2">Nombre de la marca <span class="text-brand-primary">*</span></label>
            <input type="text" name="name" value="${esc(editing ? brand.name : '')}" placeholder="Introduzca el nombre de la marca" maxlength="150"
                   class="w-full md:w-80 border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal placeholder-gray-400" />
          </div>
        </div>
        <div class="p-6 md:px-8 border-t border-gray-100 bg-white flex justify-end gap-4 mt-auto">
          <button type="button" data-close class="px-6 py-2.5 rounded-lg bg-gray-100 text-dark-2 font-bold label-normal hover:bg-gray-200 transition-colors">Cancelar</button>
          <button type="button" data-save disabled class="px-6 py-2.5 rounded-lg bg-brand-primary text-white font-bold label-normal transition-all shadow-sm hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed">${editing ? 'Guardar cambios' : 'Crear marca'}</button>
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