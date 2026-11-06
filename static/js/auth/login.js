/* Login — POST /api/auth/login y, si todo sale bien, al dashboard.
   El botón siempre está activo (así funciona aunque el navegador
   autocomplete el correo y la contraseña); se valida al enviar. */
(function () {
  'use strict';
  const Hook = window.Hook;

  const form = document.getElementById('loginForm');
  const email = document.getElementById('email');
  const password = document.getElementById('password');
  const submit = document.getElementById('loginSubmit');
  const errorEl = document.getElementById('loginError');
  const emailRe = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;

  const showError = (msg) => {
    errorEl.textContent = msg;
    errorEl.classList.toggle('show', !!msg);
  };
  [email, password].forEach((i) => i.addEventListener('input', () => showError('')));

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    const mail = email.value.trim();
    if (!mail || !password.value) return showError('Escribe tu correo y tu contraseña.');
    if (!emailRe.test(mail)) return showError('El correo no tiene un formato válido.');

    showError('');
    submit.disabled = true;
    try {
      await Hook.post('/api/auth/login', { email: mail, password: password.value });
      window.location.href = '/dashboard';
    } catch (err) {
      showError(err.message || 'Correo o contraseña incorrectos.');
      submit.disabled = false;
    }
  });

  /* ---------- Mostrar / ocultar contraseña ----------
     El ojo parpadea al tocarlo; cuando la contraseña se ve, una línea lo
     tacha y el texto aparece con un pequeño desenfoque. */
  const toggle = document.getElementById('passwordToggle');
  toggle.addEventListener('click', function () {
    const show = password.type === 'password';
    password.type = show ? 'text' : 'password';
    this.classList.toggle('is-shown', show);
    this.setAttribute('aria-pressed', show ? 'true' : 'false');
    this.setAttribute('aria-label', show ? 'Ocultar contraseña' : 'Mostrar contraseña');
    this.classList.remove('is-blink');
    void this.offsetWidth;
    this.classList.add('is-blink');
    password.classList.remove('is-reveal', 'is-hide');
    void password.offsetWidth;
    password.classList.add(show ? 'is-reveal' : 'is-hide');
    password.focus({ preventScroll: true });
    const n = password.value.length;
    try {
      password.setSelectionRange(n, n);
    } catch (_) {}
  });
  // Al tocar el ojo no se pierde el foco del campo.
  toggle.addEventListener('mousedown', (e) => e.preventDefault());
})();