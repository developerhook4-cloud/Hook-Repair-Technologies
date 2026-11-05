/* Login — POST /api/auth/login y, si todo sale bien, al dashboard. */
(function () {
  'use strict';
  const { ui } = window.Hook;

  const form = document.getElementById('loginForm');
  const email = document.getElementById('email');
  const password = document.getElementById('password');
  const submit = document.getElementById('loginSubmit');
  const errorBox = document.getElementById('loginError');
  const emailRe = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;

  ui.passwordToggles(form);

  const touched = { email: false, password: false };

  function validate() {
    const errs = {};
    if (!email.value.trim()) errs.email = 'Este campo es obligatorio.';
    else if (!emailRe.test(email.value.trim())) errs.email = 'El formato debe ser válido.';
    if (!password.value) errs.password = 'Este campo es obligatorio.';

    ['email', 'password'].forEach((k) => {
      const el = form.querySelector(`[data-error-for="${k}"]`);
      const show = touched[k] && errs[k];
      el.hidden = !show;
      el.textContent = show ? errs[k] : '';
    });
    submit.disabled = Object.keys(errs).length > 0;
    return !submit.disabled;
  }

  [email, password].forEach((input) => {
    input.addEventListener('input', () => {
      errorBox.hidden = true;
      validate();
    });
    input.addEventListener('blur', () => {
      touched[input.name] = true;
      validate();
    });
  });

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    touched.email = touched.password = true;
    if (!validate()) return;
    await ui.busy(submit, async () => {
      try {
        await window.Hook.post('/api/auth/login', { email: email.value.trim(), password: password.value });
        window.location.href = '/dashboard';
      } catch (err) {
        errorBox.textContent = err.message || 'No se pudo iniciar sesión.';
        errorBox.hidden = false;
      }
    });
  });

  validate();
})();
