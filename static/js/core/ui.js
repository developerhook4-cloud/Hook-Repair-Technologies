/*
  Hook Repair — componentes de interfaz reutilizables (JS sin frameworks).

  Bloques que se repiten en todas las páginas:
  toast de "Cambios guardados", modales, modal de "Eliminar", selects
  personalizados (dropdowns), paginación y tablas con columnas ordenables
  y redimensionables. Cada página (static/js/<sección>/<página>.js) los arma.
*/
(function () {
  'use strict';

  const Hook = (window.Hook = window.Hook || {});
  const ui = (Hook.ui = {});

  // ── Utilidades ─────────────────────────────────────────────────────

  /** Escapa texto para meterlo en HTML. */
  ui.esc = function (v) {
    if (v === null || v === undefined) return '';
    return String(v)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  };
  const esc = ui.esc;

  /** Ícono de Font Awesome (sólido por defecto). */
  ui.icon = (name, cls = '') => `<i class="fa-solid fa-${name} ${cls}" aria-hidden="true"></i>`;

  /** Formato de moneda: $1,234.50 */
  ui.money = function (n) {
    const v = Number(n || 0);
    return '$' + v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  };

  /** Número con 2 decimales (pipe number: '1.2-2'). */
  ui.number2 = (n) => Number(n || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

  /** Busca un elemento: ui.$('#id') / ui.$('.x', scope). */
  ui.$ = (sel, scope = document) => scope.querySelector(sel);
  ui.$$ = (sel, scope = document) => Array.from(scope.querySelectorAll(sel));

  /** Crea un elemento a partir de HTML. */
  ui.el = function (html) {
    const t = document.createElement('template');
    t.innerHTML = html.trim();
    return t.content.firstElementChild;
  };

  /** Normaliza texto para búsquedas (sin mayúsculas). */
  ui.norm = (v) => (v === null || v === undefined ? '' : String(v).toLowerCase());

  // ── Toast ───────────────────────────────────────────────────────────

  let toastTimer = null;
  /** Aviso flotante abajo al centro ("Cambios guardados"). */
  ui.toast = function (message = 'Cambios guardados', type = 'success') {
    let t = document.getElementById('hook-toast');
    if (!t) {
      t = ui.el(
        `<div id="hook-toast" role="status" aria-live="polite"
              class="fixed bottom-12 left-1/2 -translate-x-1/2 text-white px-8 py-3 rounded-full shadow-2xl flex items-center justify-center gap-3 z-100 transition-opacity duration-300 opacity-0 pointer-events-none"></div>`
      );
      document.body.appendChild(t);
    }
    t.classList.remove('bg-brand-primary', 'bg-error-darker', 'bg-dark-1');
    t.classList.add(type === 'error' ? 'bg-error-darker' : 'bg-brand-primary');
    t.innerHTML = `${type === 'error' ? ui.icon('circle-exclamation') : ''}<span class="label-normal font-bold">${esc(message)}</span>`;
    requestAnimationFrame(() => t.classList.replace('opacity-0', 'opacity-100'));
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => t.classList.replace('opacity-100', 'opacity-0'), type === 'error' ? 4500 : 3000);
  };

  /** Muestra el error de una llamada a la API. */
  ui.error = function (err, fallback = 'Ocurrió un error.') {
    console.error(err);
    ui.toast((err && err.message) || fallback, 'error');
  };

  // ── Modales ─────────────────────────────────────────────────────────

  const openModals = [];

  /**
   * Abre un modal. `content` es el HTML de la tarjeta interior.
   * opts.overlayClass permite cambiar el fondo; opts.onClose se llama al cerrar.
   * Devuelve { el, card, close }.
   */
  ui.modal = function (content, opts = {}) {
    const overlay = ui.el(
      `<div class="fixed inset-0 ${opts.z || 'z-50'} flex items-center justify-center ${opts.overlayClass || 'bg-gray-500/20'} backdrop-blur-sm p-4 md:p-6 transition-all duration-300" role="dialog" aria-modal="true"></div>`
    );
    overlay.innerHTML = content;
    document.body.appendChild(overlay);
    document.body.classList.add('overflow-hidden');

    const api = {
      el: overlay,
      card: overlay.firstElementChild,
      close() {
        const i = openModals.indexOf(api);
        if (i >= 0) openModals.splice(i, 1);
        overlay.remove();
        if (!openModals.length) document.body.classList.remove('overflow-hidden');
        if (opts.onClose) opts.onClose();
      },
    };
    openModals.push(api);
    overlay.addEventListener('click', (e) => {
      if (e.target.closest('[data-close]')) api.close();
    });
    const first = overlay.querySelector('input:not([readonly]):not([type=checkbox]), textarea');
    if (first && opts.autofocus !== false) setTimeout(() => first.focus(), 30);
    return api;
  };

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && openModals.length) {
      if (document.querySelector('[data-dd-open]')) return closeAllDropdowns();
      openModals[openModals.length - 1].close();
    }
  });

  /** Encabezado estándar de los modales (título + botón de cerrar). */
  ui.modalHeader = (title, extra = '') => `
    <div class="flex justify-between items-start mb-6">
      <div class="flex items-center gap-3"><h2 class="heading-5 text-dark-1">${esc(title)}</h2>${extra}</div>
      <button type="button" data-close class="text-gray-400 hover:text-dark-1 transition-colors" aria-label="Cerrar">${ui.icon('xmark', 'text-xl')}</button>
    </div>`;

  /**
   * Modal de "Eliminar …" (triángulo rojo, Cancelar / Sí, eliminar).
   * onConfirm puede ser async; si falla, el modal se queda abierto.
   */
  ui.confirmDelete = function ({ title, message, name, confirmText = 'Sí, eliminar', onConfirm }) {
    const m = ui.modal(`
      <div class="bg-white rounded-2xl shadow-2xl w-full max-w-100 p-8 flex flex-col items-center text-center border border-gray-100 animate-fade-in">
        <div class="h-16 w-16 rounded-full bg-error-subtle text-brand-primary flex items-center justify-center mb-6">${ui.icon('triangle-exclamation', 'text-3xl')}</div>
        <h3 class="heading-5 mb-3">${esc(title)}</h3>
        <p class="body-normal text-dark-2 mb-8">${esc(message)} <strong class="text-dark-1">“${esc(name)}”</strong>?</p>
        <div class="flex justify-center gap-4 w-full">
          <button type="button" data-close class="flex-1 py-3 rounded-2xl bg-gray-100 text-dark-2 label-normal hover:bg-gray-200 transition-colors">Cancelar</button>
          <button type="button" data-confirm class="flex-1 py-3 rounded-2xl bg-brand-primary text-white label-normal hover:opacity-90 transition-opacity shadow-sm disabled:opacity-50">${esc(confirmText)}</button>
        </div>
      </div>`);
    const btn = m.el.querySelector('[data-confirm]');
    btn.addEventListener('click', async () => {
      btn.disabled = true;
      try {
        await onConfirm();
        m.close();
      } catch (err) {
        btn.disabled = false;
        ui.error(err);
      }
    });
    return m;
  };

  /** Pone un botón en "cargando" mientras corre fn (evita dobles envíos). */
  ui.busy = async function (button, fn) {
    if (button.dataset.busy) return;
    button.dataset.busy = '1';
    const wasDisabled = button.disabled;
    button.disabled = true;
    try {
      return await fn();
    } finally {
      delete button.dataset.busy;
      button.disabled = wasDisabled;
    }
  };

  // ── Dropdown (select personalizado) ─────────────────────────────────

  function closeAllDropdowns(except) {
    document.querySelectorAll('[data-dd-open]').forEach((dd) => {
      if (dd !== except) dd._close();
    });
  }
  ui.closeAllDropdowns = closeAllDropdowns;

  document.addEventListener('click', (e) => {
    const dd = e.target.closest('[data-dd]');
    closeAllDropdowns(dd || undefined);
  });

  const DD_BUTTON = {
    outline:
      'w-full bg-white border border-gray-300 px-4 py-2.5 rounded-[14px] body-normal text-dark-1 flex items-center justify-between gap-2 hover:border-brand-primary transition-colors focus:outline-none focus:border-brand-primary text-left disabled:opacity-50 disabled:bg-gray-50 disabled:cursor-not-allowed',
    filled:
      'w-full bg-gray-50 border border-gray-200 px-4 py-2.5 rounded-[14px] body-normal text-dark-1 flex items-center justify-between gap-2 hover:border-brand-primary transition-colors focus:outline-none focus:border-brand-primary text-left',
    solid:
      'w-full bg-brand-primary text-white px-5 py-2.5 rounded-lg label-normal font-bold flex items-center justify-between gap-2 hover:opacity-90 transition-opacity shadow-sm',
  };

  /**
   * Convierte `container` en un select personalizado.
   *  items:       [{ value, label }]
   *  value:       valor seleccionado
   *  placeholder: texto cuando no hay valor
   *  none:        texto de la opción "Ninguna" (null para no mostrarla)
   *  up:          abre la lista hacia arriba
   *  variant:     'outline' | 'filled' | 'solid'
   *  onChange(value, item)
   * Devuelve { value, setValue, setItems, setDisabled }.
   */
  ui.dropdown = function (container, opts) {
    const o = Object.assign({ items: [], value: null, placeholder: 'Seleccionar', none: null, up: false, variant: 'outline' }, opts);
    container.classList.add('relative', 'w-full');
    container.setAttribute('data-dd', '');
    container.innerHTML = `
      <button type="button" class="${DD_BUTTON[o.variant] || DD_BUTTON.outline}" aria-haspopup="listbox" aria-expanded="false">
        <span class="truncate" data-label></span>
        ${ui.icon('chevron-down', 'text-sm transition-transform duration-200 shrink-0 ' + (o.variant === 'solid' ? '' : 'text-gray-400'))}
      </button>
      <div class="absolute ${o.up ? 'bottom-full mb-1' : 'top-full mt-1'} left-0 w-full bg-white rounded-xl shadow-[0_4px_20px_rgba(0,0,0,0.1)] border border-gray-100 z-60 p-1.5 max-h-48 overflow-y-auto" hidden>
        <ul class="flex flex-col text-sm font-semibold" role="listbox"></ul>
      </div>`;
    const btn = container.querySelector('button');
    const label = container.querySelector('[data-label]');
    const panel = container.querySelector('div');
    const list = container.querySelector('ul');
    const chevron = btn.querySelector('i');

    const api = {
      get value() {
        return o.value;
      },
      setValue(v, silent = true) {
        o.value = v === undefined ? null : v;
        paint();
        if (!silent && o.onChange) o.onChange(o.value, current());
      },
      setItems(items) {
        o.items = items || [];
        paint();
      },
      setDisabled(d) {
        btn.disabled = !!d;
      },
      setPlaceholder(p) {
        o.placeholder = p;
        paint();
      },
    };

    function current() {
      return o.items.find((i) => String(i.value) === String(o.value)) || null;
    }
    function paint() {
      const c = current();
      label.textContent = c ? c.label : o.placeholder;
      label.classList.toggle('text-gray-400', !c && o.variant !== 'solid');
      let html = '';
      if (o.none !== null) {
        html += `<li data-v="" class="px-4 py-2.5 rounded-md cursor-pointer transition-colors text-gray-400 hover:bg-gray-50">${esc(o.none)}</li>`;
      }
      html += o.items
        .map((i) => {
          const sel = String(i.value) === String(o.value);
          return `<li data-v="${esc(i.value)}" role="option" aria-selected="${sel}" class="px-4 py-2.5 rounded-md cursor-pointer transition-colors ${
            sel ? 'text-brand-primary bg-red-50' : 'text-dark-2 hover:bg-red-50 hover:text-brand-primary'
          }">${esc(i.label)}</li>`;
        })
        .join('');
      if (!o.items.length && o.none === null) {
        html = `<li class="px-4 py-2.5 text-gray-400 font-normal">Sin opciones</li>`;
      }
      list.innerHTML = html;
    }
    function open() {
      closeAllDropdowns(container);
      panel.hidden = false;
      container.setAttribute('data-dd-open', '');
      btn.setAttribute('aria-expanded', 'true');
      chevron.classList.add('rotate-180');
    }
    container._close = function () {
      panel.hidden = true;
      container.removeAttribute('data-dd-open');
      btn.setAttribute('aria-expanded', 'false');
      chevron.classList.remove('rotate-180');
    };
    btn.addEventListener('click', () => (panel.hidden ? open() : container._close()));
    list.addEventListener('click', (e) => {
      const li = e.target.closest('li[data-v]');
      if (!li) return;
      const v = li.dataset.v === '' ? null : li.dataset.v;
      container._close();
      api.setValue(v, false);
    });

    paint();
    return api;
  };

  // ── Paginación ──────────────────────────────────────────────────────

  /**
   * Pinta la paginación en `container`.
   *  layout: 'end' (pegada a la derecha) | 'center' (centrada, "Ir a pág." a la derecha)
   *  goTo:   mostrar "Ir a pág."
   */
  ui.pagination = function (container, { page, total, onChange, layout = 'end', goTo = true }) {
    const pages = Array.from({ length: total }, (_, i) => i + 1)
      .map(
        (p) =>
          `<button type="button" data-page="${p}" class="w-6 h-6 flex items-center justify-center rounded-full transition-colors label-normal ${
            p === page ? 'bg-brand-primary text-white' : 'hover:text-brand-primary'
          }">${p}</button>`
      )
      .join('');
    const pager = `
      <div class="flex items-center gap-3 bg-white px-4 py-2 rounded-full shadow-[0_2px_8px_rgba(0,0,0,0.08)]">
        <button type="button" data-page="${page - 1}" ${page === 1 ? 'disabled' : ''} class="text-gray-400 hover:text-dark-1 disabled:opacity-50 disabled:cursor-not-allowed transition-colors" aria-label="Anterior">${ui.icon('chevron-left')}</button>
        <div class="flex items-center gap-3 flex-wrap justify-center">${pages}</div>
        <button type="button" data-page="${page + 1}" ${page === total ? 'disabled' : ''} class="text-dark-1 hover:text-brand-primary disabled:opacity-50 disabled:cursor-not-allowed transition-colors" aria-label="Siguiente">${ui.icon('chevron-right')}</button>
      </div>`;
    const goto = goTo
      ? `<div class="hidden md:flex items-center gap-3 ${layout === 'center' ? 'absolute right-0' : ''}">
           <span>Ir a pág.</span>
           <input type="number" min="1" max="${total}" value="${page}" data-goto class="no-spin bg-gray-100 rounded-lg w-9 h-9 text-center p-0 text-gray-600 focus:outline-none focus:ring-1 focus:ring-brand-primary" />
         </div>`
      : '';
    container.className =
      layout === 'center'
        ? 'mt-auto relative flex justify-center items-center w-full body-normal font-semibold text-dark-2'
        : 'mt-auto pt-2 flex justify-between md:justify-end items-center gap-12 w-full body-normal font-semibold text-dark-2';
    container.innerHTML = pager + goto;
    container.onclick = (e) => {
      const b = e.target.closest('button[data-page]');
      if (!b || b.disabled) return;
      const p = Number(b.dataset.page);
      if (p >= 1 && p <= total) onChange(p);
    };
    const input = container.querySelector('[data-goto]');
    if (input) {
      input.onchange = () => {
        const p = Number(input.value);
        if (p >= 1 && p <= total) onChange(p);
        else input.value = page;
      };
    }
  };

  // ── Estado de listas (búsqueda + orden + páginas) ───────────────────

  /**
   * Estado de todas las listas: filtrar por texto,
   * ordenar por columna y paginar.
   *  perPage, search(row, term) → bool, sortValue(row, key) → valor, render(state)
   */
  ui.listState = function ({ perPage = 5, search, sortValue, filters, render }) {
    const s = {
      rows: [],
      term: '',
      sortKey: '',
      sortDir: 'asc',
      page: 1,
      perPage,
      get processed() {
        let out = this.rows;
        if (this.term && search) {
          const t = this.term.toLowerCase();
          out = out.filter((r) => search(r, t));
        }
        if (filters) out = out.filter(filters);
        if (this.sortKey) {
          const k = this.sortKey;
          const dir = this.sortDir === 'asc' ? 1 : -1;
          const val = (r) => {
            let v = sortValue ? sortValue(r, k) : r[k];
            if (typeof v === 'string') v = v.toLowerCase();
            return v === null || v === undefined ? '' : v;
          };
          out = [...out].sort((a, b) => (val(a) < val(b) ? -dir : val(a) > val(b) ? dir : 0));
        }
        return out;
      },
      get totalPages() {
        return Math.ceil(this.processed.length / this.perPage) || 1;
      },
      get pageRows() {
        if (this.page > this.totalPages) this.page = this.totalPages;
        const start = (this.page - 1) * this.perPage;
        return this.processed.slice(start, start + this.perPage);
      },
      set(rows) {
        this.rows = rows || [];
        render(this);
      },
      setTerm(t) {
        this.term = t;
        this.page = 1;
        render(this);
      },
      sortBy(key) {
        if (this.sortKey === key) this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
        else {
          this.sortKey = key;
          this.sortDir = 'asc';
        }
        render(this);
      },
      goTo(p) {
        if (p >= 1 && p <= this.totalPages) {
          this.page = p;
          render(this);
        }
      },
      refresh() {
        render(this);
      },
    };
    return s;
  };

  /** Conecta un <input> de búsqueda a un listState. */
  ui.bindSearch = function (input, state) {
    if (!input) return;
    input.addEventListener('input', () => state.setTerm(input.value.trim()));
  };

  // ── Tabla con columnas ordenables y redimensionables ────────────────

  /**
   * Tabla de Servicios / Clientes / Productos.
   *  columns: [{ key, label, width, sort (clave o false), render(row) → HTML,
   *              cellClass, title(row) }]
   *  actions(row) → HTML de la columna Acciones (opcional)
   *  emptyText, minWidth (de cada columna al arrastrar)
   *  state: listState (para el ícono de orden)
   */
  ui.table = function (container, opts) {
    const o = Object.assign({ emptyText: 'No se encontraron registros.', minWidth: 60, actionsLabel: 'Acciones' }, opts);
    const widths = o.columns.map((c) => c.width || 150);
    let lastRows = [];

    container.className = 'w-full border border-gray-100 rounded-[20px] overflow-x-auto shadow-sm mb-6';

    function headerCell(c, i) {
      const sorted = o.state && o.state.sortKey === c.sort;
      const arrow = c.sort
        ? sorted
          ? ui.icon(o.state.sortDir === 'asc' ? 'arrow-up' : 'arrow-down', 'text-brand-primary text-xs')
          : ui.icon('arrow-up', 'text-gray-400 text-xs')
        : '';
      return `
        <th class="relative py-4 px-6 text-center select-none group" style="width:${widths[i]}px;min-width:${widths[i]}px">
          <div class="flex items-center justify-center gap-2 w-full whitespace-nowrap ${c.sort ? 'cursor-pointer transition-colors hover:text-brand-primary' : ''}" ${c.sort ? `data-sort="${c.sort}"` : ''}>
            ${esc(c.label)} ${arrow}
          </div>
          <div class="absolute right-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-gray-400 opacity-0 group-hover:opacity-100 transition-opacity z-10" data-resize="${i}"></div>
        </th>`;
    }

    function render(rows) {
      lastRows = rows;
      const total = widths.reduce((a, b) => a + b, 0) + (o.actions ? 120 : 0);
      const head = o.columns.map(headerCell).join('') + (o.actions ? `<th class="py-4 px-6 text-center whitespace-nowrap min-w-30 w-30">${esc(o.actionsLabel)}</th>` : '');
      const colspan = o.columns.length + (o.actions ? 1 : 0);
      const body = rows.length
        ? rows
            .map((r, ri) => {
              const cells = o.columns
                .map((c, i) => {
                  const content = c.render ? c.render(r) : esc(r[c.key]);
                  const title = c.title ? ` title="${esc(c.title(r))}"` : '';
                  return `<td class="py-4 px-6 text-center truncate ${c.cellClass || ''}" style="width:${widths[i]}px;min-width:${widths[i]}px;max-width:${widths[i]}px"${title}>${content}</td>`;
                })
                .join('');
              const actions = o.actions ? `<td class="py-4 px-6 min-w-30 w-30"><div class="flex items-center justify-center gap-4 text-gray-400">${o.actions(r)}</div></td>` : '';
              return `<tr class="hover:bg-gray-50 transition-colors ${ri < rows.length - 1 ? 'border-b border-gray-100' : ''}" data-row="${ri}">${cells}${actions}</tr>`;
            })
            .join('')
        : `<tr><td colspan="${colspan}" class="py-12 text-center text-gray-400 body-normal">${esc(o.emptyText)}</td></tr>`;

      container.innerHTML = `
        <table class="w-full text-left border-collapse table-fixed" style="min-width:${total}px">
          <thead class="bg-gray-100 text-dark-2 label-normal border-b border-gray-200"><tr>${head}</tr></thead>
          <tbody class="body-normal text-dark-1">${body}</tbody>
        </table>`;
    }

    // Ordenar
    container.addEventListener('click', (e) => {
      const s = e.target.closest('[data-sort]');
      if (s && o.state) {
        o.state.sortBy(s.dataset.sort);
        return;
      }
      const btn = e.target.closest('[data-action]');
      if (btn && o.onAction) {
        const tr = btn.closest('tr[data-row]');
        if (tr) o.onAction(btn.dataset.action, lastRows[Number(tr.dataset.row)], btn);
      }
    });

    // Redimensionar (arrastrar el borde) y autoajustar (doble clic)
    container.addEventListener('mousedown', (e) => {
      const h = e.target.closest('[data-resize]');
      if (!h) return;
      e.preventDefault();
      e.stopPropagation();
      const i = Number(h.dataset.resize);
      const startX = e.pageX;
      const startW = widths[i];
      const move = (ev) => {
        const w = Math.max(o.minWidth, startW + (ev.pageX - startX));
        if (w !== widths[i]) {
          widths[i] = w;
          render(lastRows);
        }
      };
      const up = () => {
        document.removeEventListener('mousemove', move);
        document.removeEventListener('mouseup', up);
      };
      document.addEventListener('mousemove', move);
      document.addEventListener('mouseup', up);
    });
    container.addEventListener('dblclick', (e) => {
      const h = e.target.closest('[data-resize]');
      if (!h) return;
      const i = Number(h.dataset.resize);
      const c = o.columns[i];
      const all = o.state ? o.state.processed : lastRows;
      let max = String(c.label).length;
      all.forEach((r) => {
        const v = c.text ? c.text(r) : r[c.key];
        const len = v === null || v === undefined ? 0 : String(v).length;
        if (len > max) max = len;
      });
      widths[i] = Math.max(80, max * 9 + 65);
      render(lastRows);
    });

    return { render };
  };

  /**
   * Lista de tarjetas (Categorías, Marcas, Modelos, Roles): pinta cada renglón
   * con rowHtml(row, isLast) y avisa los clics en [data-action] con onAction.
   */
  ui.cardList = function (container, { rowHtml, emptyText = 'No se encontraron registros.', onAction }) {
    let rows = [];
    container.addEventListener('click', (e) => {
      const btn = e.target.closest('[data-action]');
      const item = e.target.closest('[data-i]');
      if (btn && item && onAction) onAction(btn.dataset.action, rows[Number(item.dataset.i)], btn);
    });
    return {
      render(list) {
        rows = list;
        container.innerHTML = list.length
          ? list.map((r, i) => rowHtml(r, i === list.length - 1).replace('<div', `<div data-i="${i}"`)).join('')
          : `<div class="py-12 flex flex-col items-center justify-center text-gray-400"><p class="body-normal">${esc(emptyText)}</p></div>`;
      },
    };
  };

  /** Botones de editar / eliminar de las listas de tarjetas. */
  ui.cardButtons = (editHover = 'hover:text-brand-primary') => `
    <div class="flex items-center gap-2">
      <button type="button" data-action="edit" title="Editar" class="w-8 h-8 flex items-center justify-center text-gray-400 ${editHover} transition-colors">${ui.icon('pen')}</button>
      <button type="button" data-action="delete" title="Eliminar" class="w-8 h-8 flex items-center justify-center text-gray-400 hover:text-red-500 hover:bg-red-50 rounded-lg transition-colors">${ui.icon('trash-can')}</button>
    </div>`;

  /** Botones de acción de tabla (lápiz / basura / …). */
  ui.actionBtn = (action, iconName, title) =>
    `<button type="button" data-action="${action}" title="${esc(title)}" class="hover:text-brand-primary transition-colors">${ui.icon(iconName)}</button>`;

  /** Píldora de estado Activo / Inactivo. */
  ui.statusPill = (active) =>
    `<span class="px-4 py-1.5 rounded-full label-small inline-block whitespace-nowrap ${
      active ? 'bg-success-subtle text-success-darker' : 'bg-error-subtle text-error-primary'
    }">${active ? 'Activo' : 'Inactivo'}</span>`;

  // ── Formularios ─────────────────────────────────────────────────────

  /** Lee los campos [name] de un contenedor como objeto. */
  ui.formValues = function (scope) {
    const out = {};
    scope.querySelectorAll('[name]').forEach((f) => {
      if (f.type === 'checkbox') out[f.name] = f.checked;
      else out[f.name] = f.value;
    });
    return out;
  };

  /** Campo de texto con etiqueta (estilo de los modales). */
  ui.field = function ({ label, name, value = '', type = 'text', placeholder = '', required = false, readonly = false, span = '', inputClass = '', prefix = '', phone = false, hint = '' }) {
    const pad = prefix ? 'pl-8 pr-4' : 'px-4';
    const input = `<input type="${type}" name="${name}" value="${esc(value)}" placeholder="${esc(placeholder)}" ${readonly ? 'readonly' : ''} ${phone ? 'data-phone' : ''}
        class="w-full border border-gray-300 rounded-[14px] ${pad} py-2.5 focus:outline-none focus:border-brand-primary body-normal text-dark-1 ${readonly ? 'bg-gray-50' : ''} ${inputClass}" ${type === 'number' ? 'step="any"' : ''}/>`;
    return `
      <div class="flex flex-col gap-1.5 ${span}">
        <label class="label-normal text-dark-1">${esc(label)}${required ? ' <span class="text-brand-primary">*</span>' : ''}</label>
        ${prefix ? `<div class="relative w-full"><span class="absolute left-4 top-1/2 -translate-y-1/2 text-gray-400 body-normal">${esc(prefix)}</span>${input}</div>` : input}
        <small data-error-for="${name}" class="label-small text-error-primary" hidden></small>${hint ? `<span class="label-small text-gray-400">${esc(hint)}</span>` : ''}
      </div>`;
  };

  /** Contenedor con etiqueta para un dropdown. */
  ui.ddField = ({ label, id, required = false, span = '' }) => `
    <div class="flex flex-col gap-1.5 ${span}">
      <label class="label-normal text-dark-1">${esc(label)}${required ? ' <span class="text-brand-primary">*</span>' : ''}</label>
      <div id="${id}"></div>
    </div>`;

  /** Botón de ojito para mostrar/ocultar contraseñas. */
  ui.passwordToggles = function (scope = document) {
    scope.querySelectorAll('[data-toggle-password]').forEach((btn) => {
      btn.addEventListener('click', () => {
        const input = btn.parentElement.querySelector('input');
        const show = input.type === 'password';
        input.type = show ? 'text' : 'password';
        btn.innerHTML = ui.icon(show ? 'eye' : 'eye-slash', 'text-lg');
      });
    });
  };

  /**
   * Validación de formularios (errores bajo cada campo cuando
   * ya se tocó). rules: { campo: [[fn(valor, valores) → bool, 'mensaje'], ...] }
   * onChange(valid) se llama cada vez que cambia algo (p. ej. para el botón).
   */
  ui.validator = function (form, rules, onChange) {
    const touched = {};
    const api = {
      values: () => ui.formValues(form),
      errors() {
        const vals = api.values();
        const errs = {};
        Object.keys(rules).forEach((k) => {
          for (const [fn, msg] of rules[k]) {
            if (!fn((vals[k] || '').trim ? (vals[k] || '').trim() : vals[k], vals)) {
              errs[k] = msg;
              break;
            }
          }
        });
        return errs;
      },
      validate(showAll = false) {
        if (showAll) Object.keys(rules).forEach((k) => (touched[k] = true));
        const errs = api.errors();
        Object.keys(rules).forEach((k) => {
          const el = form.querySelector(`[data-error-for="${k}"]`);
          if (!el) return;
          const show = touched[k] && errs[k];
          el.hidden = !show;
          el.textContent = show ? errs[k] : '';
        });
        const valid = Object.keys(errs).length === 0;
        if (onChange) onChange(valid);
        return valid;
      },
      reset() {
        Object.keys(touched).forEach((k) => delete touched[k]);
        api.validate();
      },
    };
    form.addEventListener('input', () => api.validate());
    form.addEventListener('focusout', (e) => {
      if (e.target.name && rules[e.target.name]) {
        touched[e.target.name] = true;
        api.validate();
      }
    });
    return api;
  };

  // ── Teléfonos, contraseñas y RFC (mismas reglas que el servidor) ──

  /** Teléfono válido: solo números (+, espacios, guiones, paréntesis) y máx. 15 dígitos. */
  ui.isPhone = (v) => {
    const t = String(v || '').trim();
    if (!t) return true;
    const digits = t.replace(/\D/g, '').length;
    return /^[0-9+\-\s().]+$/.test(t) && digits >= 1 && digits <= 15;
  };

  /** Contraseña segura: 8+ caracteres, con al menos una letra y un número. */
  ui.passwordError = (v) => {
    const t = String(v || '');
    if (t.length < 8) return 'La contraseña debe tener al menos 8 caracteres.';
    if (!/[A-Za-zÁÉÍÓÚáéíóúÑñÜü]/.test(t) || !/[0-9]/.test(t)) return 'La contraseña debe tener al menos una letra y un número.';
    return '';
  };
  ui.PASSWORD_HINT = 'Mínimo 8 caracteres, con al menos una letra y un número.';

  /** RFC con formato del SAT: 12 (persona moral) o 13 (persona física) caracteres. */
  ui.isRFC = (v) => /^[A-ZÑ&]{3,4}[0-9]{6}[A-Z0-9]{3}$/.test(String(v || '').trim().toUpperCase());

  /**
   * Quita lo que no sea número en los campos de teléfono mientras se escribe.
   * Marca los inputs con data-phone (o pásalos directo).
   */
  ui.phoneInputs = function (scope = document) {
    scope.querySelectorAll('input[data-phone]').forEach((input) => {
      input.setAttribute('inputmode', 'tel');
      input.setAttribute('maxlength', '20');
      input.addEventListener('input', () => {
        const clean = input.value.replace(/[^0-9+\-\s().]/g, '');
        if (clean !== input.value) input.value = clean;
      });
    });
  };

  ui.rules = {
    required: [(v) => !!v, 'Este campo es obligatorio.'],
    email: [(v) => !v || /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(v), 'El formato debe ser válido.'],
    pattern: (re) => [(v) => !v || re.test(v), 'El formato ingresado no es válido.'],
    phone: [(v) => ui.isPhone(v), 'Solo números, máximo 15 dígitos.'],
    password: [(v) => !v || !ui.passwordError(v), 'Mínimo 8 caracteres, con al menos una letra y un número.'],
  };

  // ── Alta rápida desde otro formulario ───────────────────────────────

  /**
   * Botón "+ Nueva" para la etiqueta de un campo: crea el registro sin salir
   * del formulario actual (categoría, marca, modelo, proveedor…).
   */
  ui.quickAddBtn = (key, text = 'Nueva') =>
    `<button type="button" data-quick-add="${key}" class="ml-auto label-small font-bold text-brand-primary hover:underline inline-flex items-center gap-1">${ui.icon('plus', 'text-[10px]')} ${esc(text)}</button>`;

  /**
   * Modal pequeño encima del actual para dar de alta un registro.
   *  title, fields: [{ name, label, type, placeholder, required, phone }]
   *  note: texto opcional arriba de los campos
   *  validate(values) → '' o mensaje de error
   *  save(values) → Promise con el registro creado ({ id, name, ... })
   *  onCreated(record)
   */
  ui.quickAdd = function ({ title, fields, note = '', validate, save, onCreated }) {
    const m = ui.modal(
      `<div class="bg-white rounded-2xl shadow-2xl w-full max-w-md flex flex-col border border-gray-100 overflow-visible animate-fade-in">
        <div class="p-6 md:p-8 pb-2">
          ${ui.modalHeader(title)}
          ${note ? `<p class="body-normal text-dark-2 -mt-3 mb-4">${esc(note)}</p>` : ''}
          <div class="flex flex-col gap-4" data-form>
            ${fields
              .map(
                (f) => `<div class="flex flex-col gap-1.5">
                  <label class="label-normal text-dark-1">${esc(f.label)}${f.required ? ' <span class="text-brand-primary">*</span>' : ''}</label>
                  <input type="${f.type || 'text'}" name="${f.name}" placeholder="${esc(f.placeholder || '')}" ${f.phone ? 'data-phone' : ''} autocomplete="off"
                         class="w-full border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal text-dark-1" />
                </div>`
              )
              .join('')}
            <small data-error class="label-small text-error-primary" hidden></small>
          </div>
        </div>
        <div class="p-6 md:px-8 flex justify-end gap-3 mt-2 border-t border-gray-100 rounded-b-2xl">
          <button type="button" data-close class="px-5 py-2.5 rounded-lg bg-gray-200 text-dark-2 label-normal font-bold hover:bg-gray-300 transition-colors">Cancelar</button>
          <button type="button" data-save disabled class="px-5 py-2.5 rounded-lg bg-brand-primary text-white label-normal font-bold hover:opacity-90 transition-opacity shadow-sm disabled:opacity-50 disabled:cursor-not-allowed">Crear</button>
        </div>
      </div>`,
      { z: 'z-80', overlayClass: 'bg-gray-500/30' }
    );
    const form = m.el.querySelector('[data-form]');
    const btn = m.el.querySelector('[data-save]');
    const errEl = m.el.querySelector('[data-error]');
    ui.phoneInputs(form);
    const values = () => {
      const v = ui.formValues(form);
      Object.keys(v).forEach((k) => (v[k] = String(v[k]).trim()));
      return v;
    };
    const check = () => {
      const v = values();
      const missing = fields.some((f) => f.required && !v[f.name]);
      const msg = missing ? '' : validate ? validate(v) : '';
      errEl.hidden = !msg;
      errEl.textContent = msg || '';
      btn.disabled = missing || !!msg;
    };
    form.addEventListener('input', check);
    form.addEventListener('keydown', (e) => e.key === 'Enter' && !btn.disabled && btn.click());
    check();
    btn.addEventListener('click', () =>
      ui.busy(btn, async () => {
        try {
          const rec = await save(values());
          m.close();
          ui.toast('Registro creado');
          if (onCreated) onCreated(rec);
        } catch (err) {
          ui.error(err, 'No se pudo crear el registro.');
        }
      })
    );
    return m;
  };

  /**
   * Descarga un CSV (se abre bien en Excel: UTF-8 con BOM).
   * columns: [{ label, value(row) }]
   */
  ui.downloadCSV = function (filename, columns, rows) {
    const cell = (v) => {
      const s = v === null || v === undefined ? '' : String(v);
      return /[",\n;]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
    };
    const lines = [columns.map((c) => cell(c.label)).join(',')].concat(rows.map((r) => columns.map((c) => cell(c.value(r))).join(',')));
    const blob = new Blob(['﻿' + lines.join('\r\n')], { type: 'text/csv;charset=utf-8' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    setTimeout(() => {
      URL.revokeObjectURL(a.href);
      a.remove();
    }, 0);
  };

  /** Fecha ISO → "04 nov 2026" (zona local). */
  ui.date = function (iso) {
    if (!iso) return '';
    const d = new Date(iso);
    if (isNaN(d)) return String(iso);
    return d.toLocaleDateString('es-MX', { day: '2-digit', month: 'short', year: 'numeric' });
  };

  /** Fecha y hora ISO → "04 nov 2026, 14:30". */
  ui.dateTime = function (iso) {
    if (!iso) return '';
    const d = new Date(iso);
    if (isNaN(d)) return String(iso);
    return d.toLocaleString('es-MX', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
  };
})();