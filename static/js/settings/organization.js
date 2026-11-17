/* Configuración → Organización → Información (GET/PUT /api/organizations/mine). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;

  const form = document.getElementById('orgForm');
  const submit = document.getElementById('orgSubmit');
  const nameRe = /^[a-zA-Z0-9\s.,&\-ñÑáéíóúÁÉÍÓÚüÜ]+$/;
  // Solo los administradores (Users.Update + Roles.Update, igual que el servidor)
  // pueden guardar; los demás ven los datos en solo lectura.
  const canEdit = Hook.can('Users.Update') && Hook.can('Roles.Update');

  // Con el menú lateral colapsado, las dos secciones van lado a lado
  function applyLayout(collapsed) {
    form.classList.toggle('is-wide', collapsed);
  }
  document.addEventListener('hook:sidebar', (e) => applyLayout(e.detail.collapsed));
  applyLayout(document.documentElement.classList.contains('sidebar-shrink'));

  let formOk = false;
  const v = ui.validator(
    form,
    {
      name: [ui.rules.required, ui.rules.pattern(nameRe)],
      email: [ui.rules.required, ui.rules.email],
      postCode: [ui.rules.pattern(/^[0-9]+$/)],
    },
    (valid) => {
      formOk = valid;
      submit.disabled = !valid || !!(phoneList && phoneList.error());
    }
  );
  // Teléfonos múltiples (KAN-101): la lista se dibuja al cargar la organización.
  let phoneList = null;
  form.addEventListener('input', () => (submit.disabled = !formOk || !!(phoneList && phoneList.error())));

  ui.phoneInputs(form);

  if (!canEdit) {
    form.querySelectorAll('input, select, textarea').forEach((el) => (el.disabled = true));
    const actions = form.querySelector('.form-actions');
    if (actions) actions.hidden = true;
  }

  async function load() {
    try {
      const o = await Hook.get('/api/organizations/mine');
      const a = o.address || {};
      const set = (n, val) => (form.elements[n].value = val || '');
      set('name', o.name);
      set('email', o.email);
      phoneList = ui.phoneList(document.getElementById('orgPhones'), ui.phonesOf(o));
      if (!canEdit) document.querySelectorAll('#orgPhones input, #orgPhones select, #orgPhones button').forEach((el) => (el.disabled = true));
      set('website', o.website);
      set('address', a.address);
      set('city', a.city);
      set('state', a.state);
      set('postCode', a.postCode);
      set('country', a.country);
      v.validate();
    } catch (err) {
      ui.error(err, 'No se pudo cargar la organización.');
    }
  }

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!canEdit) return;
    if (!v.validate(true) || (phoneList && phoneList.error())) return;
    const f = v.values();
    await ui.busy(submit, async () => {
      try {
        await Hook.put('/api/organizations/mine', {
          name: f.name.trim(),
          email: f.email.trim(),
          phones: phoneList ? phoneList.get() : [],
          website: f.website.trim(),
          address: { address: f.address.trim(), city: f.city.trim(), state: f.state.trim(), postCode: f.postCode.trim(), country: f.country.trim() },
        });
        ui.toast('Cambios guardados');
      } catch (err) {
        ui.error(err);
      }
    });
  });

  load();
})();
