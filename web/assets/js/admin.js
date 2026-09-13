import { state } from './state.js';
import { fetchJSON } from './api.js';
import { notify } from './toasts.js';
import { loadData } from './data.js';

const USERS_PER_PAGE = 12;

export function syncAdminNav() {
  document.querySelectorAll('[data-admin-only]').forEach((el) => {
    el.classList.toggle('hidden', !state.isAdmin);
  });
  const siteField = document.getElementById('activeSiteField');
  if (siteField) {
    siteField.classList.toggle('hidden', !state.isAdmin);
  }
  const card = document.getElementById('sidebarUserCard');
  if (card) {
    card.classList.toggle('is-authenticated', Boolean(state.authenticated));
  }
}

async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch (_error) {
    const area = document.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', '');
    area.style.position = 'fixed';
    area.style.opacity = '0';
    document.body.appendChild(area);
    area.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(area);
    return ok;
  }
}

export function showCredentialModal({ title, hint, login, password }) {
  const modal = document.getElementById('credentialModal');
  if (!modal) {
    notify(`Логин: ${login} · Пароль: ${password}`, 'success', 'Доступ');
    copyText(password);
    return;
  }
  document.getElementById('credentialTitle').textContent = title || 'Учётные данные';
  document.getElementById('credentialHint').textContent =
    hint || 'Сохраните пароль сейчас — повторно его увидеть нельзя.';
  document.getElementById('credentialLogin').value = login || '';
  document.getElementById('credentialPassword').value = password || '';
  const status = document.getElementById('credentialCopyStatus');
  if (status) status.textContent = '';
  modal.classList.remove('hidden');
  modal.setAttribute('aria-hidden', 'false');
  copyText(password).then((ok) => {
    if (status) {
      status.textContent = ok
        ? 'Пароль скопирован в буфер обмена'
        : 'Не удалось скопировать автоматически — нажмите кнопку ниже';
    }
  });
}

export function hideCredentialModal() {
  const modal = document.getElementById('credentialModal');
  if (!modal) return;
  modal.classList.add('hidden');
  modal.setAttribute('aria-hidden', 'true');
}

export async function loadSitesForAdmin() {
  if (!state.isAdmin) return;
  try {
    state.sites = await fetchJSON('/api/admin/sites') || [];
    const select = document.getElementById('activeSiteSelect');
    if (select) {
      select.innerHTML = state.sites.map((s) =>
        `<option value="${s.id}" ${s.id === state.activeSiteId ? 'selected' : ''}>${escapeHtml(s.name)}</option>`
      ).join('');
    }
    const userSite = document.getElementById('userSiteSelect');
    if (userSite) {
      userSite.innerHTML = state.sites.map((s) =>
        `<option value="${s.id}">${escapeHtml(s.name)}</option>`
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
  const username = form.username.value.trim();
  try {
    const result = await fetchJSON('/api/admin/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username,
        password: form.password.value,
        first_name: form.first_name.value,
        last_name: form.last_name.value,
        role: form.role.value,
        site_ids: siteId ? [siteId] : [],
      }),
    });
    const pwd = result?.password || form.password.value;
    showCredentialModal({
      title: 'Пользователь создан',
      hint: 'Пароль уже в буфере обмена. Передайте его сотруднику — из БД его больше не прочитать.',
      login: result?.user?.username || username,
      password: pwd,
    });
    form.reset();
    await refreshAdminUsersCache();
  } catch (error) {
    notify(error.message || 'Не удалось создать пользователя', 'error');
  }
}

function siteName(id) {
  const site = (state.sites || []).find((s) => String(s.id) === String(id));
  return site?.name || String(id || '').slice(0, 8);
}

