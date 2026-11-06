/* Configuración → Inventario → Categorías (/api/:org/categories). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('categories');

  const state = ui.listState({
    perPage: 5,
    search: (r, t) => ui.norm(r.name).includes(t),
    render,
  });

  const list = ui.cardList(document.getElementById('list'), {
    emptyText: 'No se encontraron categorías.',
    rowHtml: (c, last) => `
      <div class="card-row">
        <div class="card-row__main">
          <h4 class="label-normal card-row__title">${esc(c.name)}</h4>
          ${c.icon ? `<i class="${esc(c.icon)} card-row__glyph"></i>` : ''}
        </div>
        ${ui.cardButtons()}
      </div>`,
    onAction: (action, c) => (action === 'edit' ? openModal(c) : openDelete(c)),
  });

  function render() {
    list.render(state.pageRows);
    ui.pagination(document.getElementById('pager'), { page: state.page, total: state.totalPages, onChange: (p) => state.goTo(p) });
  }

  async function load() {
    try {
      const rows = await Hook.get(endpoint);
      state.set(rows.filter((c) => c.active));
    } catch (err) {
      ui.error(err, 'No se pudieron cargar las categorías.');
    }
  }

  // ── Buscador de íconos (app-icon-search) ────────────────────────────
  let iconsCache = null;
  async function loadIcons() {
    if (!iconsCache) {
      const raw = await fetch('/static/data/icons.json').then((r) => r.json());
      iconsCache = raw.map(([name, style, terms]) => ({ name, style, terms: terms || [] }));
    }
    return iconsCache;
  }

  function iconPicker(container, initial, onPick) {
    const parts = (initial || '').split('fa-');
    container.innerHTML = `
      <div class="icon-picker" data-dd>
        <div class="icon-picker__row">
          <input type="text" autocomplete="off" placeholder="Buscar ícono (ej. user, home, trash)..." value="${esc(parts.length > 1 ? parts[1] : '')}"
                 class="form-field__input" />
          <span data-preview class="icon-picker__preview">${initial ? `<i class="${esc(initial)}"></i>` : ''}</span>
        </div>
        <div data-grid class="icon-picker__grid" hidden></div>
      </div>`;
    const input = container.querySelector('input');
    const grid = container.querySelector('[data-grid]');
    const preview = container.querySelector('[data-preview]');
    const wrap = container.firstElementChild;

    async function show(term) {
      const icons = await loadIcons();
      const t = term.trim().toLowerCase();
      const found = (t ? icons.filter((i) => i.name.includes(t) || i.terms.some((s) => String(s).includes(t))) : icons).slice(0, 40);
      grid.innerHTML = found
        .map(
          (i) => `<button type="button" data-icon="${i.style} fa-${i.name}" title="${esc(i.name)}"
                    class="icon-picker__option">
                    <i class="${i.style} fa-${i.name}"></i>
                    <span>${esc(i.name)}</span>
                  </button>`
        )
        .join('');
      grid.hidden = found.length === 0;
      if (!grid.hidden) {
        wrap.setAttribute('data-dd-open', '');
        wrap._close = () => {
          grid.hidden = true;
          wrap.removeAttribute('data-dd-open');
        };
      }
    }
    input.addEventListener('input', () => show(input.value));
    input.addEventListener('click', () => show(input.value));
    grid.addEventListener('click', (e) => {
      const b = e.target.closest('[data-icon]');
      if (!b) return;
      const cls = b.dataset.icon;
      input.value = cls.split('fa-')[1];
      preview.innerHTML = `<i class="${esc(cls)}"></i>`;
      grid.hidden = true;
      wrap.removeAttribute('data-dd-open');
      onPick(cls);
    });
  }

  // ── Modal crear / editar ───────────────────────────────────────────
  function openModal(category) {
    const editing = !!category;
    let icon = editing ? category.icon : '';
    const m = ui.modal(`
      <div class="modal-card modal-card--2xl catalog-modal catalog-modal--visible">
        <div class="catalog-modal__body catalog-modal__body--roomy">
          ${ui.modalHeader(editing ? 'Editar categoría' : 'Agregar nueva categoría')}
          <div class="catalog-modal__field">
            <label class="label-normal field__label">Nombre de la categoría <span class="req">*</span></label>
            <input type="text" name="name" value="${esc(editing ? category.name : '')}" placeholder="Introduzca el nombre de la categoría" maxlength="150"
                   class="input catalog-modal__input" />
          </div>
          <div class="catalog-modal__field">
            <label class="label-normal field__label">Ícono para la categoría</label>
            <div data-icon-picker></div>
          </div>
        </div>
        <div class="modal-footer">
          <button type="button" data-close class="btn btn--soft">Cancelar</button>
          <button type="button" data-save disabled class="btn btn--primary">${editing ? 'Guardar cambios' : 'Agregar categoría'}</button>
        </div>
      </div>`);
    const name = m.el.querySelector('[name=name]');
    const save = m.el.querySelector('[data-save]');
    const sync = () => (save.disabled = !name.value.trim());
    name.addEventListener('input', sync);
    sync();
    iconPicker(m.el.querySelector('[data-icon-picker]'), icon, (cls) => (icon = cls));

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        const payload = { name: name.value.trim(), icon };
        try {
          if (editing) await Hook.put(`${endpoint}/${category.id}`, payload);
          else await Hook.post(endpoint, payload);
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, editing ? 'Error al actualizar categoría' : 'Error al crear categoría');
        }
      })
    );
  }

  function openDelete(category) {
    ui.confirmDelete({
      title: 'Eliminar categoría',
      message: '¿Está seguro de que desea eliminar la categoría',
      name: category.name,
      onConfirm: async () => {
        await Hook.del(`${endpoint}/${category.id}`);
        state.rows = state.rows.filter((r) => r.id !== category.id);
        state.refresh();
        ui.toast('Cambios guardados');
      },
    });
  }

  document.getElementById('addBtn').addEventListener('click', () => openModal(null));
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();
