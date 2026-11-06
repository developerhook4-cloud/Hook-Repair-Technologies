/*
  Hook Repair — menú lateral y barra de usuario.
  - El menú se retrae a solo íconos (se recuerda en este navegador).
  - Un "blob" rojo se desliza a la opción bajo el mouse y regresa a la activa.
  - El avatar de arriba abre el menú de la cuenta.
*/
(function () {
  'use strict';

  const root = document.documentElement;

  // ── Menú lateral ────────────────────────────────────────────────────
  const sidebar = document.getElementById('appSidebar');
  if (sidebar) {
    const toggle = document.getElementById('sidebarToggle');
    const nav = sidebar.querySelector('[data-nav]');
    const blob = nav.querySelector('.app-nav__blob');
    const links = Array.from(nav.querySelectorAll('.app-nav__link'));
    const active = nav.querySelector('.app-nav__link.is-active');

    const moveBlob = (el) => {
      links.forEach((l) => l.classList.toggle('is-current', l === el));
      if (!el) return blob.classList.remove('is-visible');
      blob.style.top = el.offsetTop + 'px';
      blob.style.height = el.offsetHeight + 'px';
      blob.classList.add('is-visible');
    };
    links.forEach((l) => l.addEventListener('mouseenter', () => moveBlob(l)));
    nav.addEventListener('mouseleave', () => moveBlob(active));
    // Sin animación la primera vez
    blob.style.transition = 'none';
    moveBlob(active);
    requestAnimationFrame(() => requestAnimationFrame(() => (blob.style.transition = '')));

    toggle.addEventListener('click', () => {
      const shrink = !root.classList.contains('sidebar-shrink');
      root.classList.toggle('sidebar-shrink', shrink);
      try {
        localStorage.setItem('hookSidebar', shrink ? 'shrink' : 'open');
      } catch (_) {}
      toggle.setAttribute('aria-label', shrink ? 'Expandir menú' : 'Colapsar menú');
      setTimeout(() => moveBlob(active), 320);
      document.dispatchEvent(new CustomEvent('hook:sidebar', { detail: { collapsed: shrink } }));
    });
  }

  // ── Menú de la cuenta ───────────────────────────────────────────────
  const userbar = document.getElementById('userbar');
  if (userbar) {
    const trigger = userbar.querySelector('.userbar__trigger');
    const setOpen = (open) => {
      userbar.classList.toggle('is-open', open);
      trigger.setAttribute('aria-expanded', open ? 'true' : 'false');
    };
    trigger.addEventListener('click', (e) => {
      e.stopPropagation();
      setOpen(!userbar.classList.contains('is-open'));
    });
    document.addEventListener('click', (e) => {
      if (!userbar.contains(e.target)) setOpen(false);
    });
    document.addEventListener('keydown', (e) => e.key === 'Escape' && setOpen(false));
  }

  // ── Cerrar sesión ───────────────────────────────────────────────────
  document.querySelectorAll('[data-logout]').forEach((a) =>
    a.addEventListener('click', (e) => {
      e.preventDefault();
      window.Hook.logout();
    })
  );
})();