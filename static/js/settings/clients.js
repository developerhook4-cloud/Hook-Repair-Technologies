/* Configuración → Terceros → Clientes (/api/:org/clients). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;
  const esc = ui.esc;
  const endpoint = Hook.org('clients');

  const CUSTOMER_TYPES = [
    { value: '0', label: 'Cliente' },
    { value: '1', label: 'Distribuidor' },
  ];
  const STATES = ['Sonora', 'Sinaloa', 'Baja California', 'Chihuahua', 'Nuevo León', 'Jalisco', 'Ciudad de México'];
  const TAX_REGIMES = [
    'General de Ley Personas Morales',
    'Personas Físicas con Actividades Empresariales',
    'Régimen Simplificado de Confianza (RESICO)',
  ];
  const EXPENSE_TYPES = ['Gastos en general', 'Adquisición de mercancías', 'Honorarios médicos'];
  const toItems = (arr) => arr.map((v) => ({ value: v, label: v }));

  const typePill = (t) =>
    `<span class="px-4 py-1.5 rounded-full label-small font-bold inline-block whitespace-nowrap ${
      t === 1 ? 'bg-blue-100 text-blue-500' : 'bg-orange-100 text-orange-500'
    }">${t === 1 ? 'Distribuidor' : 'Cliente'}</span>`;

  const state = ui.listState({
    perPage: 5,
    search: (c, t) => ui.norm(c.id).includes(t) || ui.norm(c.name).includes(t) || ui.norm(c.phone1).includes(t) || ui.norm(c.email).includes(t),
    sortValue: (c, k) => (k === 'type' ? c.customerType : c[k]),
    render,
  });

  const table = ui.table(document.getElementById('table'), {
    state,
    emptyText: 'No se encontraron clientes.',
    columns: [
      { key: 'id', label: 'ID', width: 80, sort: 'id', render: (c) => esc(c.id.slice(0, 8)), title: (c) => c.id },
      { key: 'name', label: 'Nombre', width: 250, sort: 'name', title: (c) => c.name },
      { key: 'phone1', label: 'Teléfono', width: 180 },
      { key: 'customerType', label: 'Tipo de cliente', width: 180, sort: 'type', render: (c) => typePill(c.customerType), text: (c) => (c.customerType === 1 ? 'Distribuidor' : 'Cliente') },
      { key: 'active', label: 'Estado', width: 120, render: (c) => ui.statusPill(c.active), text: (c) => (c.active ? 'Activo' : 'Inactivo') },
    ],
    actions: () => ui.actionBtn('edit', 'pen', 'Editar') + ui.actionBtn('delete', 'trash-can', 'Eliminar') + ui.actionBtn('more', 'ellipsis', 'Más'),
    onAction: (action, c) => (action === 'edit' ? openModal(c) : action === 'delete' ? openDelete(c) : null),
  });

  function render() {
    table.render(state.pageRows);
    ui.pagination(document.getElementById('pager'), { page: state.page, total: state.totalPages, onChange: (p) => state.goTo(p), layout: 'center' });
  }

  async function load() {
    try {
      state.set(await Hook.get(endpoint));
    } catch (err) {
      ui.error(err, 'Error al cargar clientes.');
    }
  }

  const input = (label, name, value, placeholder, { required = false, small = false, type = 'text' } = {}) => `
    <div class="flex flex-col gap-1.5">
      <label class="${small ? 'label-small text-dark-2' : 'label-normal font-semibold text-dark-1'}">${esc(label)}${required ? ' <span class="text-brand-primary">*</span>' : ''}</label>
      <input type="${type}" name="${name}" value="${esc(value || '')}" placeholder="${esc(placeholder)}"
             class="w-full border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal text-dark-1 placeholder-gray-300" />
    </div>`;

  const dd = (label, id, small = false) => `
    <div class="flex flex-col gap-1.5">
      <label class="${small ? 'label-small text-dark-2' : 'label-normal text-dark-1'}">${esc(label)}</label>
      <div id="${id}"></div>
    </div>`;

  function openModal(client) {
    const editing = !!client;
    const c = client || {};
    const addr = c.address || { country: 'México' };
    const cur = {
      type: editing ? c.customerType : 0,
      taxRegime: c.taxRegime || null,
      expenseType: c.expenseType || null,
      state: addr.state || null,
      active: editing ? c.active : true,
    };

    const m = ui.modal(`
      <div data-card class="bg-white rounded-2xl shadow-2xl w-full flex flex-col max-h-[85vh] overflow-hidden border border-gray-100 transition-all duration-300 animate-fade-in">
        <div class="p-6 md:px-8 md:pt-8 pb-4 flex justify-between items-start shrink-0">
          <h2 class="heading-5 text-dark-1">${editing ? 'Editar cliente' : 'Agregar nuevo cliente'}</h2>
          <button type="button" data-close class="text-gray-400 hover:text-dark-1 transition-colors">${ui.icon('xmark', 'text-xl')}</button>
        </div>
        <div class="px-6 md:px-8 pb-6 overflow-y-auto flex-1" data-form>
          <div class="flex flex-col md:flex-row gap-10">
            <div class="flex flex-col gap-5 flex-1">
              ${input('Nombre', 'name', c.name, 'Introduzca el nombre del cliente', { required: true })}
              ${input('Número de teléfono', 'phone1', c.phone1, 'Introduzca el número de teléfono', { required: true })}
              <div class="flex flex-col gap-1.5">
                <label class="label-normal font-semibold text-dark-1">Tipo de cliente <span class="text-brand-primary">*</span></label>
                <div id="ddType"></div>
              </div>
              <div class="flex flex-col gap-3 mt-2">
                <label class="flex items-center gap-2 cursor-pointer label-normal text-dark-1">
                  <input type="checkbox" name="requiresInvoice" ${c.rfc || c.taxRegime ? 'checked' : ''} class="w-4 h-4 rounded border-gray-300 text-brand-primary focus:ring-brand-primary accent-brand-primary cursor-pointer" />
                  Requiere facturar
                </label>
                <div data-invoice class="flex flex-col gap-4 transition-all duration-300" ${c.rfc || c.taxRegime ? '' : 'hidden'}>
                  <div class="flex flex-col gap-1.5">
                    <label class="label-normal text-dark-1">RFC</label>
                    <input type="text" name="rfc" maxlength="13" value="${esc(c.rfc || '')}" placeholder="RFC del cliente" class="w-full border border-gray-300 rounded-[14px] px-4 py-2.5 focus:outline-none focus:border-brand-primary body-normal text-dark-1 placeholder-gray-300 uppercase" />
                  </div>
                  ${dd('Régimen fiscal', 'ddTax')}
                  ${dd('Tipo de gasto', 'ddExpense')}
                </div>
              </div>
              ${editing ? dd('Estado', 'ddStatus') : ''}
            </div>

            <div data-distributor class="flex-1 flex flex-col gap-6 animate-fade-in border-t md:border-t-0 md:border-l border-gray-100 md:pl-10 pt-6 md:pt-0" hidden>
              <div class="flex flex-col gap-4">
                <h3 class="label-normal font-bold text-dark-1">Datos de distribuidor</h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  ${input('Nombre comercial', 'legalName', c.legalName, 'Introduzca el nombre del local', { small: true })}
                  ${input('Celular', 'phone2', c.phone2, '+52 662 123 4567', { small: true })}
                  ${input('Nombre del titular', 'titularName', c.titularName, 'Introduzca el nombre del titular', { small: true })}
                  ${input('Email', 'email', c.email, 'cliente@email.com', { small: true, type: 'email' })}
                </div>
              </div>
              <div class="flex flex-col gap-4">
                <h3 class="label-normal font-bold text-dark-1">Dirección</h3>
                <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                  ${input('Dirección', 'address', addr.address, 'Av Nombre 100', { small: true })}
                  ${input('Código Postal', 'postCode', addr.postCode, '12345', { small: true })}
                  ${input('Ciudad', 'city', addr.city, 'Elegir ciudad', { small: true })}
                  ${dd('Estado', 'ddState', true)}
                  ${input('País', 'country', addr.country, 'México', { small: true })}
                </div>
              </div>
            </div>
          </div>
        </div>
        <div data-footer class="p-6 md:px-8 bg-white flex justify-between md:justify-end gap-3 shrink-0">
          <button type="button" data-close class="w-full md:w-auto px-6 py-2.5 rounded-[14px] bg-gray-100 text-dark-2 label-normal font-bold hover:bg-gray-200 transition-colors">Cancelar</button>
          <button type="button" data-save class="w-full md:w-auto px-6 py-2.5 rounded-[14px] bg-brand-primary text-white label-normal font-bold hover:opacity-90 transition-opacity shadow-sm disabled:opacity-50 disabled:cursor-not-allowed">${editing ? 'Guardar cambios' : 'Crear cliente'}</button>
        </div>
      </div>`);

    const card = m.el.querySelector('[data-card]');
    const form = m.el.querySelector('[data-form]');
    const save = m.el.querySelector('[data-save]');
    const distributor = m.el.querySelector('[data-distributor]');
    const invoiceBox = m.el.querySelector('[data-invoice]');

    const applyType = () => {
      const isDist = cur.type === 1;
      distributor.hidden = !isDist;
      card.classList.toggle('max-w-250', isDist);
      card.classList.toggle('max-w-112.5', !isDist);
    };

    ui.dropdown(m.el.querySelector('#ddType'), {
      items: CUSTOMER_TYPES,
      value: String(cur.type),
      placeholder: 'Seleccionar tipo de cliente',
      up: true,
      onChange: (v) => {
        cur.type = Number(v || 0);
        applyType();
      },
    });
    ui.dropdown(m.el.querySelector('#ddTax'), { items: toItems(TAX_REGIMES), value: cur.taxRegime, placeholder: 'Seleccionar régimen fiscal', up: true, onChange: (v) => (cur.taxRegime = v) });
    ui.dropdown(m.el.querySelector('#ddExpense'), { items: toItems(EXPENSE_TYPES), value: cur.expenseType, placeholder: 'Seleccionar tipo de gasto', up: true, onChange: (v) => (cur.expenseType = v) });
    ui.dropdown(m.el.querySelector('#ddState'), { items: toItems(STATES), value: cur.state, placeholder: 'Elegir estado', up: true, onChange: (v) => (cur.state = v) });
    if (editing) {
      ui.dropdown(m.el.querySelector('#ddStatus'), {
        items: [
          { value: 'true', label: 'Activo' },
          { value: 'false', label: 'Inactivo' },
        ],
        value: String(cur.active),
        up: true,
        onChange: (v) => (cur.active = v === 'true'),
      });
    }
    m.el.querySelector('[name=requiresInvoice]').addEventListener('change', (e) => (invoiceBox.hidden = !e.target.checked));
    applyType();

    const valid = () => {
      const f = ui.formValues(form);
      return f.name.trim() !== '' && f.phone1.trim() !== '';
    };
    form.addEventListener('input', () => (save.disabled = !valid()));
    save.disabled = !valid();

    save.addEventListener('click', () =>
      ui.busy(save, async () => {
        const f = ui.formValues(form);
        const invoice = f.requiresInvoice;
        const payload = {
          name: f.name.trim(),
          customerType: cur.type,
          phone1: f.phone1.trim(),
          phone2: f.phone2.trim() || null,
          legalName: f.legalName.trim() || null,
          email: f.email.trim() || null,
          address: {
            address: f.address.trim() || null,
            city: f.city.trim() || null,
            state: cur.state || null,
            postCode: f.postCode.trim() || null,
            country: f.country.trim() || null,
          },
          titularName: f.titularName.trim() || null,
          authorizedPeople: (client && client.authorizedPeople) || [],
          rfc: invoice ? f.rfc.trim().toUpperCase() || null : null,
          taxRegime: invoice ? cur.taxRegime : null,
          expenseType: invoice ? cur.expenseType : null,
        };
        try {
          if (editing) {
            await Hook.put(`${endpoint}/${client.id}`, payload);
            if (cur.active !== client.active) await Hook.patch(`${endpoint}/${client.id}/status`, { active: cur.active });
          } else {
            await Hook.post(endpoint, payload);
          }
          m.close();
          ui.toast('Cambios guardados');
          load();
        } catch (err) {
          ui.error(err, editing ? 'Error al actualizar cliente' : 'Error al crear cliente');
        }
      })
    );
  }

  function openDelete(client) {
    ui.confirmDelete({
      title: 'Eliminar cliente',
      message: '¿Está seguro de que desea eliminar al cliente',
      name: client.name,
      onConfirm: async () => {
        await Hook.patch(`${endpoint}/${client.id}/status`, { active: false });
        ui.toast('Cambios guardados');
        load();
      },
    });
  }

  document.getElementById('addBtn').addEventListener('click', () => openModal(null));
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();
