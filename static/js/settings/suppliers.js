/* Configuración → Terceros → Proveedores (/api/:org/suppliers). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('suppliers');
  const emailRe = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;

  const state = ui.listState({
    perPage: 5,
    search: (s, t) => ui.norm(s.id).includes(t) || ui.norm(s.name).includes(t) || ui.norm(s.email).includes(t) || ui.norm(s.phone1).includes(t),
    render,
  });

  const table = ui.table(document.getElementById('table'), {
    state,
    emptyText: 'No se encontraron proveedores.',
    columns: [
      { key: 'id', label: 'ID', width: 80, sort: 'id', render: (s) => esc(s.id), title: (s) => s.id },
      { key: 'name', label: 'Nombre', width: 220, sort: 'name', title: (s) => s.name },
      { key: 'phone1', label: 'Teléfono', width: 160 },
      { key: 'phone2', label: 'Teléfono Alternativo', width: 180, render: (s) => esc(s.phone2 || '') },
      { key: 'email', label: 'Email', width: 240, sort: 'email', title: (s) => s.email },
      { key: 'active', label: 'Estado', width: 120, render: (s) => ui.statusPill(s.active), text: (s) => (s.active ? 'Activo' : 'Inactivo') },
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
      state.set(await Hook.get(endpoint));
    } catch (err) {
      ui.error(err, 'Error al cargar proveedores.');
    }
  }

  function openModal(supplier) {
    const editing = !!supplier;
    const s = supplier || {};
    let active = editing ? s.active : true;
    const m = ui.modal(`
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-md flex flex-col border border-gray-100 overflow-visible animate-fade-in">
        <div class="p-6 md:p-8 pb-4">
          ${ui.modalHeader(editing ? 'Editar proveedor' : 'Nuevo proveedor')}
          <div class="flex flex-col gap-5" data-form>
            ${ui.field({ label: 'Nombre', name: 'name', value: s.name, placeholder: 'Ej. Tech Source', required: true })}
            ${ui.field({ label: 'Teléfono', name: 'phone1', value: s.phone1, placeholder: '662 481 9204', required: true, phone: true })}
            ${ui.field({ label: 'Teléfono Alternativo', name: 'phone2', value: s.phone2, placeholder: '662 739 1058', phone: true })}
            ${ui.field({ label: 'Email', name: 'email', type: 'email', value: s.email, placeholder: 'ejemplo@proveedor.com', required: true })}
            ${editing ? ui.ddField({ label: 'Estado', id: 'ddStatus' }) : ''}
          </div>
        </div>
        <div class="p-6 md:px-8 bg-gray-50 flex justify-end gap-3 mt-4 border-t border-gray-100 rounded-b-2xl">
          <button type="button" data-close class="px-5 py-2.5 rounded-lg bg-gray-200 text-dark-2 label-normal font-bold hover:bg-gray-300 transition-colors">Cancelar</button>
          <button type="button" data-save class="px-5 py-2.5 rounded-lg bg-brand-primary text-white label-normal font-bold hover:opacity-90 transition-opacity shadow-sm disabled:opacity-50 disabled:cursor-not-allowed">Guardar cambios</button>
        </div>
      </div>`);
    const form = m.el.querySelector('[data-form]');
    const save = m.el.querySelector('[data-save]');
    if (editing) {
      ui.dropdown(m.el.querySelector('#ddStatus'), {
        items: [
          { value: 'true', label: 'Activo' },
          { value: 'false', label: 'Inactivo' },
        ],
        value: String(active),
        up: true,
        onChange: (v) => (active = v === 'true'),
      });
    }
    ui.phoneInputs(form);
    const v = ui.validator(
      form,
      {
        name: [ui.rules.required],
        phone1: [ui.rules.required, ui.rules.phone],
        phone2: [ui.rules.phone],
        email: [ui.rules.required, [(x) => emailRe.test(x), 'El formato debe ser válido.']],
      },
      (ok) => (save.disabled = !ok)
    );
    v.validate();

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        const f = ui.formValues(form);
        const payload = { name: f.name.trim(), phone1: f.phone1.trim(), phone2: f.phone2.trim() || null, email: f.email.trim() };
        try {
          if (editing) {
            await Hook.put(`${endpoint}/${supplier.id}`, payload);
            if (active !== supplier.active) await Hook.patch(`${endpoint}/${supplier.id}/status`, { active });
          } else {
            await Hook.post(endpoint, payload);
          }
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, editing ? 'Error al actualizar proveedor' : 'Error al crear proveedor');
        }
      })
    );
  }

  function openDelete(supplier) {
    ui.confirmDelete({
      title: 'Eliminar proveedor',
      message: '¿Está seguro de que desea eliminar el proveedor',
      name: supplier.name,
      onConfirm: async () => {
        await Hook.del(`${endpoint}/${supplier.id}`);
        ui.toast('Proveedor eliminado');
        load();
      },
    });
  }

  document.getElementById('addBtn').addEventListener('click', () => openModal(null));
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();