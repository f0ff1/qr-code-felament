import { state } from './state.js';
import { fetchJSON, setUnauthorizedHandler } from './api.js';
import { loadAppConfig, loadData, loadFilamentCatalog } from './data.js';
import { connectEvents, loadNotificationHistory } from './events.js';
import { refreshCloudAccountBadge } from './cloud.js';

export function showLoginOverlay(visible) {
  const overlay = document.getElementById('loginOverlay');
  if (!overlay) return;
  overlay.classList.toggle('hidden', !visible);
  overlay.setAttribute('aria-hidden', visible ? 'false' : 'true');
  const page = document.querySelector('.page-shell');
  if (page) page.style.visibility = visible ? 'hidden' : '';
}

setUnauthorizedHandler(() => showLoginOverlay(true));

export function updateAuthChrome() {
  const label = document.getElementById('authUserLabel');
  const logoutBtn = document.getElementById('logoutBtn');
  if (label) {
    label.textContent = state.authDisabled
      ? 'dev (auth off)'
      : (state.username ? `Вы вошли: ${state.username}` : '');
  }
  if (logoutBtn) {
    logoutBtn.classList.toggle('hidden', state.authDisabled || !state.authenticated);
  }
}

export async function ensureAuthenticated() {
  await loadAppConfig();
  try {
    const me = await fetchJSON('/api/auth/me');
    state.authenticated = Boolean(me?.authenticated);
    state.authDisabled = Boolean(me?.auth_disabled);
    state.username = me?.username || '';
    updateAuthChrome();
    if (state.authenticated || state.authDisabled) {
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
    state.authenticated = Boolean(result?.authenticated);
    state.authDisabled = Boolean(result?.auth_disabled);
    state.username = result?.username || form.username.value;
    updateAuthChrome();
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

export async function logout() {
  try {
    await fetchJSON('/api/auth/logout', { method: 'POST' });
  } catch (_error) {
    // ignore
  }
  state.authenticated = false;
  state.username = '';
  updateAuthChrome();
  showLoginOverlay(true);
}
