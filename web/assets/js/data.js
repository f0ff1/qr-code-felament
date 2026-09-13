import { state } from './state.js';
import { fetchJSON } from './api.js';
import { normalizeSpool, normalizePrinter, normalizeProduct, normalizeJob } from './normalize.js';
import { fillSelectOptions } from './utils.js';
import { refreshDerivedState } from './derived.js';
import { notify } from './toasts.js';
import { renderSummary } from './render-summary.js';
import { renderOverview } from './render-overview.js';
import { renderSpools } from './render-spools.js';
import { renderPrinters } from './render-printers.js';
import { renderProducts } from './render-products.js';
import { renderJobs, renderJobOptions } from './render-jobs.js';

export function renderAll() {
  renderSummary();
  renderOverview();
  renderSpools();
  renderPrinters();
  renderProducts();
  renderJobs();
  renderJobOptions();
}

export async function loadData({ soft = false } = {}) {
  if (state.loading) return;
  state.loading = true;
  try {
    const [spools, printers, products, jobs] = await Promise.all([
      fetchJSON('/api/spools'),
      fetchJSON('/api/printers'),
      fetchJSON('/api/products'),
      fetchJSON('/api/print-jobs'),
    ]);

    state.spools = Array.isArray(spools) ? spools.map(normalizeSpool) : [];
    state.printers = Array.isArray(printers) ? printers.map(normalizePrinter) : [];
    state.products = Array.isArray(products) ? products.map(normalizeProduct) : [];
    state.jobs = Array.isArray(jobs) ? jobs.map(normalizeJob) : [];
    state.lastFullSyncAt = Date.now();

    refreshDerivedState();
    renderAll();
  } catch (error) {
    if (!soft) {
      notify('Не удалось загрузить данные панели', 'error');
    }
  } finally {
    state.loading = false;
  }
}

export async function loadJobsFast() {
  try {
    const jobs = await fetchJSON('/api/print-jobs', {}, 5000);
    if (!Array.isArray(jobs)) return;
    state.jobs = jobs.map(normalizeJob);
    refreshDerivedState();
    renderSummary();
    renderOverview();
    renderPrinters();
    renderJobs();
    renderSpools();
  } catch (_error) {
    // soft fail: next tick retries
  }
}

export async function loadAppConfig() {
  try {
    const cfg = await fetchJSON('/api/config');
    if (cfg && typeof cfg === 'object') {
      state.config = { ...state.config, ...cfg };
    }
  } catch (_error) {
    // keep defaults
  }
}

export async function loadFilamentCatalog() {
  try {
    const catalog = await fetchJSON('/api/filament-catalog');
    state.filamentCatalog = {
      brands: Array.isArray(catalog.brands) ? catalog.brands : [],
      materials: Array.isArray(catalog.materials) ? catalog.materials : [],
      colors: Array.isArray(catalog.colors) ? catalog.colors : [],
    };
    fillSelectOptions(document.getElementById('spoolManufacturer'), state.filamentCatalog.brands, 'Generic');
    fillSelectOptions(document.getElementById('spoolMaterial'), state.filamentCatalog.materials, 'PLA');
    fillSelectOptions(document.getElementById('spoolColor'), state.filamentCatalog.colors, 'чёрный');
  } catch (_error) {
    // keep empty selects; create will fail validation server-side
  }
}
