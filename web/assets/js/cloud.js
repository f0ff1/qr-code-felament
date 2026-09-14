import { state, refs } from './state.js';
import { fetchJSON } from './api.js';
import { notify } from './toasts.js';
import { loadData } from './data.js';

export function updateCloudLoginButton() {
  const form = refs.cloudSyncForm;
  const btn = refs.cloudLoginBtn || form?.querySelector('#cloudLoginBtn');
  const resend = refs.cloudResendCodeBtn;
  if (!btn) return;

  const code = String(form?.cloudVerifyCode?.value || '').trim();
  if (resend) {
    resend.classList.toggle('hidden', !state.bambuCodeSent);
  }

  btn.classList.remove('is-waiting');
  if (!state.bambuCodeSent) {
    btn.disabled = false;
    btn.textContent = 'Отправить код';
    return;
  }
  if (!code) {
    btn.disabled = true;
    btn.classList.add('is-waiting');
    btn.textContent = 'Введите код из email';
    return;
  }
  btn.disabled = false;
  btn.textContent = 'Войти в Bambu Lab';
}

export function setFormBusy(form, isBusy) {
  if (!form) return;
  form.dataset.busy = String(isBusy);
  const button = form.querySelector('button[type="submit"]');
  if (!button) return;
  if (button.id === 'cloudLoginBtn') {
    if (isBusy) {
      button.disabled = true;
      button.dataset.busyText = button.textContent;
      button.textContent = '…';
    } else {
      updateCloudLoginButton();
    }
    return;
  }
  button.disabled = isBusy;
  if (isBusy) {
    button.dataset.originalText = button.textContent;
    button.textContent = '…';
  } else if (button.dataset.originalText) {
    button.textContent = button.dataset.originalText;
  }
}

export function setCloudSessionUI(linked, email = '', region = 'us') {
  if (refs.cloudSignInPanel) {
    refs.cloudSignInPanel.classList.toggle('hidden', Boolean(linked));
  }
  if (refs.cloudSignedInPanel) {
    refs.cloudSignedInPanel.classList.toggle('hidden', !linked);
  }
  if (refs.cloudAccountBadge) {
    refs.cloudAccountBadge.textContent = linked
      ? `Вы вошли как ${email || 'Bambu Lab'} · сессия активна до выхода`
      : 'Не подключено — войдите один раз, как в YouTube через Google.';
  }
  const form = refs.cloudSyncForm;
  if (form?.cloudRegion && region) form.cloudRegion.value = region;
  if (form?.cloudEmail && email && !linked) form.cloudEmail.value = email;
  if (linked) {
    state.bambuCodeSent = false;
  }
  updateCloudLoginButton();
}

export async function refreshCloudAccountBadge() {
  try {
    const account = await fetchJSON('/api/bambu/cloud/account');
    setCloudSessionUI(Boolean(account.linked), account.email || '', account.region || 'us');
    const form = refs.cloudSyncForm;
    if (account.linked && form?.cloudEmail) {
      form.cloudEmail.value = account.email || '';
    }
  } catch {
    setCloudSessionUI(false);
  }
}

export async function syncBambuCloud(event) {
  event.preventDefault();
  const form = event.currentTarget;
  if (form.dataset.busy === 'true') return;

  const code = String(form.cloudVerifyCode?.value || '').trim();
  if (state.bambuCodeSent && !code) {
    notify('Введите код из email, затем нажмите «Войти в Bambu Lab»', 'warning', 'Bambu Lab');
    updateCloudLoginButton();
    return;
  }

  setFormBusy(form, true);

  try {
    const result = await fetchJSON('/api/bambu/cloud/sync', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: form.cloudEmail?.value || '',
        password: form.cloudPassword?.value || '',
        region: form.cloudRegion?.value || 'us',
        verifyCode: code,
      }),
    });

    if (result.needs_verification) {
      state.bambuCodeSent = true;
      notify(result.message || 'Код отправлен на email. Введите его ниже.', 'warning', 'Bambu Lab');
      form.cloudVerifyCode?.focus();
      return;
    }

    state.bambuCodeSent = false;
    const names = (result.devices || []).map((d) => d.name || d.serial).join(', ');
    notify(`Вход выполнен · синхронизировано: ${result.count || 0} · ${names}`, 'success', 'Bambu Lab');
    if (form.cloudPassword) form.cloudPassword.value = '';
    if (form.cloudVerifyCode) form.cloudVerifyCode.value = '';
    await refreshCloudAccountBadge();
    await loadData();
  } catch (error) {
    notify(error.message || 'Не удалось войти в Bambu Lab', 'error', 'Bambu Lab');
  } finally {
    setFormBusy(form, false);
    updateCloudLoginButton();
  }
}

export async function syncBambuCloudSaved() {
  const btn = refs.cloudSyncOnlyBtn;
  if (btn?.dataset.busy === 'true') return;
  if (btn) btn.dataset.busy = 'true';
  try {
    const result = await fetchJSON('/api/bambu/cloud/sync', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    });
    if (result.needs_verification) {
      notify(result.message || 'Нужен повторный вход с кодом из email', 'warning', 'Bambu Lab');
      state.bambuCodeSent = true;
      setCloudSessionUI(false);
      updateCloudLoginButton();
      return;
    }
    const names = (result.devices || []).map((d) => d.name || d.serial).join(', ');
    notify(`Синхронизировано: ${result.count || 0} · ${names}`, 'success', 'Bambu Lab');
    await loadData();
  } catch (error) {
    notify(error.message || 'Не удалось синхронизировать принтеры', 'error', 'Bambu Lab');
  } finally {
    if (btn) btn.dataset.busy = 'false';
  }
}

export async function resendBambuCode() {
  const form = refs.cloudSyncForm;
  const btn = refs.cloudResendCodeBtn;
  if (btn?.dataset.busy === 'true') return;
  if (btn) btn.dataset.busy = 'true';
  try {
    const result = await fetchJSON('/api/bambu/cloud/resend-code', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: form?.cloudEmail?.value || '',
        region: form?.cloudRegion?.value || 'us',
      }),
    });
    notify(result.message || 'Код отправлен на email', 'success', 'Bambu Lab');
    state.bambuCodeSent = true;
    updateCloudLoginButton();
    form?.cloudVerifyCode?.focus();
  } catch (error) {
    notify(error.message || 'Не удалось отправить код', 'error', 'Bambu Lab');
  } finally {
    if (btn) btn.dataset.busy = 'false';
  }
}

export async function logoutBambuCloud() {
  const btn = refs.cloudLogoutBtn;
  if (btn?.dataset.busy === 'true') return;
  if (btn) btn.dataset.busy = 'true';
  try {
    await fetchJSON('/api/bambu/cloud/account', { method: 'DELETE' });
    notify('Вы вышли из Bambu Lab. Сессия удалена.', 'success', 'Bambu Lab');
    const form = refs.cloudSyncForm;
    if (form?.cloudPassword) form.cloudPassword.value = '';
    if (form?.cloudVerifyCode) form.cloudVerifyCode.value = '';
    if (form?.cloudEmail) form.cloudEmail.value = '';
    state.bambuCodeSent = false;
    updateCloudLoginButton();
    await refreshCloudAccountBadge();
    await loadData();
  } catch (error) {
    notify(error.message || 'Не удалось выйти', 'error', 'Bambu Lab');
  } finally {
    if (btn) btn.dataset.busy = 'false';
  }
}
