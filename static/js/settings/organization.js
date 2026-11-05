/* Configuración → Organización → Información (GET/PUT /api/organizations/mine). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;

  const form = document.getElementById('orgForm');
  const submit = document.getElementById('orgSubmit');
  const nameRe = /^[a-zA-Z0-9\s.,&\-ñÑáéíóúÁÉÍÓÚüÜ]+$/;
  const phoneRe = /^[0-9+\-\s()]+$/;

  // Con el menú lateral colapsado, las dos secciones van lado a lado
  function applyLayout(collapsed) {
    const wrap = form.querySelector('[data-layout="wrap"]');
    wrap.className = collapsed ? 'flex flex-col md:flex-row gap-8 md:gap-12' : 'flex flex-col gap-6';
    form.querySelectorAll('[data-layout="grid"]').forEach(
      (g) => (g.className = collapsed ? 'flex flex-col gap-5' : 'grid grid-cols-1 md:grid-cols-2 gap-x-6 gap-y-4')
    );
    const half = form.querySelector('[data-layout="half"]');
    half.className = collapsed ? 'w-full' : 'md:col-span-2 md:w-[calc(50%-0.75rem)]';
    const full = form.querySelector('[data-layout="full"]');
    full.className = collapsed ? 'w-full' : 'md:col-span-2';
    const div = form.querySelector('[data-layout="divider"]');
    div.className = collapsed ? 'bg-gray-200 hidden md:block w-px min-h-full' : 'bg-gray-200 h-px w-full';
  }
  document.addEventListener('hook:sidebar', (e) => applyLayout(e.detail.collapsed));

  const v = ui.validator(
    form,
    {
      name: [ui.rules.required, ui.rules.pattern(nameRe)],
      email: [ui.rules.required, ui.rules.email],
      phone: [ui.rules.pattern(phoneRe)],
      phone2: [ui.rules.pattern(phoneRe)],
      postCode: [ui.rules.pattern(/^[0-9]+$/)],
    },
    (valid) => (submit.disabled = !valid)
  );

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
