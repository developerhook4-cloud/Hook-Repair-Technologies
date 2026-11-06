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
  // Catálogos del SAT (CFDI 4.0)
  const TAX_REGIMES = [
    '601 - General de Ley Personas Morales',
    '603 - Personas Morales con Fines no Lucrativos',
    '605 - Sueldos y Salarios e Ingresos Asimilados a Salarios',
    '606 - Arrendamiento',
    '608 - Demás ingresos',
    '612 - Personas Físicas con Actividades Empresariales y Profesionales',
    '616 - Sin obligaciones fiscales',
    '621 - Incorporación Fiscal',
    '625 - Actividades Empresariales con ingresos a través de Plataformas Tecnológicas',
    '626 - Régimen Simplificado de Confianza (RESICO)',
  ];
  const EXPENSE_TYPES = [
    'G01 - Adquisición de mercancías',
    'G02 - Devoluciones, descuentos o bonificaciones',
    'G03 - Gastos en general',
    'I04 - Equipo de cómputo y accesorios',
    'I08 - Otra maquinaria y equipo',
    'S01 - Sin efectos fiscales',
    'CP01 - Pagos',
  ];
  const toItems = (arr, current) => (current && !arr.includes(current) ? [current, ...arr] : arr).map((v) => ({ value: v, label: v }));

  const typePill = (t) =>
    `<span class="label-small pill client-type ${t === 1 ? 'client-type--distributor' : 'client-type--public'}">${t === 1 ? 'Distribuidor' : 'Cliente'}</span>`;

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
      { key: 'id', label: 'ID', width: 80, sort: 'id', render: (c) => esc(c.id), title: (c) => c.id },
      { key: 'name', label: 'Nombre', width: 250, sort: 'name', title: (c) => c.name },
      { key: 'phone1', label: 'Teléfono', width: 180 },
      { key: 'customerType', label: 'Tipo de cliente', width: 180, sort: 'type', render: (c) => typePill(c.customerType), text: (c) => (c.customerType === 1 ? 'Distribuidor' : 'Cliente') },
      { key: 'active', label: 'Estado', width: 120, render: (c) => ui.statusPill(c.active), text: (c) => (c.active ? 'Activo' : 'Inactivo') },
    ],
    actions: () => ui.actionBtn('edit', 'pen', 'Editar') + ui.actionBtn('delete', 'trash-can', 'Eliminar'),
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

  const input = (label, name, value, placeholder, { required = false, small = false, type = 'text', phone = false } = {}) => `
    <div class="field">
      <label class="${small ? 'label-small field__label--small' : 'label-normal field__label'}">${esc(label)}${required ? ' <span class="req">*</span>' : ''}</label>
      <input type="${type}" name="${name}" value="${esc(value || '')}" placeholder="${esc(placeholder)}" ${phone ? 'data-phone' : ''}
             class="input" />
      <small data-error-for="${name}" class="label-small field__error" hidden></small>
    </div>`;

  const dd = (label, id, small = false) => `
    <div class="field">
      <label class="${small ? 'label-small field__label--small' : 'label-normal field__label'}">${esc(label)}</label>
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
      <div data-card class="modal-card modal-card--tall modal-card--clip client-modal">
        <div class="modal-top">
          <h2 class="heading-5">${editing ? 'Editar cliente' : 'Agregar nuevo cliente'}</h2>
          <button type="button" data-close class="modal-close" aria-label="Cerrar">${ui.icon('xmark')}</button>
        </div>
        <div class="modal-scroll" data-form>
          <div class="client-modal__cols">
            <div class="form-stack form-stack--roomy client-modal__col">
              ${input('Nombre', 'name', c.name, 'Introduzca el nombre del cliente', { required: true })}
              ${input('Número de teléfono', 'phone1', c.phone1, '662 123 4567', { required: true, phone: true })}
              <div class="field">
                <label class="label-normal field__label">Tipo de cliente <span class="req">*</span></label>
                <div id="ddType"></div>
              </div>
              <div class="client-modal__invoice">
                <label class="label-normal check">
                  <input type="checkbox" name="requiresInvoice" ${c.rfc || c.taxRegime ? 'checked' : ''} class="checkbox" />
                  Requiere facturar
                </label>
                <div data-invoice class="form-stack" ${c.rfc || c.taxRegime ? '' : 'hidden'}>
                  <div class="field">
                    <label class="label-normal field__label">RFC <span class="req">*</span></label>
                    <input type="text" name="rfc" maxlength="13" value="${esc(c.rfc || '')}" placeholder="Ej. XAXX010101000" class="input input--upper" />
                    <small data-error-for="rfc" class="label-small field__error" hidden></small>
                    <span class="label-small field__hint">12 caracteres (persona moral) o 13 (persona física).</span>
                  </div>
                  ${dd('Régimen fiscal *', 'ddTax')}
                  ${dd('Uso de CFDI (tipo de gasto) *', 'ddExpense')}
                  <small data-invoice-error class="label-small field__error" hidden></small>
                </div>
              </div>
              ${editing ? dd('Estado', 'ddStatus') : ''}
            </div>

            <div data-distributor class="client-modal__col client-modal__distributor" hidden>
              <div class="form-stack">
                <h3 class="label-normal section-label">Datos de distribuidor</h3>
                <div class="form-grid">
                  ${input('Nombre comercial', 'legalName', c.legalName, 'Introduzca el nombre del local', { small: true })}
                  ${input('Celular', 'phone2', c.phone2, '662 123 4567', { small: true, phone: true })}
                  ${input('Nombre del titular', 'titularName', c.titularName, 'Introduzca el nombre del titular', { small: true })}
                  ${input('Email', 'email', c.email, 'cliente@email.com', { small: true, type: 'email' })}
                </div>
              </div>
              <div class="form-stack">
                <h3 class="label-normal section-label">Dirección</h3>
                <div class="form-grid">
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
        <div data-footer class="client-modal__footer">
          <button type="button" data-close class="btn btn--soft btn--round client-modal__btn">Cancelar</button>
          <button type="button" data-save class="btn btn--primary btn--round client-modal__btn">${editing ? 'Guardar cambios' : 'Crear cliente'}</button>
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
      card.classList.toggle('is-distributor', isDist);
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
    ui.dropdown(m.el.querySelector('#ddTax'), { items: toItems(TAX_REGIMES, cur.taxRegime), value: cur.taxRegime, placeholder: 'Seleccionar régimen fiscal', up: true, onChange: (v) => ((cur.taxRegime = v), validate()) });
    ui.dropdown(m.el.querySelector('#ddExpense'), { items: toItems(EXPENSE_TYPES, cur.expenseType), value: cur.expenseType, placeholder: 'Seleccionar uso de CFDI', up: true, onChange: (v) => ((cur.expenseType = v), validate()) });
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
    applyType();
    ui.phoneInputs(form);

    // Si "Requiere facturar" está marcado, RFC, régimen y uso de CFDI son obligatorios.
    const invoiceError = m.el.querySelector('[data-invoice-error]');
    const v = ui.validator(
      form,
      {
        name: [ui.rules.required],
        phone1: [ui.rules.required, ui.rules.phone],
        phone2: [ui.rules.phone],
        email: [ui.rules.email],
        rfc: [
          [(x, all) => !all.requiresInvoice || !!x, 'El RFC es obligatorio para facturar.'],
          [(x, all) => !all.requiresInvoice || ui.isRFC(x), 'RFC no válido: 12 o 13 caracteres con el formato del SAT.'],
        ],
      },
      null
    );
    function validate() {
      const ok = v.validate();
      const invoice = form.querySelector('[name=requiresInvoice]').checked;
      const missing = invoice && (!cur.taxRegime || !cur.expenseType);
      invoiceError.hidden = !missing;
      invoiceError.textContent = missing ? 'Selecciona el régimen fiscal y el uso de CFDI.' : '';
      save.disabled = !ok || missing;
    }
    form.addEventListener('input', validate);
    form.addEventListener('focusout', () => setTimeout(validate));
    m.el.querySelector('[name=requiresInvoice]').addEventListener('change', (e) => {
      invoiceBox.hidden = !e.target.checked;
      validate();
    });
    validate();

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
        await Hook.del(`${endpoint}/${client.id}`);
        ui.toast('Cliente eliminado');
        load();
      },
    });
  }

  document.getElementById('addBtn').addEventListener('click', () => openModal(null));
  ui.bindSearch(document.getElementById('search'), state);
  load();
})();
