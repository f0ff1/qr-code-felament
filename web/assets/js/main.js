import { state, refs } from './state.js';
import { refreshDerivedState } from './derived.js';
import { loadData, loadJobsFast } from './data.js';
import { renderSummary } from './render-summary.js';
import { renderOverview } from './render-overview.js';
import { renderSpools } from './render-spools.js';
import { renderPrinters } from './render-printers.js';
import { renderJobs } from './render-jobs.js';
import { setProductPage } from './render-products.js';
import { bindProductCostEvents } from './product-cost.js';
import { clearAlertToasts, markToastSeen, notify } from './toasts.js';
import {
  createSpool,
  createPrinter,
  createProduct,
  createPrintJob,
  deleteItem,
  toggleJobStatus,
  confirmDraftJob,
  syncPrinterConnectionFields,
} from './actions.js';
import {
  syncBambuCloud,
  syncBambuCloudSaved,
  logoutBambuCloud,
  resendBambuCode,
  updateCloudLoginButton,
  refreshCloudAccountBadge,
} from './cloud.js';
import { ensureAuthenticated, submitLogin, logout, submitForgotPassword } from './auth.js';
import { connectEvents, loadNotificationHistory } from './events.js';
import { loadFilamentCatalog } from './data.js';
import { bindAdminEvents, loadSitesForAdmin, renderAdminUsers } from './admin.js';

function bindEvents() {
  // Local UI tick: progress/status without hitting the network.
  setInterval(() => {
    refreshDerivedState();
    // Завершение Bambu приходит с сервера один раз. Локальный ETA — только для ручных задач.
    for (const job of state.jobs) {
      if (job.source === 'bambu') continue;
      if (job.status !== 'completed' || state.completedNotified.has(job.id)) continue;
      state.completedNotified.add(job.id);
      notify(`Печать завершена: ${String(job.id).slice(0, 8)}`, 'success', 'Печать', 'print_completed');
      markToastSeen({ id: job.id, type: 'print_completed', payload: { job_id: job.id } });
    }
    renderSummary();
    renderOverview();
    renderSpools();
    renderPrinters();
    renderJobs();
  }, 1000);

  // Light API poll for jobs/status (~2s).
  setInterval(() => {
    loadJobsFast();
  }, 2000);

  // Full snapshot less often to keep DB/API load modest.
  setInterval(() => {
    loadData({ soft: true });
  }, 12000);

  bindProductCostEvents();

  refs.searchInput.addEventListener('input', (event) => {
    state.query = event.target.value;
    renderSpools();
  });

  document.addEventListener('click', async (event) => {
    const editWeightButton = event.target.closest('[data-edit-spool-id]');
    if (editWeightButton) {
      const id = editWeightButton.dataset.editSpoolId;
      const current = Number(editWeightButton.dataset.currentWeight ?? 0);
      const next = window.prompt('Введите новый остаток в граммах', String(current));
      if (next === null) return;
      const parsed = Number(next);
      if (!Number.isFinite(parsed) || parsed < 0) {
        notify('Остаток не может быть отрицательным', 'error', 'Склад');
        return;
      }

      try {
        const response = await fetch(`/api/spools/${id}/weight`, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ remainingWeight: Math.round(parsed) }),
        });
        if (!response.ok) {
          throw new Error('Не удалось обновить остаток');
        }
        notify(`Остаток обновлён: ${Math.round(parsed)} г`, 'success', 'Склад');
        await loadData();
      } catch (error) {
        notify(error.message || 'Ошибка обновления остатка', 'error', 'Склад');
      }
      return;
    }

    const deleteButton = event.target.closest('[data-delete-type]');
    if (deleteButton) {
      await deleteItem(deleteButton.dataset.deleteType, deleteButton.dataset.deleteId);
      return;
    }

    const jobToggle = event.target.closest('[data-job-toggle-id]');
    if (jobToggle) {
      await toggleJobStatus(jobToggle.dataset.jobToggleId, jobToggle.dataset.jobToggleAction);
      return;
    }

    const confirmDraft = event.target.closest('[data-confirm-draft-id]');
    if (confirmDraft) {
      await confirmDraftJob(confirmDraft.dataset.confirmDraftId);
    }
  });

  refs.filterGroup.addEventListener('click', (event) => {
    const button = event.target.closest('.chip');
    if (!button) return;

    refs.filterGroup.querySelectorAll('.chip').forEach((chip) => chip.classList.remove('active'));
    button.classList.add('active');
    state.filter = button.dataset.filter;
    renderOverview();
    renderSpools();
  });

  refs.refreshButton.addEventListener('click', () => {
    loadData();
  });

  refs.exportButton.addEventListener('click', () => {
    const csv = [
      ['Материал', 'Цвет', 'Производитель', 'Остаток', 'QR', 'Статус'].join(','),
      ...state.spools.map((spool) => [spool.material, spool.color, spool.manufacturer, spool.remaining, spool.qr, spool.status].join(',')),
    ].join('\n');

    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = 'filament-inventory.csv';
    anchor.click();
    URL.revokeObjectURL(url);
  });

  refs.navItems.forEach((button) => {
    button.addEventListener('click', () => {
      const view = button.dataset.view;

      refs.navItems.forEach((item) => item.classList.toggle('active', item === button));
      refs.viewPanels.forEach((panel) => {
        const isActive = panel.dataset.viewPanel === view;
        panel.classList.toggle('hidden', !isActive);
      });
      if (view === 'admin') {
        loadSitesForAdmin();
        renderAdminUsers();
      }
    });
  });

  refs.spoolForm.addEventListener('submit', createSpool);
  refs.cloudSyncForm?.addEventListener('submit', syncBambuCloud);
  refs.cloudSyncForm?.cloudVerifyCode?.addEventListener('input', updateCloudLoginButton);
  refs.cloudSyncOnlyBtn?.addEventListener('click', syncBambuCloudSaved);
  refs.cloudLogoutBtn?.addEventListener('click', logoutBambuCloud);
  refs.cloudResendCodeBtn?.addEventListener('click', resendBambuCode);
  updateCloudLoginButton();
  refs.productPager?.addEventListener('click', (event) => {
    const target = event.target.closest('[data-product-page], [data-product-page-dir]');
    if (!target) return;
    if (target.dataset.productPage != null) {
      setProductPage(Number(target.dataset.productPage));
      return;
    }
    if (target.dataset.productPageDir != null) {
      setProductPage(state.productPage + Number(target.dataset.productPageDir));
    }
  });
  refs.printerForm.addEventListener('submit', createPrinter);
  refs.printerForm?.connectionMode?.addEventListener('change', syncPrinterConnectionFields);
  syncPrinterConnectionFields();
  refs.productForm.addEventListener('submit', createProduct);
  refs.jobForm.addEventListener('submit', createPrintJob);
  refs.clearAlertsBtn?.addEventListener('click', (event) => {
    event.preventDefault();
    clearAlertToasts();
  });
}

function init() {
  bindEvents();
  bindAdminEvents();
  document.getElementById('loginForm')?.addEventListener('submit', submitLogin);
  document.getElementById('forgotPasswordBtn')?.addEventListener('click', submitForgotPassword);
  document.getElementById('logoutBtn')?.addEventListener('click', logout);
  ensureAuthenticated().then(async (ok) => {
    if (!ok) return;
    connectEvents();
    refreshCloudAccountBadge();
    loadFilamentCatalog();
    loadNotificationHistory();
    await loadSitesForAdmin();
    await renderAdminUsers();
    loadData();
  });
}

init();
