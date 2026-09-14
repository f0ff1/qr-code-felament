import { state, refs } from './state.js';
import { fetchJSON, setUnauthorizedHandler } from './api.js';
import { loadAppConfig, loadData, loadFilamentCatalog, renderAll } from './data.js';
import { connectEvents, loadNotificationHistory, disconnectEvents, clearSessionRuntime } from './events.js';
import { refreshCloudAccountBadge } from './cloud.js';
import { syncAdminNav, loadSitesForAdmin, refreshAdminUsersCache } from './admin.js';

export function showLoginOverlay(visible) {
  const overlay = document.getElementById('loginOverlay');
  if (!overlay) return;
  overlay.classList.toggle('hidden', !visible);
  overlay.setAttribute('aria-hidden', visible ? 'false' : 'true');
  document.body.classList.toggle('login-open', visible);
  const page = document.querySelector('.page-shell');
  if (page) {
    page.style.visibility = visible ? 'hidden' : '';
    page.setAttribute('aria-hidden', visible ? 'true' : 'false');
  }
}

setUnauthorizedHandler(() => {
  clearSessionRuntime();
  state.authenticated = false;
  state.canWrite = false;
  showLoginOverlay(true);
});

export function resetToOverview() {
  refs.navItems.forEach((item) => {
    item.classList.toggle('active', item.dataset.view === 'overview');
  });
  refs.viewPanels.forEach((panel) => {
    panel.classList.toggle('hidden', panel.dataset.viewPanel !== 'overview');
  });
  state.filter = 'all';
  state.query = '';
  state.productPage = 0;
  if (refs.searchInput) refs.searchInput.value = '';
  refs.filterGroup?.querySelectorAll('.chip').forEach((chip) => {
    chip.classList.toggle('active', chip.dataset.filter === 'all');
  });
  syncAdminNav();
}

export function applySession(me) {
  state.authenticated = Boolean(me?.authenticated);
  state.authDisabled = false;
  state.username = me?.username || '';
  state.role = me?.role || '';
  state.isAdmin = Boolean(me?.is_admin);
  state.canWrite = state.authenticated && state.role !== 'viewer';
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
    clearSessionRuntime();
    applySession(result);
    resetToOverview();
    showLoginOverlay(false);
    form.reset();
    connectEvents();
    refreshCloudAccountBadge();
    loadFilamentCatalog();
    loadNotificationHistory();
    await loadData();
    renderAll();
    loadSitesForAdmin().catch(() => {});
    if (state.isAdmin) {
      refreshAdminUsersCache().catch(() => {});
    }
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
  clearSessionRuntime();
  disconnectEvents();
  state.authenticated = false;
  state.username = '';
  state.role = '';
  state.isAdmin = false;
  state.canWrite = false;
  state.activeSiteId = '';
  state.siteIds = [];
  resetToOverview();
  updateAuthChrome();
  syncAdminNav();
  showLoginOverlay(true);
}
