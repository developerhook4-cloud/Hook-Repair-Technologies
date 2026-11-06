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

  const checkbox = 'w-4.5 h-4.5 rounded border-gray-300 text-brand-primary focus:ring-brand-primary accent-brand-primary';

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
      t.className = `rounded-lg py-2 px-4 w-72 flex flex-col justify-center cursor-pointer transition-colors ${
        on ? 'bg-brand-primary text-white' : 'bg-gray-50 border border-gray-200 text-gray-500 hover:bg-gray-100'
      }`;
      t.querySelector('h4').className = `label-normal font-bold ${on ? 'text-white' : 'text-dark-1'}`;
      t.querySelector('p').className = `text-xs mt-1 whitespace-nowrap overflow-hidden text-ellipsis ${on ? 'text-white/80' : 'text-gray-400'}`;
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
      <div class="py-5 flex justify-between items-center ${last ? '' : 'border-b border-gray-200'}">
        <div>
          <h4 class="label-normal font-bold text-dark-1 mb-1">${esc(r.name)}</h4>
          <p class="body-normal text-gray-400">${esc(displayPerms(r.permissions))}</p>
        </div>
        <button type="button" data-action="delete" title="Eliminar" class="w-8 h-8 flex items-center justify-center text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg transition-colors">${ui.icon('trash-can')}</button>
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
      <label class="flex items-center gap-2 ${disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'} transition-opacity duration-200">
        <input type="checkbox" data-i="${i}" data-k="${key}" ${item[key] ? 'checked' : ''} ${disabled ? 'disabled' : ''} class="${checkbox} ${disabled ? 'cursor-not-allowed' : 'cursor-pointer'}" /> ${label}
      </label>`;
    return `
      <div class="border border-gray-200 rounded-2xl p-4 md:px-6 flex flex-col md:flex-row md:items-center justify-between gap-4 transition-colors hover:border-gray-300">
        <h4 class="label-normal font-bold text-dark-1">${esc(item.label)}</h4>
        <div class="flex items-center gap-6 md:gap-8 body-normal font-semibold text-dark-2">
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
      : '<p class="text-center py-8 text-gray-400 body-normal">No hay permisos que coincidan.</p>';
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
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-4xl flex flex-col max-h-[90vh] overflow-hidden border border-gray-100 animate-fade-in">
        <div class="p-8 pb-4">
          ${ui.modalHeader('Agregar nuevo rol')}
          <div class="flex flex-col mb-6">
            <label class="label-normal font-semibold text-dark-1 mb-2">Nombre del rol <span class="text-brand-primary">*</span></label>
            <input type="text" name="name" maxlength="100" placeholder="Introduzca el nombre del rol"
                   class="w-full md:w-80 border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal placeholder-gray-400" />
          </div>
          <div class="mb-2 flex flex-col gap-1"><label class="label-normal text-dark-1">Permisos para el nuevo rol <span class="text-brand-primary">*</span></label>
            <small data-perm-hint class="label-small text-error-primary" hidden>Selecciona al menos un permiso.</small></div>
        </div>
        <div class="px-8 overflow-y-auto flex-1 flex flex-col gap-3 pb-4" data-perms></div>
        <div class="p-6 md:px-8 border-t border-gray-100 bg-white flex justify-end gap-4">
          <button type="button" data-close class="px-6 py-2.5 rounded-lg bg-gray-100 text-dark-2 font-bold label-normal hover:bg-gray-200 transition-colors">Cancelar</button>
          <button type="button" data-save disabled class="px-6 py-2.5 rounded-lg bg-brand-primary text-white font-bold label-normal transition-all shadow-sm hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed">Crear rol</button>
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