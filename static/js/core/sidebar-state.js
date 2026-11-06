/* Se carga en el <head> (antes de pintar) para que el menú lateral salga
   retraído desde el inicio si así se dejó, sin brinco. */
try {
  if (localStorage.getItem('hookSidebar') === 'shrink') document.documentElement.classList.add('sidebar-shrink');
} catch (_) {}