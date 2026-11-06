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
    const head = columns
      .map((c) => `<th scope="col" class="org-table__th${c.type === 'action' ? ' is-right' : ''}">${esc(c.label)}</th>`)
      .join('');
    const body = rows.length
      ? rows
          .map((r, i) => {
            const cells = columns
              .map((c) => {
                let v;
                if (c.type === 'badge') {
                  v = `<span class="${compact ? 'label-small' : 'label-normal'} org-badge ${r[c.key] ? 'org-badge--on' : 'org-badge--off'}">${r[c.key] ? 'Activo' : 'Inactivo'}</span>`;
                } else if (c.type === 'action') {
                  v = `<div class="org-table__actions">
                         ${canUpdate ? `<button type="button" data-act="edit" class="org-table__btn org-table__btn--edit" title="Editar">${ui.icon('pencil')}</button>` : ''}
                         ${canDelete && !(Hook.user && r.id === Hook.user.id) ? `<button type="button" data-act="delete" class="org-table__btn org-table__btn--delete" title="Eliminar">${ui.icon('trash-can')}</button>` : ''}
                       </div>`;
                } else {
                  v = `<span>${esc(r[c.key])}</span>`;
                }
                return `<td class="body-small org-table__td${c.type === 'action' ? ' is-right' : ''}${c.type !== 'action' && c.type !== 'badge' ? ' is-text' : ''}">${v}</td>`;
              })
              .join('');
            return `<tr data-i="${i}" class="org-table__row${selectedId && selectedId === r.id ? ' is-selected' : ''}">${cells}</tr>`;
          })
          .join('')
      : `<tr><td colspan="${columns.length}" class="body-normal org-table__empty">No hay datos para mostrar</td></tr>`;
    const el = ui.el(`
      <div class="org-table${compact ? ' org-table--compact' : ''}">
        <table class="org-table__table">
          <thead class="heading-6 org-table__thead"><tr>${head}</tr></thead>
          <tbody class="org-table__tbody">${body}</tbody>
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
    list.classList.toggle('has-detail', !!selected);
    panel.hidden = !selected;
    if (!selected) return;
    const o = selected;
    const a = o.address || {};
    panel.innerHTML = `
      <button type="button" data-close-detail class="org-detail__close" aria-label="Cerrar">${ui.icon('xmark')}</button>
      <div class="org-detail__head"><h3 class="org-detail__name">${esc(o.name)}</h3></div>
      <div class="org-detail__info">
        <div class="org-detail__group">
          <p class="label-normal">Contacto:</p>
          <div class="org-detail__line">${ui.icon('envelope', 'org-detail__icon')} ${esc(o.email)}</div>
          <div class="org-detail__line" title="Teléfono">${ui.icon('phone', 'org-detail__icon')} ${esc(o.phone1 || o.phone || 'Teléfono no registrado')}</div>
          <div class="org-detail__line" title="Celular">${ui.icon('mobile-screen', 'org-detail__icon')} ${esc(o.phone2 || 'Celular no registrado')}</div>
          <div class="org-detail__line">${ui.icon('link', 'org-detail__icon')} ${esc(o.website || 'No registrado')}</div>
        </div>
        <div class="org-detail__group">
          <p class="label-normal">Ubicación:</p>
          <div class="org-detail__line org-detail__line--top">${ui.icon('location-dot', 'org-detail__icon')}
            <div>
              <p>${esc(a.address || 'No registrado')}</p>
              <p>${esc(a.city || 'No registrada')}, ${esc(a.state || 'No registrado')}</p>
              <p>C.P. ${esc(a.postCode || 'No registrado')}</p>
              <p>${esc(a.country || '')}</p>
            </div>
          </div>
        </div>
      </div>
      <div class="org-detail__users-head">
        <h4 class="org-detail__users-title">Usuarios</h4>
        ${canUpdate ? `<button type="button" data-add-user class="btn btn--primary-dark org-detail__add">${ui.icon('plus')}<span class="org-detail__add-text">Crear usuario</span></button>` : ''}
      </div>
      <div class="org-detail__users" data-users></div>
      <div class="org-detail__footer">
        <div class="org-detail__actions">
          ${canUpdate ? `<button type="button" data-edit-org class="btn btn--light btn--lg btn--block">Modificar</button>` : ''}
          ${canDelete ? `<button type="button" data-toggle-org class="btn btn--primary-dark btn--lg btn--block">${o.active ? 'Desactivar' : 'Activar'}</button>` : ''}
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
      `<div class="modal-card ${size === 'sm' ? 'modal-card--sm' : 'modal-card--lg'} org-modal">
         <div class="org-modal__head">
           <h3 class="org-modal__title">${esc(title)}</h3>
           <button type="button" data-close class="org-modal__close" aria-label="Cerrar">${ui.icon('xmark')}</button>
         </div>
         <div class="org-modal__body">${body}</div>
       </div>`,
      { overlay: 'modal-overlay--dark' }
    );
  }

  const input = (label, name, value = '', type = 'text', extra = '') => `
    <div class="form-field">
      <label class="label-normal form-field__label">${esc(label)}</label>
      <input type="${type}" name="${name}" value="${esc(value || '')}" ${type === 'password' ? 'autocomplete="new-password"' : ''} ${extra}
             class="form-field__input" />
      <small data-error-for="${name}" class="label-small form-field__error" hidden></small>
    </div>`;

  const buttons = (saveText) => `
    <div class="org-modal__buttons org-modal__buttons--bordered">
      <button type="button" data-close class="btn btn--light btn--lg">Cancelar</button>
      <button type="submit" data-save class="btn btn--success btn--lg">${esc(saveText)}</button>
    </div>`;

  function openOrgModal(org) {
    const editing = !!org;
    const o = org || {};
    const a = o.address || {};
    const m = appModal(
      editing ? 'Editar Organización' : 'Nueva Organización',
      `<form class="org-modal__form" novalidate>
         <section><div class="form-grid form-grid--sm">
           ${input('Nombre *', 'name', o.name)}
           ${input('Email *', 'email', o.email, 'email')}
           ${input('Teléfono', 'phone', o.phone1 || o.phone, 'text', 'data-phone placeholder="662 123 4567"')}
           ${input('Celular', 'phone2', o.phone2, 'text', 'data-phone placeholder="662 123 4567"')}
           <div class="span-all">${input('Página Web', 'website', o.website, 'text', 'placeholder="https://www.ejemplo.com"')}</div>
         </div></section>
         <div class="org-modal__divider"></div>
         <section><h4 class="heading-6 org-modal__section-title">Ubicación</h4><div class="form-grid form-grid--sm">
           <div class="span-all">${input('Dirección', 'address', a.address, 'text', 'placeholder="Calle, número y colonia"')}</div>
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
      `<form class="form-stack" novalidate>
         <p class="org-modal__intro">${editing ? 'Editar un usuario' : 'Registrar un nuevo usuario'} de la organización <strong>${esc(org.name)}</strong>.${editing ? '' : ' Se crea con el rol Administrador.'}</p>
         ${input('Nombre completo', 'name', editing ? user.name : '')}
         ${input('Email', 'email', editing ? user.email : '', 'email')}
         ${input(editing ? 'Nueva contraseña (opcional)' : 'Contraseña', 'password', '', 'password', editing ? 'placeholder="Déjala vacía para no cambiarla"' : '')}
         <span class="label-small field__hint org-modal__hint">${ui.PASSWORD_HINT}</span>
         ${input('Confirmar contraseña', 'confirmPassword', '', 'password')}
         ${editing ? '<div class="form-field"><label class="label-normal form-field__label">Estado</label><div id="ddUserStatus"></div></div>' : ''}
         <div class="org-modal__buttons">
           <button type="button" data-close class="btn btn--light btn--lg">Cancelar</button>
           <button type="submit" data-save class="btn btn--success btn--lg">${editing ? 'Guardar cambios' : 'Crear Empleado'}</button>
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
      `<div class="org-toggle">
         <div class="org-toggle__icon">${ui.icon('triangle-exclamation')}</div>
         <div><p class="label-normal">¿Está seguro de que desea ${deactivating ? 'desactivar' : 'activar'} la organización <strong>${esc(org.name)}</strong>?</p></div>
         <div class="org-toggle__buttons">
           <button type="button" data-close class="btn btn--light btn--lg btn--block">Cancelar</button>
           <button type="button" data-confirm class="btn btn--primary-dark btn--lg btn--block">Sí, ${deactivating ? 'desactivar' : 'activar'}</button>
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
