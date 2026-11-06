/*
  Hook Repair — barra superior (navbar) y menú lateral (sidebar).
*/
(function () {
  'use strict';

  // ── Navbar: la "pestaña" clara que sigue al mouse ───────────────────
  const nav = document.getElementById('hook-navbar');
  if (nav) {
    const tab = nav.querySelector('[data-nav-tab]');
    let active = nav.querySelector('[data-nav-link].is-active');

    const moveTo = (el) => {
      if (!el) {
        tab.style.opacity = '0';
        return;
      }
      const n = nav.getBoundingClientRect();
      const r = el.getBoundingClientRect();
      tab.style.left = r.left - n.left + 'px';
      tab.style.top = r.top - n.top + 'px';
      tab.style.width = r.width + 'px';
      tab.style.height = r.height + 'px';
      tab.style.opacity = '1';
    };

    nav.querySelectorAll('[data-nav-link]').forEach((a) => {
      a.addEventListener('mouseenter', () => moveTo(a));
      a.addEventListener('click', () => (active = a));
    });
    nav.addEventListener('mouseleave', () => moveTo(active));
    if (active) requestAnimationFrame(() => moveTo(active));
    window.addEventListener('resize', () => moveTo(active));

    nav.querySelectorAll('[data-logout]').forEach((a) =>
      a.addEventListener('click', (e) => {
        e.preventDefault();
        window.Hook.logout();
      })
    );
  }

  // ── Sidebar: colapsar / expandir y grupos desplegables ──────────────
  const sb = document.getElementById('hook-sidebar');
  if (sb) {
    const toggle = sb.querySelector('[data-sidebar-toggle]');
    const groups = Array.from(sb.querySelectorAll('[data-sb-item]'));

    const setCollapsed = (collapsed) => {
      sb.dataset.collapsed = collapsed ? '1' : '';
      sb.classList.toggle('is-collapsed', collapsed);
      if (collapsed) groups.forEach((g) => setExpanded(g, false));
      toggle.innerHTML = `<i class="fa-solid fa-chevron-${collapsed ? 'right' : 'left'}"></i>`;
      document.dispatchEvent(new CustomEvent('hook:sidebar', { detail: { collapsed } }));
    };

    const setExpanded = (g, expanded) => {
      const subs = g.querySelector('[data-sb-subs]');
      if (!subs) return;
      subs.hidden = !expanded;
      const chev = g.querySelector('[data-sb-chevron]');
      if (chev) chev.classList.toggle('is-open', expanded);
    };

    toggle.addEventListener('click', () => setCollapsed(!sb.dataset.collapsed));

    groups.forEach((g) => {
      const row = g.querySelector('[data-sb-row]');
      const subs = g.querySelector('[data-sb-subs]');
      if (!subs) return; // las opciones sin subopciones son enlaces normales
      row.addEventListener('click', () => {
        if (sb.dataset.collapsed) setCollapsed(false);
        const willOpen = subs.hidden;
        groups.forEach((o) => o !== g && setExpanded(o, false));
        setExpanded(g, willOpen);
      });
    });
  }
})();
