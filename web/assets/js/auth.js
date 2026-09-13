import { state } from './state.js';
import { fetchJSON, setUnauthorizedHandler } from './api.js';
import { loadAppConfig, loadData, loadFilamentCatalog } from './data.js';
import { connectEvents, loadNotificationHistory } from './events.js';
import { refreshCloudAccountBadge } from './cloud.js';
import { syncAdminNav } from './admin.js';

export function showLoginOverlay(visible) {
  const overlay = document.getElementById('loginOverlay');
  if (!overlay) return;
  overlay.classList.toggle('hidden', !visible);
  overlay.setAttribute('aria-hidden', visible ? 'false' : 'true');
  const page = document.querySelector('.page-shell');
  if (page) page.style.visibility = visible ? 'hidden' : '';
}

setUnauthorizedHandler(() => showLoginOverlay(true));

export function applySession(me) {
  state.authenticated = Boolean(me?.authenticated);
  state.authDisabled = false;
  state.username = me?.username || '';
  state.role = me?.role || '';
  state.isAdmin = Boolean(me?.is_admin);
  state.activeSiteId = me?.active_site_id || '';
  state.siteIds = Array.isArray(me?.site_ids) ? me.site_ids : [];
  updateAuthChrome();
  syncAdminNav();
}

export function updateAuthChrome() {
  const label = document.getElementById('authUserLabel');
  const roleEl = document.getElementById('authUserRole');
  const logoutBtn = document.getElementById('logoutBtn');
  if (label) {
    label.textContent = state.username || '—';
  }
  if (roleEl) {
    const roleLabel = state.isAdmin ? 'Администратор' : (state.role || 'пользователь');
    roleEl.textContent = state.authenticated ? roleLabel : '';
  }
  if (logoutBtn) {
    logoutBtn.classList.toggle('hidden', !state.authenticated);
  }
}

export async function ensureAuthenticated() {
  await loadAppConfig();
  try {
    const me = await fetchJSON('/api/auth/me');
    applySession(me);
    if (state.authenticated) {
      showLoginOverlay(false);
      return true;
    }
  } catch (_error) {
    state.authenticated = false;
  }
  showLoginOverlay(true);
  return false;
}

export async function submitLogin(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const errorEl = document.getElementById('loginError');
  if (errorEl) errorEl.textContent = '';
  try {
    const result = await fetchJSON('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: form.username.value,
        password: form.password.value,
      }),
    });
    applySession(result);
    showLoginOverlay(false);
    form.reset();
    connectEvents();
    refreshCloudAccountBadge();
    loadFilamentCatalog();
    loadNotificationHistory();
    await loadData();
  } catch (error) {
    if (errorEl) errorEl.textContent = error.message || 'Неверный логин или пароль';
  }
}

export async function submitForgotPassword(event) {
  event.preventDefault();
  const form = document.getElementById('loginForm');
  const errorEl = document.getElementById('loginError');
  const username = form?.username?.value?.trim();
  if (!username) {
    if (errorEl) errorEl.textContent = 'Введите логин для запроса сброса пароля';
    return;
  }
  try {
    const result = await fetchJSON('/api/auth/forgot-password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username }),
    });
    if (errorEl) errorEl.textContent = result?.message || 'Заявка отправлена администратору';
  } catch (error) {
    if (errorEl) errorEl.textContent = error.message || 'Не удалось отправить заявку';
  }
}

export async function logout() {
  try {
    await fetchJSON('/api/auth/logout', { method: 'POST' });
  } catch (_error) {
    // ignore
  }
  state.authenticated = false;
  state.username = '';
  state.role = '';
  state.isAdmin = false;
  updateAuthChrome();
  syncAdminNav();
  showLoginOverlay(true);
}
