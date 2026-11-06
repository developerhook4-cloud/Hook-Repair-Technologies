/* Configuración → Organización → Roles y permisos (/api/:org/roles). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('roles');

  const MODULES = {
    Organizations: 'Organizaciones',
    Users: 'Usuarios',
    Roles: 'Roles',
    Pos: 'Punto de venta',
    Repairs: 'Reparaciones',
    Inventory: 'Inventario',
    Customers: 'Clientes',
    Suppliers: 'Distribuidores',
    Catalogs: 'Catálogos',
    Devices: 'Dispositivos',
    Services: 'Servicios',
    RepairStatus: 'Estado de reparaciones',
    Reports: 'Reportes',
  };

  let systemModules = [];
  let roles = [];
  let selectedRole = null;
  let currentPerms = [];
  let activeTab = 'roles';
  let ddRole = null;


  // ── Permisos ⇄ filas de la interfaz ─────────────────────────────────
  const has = (perms, p) => (perms || []).some((x) => x.toLowerCase() === p.toLowerCase());
  const toRows = (perms) =>
    systemModules.map((m) => ({
      module: m,
      label: MODULES[m] || m,
      view: has(perms, `${m}.View`),
      update: has(perms, `${m}.Update`),
      delete: has(perms, `${m}.Delete`),
    }));
  const toPerms = (rows) =>
    rows.flatMap((r) => [r.view && `${r.module}.View`, r.update && `${r.module}.Update`, r.delete && `${r.module}.Delete`].filter(Boolean));
  const displayPerms = (perms) => {
    if (!perms || !perms.length) return 'Sin permisos';
    return [...new Set(perms.map((p) => MODULES[p.split('.')[0]] || p.split('.')[0]).filter(Boolean))].join(', ');
  };

  // ── Pestañas ────────────────────────────────────────────────────────
  function paintTabs() {
    document.querySelectorAll('[data-tab]').forEach((t) => {
      const on = t.dataset.tab === activeTab;
      t.classList.toggle('is-active', on);
    });
    document.getElementById('tabRoles').hidden = activeTab !== 'roles';
    document.getElementById('tabPerms').hidden = activeTab !== 'permisos';
  }
  document.querySelectorAll('[data-tab]').forEach((t) => {
    const go = () => {
      activeTab = t.dataset.tab;
      ui.closeAllDropdowns();
      paintTabs();
    };
    t.addEventListener('click', go);
    t.addEventListener('keydown', (e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), go()));
  });

  // ── Lista de roles ──────────────────────────────────────────────────
  const state = ui.listState({
    perPage: 5,
    search: (r, t) => ui.norm(r.name).includes(t) || (r.permissions || []).some((p) => p.toLowerCase().includes(t)),
    render: renderRoles,
  });

  const list = ui.cardList(document.getElementById('rolesList'), {
    emptyText: 'No se encontraron roles.',
    rowHtml: (r, last) => `
      <div class="role-row">
        <div>
          <h4 class="label-normal role-row__name">${esc(r.name)}</h4>
          <p class="body-normal role-row__perms">${esc(displayPerms(r.permissions))}</p>
        </div>
        <button type="button" data-action="delete" title="Eliminar" class="card-btn card-btn--delete">${ui.icon('trash-can')}</button>
      </div>`,
    onAction: (action, r) => action === 'delete' && openDelete(r),
  });

  function renderRoles() {
    list.render(state.pageRows);
    ui.pagination(document.getElementById('pager'), { page: state.page, total: state.totalPages, onChange: (p) => state.goTo(p) });
  }

  // ── Permisos del rol seleccionado ───────────────────────────────────
  function permRow(item, i, disabled = false) {
    const box = (key, label) => `
      <label class="check perm-row__check${disabled ? ' is-disabled' : ''}">
        <input type="checkbox" data-i="${i}" data-k="${key}" ${item[key] ? 'checked' : ''} ${disabled ? 'disabled' : ''} /> ${label}
      </label>`;
    return `
      <div class="perm-row">
        <h4 class="label-normal perm-row__name">${esc(item.label)}</h4>
        <div class="body-normal perm-row__checks">
          ${box('view', 'Ver')}${box('update', 'Editar')}${box('delete', 'Eliminar')}
        </div>
      </div>`;
  }

  function renderPerms() {
    const term = document.getElementById('searchPerms').value.trim().toLowerCase();
    const listEl = document.getElementById('permsList');
    const rows = currentPerms
      .map((r, i) => [r, i])
      .filter(([r]) => !term || r.label.toLowerCase().includes(term) || r.module.toLowerCase().includes(term));
    listEl.innerHTML = rows.length
      ? rows.map(([r, i]) => permRow(r, i)).join('')
      : '<p class="body-normal perms-empty">No hay permisos que coincidan.</p>';
    syncPermsSave();
  }
  function syncPermsSave() {
    document.getElementById('savePerms').disabled = !selectedRole || toPerms(currentPerms).length === 0;
  }
  document.getElementById('permsList').addEventListener('change', (e) => {
    const cb = e.target.closest('input[data-i]');
    if (cb) currentPerms[Number(cb.dataset.i)][cb.dataset.k] = cb.checked;
    syncPermsSave();
  });
  document.getElementById('searchPerms').addEventListener('input', renderPerms);

  function selectRole(role) {
    selectedRole = role || null;
    currentPerms = toRows(role ? role.permissions : []);
    if (ddRole) ddRole.setValue(role ? role.id : null);
    renderPerms();
  }

  function scrollTop() {
    document.getElementById('scrollContainer').scrollTo({ top: 0, behavior: 'smooth' });
  }

  // ── Carga ───────────────────────────────────────────────────────────
  async function load(keepSelected) {
    try {
      const [perms, res] = await Promise.all([Hook.get(`${endpoint}/permissions`), Hook.get(endpoint)]);
      systemModules = [...new Set(perms.map((p) => p.split('.')[0]).filter(Boolean))];
      roles = (res.roles || []).filter((r) => r.active);
      state.set(roles);
      const items = roles.map((r) => ({ value: r.id, label: r.name }));
      if (!ddRole) {
        ddRole = ui.dropdown(document.getElementById('ddRole'), {
          items,
          variant: 'solid',
          placeholder: 'Seleccionar rol',
          onChange: (v) => selectRole(roles.find((r) => r.id === v)),
        });
      } else {
        ddRole.setItems(items);
      }
      const keep = keepSelected && selectedRole && roles.find((r) => r.id === selectedRole.id);
      selectRole(keep || roles[0]);
    } catch (err) {
      ui.error(err, 'Error cargando roles y permisos.');
    }
  }

  document.getElementById('cancelPerms').addEventListener('click', () => {
    if (selectedRole) selectRole(selectedRole);
    scrollTop();
  });

  const saveBtn = document.getElementById('savePerms');
  saveBtn.addEventListener('click', () =>
    ui.busy(saveBtn, async () => {
      if (!selectedRole) return;
      try {
        await Hook.put(`${endpoint}/${selectedRole.id}`, { name: selectedRole.name, permissions: toPerms(currentPerms) });
        await load(true);
        scrollTop();
        ui.toast('Cambios guardados');
      } catch (err) {
        ui.error(err, 'Error al actualizar rol');
      }
    })
  );

  // ── Nuevo rol ───────────────────────────────────────────────────────
  function openAdd() {
    const rows = toRows([]);
    const m = ui.modal(`
      <div class="modal-card modal-card--2xl role-modal">
        <div class="role-modal__head">
          ${ui.modalHeader('Agregar nuevo rol')}
          <div class="role-modal__name">
            <label class="label-normal field__label">Nombre del rol <span class="req">*</span></label>
            <input type="text" name="name" maxlength="100" placeholder="Introduzca el nombre del rol"
                   class="input role-modal__input" />
          </div>
          <div class="role-modal__perms-label"><label class="label-normal field__label">Permisos para el nuevo rol <span class="req">*</span></label>
            <small data-perm-hint class="label-small field__error" hidden>Selecciona al menos un permiso.</small></div>
        </div>
        <div class="role-modal__perms" data-perms></div>
        <div class="modal-footer">
          <button type="button" data-close class="btn btn--soft">Cancelar</button>
          <button type="button" data-save disabled class="btn btn--primary">Crear rol</button>
        </div>
      </div>`);
    const name = m.el.querySelector('[name=name]');
    const save = m.el.querySelector('[data-save]');
    const permsEl = m.el.querySelector('[data-perms]');
    const hint = m.el.querySelector('[data-perm-hint]');
    const syncSave = () => {
      const none = toPerms(rows).length === 0;
      save.disabled = !name.value.trim() || none;
      hint.hidden = !name.value.trim() || !none;
    };
    const paint = () => {
      const off = !name.value.trim();
      permsEl.innerHTML = rows.map((r, i) => permRow(r, i, off)).join('');
      syncSave();
    };
    permsEl.addEventListener('change', (e) => {
      const cb = e.target.closest('input[data-i]');
      if (cb) rows[Number(cb.dataset.i)][cb.dataset.k] = cb.checked;
      syncSave();
    });
    let wasEmpty = true;
    name.addEventListener('input', () => {
      const empty = !name.value.trim();
      if (empty !== wasEmpty) paint();
      else syncSave();
      wasEmpty = empty;
    });
    paint();

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        try {
          await Hook.post(endpoint, { name: name.value.trim(), permissions: toPerms(rows) });
          m.close();
          ui.toast('Cambios guardados');
          load(true);
        } catch (err) {
          ui.error(err, 'Error al crear rol');
        }
      })
    );
  }

  function openDelete(role) {
    ui.confirmDelete({
      title: 'Eliminar rol',
      message: '¿Está seguro de que desea eliminar el rol',
      name: role.name,
      onConfirm: async () => {
        await Hook.del(`${endpoint}/${role.id}`);
        ui.toast('Rol eliminado');
        load(selectedRole && selectedRole.id !== role.id);
      },
    });
  }

  document.getElementById('addRoleBtn').addEventListener('click', openAdd);
  ui.bindSearch(document.getElementById('searchRoles'), state);
  paintTabs();
  load(false);
})();