function filteredUsers(users) {
  const q = String(state.adminUsersQuery || '').trim().toLowerCase();
  const role = state.adminUsersRole || 'all';
  const active = state.adminUsersActive || 'all';
  return users.filter((u) => {
    if (role !== 'all' && String(u.role) !== role) return false;
    if (active === 'active' && !u.is_active) return false;
    if (active === 'inactive' && u.is_active) return false;
    if (!q) return true;
    const hay = `${u.username} ${u.first_name} ${u.last_name} ${u.role}`.toLowerCase();
    return hay.includes(q);
  });
}

function renderPasswordResets() {
  const resetsBox = document.getElementById('adminResetsBox');
  if (!resetsBox) return;
  const resets = state.adminPasswordResets || [];
  resetsBox.innerHTML = resets.length
    ? `<div class="admin-resets">
        <div class="panel-kicker">Заявки на сброс пароля</div>
        ${resets.map((r) => {
          const user = (state.adminUsersCache || []).find((u) => String(u.id) === String(r.user_id));
          const label = user ? `${user.username} (${user.first_name} ${user.last_name})` : r.user_id;
          return `<div class="admin-reset-row">
            <span>${escapeHtml(label)}</span>
            <button type="button" class="mini-btn" data-resolve-reset="${r.id}">Выдать новый пароль</button>
          </div>`;
        }).join('')}
      </div>`
    : '';
}

export async function renderAdminUsers() {
  const box = document.getElementById('adminUsersList');
  const pager = document.getElementById('adminUsersPager');
  if (!box || !state.isAdmin) return;
  try {
    renderPasswordResets();
    const filtered = filteredUsers(state.adminUsersCache || []);
    const totalPages = Math.max(1, Math.ceil(filtered.length / USERS_PER_PAGE));
    if (state.adminUsersPage >= totalPages) state.adminUsersPage = totalPages - 1;
    if (state.adminUsersPage < 0) state.adminUsersPage = 0;
    const start = state.adminUsersPage * USERS_PER_PAGE;
    const pageItems = filtered.slice(start, start + USERS_PER_PAGE);

    box.innerHTML = pageItems.length
      ? `<div class="admin-users-table-wrap">
          <table class="admin-users-table">
            <thead>
              <tr>
                <th>Пользователь</th>
                <th>Роль</th>
                <th>Склад</th>
                <th>Статус</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              ${pageItems.map((u) => `
                <tr class="${u.is_active ? '' : 'is-inactive'}">
                  <td>
                    <div class="admin-user-primary">${escapeHtml(u.username)}</div>
                    <div class="muted admin-user-secondary">${escapeHtml(`${u.first_name || ''} ${u.last_name || ''}`.trim() || '—')}</div>
                  </td>
                  <td><span class="admin-role-pill role-${escapeHtml(u.role)}">${escapeHtml(u.role)}</span></td>
                  <td>${escapeHtml((u.site_ids || []).map(siteName).join(', ') || '—')}</td>
                  <td>${u.is_active ? 'активен' : 'отключён'}</td>
                  <td class="admin-user-actions">
                    <button type="button" class="mini-btn" data-reset-user="${u.id}">Новый пароль</button>
                    <button type="button" class="mini-btn" data-toggle-active="${u.id}" data-active="${u.is_active ? '1' : '0'}">
                      ${u.is_active ? 'Отключить' : 'Включить'}
                    </button>
                  </td>
                </tr>`).join('')}
            </tbody>
          </table>
        </div>`
      : `<div class="muted admin-users-empty">Никого не найдено</div>`;

    if (pager) {
      pager.innerHTML = `
        <button type="button" class="product-page-btn" data-users-page-dir="-1" ${state.adminUsersPage <= 0 ? 'disabled' : ''}>←</button>
        <span class="muted">${filtered.length} · стр. ${state.adminUsersPage + 1} / ${totalPages}</span>
        <button type="button" class="product-page-btn" data-users-page-dir="1" ${state.adminUsersPage >= totalPages - 1 ? 'disabled' : ''}>→</button>`;
    }
  } catch (error) {
    box.textContent = error.message || 'Ошибка загрузки';
  }
}

