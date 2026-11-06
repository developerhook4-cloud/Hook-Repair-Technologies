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
    actions: (u) => ui.actionBtn('edit', 'pen', 'Editar') + (Hook.user && u.id === Hook.user.id ? '' : ui.actionBtn('delete', 'trash-can', 'Eliminar')),
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
    <div class="field">
      <label class="label-normal field__label">${label}</label>
      <input type="${type}" name="${name}" value="${ui.esc(value || '')}" placeholder="${ui.esc(placeholder)}" autocomplete="off"
             class="input" />
    </div>`;

  function openModal(user) {
    const editing = !!user;
    const u = user || {};
    const cur = { roleId: editing ? u.roleId : null, active: editing ? u.active : true };
    const m = ui.modal(`
      <div class="modal-card modal-card--sm modal-card--visible">
        <div class="modal-body">
          ${ui.modalHeader(editing ? 'Editar usuario' : 'Crear usuario')}
          <div class="form-stack form-stack--roomy" data-form>
            ${plain('Nombre', 'name', u.name, 'Nombre del empleado')}
            ${plain('Email', 'email', u.email, 'email@empresa.com', 'email')}
            <div class="field">
              <label class="label-normal field__label">${editing ? 'Nueva contraseña (opcional)' : 'Contraseña'}</label>
              <input type="password" name="password" placeholder="${editing ? 'Déjala vacía para no cambiarla' : 'Introduzca una contraseña'}" autocomplete="new-password"
                     class="input" />
              <span data-pass-hint class="label-small field__hint">${ui.PASSWORD_HINT}</span>
            </div>
            <div class="field">
              <label class="label-normal field__label">Confirmar contraseña</label>
              <input type="password" name="confirmPassword" placeholder="Confirme la contraseña" autocomplete="new-password"
                     class="input" />
              <span data-mismatch class="label-small field__error" hidden>Las contraseñas no coinciden.</span>
            </div>
            ${ui.ddField({ label: 'Rol', id: 'ddRole' })}
            ${editing ? ui.ddField({ label: 'Estado', id: 'ddStatus' }) : ''}
          </div>
        </div>
        <div class="modal-footer modal-footer--soft">
          <button type="button" data-close class="btn btn--muted">Cancelar</button>
          <button type="button" data-save class="btn btn--primary">Guardar cambios</button>
        </div>
      </div>`);
    const form = m.el.querySelector('[data-form]');
    const save = m.el.querySelector('[data-save]');

    const valid = () => {
      const f = ui.formValues(form);
      const mismatch = !!(f.password || f.confirmPassword) && f.confirmPassword !== '' && f.password !== f.confirmPassword;
      form.querySelector('[data-mismatch]').hidden = !mismatch;
      const passErr = f.password ? ui.passwordError(f.password) : '';
      const hint = form.querySelector('[data-pass-hint]');
      hint.textContent = passErr || ui.PASSWORD_HINT;
      hint.className = `label-small ${passErr ? 'field__error' : 'field__hint'}`;
      const passOk = editing && !f.password ? !f.confirmPassword : !!f.password && !passErr && f.password === f.confirmPassword;
      return f.name.trim() !== '' && emailRe.test(f.email.trim()) && !!cur.roleId && passOk;
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
            if (f.password) payload.password = f.password;
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
        await Hook.del(`${endpoint}/${user.id}`);
        ui.toast('Usuario eliminado');
        load();
      },
    });
  }

  document.getElementById('addBtn').addEventListener('click', () => openModal(null));
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();
