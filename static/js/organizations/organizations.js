/* Configuración → Organización → Información (GET/PUT /api/organizations/mine). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;

  const form = document.getElementById('orgForm');
  const submit = document.getElementById('orgSubmit');
  const nameRe = /^[a-zA-Z0-9\s.,&\-ñÑáéíóúÁÉÍÓÚüÜ]+$/;

  // Con el menú lateral colapsado, las dos secciones van lado a lado
  function applyLayout(collapsed) {
    form.classList.toggle('is-wide', collapsed);
  }
  document.addEventListener('hook:sidebar', (e) => applyLayout(e.detail.collapsed));
  applyLayout(document.documentElement.classList.contains('sidebar-shrink'));

  const v = ui.validator(
    form,
    {
      name: [ui.rules.required, ui.rules.pattern(nameRe)],
      email: [ui.rules.required, ui.rules.email],
      phone: [ui.rules.phone],
      phone2: [ui.rules.phone],
      postCode: [ui.rules.pattern(/^[0-9]+$/)],
    },
    (valid) => (submit.disabled = !valid)
  );

  ui.phoneInputs(form);

  async function load() {
    try {
      const o = await Hook.get('/api/organizations/mine');
      const a = o.address || {};
      const set = (n, val) => (form.elements[n].value = val || '');
      set('name', o.name);
      set('email', o.email);
      set('phone', o.phone1 || o.phone);
      set('phone2', o.phone2);
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
    if (!v.validate(true)) return;
    const f = v.values();
    await ui.busy(submit, async () => {
      try {
        await Hook.put('/api/organizations/mine', {
          name: f.name.trim(),
          email: f.email.trim(),
          phone1: f.phone.trim(),
          phone2: f.phone2.trim(),
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