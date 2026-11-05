/* Configuración → Organización → Empleados (/api/:org/users). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const endpoint = Hook.org('users');
  const emailRe = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;
  let roles = [];

  const state = ui.listState({
    perPage: 5,
    search: (u, t) => ui.norm(u.name).includes(t) || ui.norm(u.email).includes(t),
    render,
  });

  const table = ui.table(document.getElementById('table'), {
    state,
    emptyText: 'No se encontraron usuarios.',
    columns: [
      { key: 'name', label: 'Nombre', width: 220, sort: 'name', title: (u) => u.name },
      { key: 'email', label: 'Email', width: 280, sort: 'email', title: (u) => u.email },
      { key: 'roleName', label: 'Rol', width: 250 },
      { key: 'active', label: 'Estado', width: 150, render: (u) => ui.statusPill(u.active), text: (u) => (u.active ? 'Activo' : 'Inactivo') },
    ],
    actions: () => ui.actionBtn('edit', 'pen', 'Editar') + ui.actionBtn('delete', 'trash-can', 'Eliminar') + ui.actionBtn('more', 'ellipsis', 'Más'),
    onAction: (action, u) => (action === 'edit' ? openModal(u) : action === 'delete' ? openDelete(u) : null),
  });

  function render() {
    table.render(state.pageRows);
    ui.pagination(document.getElementById('pager'), { page: state.page, total: state.totalPages, onChange: (p) => state.goTo(p) });
  }

  async function load() {
    try {
      const [users, roleRes] = await Promise.all([Hook.get(endpoint), Hook.get(Hook.org('roles')).catch(() => ({ roles: [] }))]);
      roles = (roleRes.roles || []).filter((r) => r.active);
      state.set(users);
    } catch (err) {
      ui.error(err, 'No se pudieron cargar los usuarios.');
    }
  }

  const plain = (label, name, value, placeholder, type = 'text') => `
    <div class="flex flex-col gap-1.5">
      <label class="label-normal text-dark-1">${label}</label>
      <input type="${type}" name="${name}" value="${ui.esc(value || '')}" placeholder="${ui.esc(placeholder)}" autocomplete="off"
             class="w-full border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal text-dark-1" />
    </div>`;

  function openModal(user) {
    const editing = !!user;
    const u = user || {};
    const cur = { roleId: editing ? u.roleId : null, active: editing ? u.active : true };
    const m = ui.modal(`
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-md flex flex-col border border-gray-100 overflow-visible animate-fade-in">
        <div class="p-6 md:p-8 pb-4">
          ${ui.modalHeader(editing ? 'Editar usuario' : 'Crear usuario')}
          <div class="flex flex-col gap-5" data-form>
            ${plain('Nombre', 'name', u.name, 'Nombre del empleado')}
            ${plain('Email', 'email', u.email, 'email@empresa.com', 'email')}
            ${editing ? '' : plain('Contraseña', 'password', '', 'Introduzca una contraseña', 'password')}
            ${
              editing
                ? ''
                : `<div class="flex flex-col gap-1.5">
                     <label class="label-normal text-dark-1">Confirmar contraseña</label>
                     <input type="password" name="confirmPassword" placeholder="Confirme la contraseña" autocomplete="new-password"
                            class="w-full border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal text-dark-1" />
                     <span data-mismatch class="label-small text-error-primary" hidden>Las contraseñas no coinciden.</span>
                   </div>`
            }
            ${ui.ddField({ label: 'Rol', id: 'ddRole' })}
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

    const valid = () => {
      const f = ui.formValues(form);
      const mismatch = !editing && f.password && f.confirmPassword && f.password !== f.confirmPassword;
      const mm = form.querySelector('[data-mismatch]');
      if (mm) mm.hidden = !mismatch;
      return (
        f.name.trim() !== '' &&
        emailRe.test(f.email.trim()) &&
        !!cur.roleId &&
        (editing || (f.password && f.password === f.confirmPassword))
      );
    };
    const sync = () => (save.disabled = !valid());

    ui.dropdown(m.el.querySelector('#ddRole'), {
      items: roles.map((r) => ({ value: r.id, label: r.name })),
      value: cur.roleId,
      placeholder: 'Seleccionar rol',
      up: true,
      onChange: (v) => {
        cur.roleId = v;
        sync();
      },
    });
    if (editing) {
      ui.dropdown(m.el.querySelector('#ddStatus'), {
        items: [
          { value: 'Activo', label: 'Activo' },
          { value: 'Inactivo', label: 'Inactivo' },
        ],
        value: cur.active ? 'Activo' : 'Inactivo',
        up: true,
        onChange: (v) => (cur.active = v === 'Activo'),
      });
    }
    form.addEventListener('input', sync);
    sync();

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        const f = ui.formValues(form);
        const payload = { name: f.name.trim(), email: f.email.trim(), roleId: cur.roleId };
        try {
          if (editing) {
            await Hook.put(`${endpoint}/${user.id}`, payload);
            if (cur.active !== user.active) await Hook.patch(`${endpoint}/${user.id}/status`, { active: cur.active });
          } else {
            await Hook.post(endpoint, Object.assign(payload, { password: f.password, active: true }));
          }
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err);
        }
      })
    );
  }

  function openDelete(user) {
    ui.confirmDelete({
      title: 'Eliminar usuario',
      message: '¿Está seguro de que desea eliminar al usuario',
      name: user.name,
      onConfirm: async () => {
        await Hook.patch(`${endpoint}/${user.id}/status`, { active: false });
        ui.toast('Cambios guardados');
        load();
      },
    });
  }

  document.getElementById('addBtn').addEventListener('click', () => openModal(null));
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();