export async function refreshAdminUsersCache() {
  const [users, resets] = await Promise.all([
    fetchJSON('/api/admin/users'),
    fetchJSON('/api/admin/password-resets'),
  ]);
  state.adminUsersCache = users || [];
  state.adminPasswordResets = resets || [];
  await renderAdminUsers();
}

function escapeHtml(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;');
}

export function bindAdminEvents() {
  document.getElementById('activeSiteSelect')?.addEventListener('change', onActiveSiteChange);
  document.getElementById('adminCreateSiteForm')?.addEventListener('submit', createSiteInline);
  document.getElementById('adminCreateUserForm')?.addEventListener('submit', createUser);
  document.getElementById('credentialCloseBtn')?.addEventListener('click', hideCredentialModal);
  document.getElementById('credentialCopyBtn')?.addEventListener('click', async () => {
    const password = document.getElementById('credentialPassword')?.value || '';
    const status = document.getElementById('credentialCopyStatus');
    const ok = await copyText(password);
    if (status) status.textContent = ok ? 'Скопировано' : 'Не удалось скопировать';
  });
  document.getElementById('credentialModal')?.addEventListener('click', (event) => {
    if (event.target.id === 'credentialModal') hideCredentialModal();
  });

  document.getElementById('adminUsersSearch')?.addEventListener('input', (event) => {
    state.adminUsersQuery = event.target.value;
    state.adminUsersPage = 0;
    renderAdminUsers();
  });
  document.getElementById('adminUsersRoleFilter')?.addEventListener('change', (event) => {
    state.adminUsersRole = event.target.value;
    state.adminUsersPage = 0;
    renderAdminUsers();
  });
  document.getElementById('adminUsersActiveFilter')?.addEventListener('change', (event) => {
    state.adminUsersActive = event.target.value;
    state.adminUsersPage = 0;
    renderAdminUsers();
  });
  document.getElementById('adminUsersPager')?.addEventListener('click', (event) => {
    const btn = event.target.closest('[data-users-page-dir]');
    if (!btn) return;
    state.adminUsersPage += Number(btn.dataset.usersPageDir);
    renderAdminUsers();
  });

  document.getElementById('adminUsersList')?.addEventListener('click', async (event) => {
    const resetUser = event.target.closest('[data-reset-user]');
    if (resetUser) {
      try {
        const result = await fetchJSON(`/api/admin/users/${resetUser.dataset.resetUser}/password`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({}),
        });
        const user = (state.adminUsersCache || []).find((u) => String(u.id) === String(resetUser.dataset.resetUser));
        showCredentialModal({
          title: 'Новый пароль',
          hint: 'Пароль скопирован в буфер. Старые сессии пользователя сброшены.',
          login: user?.username || '',
          password: result.password,
        });
      } catch (error) {
        notify(error.message || 'Ошибка сброса', 'error');
      }
      return;
    }
    const toggle = event.target.closest('[data-toggle-active]');
    if (toggle) {
      const active = toggle.dataset.active === '1';
      try {
        await fetchJSON(`/api/admin/users/${toggle.dataset.toggleActive}/active`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ active: !active }),
        });
        await refreshAdminUsersCache();
      } catch (error) {
        notify(error.message || 'Не удалось изменить статус', 'error');
      }
      return;
    }
  });

  document.getElementById('adminResetsBox')?.addEventListener('click', async (event) => {
    const resolve = event.target.closest('[data-resolve-reset]');
    if (!resolve) return;
    try {
      const result = await fetchJSON(`/api/admin/password-resets/${resolve.dataset.resolveReset}/resolve`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
      });
      showCredentialModal({
        title: 'Пароль сброшен',
        hint: 'Пароль уже в буфере обмена. Передайте его пользователю.',
        login: '',
        password: result.password,
      });
      await refreshAdminUsersCache();
    } catch (error) {
      notify(error.message || 'Ошибка', 'error');
    }
  });
}
