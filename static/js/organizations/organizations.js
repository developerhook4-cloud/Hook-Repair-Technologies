/* Organizaciones (vista de plataforma): /api/organizations. */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = '/api/organizations';
  const canUpdate = Hook.can('Organizations.Update');
  const canDelete = Hook.can('Organizations.Delete');

  let organizations = [];
  let selected = null;
  let orgUsers = [];

  // ── Tabla genérica (app-table) ──────────────────────────────────────
  function appTable(columns, rows, { selectedId, onRow, onAction, compact = false } = {}) {
    const px = compact ? 'px-3' : 'px-6';
    const head = columns
      .map((c) => `<th scope="col" class="${px} py-3 font-semibold ${c.type === 'action' ? 'text-right' : ''}">${esc(c.label)}</th>`)
      .join('');
    const body = rows.length
      ? rows
          .map((r, i) => {
            const cells = columns
              .map((c) => {
                let v;
                if (c.type === 'badge') {
                  v = `<span class="inline-flex items-center justify-center ${compact ? 'px-2 py-0.5 label-small' : 'px-3 py-1 label-normal'} rounded-full ${
                    r[c.key] ? 'bg-success-subtle text-success-darker' : 'bg-error-subtle text-error-darker'
                  }">${r[c.key] ? 'Activo' : 'Inactivo'}</span>`;
                } else if (c.type === 'action') {
                  v = `<div class="flex justify-end ${compact ? 'gap-2' : 'gap-3'}">
                         ${canUpdate ? `<button type="button" data-act="edit" class="text-dark-3 hover:text-info-primary transition-colors" title="Editar">${ui.icon('pencil')}</button>` : ''}
                         ${canDelete && !(Hook.user && r.id === Hook.user.id) ? `<button type="button" data-act="delete" class="text-dark-3 hover:text-error-primary transition-colors" title="Eliminar">${ui.icon('trash-can')}</button>` : ''}
                       </div>`;
                } else {
                  v = `<span>${esc(r[c.key])}</span>`;
                }
                return `<td class="${px} py-4 text-dark-1 body-small ${c.type === 'action' ? 'text-right' : ''} ${compact && c.type !== 'action' && c.type !== 'badge' ? 'whitespace-normal break-words' : ''}">${v}</td>`;
              })
              .join('');
            return `<tr data-i="${i}" class="hover:bg-light-4 transition-colors cursor-pointer ${selectedId && selectedId === r.id ? 'bg-light-4' : ''}">${cells}</tr>`;
          })
          .join('')
      : `<tr><td colspan="${columns.length}" class="px-6 py-8 text-center text-dark-2 body-normal">No hay datos para mostrar</td></tr>`;
    const el = ui.el(`
      <div class="overflow-x-auto rounded-xl border border-light-2 bg-white shadow-sm">
        <table class="min-w-full text-left whitespace-nowrap">
          <thead class="bg-light-3 heading-6 text-dark-2 border-b border-light-2"><tr>${head}</tr></thead>
          <tbody class="divide-y divide-light-2">${body}</tbody>
        </table>
      </div>`);
    el.addEventListener('click', (e) => {
      const tr = e.target.closest('tr[data-i]');
      if (!tr) return;
      const row = rows[Number(tr.dataset.i)];
      const act = e.target.closest('[data-act]');
      if (act) {
        e.stopPropagation();
        if (onAction) onAction(act.dataset.act, row);
      } else if (onRow) onRow(row);
    });
    return el;
  }

  // ── Lista ───────────────────────────────────────────────────────────
  const orgColumns = [
    { key: 'name', label: 'Nombre' },
    { key: 'email', label: 'Email' },
    { key: 'phone', label: 'Teléfono' },
    { key: 'employees', label: 'Empleados' },
    { key: 'active', label: 'Estado', type: 'badge' },
  ];

  function renderList() {
    const term = document.getElementById('search').value.trim().toLowerCase();
    const rows = term ? organizations.filter((o) => ui.norm(o.name).includes(term) || ui.norm(o.email).includes(term)) : organizations;
    const box = document.getElementById('orgTable');
    box.innerHTML = '';
    box.appendChild(appTable(orgColumns, rows, { selectedId: selected && selected.id, onRow: selectOrg }));
  }

  async function load() {
    try {
      organizations = await Hook.get(endpoint);
      if (selected) selected = organizations.find((o) => o.id === selected.id) || null;
      renderList();
      renderDetail();
    } catch (err) {
      ui.error(err, 'Error al cargar organizaciones');
    }
  }

  // ── Detalle ─────────────────────────────────────────────────────────
  async function selectOrg(org) {
    selected = org;
    orgUsers = [];
    renderList();
    renderDetail();
    if (!org) return;
    try {
      orgUsers = await Hook.get(`${endpoint}/${org.id}/users`);
      renderDetail();
    } catch (err) {
      ui.error(err, 'Error al cargar usuarios');
    }
  }

  function renderDetail() {
    const panel = document.getElementById('orgDetail');
    const list = document.getElementById('orgListPanel');
    list.classList.toggle('lg:w-2/3', !!selected);
    list.classList.toggle('hidden', !!selected);
    list.classList.toggle('lg:flex', !!selected);
    list.classList.toggle('flex', !selected);
    panel.hidden = !selected;
    if (!selected) return;
    const o = selected;
    const a = o.address || {};
    panel.innerHTML = `
      <button type="button" data-close-detail class="absolute top-6 right-6 text-dark-3 hover:text-info-primary h-8 w-8 flex items-center justify-center">${ui.icon('xmark')}</button>
      <div class="mb-8 pr-10"><h3 class="text-2xl font-bold text-dark-1 leading-tight">${esc(o.name)}</h3></div>
      <div class="flex flex-col xl:flex-row gap-6">
        <div class="flex flex-col gap-1">
          <p class="text-dark-1 label-normal">Contacto:</p>
          <div class="flex items-center gap-3">${ui.icon('envelope', 'w-4 text-dark-2')} ${esc(o.email)}</div>
          <div class="flex items-center gap-3" title="Teléfono">${ui.icon('phone', 'w-4 text-dark-2')} ${esc(o.phone1 || o.phone || 'Teléfono no registrado')}</div>
          <div class="flex items-center gap-3" title="Celular">${ui.icon('mobile-screen', 'w-4 text-dark-2')} ${esc(o.phone2 || 'Celular no registrado')}</div>
          <div class="flex items-center gap-3">${ui.icon('link', 'w-4 text-dark-2')} ${esc(o.website || 'No registrado')}</div>
        </div>
        <div class="flex flex-col gap-1">
          <p class="text-dark-1 label-normal">Ubicación:</p>
          <div class="flex items-start gap-3">${ui.icon('location-dot', 'w-4 text-dark-2 mt-1')}
            <div>
              <p>${esc(a.address || 'No registrado')}</p>
              <p>${esc(a.city || 'No registrada')}, ${esc(a.state || 'No registrado')}</p>
              <p>C.P. ${esc(a.postCode || 'No registrado')}</p>
              <p>${esc(a.country || '')}</p>
            </div>
          </div>
        </div>
      </div>
      <div class="my-4 flex items-center justify-between">
        <h4 class="font-bold text-dark-1">Usuarios</h4>
        ${canUpdate ? `<button type="button" data-add-user class="label-large py-2.5 rounded-lg transition-colors shadow-small px-4 bg-brand-primary hover:bg-brand-darker text-light-4">${ui.icon('plus')}<span class="pl-1 hidden sm:inline">Crear usuario</span></button>` : ''}
      </div>
      <div class="bg-white rounded-2xl shadow-sm mb-8" data-users></div>
      <div class="mt-auto pt-4 border-t mx-auto border-light-2 w-full">
        <div class="flex flex-col xl:flex-row gap-3">
          ${canUpdate ? `<button type="button" data-edit-org class="w-full label-large py-3.5 rounded-lg transition-colors shadow-small px-4 bg-light-1 hover:bg-dark-4 text-dark-1">Modificar</button>` : ''}
          ${canDelete ? `<button type="button" data-toggle-org class="w-full label-large py-3.5 rounded-lg transition-colors shadow-small px-4 bg-brand-primary hover:bg-brand-darker text-light-4">${o.active ? 'Desactivar' : 'Activar'}</button>` : ''}
        </div>
      </div>`;
    const empColumns = [
      { key: 'name', label: 'Usuario' },
      { key: 'role', label: 'Rol' },
      { key: 'active', label: 'Estado', type: 'badge' },
    ];
    if (canUpdate || canDelete) empColumns.push({ key: 'actions', label: '', type: 'action' });
    panel.querySelector('[data-users]').appendChild(appTable(empColumns, orgUsers, { onAction: userAction, compact: true }));
    panel.querySelector('[data-close-detail]').addEventListener('click', () => selectOrg(null));
    const addUser = panel.querySelector('[data-add-user]');
    if (addUser) addUser.addEventListener('click', () => openUserModal(null));
    const editOrg = panel.querySelector('[data-edit-org]');
    if (editOrg) editOrg.addEventListener('click', () => openOrgModal(o));
    const toggleOrg = panel.querySelector('[data-toggle-org]');
    if (toggleOrg) toggleOrg.addEventListener('click', () => openToggleOrg(o));
  }

  // ── Modales (app-modal) ─────────────────────────────────────────────
  function appModal(title, body, size = 'md') {
    return ui.modal(
      `<div class="relative w-full bg-white rounded-[24px] shadow-2xl flex flex-col ${size === 'sm' ? 'max-w-md' : 'max-w-2xl'} animate-fade-in">
         <div class="flex items-center justify-between px-6 py-5 border-b border-gray-100">
           <h3 class="text-xl font-bold text-gray-900">${esc(title)}</h3>
           <button type="button" data-close class="text-gray-400 hover:text-gray-800 transition-colors h-8 w-8 flex items-center justify-center rounded-full hover:bg-gray-100">${ui.icon('xmark', 'text-lg')}</button>
         </div>
         <div class="p-6 overflow-y-auto max-h-[70vh]">${body}</div>
       </div>`,
      { overlayClass: 'bg-black/60 overflow-y-auto overflow-x-hidden' }
    );
  }

  const input = (label, name, value = '', type = 'text', extra = '') => `
    <div class="flex flex-col gap-2 w-full">
      <label class="label-normal text-dark-1">${esc(label)}</label>
      <input type="${type}" name="${name}" value="${esc(value || '')}" ${type === 'password' ? 'autocomplete="new-password"' : ''} ${extra}
             class="w-full bg-white border border-light-2 rounded-lg p-3 body-normal text-dark-1 outline-none focus:border-brand-primary transition-colors placeholder:text-dark-4" />
      <small data-error-for="${name}" class="label-small text-error-primary" hidden></small>
    </div>`;

  const buttons = (saveText) => `
    <div class="flex justify-end mx-auto gap-3 mt-4 pt-4 border-t border-gray-100">
      <button type="button" data-close class="label-large py-3.5 rounded-lg transition-colors shadow-small px-8 bg-light-1 hover:bg-dark-4 text-dark-1">Cancelar</button>
      <button type="submit" data-save class="label-large py-3.5 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors shadow-small px-8 bg-success-primary hover:bg-success-darker text-light-4">${esc(saveText)}</button>
    </div>`;

  function openOrgModal(org) {
    const editing = !!org;
    const o = org || {};
    const a = o.address || {};
    const m = appModal(
      editing ? 'Editar Organización' : 'Nueva Organización',
      `<form class="flex flex-col gap-6" novalidate>
         <section><div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
           ${input('Nombre *', 'name', o.name)}
           ${input('Email *', 'email', o.email, 'email')}
           ${input('Teléfono', 'phone', o.phone1 || o.phone, 'text', 'data-phone placeholder="662 123 4567"')}
           ${input('Celular', 'phone2', o.phone2, 'text', 'data-phone placeholder="662 123 4567"')}
           <div class="sm:col-span-2">${input('Página Web', 'website', o.website, 'text', 'placeholder="https://www.ejemplo.com"')}</div>
         </div></section>
         <div class="h-px w-full bg-gray-200"></div>
         <section><h4 class="heading-6 text-dark-1 mb-4">Ubicación</h4><div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
           <div class="sm:col-span-2">${input('Dirección', 'address', a.address, 'text', 'placeholder="Calle, número y colonia"')}</div>
           ${input('Ciudad', 'city', a.city)}${input('Estado', 'state', a.state)}
           ${input('Código Postal', 'postCode', a.postCode, 'text', 'inputmode="numeric" maxlength="10"')}${input('País', 'country', a.country || (editing ? '' : 'México'))}
         </div></section>
         ${buttons('Guardar Organización')}
       </form>`
    );
    const form = m.el.querySelector('form');
    const save = m.el.querySelector('[data-save]');
    ui.phoneInputs(form);
    const v = ui.validator(
      form,
      {
        name: [ui.rules.required],
        email: [ui.rules.required, ui.rules.email],
        phone: [ui.rules.phone],
        phone2: [ui.rules.phone],
        postCode: [ui.rules.pattern(/^[0-9]+$/)],
      },
      (ok) => (save.disabled = !ok)
    );
    v.validate();
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      if (!v.validate(true)) return;
      ui.busy(save, async () => {
        const f = ui.formValues(form);
        const payload = {
          name: f.name.trim(),
          email: f.email.trim(),
          phone1: (f.phone || '').trim(),
          phone2: (f.phone2 || '').trim(),
          website: (f.website || '').trim(),
          address: {
            address: (f.address || '').trim(),
            city: (f.city || '').trim(),
            state: (f.state || '').trim(),
            postCode: (f.postCode || '').trim(),
            country: (f.country || '').trim(),
          },
          active: editing ? o.active : true,
        };
        try {
          if (editing) selected = await Hook.put(`${endpoint}/${o.id}`, payload);
          else await Hook.post(endpoint, payload);
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, editing ? 'Error al actualizar organización' : 'Error al crear organización');
        }
      });
    });
  }

  function openUserModal(user) {
    const org = selected;
    if (!org) return;
    const editing = !!user;
    let active = editing ? user.active : true;
    const m = appModal(
      editing ? 'Editar empleado' : 'Nuevo empleado',
      `<form class="flex flex-col gap-4" novalidate>
         <p class="text-sm text-dark-2 mb-2">${editing ? 'Editar un usuario' : 'Registrar un nuevo usuario'} de la organización <strong>${esc(org.name)}</strong>.${editing ? '' : ' Se crea con el rol Administrador.'}</p>
         ${input('Nombre completo', 'name', editing ? user.name : '')}
         ${input('Email', 'email', editing ? user.email : '', 'email')}
         ${input(editing ? 'Nueva contraseña (opcional)' : 'Contraseña', 'password', '', 'password', editing ? 'placeholder="Déjala vacía para no cambiarla"' : '')}
         <span class="label-small text-gray-400 -mt-2">${ui.PASSWORD_HINT}</span>
         ${input('Confirmar contraseña', 'confirmPassword', '', 'password')}
         ${editing ? '<div class="flex flex-col gap-2 w-full"><label class="label-normal text-dark-1">Estado</label><div id="ddUserStatus"></div></div>' : ''}
         <div class="flex justify-end gap-3 mt-6">
           <button type="button" data-close class="label-large py-3.5 rounded-lg transition-colors shadow-small px-8 bg-light-1 hover:bg-dark-4 text-dark-1">Cancelar</button>
           <button type="submit" data-save class="label-large py-3.5 rounded-lg disabled:opacity-50 disabled:cursor-not-allowed transition-colors shadow-small px-8 bg-success-primary hover:bg-success-darker text-light-4">${editing ? 'Guardar cambios' : 'Crear Empleado'}</button>
         </div>
       </form>`,
      'sm'
    );
    const form = m.el.querySelector('form');
    const save = m.el.querySelector('[data-save]');
    if (editing) {
      ui.dropdown(m.el.querySelector('#ddUserStatus'), {
        items: [
          { value: 'true', label: 'Activo' },
          { value: 'false', label: 'Inactivo' },
        ],
        value: String(active),
        up: true,
        onChange: (val) => (active = val === 'true'),
      });
    }
    const rules = {
      name: [ui.rules.required],
      email: [ui.rules.required, ui.rules.email],
      password: editing ? [ui.rules.password] : [ui.rules.required, ui.rules.password],
      confirmPassword: [[(val, all) => !all.password || val === all.password, 'Las contraseñas no coinciden.']],
    };
    const v = ui.validator(form, rules, (ok) => (save.disabled = !ok));
    v.validate();
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      if (!v.validate(true)) return;
      ui.busy(save, async () => {
        const f = ui.formValues(form);
        const body = { name: f.name.trim(), email: f.email.trim(), password: f.password };
        try {
          if (editing) await Hook.put(`${endpoint}/${org.id}/users/${user.id}`, Object.assign(body, { active }));
          else await Hook.post(`${endpoint}/${org.id}/users`, body);
          m.close();
          ui.toast('Cambios guardados');
          selectOrg(org);
          load();
        } catch (err) {
          ui.error(err, 'Error al guardar el usuario');
        }
      });
    });
  }

  function userAction(action, user) {
    if (action === 'edit') return openUserModal(user);
    const org = selected;
    ui.confirmDelete({
      title: 'Eliminar usuario',
      message: '¿Está seguro de que desea eliminar al usuario',
      name: user.name,
      onConfirm: async () => {
        await Hook.del(`${endpoint}/${org.id}/users/${user.id}`);
        ui.toast('Usuario eliminado');
        selectOrg(org);
        load();
      },
    });
  }

  function openToggleOrg(org) {
    const deactivating = org.active;
    const m = appModal(
      deactivating ? 'Desactivar organización' : 'Activar organización',
      `<div class="flex flex-col gap-6 text-center">
         <div class="mx-auto flex items-center justify-center h-16 w-16 rounded-full bg-error-subtle">${ui.icon('triangle-exclamation', 'text-2xl text-error-darker')}</div>
         <div><p class="text-dark-1 label-normal">¿Está seguro de que desea ${deactivating ? 'desactivar' : 'activar'} la organización <strong>${esc(org.name)}</strong>?</p></div>
         <div class="flex justify-center gap-3 w-full">
           <button type="button" data-close class="w-full label-large py-3.5 rounded-lg transition-colors shadow-small px-4 bg-light-1 hover:bg-dark-4 text-dark-1">Cancelar</button>
           <button type="button" data-confirm class="w-full label-large py-3.5 rounded-lg transition-colors shadow-small px-4 bg-brand-primary hover:bg-brand-darker text-light-4 disabled:opacity-50">Sí, ${deactivating ? 'desactivar' : 'activar'}</button>
         </div>
       </div>`,
      'sm'
    );
    const btn = m.el.querySelector('[data-confirm]');
    btn.addEventListener('click', () =>
      ui.busy(btn, async () => {
        try {
          await Hook.patch(`${endpoint}/${org.id}/status`, { active: !org.active });
          m.close();
          selected = null;
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, 'Error al cambiar estado');
        }
      })
    );
  }

  const addBtn = document.getElementById('addOrgBtn');
  if (addBtn) addBtn.addEventListener('click', () => openOrgModal(null));
  document.getElementById('search').addEventListener('input', renderList);
  load();
})();