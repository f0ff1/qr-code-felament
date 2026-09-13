import { state } from './state.js';
import { fetchJSON } from './api.js';
import { notify } from './toasts.js';
import { loadData } from './data.js';

export function syncAdminNav() {
  document.querySelectorAll('[data-admin-only]').forEach((el) => {
    el.classList.toggle('hidden', !state.isAdmin);
  });
  const siteSelect = document.getElementById('activeSiteSelect');
  if (siteSelect) {
    siteSelect.classList.toggle('hidden', !state.isAdmin);
  }
}

export async function loadSitesForAdmin() {
  if (!state.isAdmin) return;
  try {
    state.sites = await fetchJSON('/api/admin/sites') || [];
    const select = document.getElementById('activeSiteSelect');
    if (select) {
      select.innerHTML = state.sites.map((s) =>
        `<option value="${s.id}" ${s.id === state.activeSiteId ? 'selected' : ''}>${s.name}</option>`
      ).join('');
    }
    const userSite = document.getElementById('userSiteSelect');
    if (userSite) {
      userSite.innerHTML = state.sites.map((s) =>
        `<option value="${s.id}">${s.name}</option>`
      ).join('');
    }
  } catch (error) {
    notify(error.message || 'Не удалось загрузить склады', 'error');
  }
}

export async function onActiveSiteChange(event) {
  const siteId = event.target.value;
  try {
    await fetchJSON('/api/auth/active-site', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ site_id: siteId }),
    });
    state.activeSiteId = siteId;
    await loadData();
  } catch (error) {
    notify(error.message || 'Не удалось сменить склад', 'error');
  }
}

export async function createSiteInline(event) {
  event.preventDefault();
  const form = event.currentTarget;
  try {
    const site = await fetchJSON('/api/admin/sites', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        organization_name: form.organization_name.value,
        name: form.name.value,
        address: form.address.value,
      }),
    });
    notify(`Склад «${site.name}» создан`, 'success');
    form.reset();
    await loadSitesForAdmin();
    const userSite = document.getElementById('userSiteSelect');
    if (userSite && site?.id) userSite.value = site.id;
  } catch (error) {
    notify(error.message || 'Не удалось создать склад', 'error');
  }
}

export async function createUser(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const siteId = form.site_id.value;
  try {
    const result = await fetchJSON('/api/admin/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: form.username.value,
        password: form.password.value,
        first_name: form.first_name.value,
        last_name: form.last_name.value,
        role: form.role.value,
        site_ids: siteId ? [siteId] : [],
      }),
    });
    const pwd = result?.password || form.password.value;
    notify(`Пользователь ${result?.user?.username} создан. Пароль: ${pwd}`, 'success', 'Админ');
    form.reset();
    await renderAdminUsers();
  } catch (error) {
    notify(error.message || 'Не удалось создать пользователя', 'error');
  }
}

export async function renderAdminUsers() {
  const box = document.getElementById('adminUsersList');
  if (!box || !state.isAdmin) return;
  try {
    const users = await fetchJSON('/api/admin/users') || [];
    const resets = await fetchJSON('/api/admin/password-resets') || [];
    box.innerHTML = `
      ${resets.length ? `<div class="admin-resets"><strong>Заявки на сброс пароля:</strong>
        ${resets.map((r) => `<div class="admin-reset-row">
          <span>${r.user_id.slice(0, 8)}…</span>
          <button type="button" class="mini-btn" data-resolve-reset="${r.id}">Сбросить пароль</button>
        </div>`).join('')}
      </div>` : ''}
      <div class="admin-users-table">
        ${users.map((u) => `<div class="admin-user-row">
          <strong>${u.username}</strong> — ${u.first_name} ${u.last_name} · ${u.role}
          ${u.is_active ? '' : '(отключён)'}
          <button type="button" class="mini-btn" data-reset-user="${u.id}">Новый пароль</button>
        </div>`).join('')}
      </div>`;
  } catch (error) {
    box.textContent = error.message || 'Ошибка загрузки';
  }
}

export function bindAdminEvents() {
  document.getElementById('activeSiteSelect')?.addEventListener('change', onActiveSiteChange);
  document.getElementById('adminCreateSiteForm')?.addEventListener('submit', createSiteInline);
  document.getElementById('adminCreateUserForm')?.addEventListener('submit', createUser);
  document.getElementById('adminUsersList')?.addEventListener('click', async (event) => {
    const resetUser = event.target.closest('[data-reset-user]');
    if (resetUser) {
      try {
        const result = await fetchJSON(`/api/admin/users/${resetUser.dataset.resetUser}/password`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({}),
        });
        notify(`Новый пароль: ${result.password}`, 'success', 'Админ');
      } catch (error) {
        notify(error.message || 'Ошибка сброса', 'error');
      }
      return;
    }
    const resolve = event.target.closest('[data-resolve-reset]');
    if (resolve) {
      try {
        const result = await fetchJSON(`/api/admin/password-resets/${resolve.dataset.resolveReset}/resolve`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({}),
        });
        notify(`Пароль сброшен: ${result.password}`, 'success', 'Админ');
        await renderAdminUsers();
      } catch (error) {
        notify(error.message || 'Ошибка', 'error');
      }
    }
  });
}
