/*
  Hook Repair — núcleo del frontend.

  La sesión viene del servidor en <script id="hook-session"> y todas las
  llamadas a la API pasan por Hook.api(), que manda la cookie de sesión y,
  si la sesión ya no es válida (401), regresa al login.
*/
(function () {
  'use strict';

  const Hook = (window.Hook = window.Hook || {});

  // ── Sesión ──────────────────────────────────────────────────────────
  let session = null;
  const sessionEl = document.getElementById('hook-session');
  if (sessionEl) {
    try {
      session = JSON.parse(sessionEl.textContent || 'null');
    } catch (_) {
      session = null;
    }
  }

  Hook.session = session;
  Hook.user = session ? session.user : null;
  Hook.orgId = Hook.user ? Hook.user.organizationId : null;

  /** ¿El usuario tiene este permiso? */
  Hook.can = function (permission) {
    const perms = (Hook.user && Hook.user.permissions) || [];
    return perms.some((p) => p === '*' || String(p).toLowerCase() === String(permission).toLowerCase());
  };

  /** Ruta de la API para un recurso de la organización: Hook.org('clients') → /api/<org>/clients */
  Hook.org = function (resource) {
    if (!Hook.orgId) throw new Error('Se requiere una organización con sesión.');
    return `/api/${Hook.orgId}/${resource}`;
  };

  // ── Llamadas a la API ───────────────────────────────────────────────
  class ApiError extends Error {
    constructor(status, message, body) {
      super(message);
      this.status = status;
      this.body = body;
    }
  }
  Hook.ApiError = ApiError;

  Hook.api = async function (method, path, body) {
    const headers = { Accept: 'application/json' };
    const init = { method, headers, credentials: 'same-origin' };
    if (body !== undefined) {
      headers['Content-Type'] = 'application/json';
      init.body = JSON.stringify(body);
    }

    let res;
    try {
      res = await fetch(path, init);
    } catch (_) {
      throw new ApiError(0, 'No se pudo conectar con el servidor. Revisa tu conexión.');
    }

    const text = await res.text();
    let data = null;
    if (text) {
      try {
        data = JSON.parse(text);
      } catch (_) {
        data = text;
      }
    }

    if (res.status === 401 && !path.startsWith('/api/auth/')) {
      window.location.href = '/login';
      throw new ApiError(401, 'Tu sesión terminó. Inicia sesión de nuevo.', data);
    }
    if (!res.ok) {
      const msg =
        (data && (data.detail || data.message || data.title)) ||
        `Ocurrió un error (${res.status}).`;
      throw new ApiError(res.status, msg, data);
    }
    return data;
  };

  Hook.get = (path) => Hook.api('GET', path);
  Hook.post = (path, body) => Hook.api('POST', path, body === undefined ? {} : body);
  Hook.put = (path, body) => Hook.api('PUT', path, body);
  Hook.patch = (path, body) => Hook.api('PATCH', path, body);

  /** Cerrar sesión y volver al login. */
  Hook.logout = async function () {
    try {
      await Hook.post('/api/auth/logout');
    } finally {
      window.location.href = '/login';
    }
  };
})();
