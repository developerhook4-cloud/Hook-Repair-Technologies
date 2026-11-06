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
      <div class="py-4 px-6 flex justify-between items-center transition-colors hover:bg-gray-50 ${last ? '' : 'border-b border-gray-200'}">
        <div class="flex items-center justify-items-start">
          <h4 class="label-normal font-bold text-dark-1 mb-1 pr-3">${esc(c.name)}</h4>
          ${c.icon ? `<i class="${esc(c.icon)} text-dark-1"></i>` : ''}
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
      <div class="w-full max-w-sm relative" data-dd>
        <div class="flex items-center gap-3">
          <input type="text" autocomplete="off" placeholder="Buscar ícono (ej. user, home, trash)..." value="${esc(parts.length > 1 ? parts[1] : '')}"
                 class="w-full bg-white border border-light-2 rounded-lg p-3 body-normal text-dark-1 outline-none focus:border-brand-primary transition-colors placeholder:text-dark-4" />
          <span data-preview class="w-10 h-10 shrink-0 rounded-lg bg-gray-50 border border-gray-100 flex items-center justify-center text-xl text-dark-1">${initial ? `<i class="${esc(initial)}"></i>` : ''}</span>
        </div>
        <div data-grid class="absolute bottom-full left-0 mb-2 w-full bg-white border border-gray-100 shadow-lg rounded-xl max-h-40 overflow-y-auto z-50 p-2 grid grid-cols-4 gap-2" hidden></div>
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
                    class="flex flex-col items-center justify-center p-3 rounded-lg hover:bg-brand-primary hover:text-white transition-colors text-gray-600">
                    <i class="${i.style} fa-${i.name} text-xl mb-1"></i>
                    <span class="text-[10px] truncate w-full text-center">${esc(i.name)}</span>
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
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-4xl flex flex-col max-h-[90vh] overflow-visible border border-gray-100 animate-fade-in">
        <div class="p-8 md:p-12 pb-4">
          ${ui.modalHeader(editing ? 'Editar categoría' : 'Agregar nueva categoría')}
          <div class="flex flex-col mb-6">
            <label class="label-normal font-semibold text-dark-1 mb-2">Nombre de la categoría <span class="text-brand-primary">*</span></label>
            <input type="text" name="name" value="${esc(editing ? category.name : '')}" placeholder="Introduzca el nombre de la categoría" maxlength="150"
                   class="w-full md:w-80 border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal placeholder-gray-400" />
          </div>
          <div class="flex flex-col mb-6">
            <label class="label-normal font-semibold text-dark-1 mb-2">Ícono para la categoría</label>
            <div data-icon-picker></div>
          </div>
        </div>
        <div class="p-6 md:px-8 border-t border-gray-100 bg-white flex justify-end gap-4 mt-auto rounded-b-2xl">
          <button type="button" data-close class="px-6 py-2.5 rounded-lg bg-gray-100 text-dark-2 font-bold label-normal hover:bg-gray-200 transition-colors">Cancelar</button>
          <button type="button" data-save disabled class="px-6 py-2.5 rounded-lg bg-brand-primary text-white font-bold label-normal transition-all shadow-sm hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed">${editing ? 'Guardar cambios' : 'Agregar categoría'}</button>
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