/* Configuración → Perfil (PUT /api/users/me). */
(function () {
  'use strict';
  const Hook = window.Hook;
  const { ui } = Hook;

  const form = document.getElementById('profileForm');
  const submit = document.getElementById('profileSubmit');
  const toggle = document.getElementById('passwordToggle');
  const panel = document.getElementById('passwordPanel');
  const nameRe = /^[a-zA-Z\sñÑáéíóúÁÉÍÓÚüÜ]+$/;

  form.elements.name.value = (Hook.user && Hook.user.name) || '';
  form.elements.email.value = (Hook.user && Hook.user.email) || '';
  ui.passwordToggles(form);

  // Desplegable "Cambiar contraseña"
  let open = false;
  const setOpen = (v) => {
    open = v;
    panel.classList.toggle('max-h-0', !open);
    panel.classList.toggle('opacity-0', !open);
    panel.classList.toggle('max-h-125', open);
    panel.classList.toggle('mt-4', open);
    panel.classList.toggle('opacity-100', open);
    toggle.querySelector('[data-chevron]').classList.toggle('rotate-180', open);
  };
  toggle.addEventListener('click', () => setOpen(!open));
  toggle.addEventListener('keydown', (e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), setOpen(!open)));

  const v = ui.validator(
    form,
    {
      name: [ui.rules.required, ui.rules.pattern(nameRe)],
      email: [ui.rules.required, ui.rules.email],
      repeatNewPassword: [[(val, all) => !all.newPassword || val === all.newPassword, 'Las contraseñas no coinciden.']],
    },
    (valid) => (submit.disabled = !valid)
  );
  v.validate();

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!v.validate(true)) return;
    const vals = v.values();
    await ui.busy(submit, async () => {
      try {
        await Hook.put('/api/users/me', {
          name: vals.name.trim(),
          email: vals.email.trim(),
          oldPassword: vals.oldPassword || null,
          newPassword: vals.newPassword || null,
        });
        ['oldPassword', 'newPassword', 'repeatNewPassword'].forEach((n) => (form.elements[n].value = ''));
        if (Hook.user) {
          Hook.user.name = vals.name.trim();
          Hook.user.email = vals.email.trim();
        }
        v.reset();
        ui.toast('Cambios guardados');
      } catch (err) {
        ui.error(err, 'Error al actualizar el perfil');
      }
    });
  });
})();
