const state = {
  spools: [],
  printers: [],
  products: [],
  jobs: [],
  events: [],
  serverEvents: [],
  toastSessionShown: {},
  localNotifyCooldown: {},
  pendingAlertCount: 0,
  filter: 'all',
  query: '',
  jobRuntimeStarts: {},
  loading: false,
  lastFullSyncAt: 0,
  completedNotified: new Set(),
  bambuCodeSent: false,
  productPage: 0,
  authenticated: false,
  authDisabled: false,
  username: '',
  config: {
    spool_low_weight_g: 200,
    printer_power_kw: 0.35,
    labor_per_hour: 12,
    electricity_rate_person: 0.1176,
    electricity_rate_legal: 0.18381,
  },
};

const refs = {
  totalSpools: document.getElementById('totalSpools'),
  lowStockCount: document.getElementById('lowStockCount'),
  activePrints: document.getElementById('activePrints'),
  printerCount: document.getElementById('printerCount'),
  updatedAt: document.getElementById('updatedAt'),
  spoolTableBody: document.getElementById('spoolTableBody'),
  printerList: document.getElementById('printerList'),
  jobList: document.getElementById('jobList'),
  activityList: document.getElementById('activityList'),
  searchInput: document.getElementById('searchInput'),
  filterGroup: document.getElementById('filterGroup'),
  refreshButton: document.getElementById('refreshButton'),
  alertCount: document.getElementById('alertCount'),
  clearAlertsBtn: document.getElementById('clearAlertsBtn'),
  exportButton: document.getElementById('exportButton'),
  navItems: [...document.querySelectorAll('.nav-item')],
  viewPanels: [...document.querySelectorAll('.view-panel')],
  overviewSpoolList: document.getElementById('overviewSpoolList'),
  overviewJobList: document.getElementById('overviewJobList'),
  productList: document.getElementById('productList'),
  productSpoolId: document.getElementById('productSpoolId'),
  productCostPreview: document.getElementById('productCostPreview'),
  spoolForm: document.getElementById('spoolForm'),
  printerForm: document.getElementById('printerForm'),
  cloudSyncForm: document.getElementById('cloudSyncForm'),
  cloudSyncStatus: document.getElementById('cloudSyncStatus'),
  cloudAccountBadge: document.getElementById('cloudAccountBadge'),
  cloudSignInPanel: document.getElementById('cloudSignInPanel'),
  cloudSignedInPanel: document.getElementById('cloudSignedInPanel'),
  cloudSyncOnlyBtn: document.getElementById('cloudSyncOnlyBtn'),
  cloudLogoutBtn: document.getElementById('cloudLogoutBtn'),
  cloudResendCodeBtn: document.getElementById('cloudResendCodeBtn'),
  cloudLoginBtn: document.getElementById('cloudLoginBtn'),
  productForm: document.getElementById('productForm'),
  productPager: document.getElementById('productPager'),
  jobForm: document.getElementById('jobForm'),
  spoolFormStatus: document.getElementById('spoolFormStatus'),
  printerFormStatus: document.getElementById('printerFormStatus'),
  productFormStatus: document.getElementById('productFormStatus'),
  jobFormStatus: document.getElementById('jobFormStatus'),
  jobPrinterId: document.getElementById('jobPrinterId'),
  jobProductId: document.getElementById('jobProductId'),
  jobSpoolId: document.getElementById('jobSpoolId'),
  toastStack: document.getElementById('toastStack'),
};

async function fetchJSON(url, options = {}, timeoutMs = 10000) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const finalOptions = {
      ...options,
      credentials: options.credentials || 'same-origin',
      signal: options.signal || controller.signal,
    };

    const response = await fetch(url, finalOptions);
    if (response.status === 401) {
      showLoginOverlay(true);
      throw new Error('Требуется вход в систему');
    }
    if (!response.ok) {
      const text = await response.text();
      throw new Error(translateApiError(text, response.status));
    }

    const contentType = response.headers.get('content-type') || '';
    return contentType.includes('application/json') ? response.json() : null;
  } catch (error) {
    if (error?.name === 'AbortError') {
      throw new Error('Сервер долго отвечает. Проверьте соединение и попробуйте ещё раз.');
    }
    if (error instanceof Error) throw error;
    throw new Error(translateApiError(String(error?.message || error || ''), 0));
  } finally {
    clearTimeout(timeoutId);
  }
}

function translateApiError(raw, status = 0) {
  const text = String(raw || '').trim();
  if (!text) {
    if (status >= 500) return 'Ошибка сервера. Попробуйте позже.';
    if (status === 404) return 'Объект не найден';
    if (status === 400) return 'Некорректные данные запроса';
    return 'Не удалось выполнить запрос';
  }

  let message = text;
  try {
    const parsed = JSON.parse(text);
    if (parsed && typeof parsed === 'object') {
      if (parsed.code === 'unauthorized') return 'Требуется вход в систему';
      message = String(parsed.message || parsed.error || parsed.status || text);
    }
  } catch (_error) {
    // plain text
  }

  const lower = message.toLowerCase();
  if (lower.includes('insufficient filament') || lower.includes('не хватает пластика')) {
    return 'На катушке не хватает пластика для этой печати';
  }
  if (lower.includes('not found')) return 'Объект не найден';
  if (lower.includes('method not allowed')) return 'Метод не поддерживается';
  if (lower.includes('invalid printer')) return 'Некорректный принтер';
  if (lower.includes('invalid product')) return 'Некорректный продукт';
  if (lower.includes('invalid spool')) return 'Некорректная катушка';
  if (lower.includes('invalid job')) return 'Некорректная задача';
  if (lower.includes('invalid payload') || lower.includes('invalid input')) {
    return 'Некорректные данные запроса';
  }
  if (message.startsWith('{') || message.startsWith('[')) {
    return status >= 500 ? 'Ошибка сервера. Попробуйте позже.' : 'Не удалось выполнить запрос';
  }
  return message;
}

function isActiveJobStatus(status) {
  return ['preparing', 'calibrating', 'printing', 'paused', 'queued', 'draft'].includes(String(status ?? '').toLowerCase());
}

function getSpoolStatus(spool) {
  const remaining = Number(spool.remaining_weight ?? spool.remaining ?? 0);
  const initial = Number(spool.initial_weight ?? spool.initial ?? 0);
  const spoolId = String(spool.id ?? '');
  const hasActiveJob = state.jobs.some((job) => {
    if (String(job.spoolId) !== spoolId) return false;
    return isActiveJobStatus(getEffectiveJobStatus(job));
  });

  if (hasActiveJob) return 'in_use';
  if (remaining <= 0) return 'empty';
  const lowThreshold = Number(state.config.spool_low_weight_g ?? 200);
  if (remaining < lowThreshold || (initial > 0 && remaining <= Math.max(50, initial * 0.2))) return 'low';
  return 'available';
}

function normalizeSpool(spool) {
  const remaining = Number(spool.remaining_weight ?? spool.remaining ?? 0);
  const initial = Number(spool.initial_weight ?? spool.initial ?? 0);
  const currentRemaining = spool.current_remaining != null
    ? Number(spool.current_remaining)
    : null;

  return {
    id: spool.id,
    material: spool.material,
    color: spool.color,
    manufacturer: spool.manufacturer,
    remaining,
    currentRemaining,
    initial,
    price: Number(spool.price ?? 0),
    status: 'available',
    qr: spool.qr_token ?? spool.qr ?? '—',
  };
}

function normalizePrinter(printer) {
  return {
    id: printer.id,
    name: printer.name,
    model: printer.model,
    status: String(printer.status ?? 'idle').toLowerCase(),
    lanEnabled: Boolean(printer.lan_enabled),
    lanHost: printer.lan_host ?? '',
    lanSerial: printer.lan_serial ?? '',
    cloudEnabled: Boolean(printer.cloud_enabled),
    cloudRegion: printer.cloud_region ?? 'us',
    cloudEmail: printer.cloud_email ?? '',
    cloudLinked: Boolean(printer.cloud_linked),
    needsVerification: Boolean(printer.needs_verification),
    connection: printer.connection || (printer.cloud_enabled ? 'cloud' : printer.lan_enabled ? 'lan' : 'none'),
    defaultSpoolId: printer.default_spool_id ?? null,
  };
}

function normalizeProduct(product) {
  return {
    id: product.id,
    name: product.name,
    description: product.description,
    material: product.material,
    estimatedWeight: Number(product.estimated_weight ?? product.estimatedWeight ?? 0),
    estimatedPrintTime: product.estimated_print_time ?? product.estimatedPrintTime ?? '0s',
    price: Number(product.price ?? 0),
    priceLegal: Number(product.price_legal ?? product.priceLegal ?? 0),
    billingMode: String(product.billing_mode ?? product.billingMode ?? 'person'),
  };
}

function normalizeJob(job) {
  const toDate = (value) => {
    if (!value) return null;
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? null : parsed;
  };

  return {
    id: job.id,
    status: String(job.status ?? 'queued').toLowerCase(),
    progress: Number(job.progress ?? 0),
    printer: job.printer_id ? String(job.printer_id).slice(0, 8) : '—',
    printerId: job.printer_id ?? null,
    product: job.product_id ? String(job.product_id).slice(0, 8) : '—',
    productId: job.product_id ?? null,
    spoolId: job.spool_id ?? null,
    startedAt: toDate(job.started_at),
    source: job.source ?? 'manual',
    fileName: job.file_name ?? '',
    isDraft: Boolean(job.is_draft),
    estimatedWeight: Number(job.estimated_weight ?? 0),
    consumedWeight: Number(job.consumed_weight ?? 0),
    needsFilamentTopUp: Boolean(job.needs_filament_top_up),
    remainingMinutes: Number(job.remaining_minutes ?? 0),
    estimatedDurationSec: Number(job.estimated_duration_sec ?? 0),
    layerCurrent: Number(job.layer_current ?? 0),
    layerTotal: Number(job.layer_total ?? 0),
  };
}

function parseDurationToMs(value) {
  if (!value || typeof value !== 'string') return 0;
  const normalized = value.trim().toLowerCase();
  const hourMatch = normalized.match(/(\d+)h/);
  const minuteMatch = normalized.match(/(\d+)m/);
  const secondMatch = normalized.match(/(\d+)s/);
  const hours = Number(hourMatch ? hourMatch[1] : 0);
  const minutes = Number(minuteMatch ? minuteMatch[1] : 0);
  const seconds = Number(secondMatch ? secondMatch[1] : 0);
  return ((hours * 60 * 60) + (minutes * 60) + seconds) * 1000;
}

function formatDuration(ms) {
  if (!Number.isFinite(ms) || ms <= 0) return '0 мин';
  const totalMinutes = Math.max(0, Math.round(ms / 60000));
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (hours > 0) {
    return `${hours}ч ${minutes}м`;
  }
  return `${minutes}м`;
}

function buildEstimatedPrintTime(hours, minutes) {
  const totalHours = Number(hours) || 0;
  const totalMinutes = Number(minutes) || 0;
  return `${totalHours}h${totalMinutes}m`;
}

function getEffectiveJobStatus(job) {
  if (!job) return 'queued';
  if (job.status === 'completed' || job.status === 'failed' || job.status === 'cancelled') {
    return job.status;
  }

  // Bambu: trust Cloud status as-is; isDraft is only "нужна катушка", not a print status.
  if (job.source === 'bambu') {
    return job.status === 'draft' ? 'printing' : job.status;
  }

  const progress = getJobProgress(job);
  if (isActiveJobStatus(job.status) && progress >= 100) {
    return 'completed';
  }

  return job.status === 'draft' ? 'printing' : job.status;
}

function getJobProgress(job) {
  if (!job) return 0;

  // Trust Cloud/printer % for Bambu jobs — never invent from wall-clock.
  if (job.source === 'bambu') {
    return Math.min(100, Math.max(0, Number(job.progress ?? 0)));
  }

  if (!['printing', 'paused', 'preparing', 'calibrating'].includes(job.status)) {
    return job.status === 'completed' ? 100 : Number(job.progress ?? 0);
  }

  const product = state.products.find((item) => item.id === job.productId);
  const durationMs = parseDurationToMs(product?.estimatedPrintTime || '0m');
  const startTime = state.jobRuntimeStarts[job.id] ?? (job.startedAt ? job.startedAt.getTime() : null);

  if (!startTime || durationMs <= 0) {
    return Number(job.progress ?? 0);
  }

  const elapsed = Math.max(0, Date.now() - startTime);
  const progress = (elapsed / durationMs) * 100;
  return Math.min(100, Math.max(0, progress));
}

function getJobTimeLabel(job) {
  if (!job) return '0 мин';
  if (Number(job.remainingMinutes) > 0 && ['printing', 'paused', 'preparing', 'calibrating'].includes(job.status)) {
    return `осталось ${formatDuration(Number(job.remainingMinutes) * 60000)}`;
  }
  if (Number(job.estimatedDurationSec) > 0) {
    return formatDuration(Number(job.estimatedDurationSec) * 1000);
  }
  const product = state.products.find((item) => item.id === job.productId);
  return formatDuration(parseDurationToMs(product?.estimatedPrintTime || '0m'));
}

function getJobRemainingEstimate(job) {
  if (Number(job?.estimatedWeight) > 0) {
    return Number(job.estimatedWeight);
  }
  if (Number(job?.consumedWeight) > 0) {
    return Number(job.consumedWeight);
  }
  const product = state.products.find((item) => item.id === job.productId);
  return Number(product?.estimatedWeight ?? 0);
}

function jobReservesSpool(job, spoolId) {
  if (!job || String(job.spoolId ?? '') !== String(spoolId ?? '')) return false;
  // Use raw status: getEffectiveJobStatus() may mark a live print as completed at 100%.
  const status = String(job.status ?? '').toLowerCase();
  return ['printing', 'paused', 'preparing', 'calibrating', 'queued'].includes(status);
}

function getJobPrintProgressPercent(job) {
  // Filament burn-down must follow printer/job %, same for manual and Bambu.
  let progress = Number(job?.progress ?? 0);
  if (!Number.isFinite(progress) || progress < 0) progress = 0;
  if (progress > 100) progress = 100;
  const status = String(job?.status ?? '').toLowerCase();
  const live = ['printing', 'paused', 'preparing', 'calibrating', 'queued'].includes(status);
  // Stale Cloud MQTT can report 100% while the job is still live with ETA left.
  if (live && Number(job?.remainingMinutes) > 0 && progress >= 100) {
    progress = 99;
  }
  return progress;
}

function getSpoolCurrentDisplay(spool) {
  // Future remaining is already reduced by the full reserved job weight in DB.
  // Current remaining adds back filament that has not been extruded yet.
  const futureRemaining = Number(spool.remaining ?? 0);
  const notYetUsed = state.jobs.reduce((total, job) => {
    if (!jobReservesSpool(job, spool.id)) return total;
    const weight = getJobRemainingEstimate(job);
    if (weight <= 0) return total;
    const progress = getJobPrintProgressPercent(job) / 100;
    return total + weight * (1 - progress);
  }, 0);
  return Math.max(0, futureRemaining + notYetUsed);
}

function getJobLayerLabel(job) {
  const total = Number(job?.layerTotal ?? 0);
  if (total <= 0) return '';
  const current = Math.max(0, Number(job?.layerCurrent ?? 0));
  return `слой ${current} из ${total}`;
}

function refreshDerivedState() {
  let completedNow = false;
  state.jobs.forEach((job) => {
    if (!isActiveJobStatus(job.status)) return;
    // Bambu completion comes from the printer/monitor, not client-side ETA.
    if (job.source === 'bambu') return;
    if (getJobProgress(job) < 100) return;
    job.status = 'completed';
    job.progress = 100;
    completedNow = true;
  });

  state.spools.forEach((spool) => {
    spool.status = getSpoolStatus(spool);
  });

  return completedNow;
}

function getActiveJobs() {
  return state.jobs.filter((job) => isActiveJobStatus(getEffectiveJobStatus(job)));
}

function updateClock() {
  refs.updatedAt.textContent = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function translateStatus(status) {
  const map = {
    draft: 'печать',
    available: 'достаточно',
    low: 'заканчивается',
    in_use: 'в работе',
    preparing: 'подготовка',
    calibrating: 'калибровка',
    printing: 'печать',
    paused: 'пауза',
    completed: 'завершено',
    failed: 'ошибка',
    cancelled: 'отменено',
    idle: 'простаивает',
    offline: 'офлайн',
    queued: 'в очереди',
    success: 'успешно',
    warning: 'предупреждение',
    error: 'ошибка',
    info: 'инфо',
  };

  return map[status] || status;
}

function formatProductPrice(product) {
  const mode = String(product?.billingMode || 'person');
  const person = Number(product?.price ?? 0);
  const legal = Number(product?.priceLegal ?? 0);
  if (mode === 'both') {
    return `Для физ. лица: ${person.toFixed(2)} ₽ · Для юр. лица: ${legal.toFixed(2)} ₽`;
  }
  if (mode === 'legal') {
    return `Для юр. лица: ${legal.toFixed(2)} ₽`;
  }
  return `Для физ. лица: ${person.toFixed(2)} ₽`;
}

async function loadData({ soft = false } = {}) {
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

async function loadJobsFast() {
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

function renderAll() {
  renderSummary();
  renderOverview();
  renderSpools();
  renderPrinters();
  renderProducts();
  renderJobs();
  renderJobOptions();
}

function getMaterialColor(material) {
  const palette = {
    PLA: '#58a6ff',
    PETG: '#7ee787',
    ABS: '#ffa657',
    'PLA+': '#bf7af2',
  };

  return palette[material] || '#c9d1d9';
}

function getFilteredSpools() {
  const query = state.query.trim().toLowerCase();

  return state.spools.filter((spool) => {
    const matchesQuery = !query || [spool.material, spool.color, spool.manufacturer, spool.qr].some((value) =>
      String(value).toLowerCase().includes(query)
    );
    if (!matchesQuery) return false;

    if (state.filter === 'all') return true;
    if (state.filter === 'low') return spool.status === 'low' || spool.status === 'empty' || Number(spool.remaining ?? 0) < Number(state.config.spool_low_weight_g ?? 200);
    if (state.filter === 'available') return spool.status === 'available' || Number(spool.remaining ?? 0) > 0;
    return true;
  });
}

function renderSummary() {
  refreshDerivedState();
  const lowCount = state.spools.filter((spool) => spool.status === 'low' || spool.status === 'empty').length;
  const activeCount = getActiveJobs().length;

  refs.totalSpools.textContent = String(state.spools.length);
  refs.lowStockCount.textContent = String(lowCount);
  refs.activePrints.textContent = String(activeCount);
  refs.printerCount.textContent = String(state.printers.length);
  updatePendingAlertCount();
  updateClock();
}

function getEffectivePrinterStatus(printerId) {
  const printer = state.printers.find((item) => String(item.id) === String(printerId));
  const apiStatus = String(printer?.status ?? '').toLowerCase();
  if (apiStatus && !['idle', ''].includes(apiStatus)) {
    return apiStatus;
  }
  const hasActiveJob = state.jobs.some((job) => {
    if (String(job.printerId) !== String(printerId)) return false;
    return isActiveJobStatus(getEffectiveJobStatus(job));
  });
  if (hasActiveJob) return 'printing';
  return apiStatus || 'idle';
}

function renderOverview() {
  refreshDerivedState();

  const spoolList = [...state.spools]
    .filter((spool) => {
      if (state.filter === 'all') return true;
      if (state.filter === 'low') return spool.status === 'low' || spool.status === 'empty' || Number(spool.remaining ?? 0) < Number(state.config.spool_low_weight_g ?? 200);
      if (state.filter === 'available') return spool.status === 'available' || Number(spool.remaining ?? 0) > 0;
      return false;
    })
    .sort((a, b) => b.remaining - a.remaining)
    .slice(0, 6);

  const taskList = state.jobs
    .filter((job) => {
      const effectiveStatus = getEffectiveJobStatus(job);
      if (state.filter === 'completed') return effectiveStatus === 'completed';
      if (state.filter === 'printing') return isActiveJobStatus(effectiveStatus);
      if (state.filter === 'all' || state.filter === 'low' || state.filter === 'available') {
        return isActiveJobStatus(effectiveStatus);
      }
      return false;
    })
    .slice(0, 6);

  if (!refs.overviewSpoolList || !refs.overviewJobList) return;

  if (state.filter === 'printing' || state.filter === 'completed') {
    refs.overviewSpoolList.innerHTML = '<div class="empty-state compact"><h3>Катушки скрыты фильтром</h3><p>Сейчас показаны только задачи.</p></div>';
  } else {
    refs.overviewSpoolList.innerHTML = spoolList.length
      ? spoolList.map((spool) => {
          const currentDisplay = getSpoolCurrentDisplay(spool);
          return `
            <div class="overview-item">
              <div>
                <strong>${spool.material}</strong>
                <span>${spool.color} • ${spool.manufacturer}</span>
              </div>
              <div class="overview-meta">
                <span>${Math.round(currentDisplay)}g</span>
                <em class="overview-status ${spool.status === 'low' ? 'low' : spool.status}">${spool.status === 'empty' ? 'закончился' : translateStatus(spool.status)}</em>
              </div>
            </div>
          `;
        }).join('')
      : '<div class="empty-state compact"><h3>Катушки отсутствуют</h3><p>Добавьте катушку, чтобы увидеть остатки.</p></div>';
  }

  refs.overviewJobList.innerHTML = taskList.length
    ? taskList.map((job) => {
        const status = getEffectiveJobStatus(job);
        const progress = status === 'completed' ? 100 : getJobProgress(job);
        const product = state.products.find((item) => item.id === job.productId);
        const title = job.fileName || product?.name || job.product;
        const layerLabel = getJobLayerLabel(job);
        const needsTopUp = jobNeedsFilamentTopUp(job);
        return `
          <div class="overview-item">
            <div>
              <strong>${title}</strong>
              <span>${translateStatus(status)}${job.isDraft ? ' • нужна катушка' : ''} • ${getJobTimeLabel(job)}${layerLabel ? ` • ${layerLabel}` : ''}</span>
              ${needsTopUp ? '<span class="job-flag top-up">догрузите пластик</span>' : ''}
              ${job.isDraft ? '<span class="job-flag needs-spool">нужна катушка</span>' : ''}
            </div>
            <div class="overview-meta">
              <span>${status === 'completed' ? '100%' : `${Math.round(progress)}%`}</span>
              <em class="overview-status ${status}">${translateStatus(status)}</em>
            </div>
          </div>
        `;
      }).join('')
    : '<div class="empty-state compact"><h3>Активных печатей нет</h3><p>Завершённые задачи сразу исчезают из этой графы.</p></div>';
}

function getSpoolBarStyle(remaining, initial) {
  const safeInitial = Math.max(1, Number(initial) || 1);
  const percent = Math.max(0, Math.min(100, (Number(remaining) / safeInitial) * 100));
  const hue = Math.max(0, 120 - (100 - percent) * 1.2);
  return {
    percent,
    color: `linear-gradient(90deg, hsl(${hue} 78% 55%), hsl(${Math.max(0, hue - 18)} 85% 48%))`,
  };
}

function renderSpools() {
  const filtered = getFilteredSpools();

  if (!filtered.length) {
    refs.spoolTableBody.innerHTML = `
      <tr>
        <td colspan="7">
          <div class="empty-state">
            <h3>Катушки не найдены</h3>
            <p>Попробуйте другой запрос или смените фильтр.</p>
          </div>
        </td>
      </tr>
    `;
    return;
  }

  refs.spoolTableBody.innerHTML = filtered
    .map((spool) => {
      const currentDisplay = getSpoolCurrentDisplay(spool);
      const futureRemaining = Number(spool.remaining ?? 0);
      const bar = getSpoolBarStyle(currentDisplay, spool.initial);
      const statusClass = spool.status === 'low' ? 'low' : spool.status;
      const displayStatus = spool.status === 'empty' ? 'закончился' : translateStatus(spool.status);
      const qrImageUrl = `/public/spools/qr/${spool.qr}`;
      const qrPageUrl = `/spool/${spool.qr}`;

      return `
        <tr class="spool-row ${statusClass}">
          <td>
            <span class="material-cell">
              <span class="material-swatch" style="background:${getMaterialColor(spool.material)}; color:${getMaterialColor(spool.material)}"></span>
              ${spool.material}
            </span>
          </td>
          <td>${spool.color}</td>
          <td>${spool.manufacturer}</td>
          <td>
            <div class="weight-metric">
              <span class="text">${Math.round(currentDisplay)}g</span>
              <button class="mini-btn edit-weight" data-edit-spool-id="${spool.id}" data-current-weight="${Math.round(Number(spool.remaining ?? 0))}" aria-label="Изменить остаток" title="Изменить остаток">✎</button>
              <div class="percent-track"><span style="width:${Math.max(0, Math.min(100, bar.percent))}%; background:${bar.color};"></span></div>
            </div>
          </td>
          <td>${futureRemaining}g</td>
          <td>
            <a class="qr-link" href="${qrPageUrl}" target="_blank" rel="noopener noreferrer">
              <img class="qr-thumb" src="${qrImageUrl}" alt="QR-${spool.qr}" title="Открыть статистику катушки" />
            </a>
          </td>
          <td class="status-cell">
            <div class="row-actions">
              <span class="status-pill ${statusClass}">${displayStatus}</span>
              <button class="mini-btn delete" data-delete-type="spool" data-delete-id="${spool.id}" aria-label="Удалить катушку" title="Удалить">×</button>
            </div>
          </td>
        </tr>
      `;
    })
    .join('');
}

function renderPrinters() {
  if (!state.printers.length) {
    refs.printerList.innerHTML = '<div class="empty-state"><h3>Принтеры не найдены</h3><p>Добавьте принтер, чтобы начать задачи.</p></div>';
    return;
  }

  refs.printerList.innerHTML = state.printers
    .map((printer) => {
      const status = getEffectivePrinterStatus(printer.id);
      let modeBadge = '<span class="printer-tag idle">manual</span>';
      if (printer.cloudEnabled) {
        const cloudLabel = printer.cloudLinked
          ? `Cloud${printer.status === 'printing' ? ' · печать' : ''}`
          : 'Cloud · код';
        modeBadge = `<span class="printer-tag cloud">${cloudLabel}</span>`;
      } else if (printer.lanEnabled) {
        modeBadge = `<span class="printer-tag lan">LAN ${printer.lanHost || 'on'}</span>`;
      }
      return `
        <div class="printer-card">
          <div class="printer-copy">
            <strong>${printer.name}</strong>
            <span>${printer.model}${printer.lanSerial ? ` • ${printer.lanSerial}` : ''}${printer.cloudEmail ? ` • ${printer.cloudEmail}` : ''}</span>
          </div>
          <div class="card-actions">
            ${modeBadge}
            <span class="printer-tag ${status}">${translateStatus(status)}</span>
            <button class="mini-btn delete" data-delete-type="printer" data-delete-id="${printer.id}" aria-label="Удалить принтер" title="Удалить">×</button>
          </div>
        </div>
      `;
    })
    .join('');
}

const PRODUCTS_PER_PAGE = 6;

function productCardHTML(product) {
  return `
    <div class="product-card">
      <strong>${product.name}</strong>
      <span>${product.material} • ${product.estimatedWeight} г • ${product.estimatedPrintTime}</span>
      <span>${product.description}</span>
      <span>${formatProductPrice(product)}</span>
      <div class="card-actions top-gap">
        <button class="mini-btn delete" data-delete-type="product" data-delete-id="${product.id}" aria-label="Удалить продукт" title="Удалить">×</button>
      </div>
    </div>
  `;
}

function renderProducts() {
  if (!refs.productList) return;

  if (!state.products.length) {
    refs.productList.innerHTML = '<div class="empty-state"><h3>Продукты ещё не добавлены</h3><p>Сначала добавьте продукт в каталог.</p></div>';
    if (refs.productPager) {
      refs.productPager.classList.add('hidden');
      refs.productPager.innerHTML = '';
    }
    return;
  }

  const pages = [];
  for (let i = 0; i < state.products.length; i += PRODUCTS_PER_PAGE) {
    pages.push(state.products.slice(i, i + PRODUCTS_PER_PAGE));
  }
  if (state.productPage >= pages.length) state.productPage = Math.max(0, pages.length - 1);

  refs.productList.className = 'product-list product-carousel';
  refs.productList.innerHTML = `
    <div class="product-carousel-track" style="transform: translateX(-${state.productPage * 100}%)">
      ${pages.map((page) => `
        <div class="product-page">
          ${page.map(productCardHTML).join('')}
        </div>
      `).join('')}
    </div>
  `;

  if (!refs.productPager) return;
  if (pages.length <= 1) {
    refs.productPager.classList.add('hidden');
    refs.productPager.innerHTML = '';
    return;
  }

  refs.productPager.classList.remove('hidden');
  refs.productPager.innerHTML = `
    <button type="button" class="product-page-btn" data-product-page-dir="-1" ${state.productPage <= 0 ? 'disabled' : ''}>‹</button>
    ${pages.map((_, index) => `
      <button type="button" class="product-page-dot ${index === state.productPage ? 'active' : ''}" data-product-page="${index}" aria-label="Страница ${index + 1}"></button>
    `).join('')}
    <button type="button" class="product-page-btn" data-product-page-dir="1" ${state.productPage >= pages.length - 1 ? 'disabled' : ''}>›</button>
  `;
}

function setProductPage(page) {
  const totalPages = Math.max(1, Math.ceil(state.products.length / PRODUCTS_PER_PAGE));
  state.productPage = Math.max(0, Math.min(totalPages - 1, page));
  renderProducts();
}

function renderJobs() {
  const ordered = [...state.jobs].sort((a, b) => {
    const aActive = isActiveJobStatus(getEffectiveJobStatus(a)) ? 0 : 1;
    const bActive = isActiveJobStatus(getEffectiveJobStatus(b)) ? 0 : 1;
    return aActive - bActive;
  });

  if (!ordered.length) {
    refs.jobList.innerHTML = '<div class="empty-state"><h3>Задач нет</h3><p>Нет активных задач печати.</p></div>';
    return;
  }

  refs.jobList.innerHTML = ordered
    .slice(0, 8)
    .map((job) => {
      const effectiveStatus = getEffectiveJobStatus(job);
      const isCompleted = effectiveStatus === 'completed';
      const needsSpool = Boolean(job.isDraft);
      const needsTopUp = !needsSpool && jobNeedsFilamentTopUp(job);
      const isPaused = job.status === 'paused' && !isCompleted;
      const actionLabel = isPaused ? 'Продолжить' : 'Приостановить';
      const progress = isCompleted ? 100 : Math.round(getJobProgress(job));
      const product = state.products.find((item) => item.id === job.productId);
      const title = job.fileName || product?.name || job.product;
      const layerLabel = getJobLayerLabel(job);
      return `
        <div class="job-item">
          <div class="job-title">
            <strong>${title}</strong>
            <span>Принтер ${job.printer}${job.source === 'bambu' ? ' • Bambu' : ''} • ${getJobTimeLabel(job)}${layerLabel ? ` • ${layerLabel}` : ''}${job.estimatedWeight > 0 ? ` • ${job.estimatedWeight} г` : ''}</span>
          </div>
          <div class="job-body">
            ${isCompleted ? '' : `
              <div class="job-progress-row">
                <span class="job-progress-label">Прогресс</span>
                <strong>${progress}%</strong>
              </div>
              <div class="job-progress-bar"><span style="width:${progress}%"></span></div>
            `}
          </div>
          <div class="card-actions">
            <span class="job-tag ${isCompleted ? 'completed' : effectiveStatus}">${translateStatus(isCompleted ? 'completed' : effectiveStatus)}</span>
            ${needsTopUp ? '<span class="job-tag topup">догрузите пластик</span>' : ''}
            ${needsSpool ? `<span class="job-tag draft">катушка?</span><button class="mini-btn" data-confirm-draft-id="${job.id}">Привязать</button>` : ''}
            ${isCompleted || needsSpool ? '' : `<button class="mini-btn" data-job-toggle-id="${job.id}" data-job-toggle-action="${isPaused ? 'resume' : 'pause'}">${actionLabel}</button>`}
            <button class="mini-btn delete" data-delete-type="job" data-delete-id="${job.id}" aria-label="Удалить задачу" title="Удалить">×</button>
          </div>
        </div>
      `;
    })
    .join('');
}

function renderJobOptions() {
  if (!refs.jobPrinterId || !refs.jobProductId || !refs.jobSpoolId) return;

  refs.jobPrinterId.innerHTML = state.printers.length
    ? state.printers.map((printer) => `<option value="${printer.id}">${printer.name} (${printer.model})</option>`).join('')
    : '<option value="">Нет принтеров</option>';

  refs.jobProductId.innerHTML = state.products.length
    ? state.products.map((product) => `<option value="${product.id}">${product.name}</option>`).join('')
    : '<option value="">Нет продуктов</option>';

  refs.jobSpoolId.innerHTML = state.spools.length
    ? state.spools.map((spool) => `<option value="${spool.id}">${spool.material} / ${spool.color} / ${spool.qr}</option>`).join('')
    : '<option value="">Нет катушек</option>';

  if (refs.productSpoolId) {
    refs.productSpoolId.innerHTML = state.spools.length
      ? state.spools.map((spool) => `<option value="${spool.id}">${spool.material} / ${spool.color} / ${spool.manufacturer}</option>`).join('')
      : '<option value="">Нет доступных катушек</option>';

    if (state.spools.length && !refs.productSpoolId.value) {
      refs.productSpoolId.selectedIndex = 0;
    }

    updateProductCostPreview();
  }

  const printerDefaultSpool = document.getElementById('printerDefaultSpoolId');
  if (printerDefaultSpool) {
    printerDefaultSpool.innerHTML = '<option value="">Автоматически</option>' + state.spools
      .map((spool) => `<option value="${spool.id}">${spool.material} / ${spool.color} / ${spool.qr}</option>`)
      .join('');
  }
}

function updateProductCostPreview() {
  if (!refs.productForm || !refs.productCostPreview) return;

  const cost = calculateProductCost(refs.productForm);
  const legalChecked = Boolean(refs.productForm.querySelector('.billing-toggle[value="legal"]')?.checked);
  const label = legalChecked ? 'Для юр. лица' : 'Для физ. лица';
  refs.productCostPreview.textContent = `${label}: ${cost.toFixed(2)} ₽`;

  const hiddenPrice = refs.productForm.querySelector('input[name="price"]');
  if (hiddenPrice) hiddenPrice.value = String(cost);
}

function fillSelectOptions(select, options, preferred = '') {
  if (!select) return;
  const current = select.value || preferred;
  select.innerHTML = options.map((value) => `<option value="${value}">${value}</option>`).join('');
  if (current && options.includes(current)) {
    select.value = current;
  } else if (options.length) {
    select.value = options.includes(preferred) ? preferred : options[0];
  }
}

async function loadFilamentCatalog() {
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

const TOAST_STORAGE_KEY = 'filament.toastState';
const ONESHOT_EVENT_TYPES = new Set([
  'print_completed',
  'print_started',
  'filament_short',
  'spool_created',
  'printer_created',
  'product_created',
  'bambu_cloud_synced',
  'bambu_cloud_logout',
]);
const REPEATABLE_EVENT_TYPES = new Set(['spool_low', 'spool_empty']);

function loadToastState() {
  try {
    const raw = JSON.parse(localStorage.getItem(TOAST_STORAGE_KEY) || '{}');
    return {
      seen: raw.seen && typeof raw.seen === 'object' ? raw.seen : {},
      dismissed: raw.dismissed && typeof raw.dismissed === 'object' ? raw.dismissed : {},
      sessionShown: state.toastSessionShown || {},
    };
  } catch (_error) {
    return { seen: {}, dismissed: {}, sessionShown: state.toastSessionShown || {} };
  }
}

function saveToastState(partial) {
  const current = loadToastState();
  const next = {
    seen: partial.seen || current.seen,
    dismissed: partial.dismissed || current.dismissed,
  };
  localStorage.setItem(TOAST_STORAGE_KEY, JSON.stringify(next));
}

function eventToastKey(evt) {
  const type = String(evt?.type || '');
  const spoolId = evt?.payload?.spool_id || evt?.spool_id || '';
  if ((type === 'spool_low' || type === 'spool_empty') && spoolId) {
    return `${type}:${spoolId}`;
  }
  if (type === 'print_completed' && (evt?.payload?.job_id || evt?.job_id)) {
    return `print_completed:${evt.payload?.job_id || evt.job_id}`;
  }
  if (type === 'print_started' && (evt?.payload?.job_id || evt?.job_id)) {
    return `print_started:${evt.payload?.job_id || evt.job_id}`;
  }
  if (type === 'filament_short' && (evt?.payload?.job_id || evt?.job_id)) {
    return `filament_short:${evt.payload?.job_id || evt.job_id}`;
  }
  return String(evt?.id || '');
}

function markToastSeen(evt) {
  const stateToast = loadToastState();
  const key = eventToastKey(evt);
  if (!key) return;
  stateToast.seen[key] = Date.now();
  if (evt?.id) stateToast.seen[evt.id] = Date.now();
  saveToastState(stateToast);
  state.toastSessionShown = state.toastSessionShown || {};
  state.toastSessionShown[key] = true;
}

function isToastDismissed(evt) {
  const stateToast = loadToastState();
  const key = eventToastKey(evt);
  if (key && stateToast.dismissed[key]) return true;
  if (evt?.id && stateToast.dismissed[evt.id]) return true;
  return false;
}

function isToastAlreadySeen(evt) {
  const stateToast = loadToastState();
  const key = eventToastKey(evt);
  if (key && stateToast.seen[key]) return true;
  if (evt?.id && stateToast.seen[evt.id]) return true;
  return false;
}

function wasShownThisSession(evt) {
  const key = eventToastKey(evt);
  return Boolean(key && state.toastSessionShown?.[key]);
}

function shouldToastEvent(evt, { fromHistory = false } = {}) {
  if (!evt?.type) return false;
  if (isToastDismissed(evt)) return false;

  const type = String(evt.type);
  if (ONESHOT_EVENT_TYPES.has(type)) {
    if (isToastAlreadySeen(evt)) return false;
    const cool = state.localNotifyCooldown?.[type];
    if (cool && Date.now() - cool < 8000) return false;
    return true;
  }
  if (REPEATABLE_EVENT_TYPES.has(type)) {
    if (wasShownThisSession(evt)) return false;
    return true;
  }
  return false;
}

function clearAlertToasts() {
  const stateToast = loadToastState();
  const now = Date.now();
  const dismiss = { ...stateToast.dismissed };

  (state.serverEvents || []).forEach((evt) => {
    const key = eventToastKey(evt);
    if (key) dismiss[key] = now;
    if (evt.id) dismiss[evt.id] = now;
  });
  (state.events || []).forEach((evt) => {
    if (evt.id) dismiss[evt.id] = now;
  });

  saveToastState({ seen: stateToast.seen, dismissed: dismiss });
  state.toastSessionShown = {};
  state.events = [];
  state.pendingAlertCount = 0;
  renderActivity();
  renderSummary();
  showToast('Уведомления очищены', 'info', 'Система');
}

function formatEventTime(iso) {
  const ts = Date.parse(iso);
  if (!Number.isFinite(ts)) return '';
  const ageMin = Math.round((Date.now() - ts) / 60000);
  if (ageMin < 1) return 'только что';
  if (ageMin < 60) return `${ageMin} мин назад`;
  const hours = Math.round(ageMin / 60);
  if (hours < 24) return `${hours} ч назад`;
  return new Date(ts).toLocaleString();
}

function jobNeedsFilamentTopUp(job) {
  if (!job || job.isDraft) return false;
  if (!isActiveJobStatus(getEffectiveJobStatus(job))) return false;
  if (job.needsFilamentTopUp) return true;
  if (job.estimatedWeight > 0 && job.consumedWeight < job.estimatedWeight) return true;
  if (!job.spoolId) return false;

  const spool = state.spools.find((item) => String(item.id) === String(job.spoolId));
  if (!spool) return false;

  const stillNeeded = Math.ceil(getJobRemainingEstimate(job) * (1 - getJobPrintProgressPercent(job) / 100));
  if (stillNeeded <= 0) return false;

  const current = getSpoolCurrentDisplay(spool);
  if (Number(spool.remaining ?? 0) <= 0 && stillNeeded > 0) return true;
  if (stillNeeded > current + 1) return true;

  const totalStillNeeded = state.jobs.reduce((sum, item) => {
    if (!jobReservesSpool(item, spool.id)) return sum;
    const need = Math.ceil(getJobRemainingEstimate(item) * (1 - getJobPrintProgressPercent(item) / 100));
    return sum + Math.max(0, need);
  }, 0);
  return totalStillNeeded > current + 1;
}

async function loadNotificationHistory() {
  try {
    const items = await fetchJSON('/api/notifications');
    if (!Array.isArray(items)) return;
    state.serverEvents = items.map((evt) => ({
      id: evt.id,
      type: evt.type,
      message: evt.message,
      created_at: evt.created_at,
      payload: evt.payload || {},
      spool_id: evt.payload?.spool_id,
      job_id: evt.payload?.job_id,
    }));

    state.events = state.serverEvents.slice(0, 12).map((evt) => {
      const mapped = mapServerEvent(String(evt.type || 'info'), evt.message || 'Событие');
      return {
        id: evt.id,
        type: mapped.level,
        label: mapped.message,
        time: formatEventTime(evt.created_at),
      };
    });
    renderActivity();

    // Страница была закрыта — догоняем непросмотренные уведомления.
    state.serverEvents.forEach((evt) => {
      if (!shouldToastEvent(evt, { fromHistory: true })) return;
      const mapped = mapServerEvent(String(evt.type || 'info'), evt.message || 'Событие');
      markToastSeen(evt);
      showToast(mapped.message, mapped.level, mapped.title);
    });
    updatePendingAlertCount();
  } catch (_error) {
    // history optional
  }
}

function updatePendingAlertCount() {
  const undismissed = (state.serverEvents || []).filter((evt) => !isToastDismissed(evt));
  state.pendingAlertCount = undismissed.length;
  if (refs.alertCount) {
    refs.alertCount.textContent = `${state.pendingAlertCount} уведомлений`;
  }
}

function handleServerEvent(data, { allowToast = true } = {}) {
  const type = String(data.type || 'info');
  const mapped = mapServerEvent(type, data.message || 'Получено событие');
  const evt = {
    id: data.id,
    type,
    message: data.message,
    created_at: data.created_at || new Date().toISOString(),
    payload: data.payload || {},
    spool_id: data.payload?.spool_id || data.spool_id,
    job_id: data.payload?.job_id || data.job_id,
  };

  state.serverEvents = [evt, ...(state.serverEvents || []).filter((item) => item.id !== evt.id)].slice(0, 50);
  appendActivity(mapped.message, mapped.level, data.id, evt.created_at);

  if (allowToast && shouldToastEvent(evt, { fromHistory: false })) {
    markToastSeen(evt);
    showToast(mapped.message, mapped.level, mapped.title);
  } else if (ONESHOT_EVENT_TYPES.has(type)) {
    // Уже показали локально при действии пользователя — фиксируем, чтобы не повторить.
    markToastSeen(evt);
  }
  updatePendingAlertCount();
}

function calculateProductCost(form) {
  const spoolId = form.productSpoolId?.value;
  const selectedSpool = state.spools.find((spool) => spool.id === spoolId);
  if (!selectedSpool) return 0;

  const weight = Number(form.estimatedWeight.value || 0);
  const hours = Number(form.estimatedHours.value || 0);
  const minutes = Number(form.estimatedMinutes.value || 0);
  const totalHours = hours + minutes / 60;

  const materialCost = (weight / 1000) * Number(selectedSpool.price || 0);
  const electricityRate = form.querySelector('.billing-toggle[value="legal"]').checked
    ? Number(state.config.electricity_rate_legal ?? 0.18381)
    : Number(state.config.electricity_rate_person ?? 0.1176);
  const electricityCost = totalHours * Number(state.config.printer_power_kw ?? 0.35) * electricityRate;
  const laborCost = totalHours * Number(state.config.labor_per_hour ?? 12);

  return Number((materialCost + electricityCost + laborCost).toFixed(2));
}

function syncBillingType(event) {
  const toggles = refs.productForm ? refs.productForm.querySelectorAll('.billing-toggle') : document.querySelectorAll('.billing-toggle');
  toggles.forEach((toggle) => {
    if (toggle !== event.target) {
      toggle.checked = false;
    }
  });
  updateProductCostPreview();
}

function bindProductCostEvents() {
  if (!refs.productForm) return;
  const toggles = refs.productForm.querySelectorAll('.billing-toggle');
  toggles.forEach((toggle) => toggle.addEventListener('change', () => {
    if (toggle.checked) {
      syncBillingType({ target: toggle });
    }
  }));

  ['productSpoolId', 'estimatedWeight', 'estimatedHours', 'estimatedMinutes'].forEach((fieldName) => {
    const field = refs.productForm.elements[fieldName];
    if (field) {
      field.addEventListener('input', updateProductCostPreview);
    }
  });
}

function showToast(message, type = 'info', title = '') {
  if (!refs.toastStack) return;

  const iconMap = {
    success: '✓',
    warning: '!',
    error: '×',
    info: '◌',
  };

  const titleMap = {
    success: 'Успешно',
    warning: 'Внимание',
    error: 'Ошибка',
    info: 'Уведомление',
  };

  const toast = document.createElement('div');
  toast.className = `toast ${type}`;
  toast.innerHTML = `
    <div class="toast-icon">${iconMap[type] || '◌'}</div>
    <div class="toast-copy">
      <strong>${title || titleMap[type] || 'Уведомление'}</strong>
      <span>${message}</span>
    </div>
    <button class="toast-close" type="button" aria-label="Закрыть">×</button>
  `;

  const remove = () => {
    toast.classList.add('hide');
    window.setTimeout(() => toast.remove(), 240);
  };

  toast.querySelector('.toast-close')?.addEventListener('click', remove);
  refs.toastStack.appendChild(toast);
  while (refs.toastStack.children.length > 4) {
    refs.toastStack.firstElementChild?.remove();
  }
  window.setTimeout(remove, 4200);
}

function notify(message, type = 'info', title = '', eventType = '') {
  appendActivity(message, type);
  showToast(message, type, title);
  if (eventType) {
    state.localNotifyCooldown = state.localNotifyCooldown || {};
    state.localNotifyCooldown[eventType] = Date.now();
  }
}

function appendActivity(message, type = 'info', id = '', createdAt = '') {
  const item = {
    id: id || (globalThis.crypto?.randomUUID ? globalThis.crypto.randomUUID() : String(Date.now())),
    type,
    label: message,
    time: createdAt ? formatEventTime(createdAt) : 'только что',
  };

  state.events = [item, ...state.events.filter((entry) => entry.id !== item.id)].slice(0, 12);
  renderActivity();
}

function renderActivity() {
  const iconMap = {
    success: '✓',
    warning: '!',
    error: '×',
    info: '◌',
  };

  const events = state.events.length ? state.events : [{ type: 'info', label: 'История пуста', time: '' }];

  refs.activityList.innerHTML = events
    .map((entry) => `
      <li class="activity-item">
        <div class="activity-icon">${iconMap[entry.type] || '◌'}</div>
        <div class="activity-copy">
          <strong>${entry.label}</strong>
          <span>${entry.time || ''}</span>
        </div>
      </li>
    `)
    .join('');
}

function connectEvents() {
  if (!window.EventSource) return;

  let reconnectToastAt = 0;
  const source = new EventSource('/events');

  source.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      handleServerEvent(data, { allowToast: true });
    } catch (_error) {
      // ignore malformed payloads
    }
    loadData({ soft: true });
  };

  source.onerror = () => {
    const now = Date.now();
    if (now - reconnectToastAt > 15000) {
      reconnectToastAt = now;
      showToast('Связь в реальном времени переподключается…', 'warning', 'Сеть');
    }
  };
}

function mapServerEvent(type, message) {
  if (type === 'spool_created') {
    return { title: 'Склад', message: 'Добавлена новая катушка', level: 'success' };
  }
  if (type === 'printer_created') {
    return { title: 'Принтеры', message: 'Добавлен новый принтер', level: 'success' };
  }
  if (type === 'product_created') {
    return { title: 'Продукты', message: 'Добавлен новый продукт', level: 'success' };
  }
  if (type === 'print_started') {
    return { title: 'Печать', message: 'Запущена новая задача печати', level: 'success' };
  }
  if (type === 'spool_low') {
    return { title: 'Склад', message: message || 'Мало пластика', level: 'warning' };
  }
  if (type === 'spool_empty') {
    return { title: 'Склад', message: message || 'Пластик закончился', level: 'error' };
  }
  if (type === 'print_completed') {
    return { title: 'Печать', message: 'Задача печати завершена', level: 'success' };
  }
  if (type === 'filament_short') {
    return {
      title: 'Печать',
      message: message || 'На катушке не хватает пластика — догрузите во время печати',
      level: 'warning',
    };
  }
  return { title: 'Событие', message, level: 'info' };
}

function setFormBusy(form, isBusy) {
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

function updateCloudLoginButton() {
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

async function createSpool(event) {
  event.preventDefault();
  const form = event.currentTarget;
  if (form.dataset.busy === 'true') return;
  setFormBusy(form, true);
  const payload = {
    material: form.material.value,
    color: form.color.value,
    manufacturer: form.manufacturer.value,
    initialWeight: Number(form.initialWeight.value),
    price: Number(form.price.value),
  };

  try {
    const result = await fetchJSON('/api/spools', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (refs.spoolFormStatus) refs.spoolFormStatus.innerHTML = '';
    notify(`Катушка ${result.qr_token} добавлена на склад`, 'success', 'Склад', 'spool_created');
    form.reset();
    await loadData();
  } catch (error) {
    if (refs.spoolFormStatus) refs.spoolFormStatus.innerHTML = '';
    notify(error.message || 'Не удалось создать катушку', 'error', 'Склад');
  } finally {
    setFormBusy(form, false);
  }
}

async function syncBambuCloud(event) {
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

async function syncBambuCloudSaved() {
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

async function resendBambuCode() {
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

async function logoutBambuCloud() {
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

function setCloudSessionUI(linked, email = '', region = 'us') {
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

async function refreshCloudAccountBadge() {
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

async function createPrinter(event) {
  event.preventDefault();
  const form = event.currentTarget;
  if (form.dataset.busy === 'true') return;
  setFormBusy(form, true);

  const mode = form.connectionMode?.value || 'none';
  const payload = {
    name: form.name.value,
    model: form.model.value,
    lanEnabled: mode === 'lan',
    cloudEnabled: false,
    lanHost: form.lanHost?.value || '',
    lanSerial: form.lanSerial?.value || '',
    lanAccessCode: form.lanAccessCode?.value || '',
    defaultSpoolId: form.defaultSpoolId?.value || '',
  };

  try {
    await fetchJSON('/api/printers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (refs.printerFormStatus) refs.printerFormStatus.textContent = '';
    notify(`Принтер «${form.name.value}» добавлен`, 'success', 'Принтеры', 'printer_created');
    form.reset();
    if (form.connectionMode) form.connectionMode.value = 'none';
    syncPrinterConnectionFields();
    await loadData();
  } catch (error) {
    if (refs.printerFormStatus) refs.printerFormStatus.textContent = '';
    notify(error.message || 'Не удалось создать принтер', 'error', 'Принтеры');
  } finally {
    setFormBusy(form, false);
  }
}

function syncPrinterConnectionFields() {
  const form = refs.printerForm;
  if (!form) return;
  const mode = form.connectionMode?.value || 'none';
  const lan = document.getElementById('printerLanFields');
  if (lan) lan.classList.toggle('hidden', mode !== 'lan');
}

async function createProduct(event) {
  event.preventDefault();
  const form = event.currentTarget;
  if (form.dataset.busy === 'true') return;
  setFormBusy(form, true);
  const hours = Number(form.estimatedHours.value || 0);
  const minutes = Number(form.estimatedMinutes.value || 0);
  const estimatedPrintTime = buildEstimatedPrintTime(hours, minutes);
  form.estimatedPrintTime.value = estimatedPrintTime;

  const spoolId = form.productSpoolId?.value;
  const selectedSpool = state.spools.find((spool) => spool.id === spoolId);
  if (!selectedSpool) {
    if (refs.productFormStatus) refs.productFormStatus.textContent = '';
    notify('Выберите доступную катушку', 'warning', 'Продукты');
    setFormBusy(form, false);
    return;
  }

  const legalChecked = Boolean(form.querySelector('.billing-toggle[value="legal"]')?.checked);
  const billingMode = legalChecked ? 'legal' : 'person';
  const computedPrice = calculateProductCost(form);
  form.price.value = String(computedPrice);

  try {
    const result = await fetchJSON('/api/products', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: form.name.value,
        description: form.description.value,
        material: selectedSpool.material,
        estimatedWeight: Number(form.estimatedWeight.value),
        estimatedPrintTime,
        price: billingMode === 'person' ? computedPrice : 0,
        priceLegal: billingMode === 'legal' ? computedPrice : 0,
        billingMode,
      }),
    });
    if (refs.productFormStatus) refs.productFormStatus.textContent = '';
    notify(`Продукт «${result.name}» добавлен`, 'success', 'Продукты', 'product_created');
    form.reset();
    const personToggle = form.querySelector('.billing-toggle[value="person"]');
    if (personToggle) personToggle.checked = true;
    if (refs.productCostPreview) refs.productCostPreview.textContent = '0.00 ₽';
    await loadData();
  } catch (error) {
    if (refs.productFormStatus) refs.productFormStatus.textContent = '';
    notify(error.message || 'Не удалось создать продукт', 'error', 'Продукты');
  } finally {
    setFormBusy(form, false);
  }
}

async function createPrintJob(event) {
  event.preventDefault();
  const form = event.currentTarget;
  if (form.dataset.busy === 'true') return;
  setFormBusy(form, true);
  const printerId = form.printerId.value;
  const productId = form.productId.value;
  const spoolId = form.spoolId.value;

  if (!printerId || !productId || !spoolId) {
    if (refs.jobFormStatus) refs.jobFormStatus.textContent = '';
    notify('Выберите принтер, продукт и катушку', 'warning', 'Печать');
    setFormBusy(form, false);
    return;
  }

  const product = state.products.find((item) => String(item.id) === String(productId));
  const spool = state.spools.find((item) => String(item.id) === String(spoolId));
  const need = Number(product?.estimatedWeight ?? 0);
  const have = Number(spool?.remaining ?? 0);
  if (product && spool && need > have) {
    const ok = window.confirm(
      `На катушке не хватает пластика (${have} г из нужных ${need} г).\n\nНажмите ОК, чтобы всё равно запустить печать со статусом «догрузите пластик».\nНажмите Отмена, чтобы не создавать задачу.`,
    );
    if (!ok) {
      setFormBusy(form, false);
      return;
    }
  }

  try {
    const result = await fetchJSON('/api/print-jobs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ printerId, productId, spoolId }),
    });
    state.jobRuntimeStarts[result.id] = Date.now();
    if (refs.jobFormStatus) refs.jobFormStatus.textContent = '';
    if (result.needs_filament_top_up) {
      notify('Задача запущена. На катушке не хватает пластика — догрузите во время печати', 'warning', 'Печать', 'filament_short');
    } else {
      notify('Задача печати запущена', 'success', 'Печать', 'print_started');
    }
    form.reset();
    await loadData();
  } catch (error) {
    if (refs.jobFormStatus) refs.jobFormStatus.textContent = '';
    notify(error.message || 'Не удалось создать задачу печати', 'error', 'Печать');
  } finally {
    setFormBusy(form, false);
  }
}

async function deleteItem(type, id) {
  const labels = {
    spool: 'катушку',
    printer: 'принтер',
    product: 'продукт',
    job: 'задачу',
  };

  const endpoints = {
    spool: '/api/spools/',
    printer: '/api/printers/',
    product: '/api/products/',
    job: '/api/print-jobs/',
  };

  const label = labels[type] || 'элемент';
  const confirmed = window.confirm(`Удалить ${label}?`);
  if (!confirmed) {
    return;
  }

  const url = `${endpoints[type]}${id}`;
  try {
    const response = await fetch(url, { method: 'DELETE' });
    if (!response.ok) {
      throw new Error('Не удалось удалить запись');
    }
    notify(`Удалено: ${label}`, 'success');
    await loadData();
  } catch (error) {
    notify(error.message || 'Ошибка удаления', 'error');
  }
}

async function toggleJobStatus(id, action) {
  try {
    const response = await fetch(`/api/print-jobs/${id}/${action}`, { method: 'POST' });
    if (!response.ok) {
      throw new Error('Не удалось изменить состояние задачи');
    }
    notify(action === 'pause' ? 'Задача приостановлена' : 'Задача продолжена', 'success', 'Печать');
    await loadData();
  } catch (error) {
    notify(error.message || 'Ошибка изменения статуса', 'error', 'Печать');
  }
}

async function confirmDraftJob(jobId) {
  if (!state.products.length || !state.spools.length) {
    notify('Сначала добавьте продукт и катушку', 'warning', 'Печать');
    return;
  }
  const productOptions = state.products.map((p, i) => `${i + 1}. ${p.name}`).join('\n');
  const productPick = window.prompt(`Выберите продукт (номер):\n${productOptions}`, '1');
  if (productPick === null) return;
  const product = state.products[Number(productPick) - 1];
  if (!product) {
    notify('Неверный номер продукта', 'error', 'Печать');
    return;
  }

  const spoolOptions = state.spools.map((s, i) => `${i + 1}. ${s.material}/${s.color}/${s.qr}`).join('\n');
  const spoolPick = window.prompt(`Выберите катушку (номер):\n${spoolOptions}`, '1');
  if (spoolPick === null) return;
  const spool = state.spools[Number(spoolPick) - 1];
  if (!spool) {
    notify('Неверный номер катушки', 'error', 'Печать');
    return;
  }

  const need = Number(product.estimatedWeight ?? 0);
  const have = Number(spool.remaining ?? 0);
  if (need > have) {
    const ok = window.confirm(
      `На катушке не хватает пластика (${have} г из нужных ${need} г).\n\nНажмите ОК, чтобы привязать со статусом «догрузите пластик».\nНажмите Отмена, чтобы не продолжать.`,
    );
    if (!ok) return;
  }

  try {
    const result = await fetchJSON(`/api/print-jobs/${jobId}/confirm`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ productId: product.id, spoolId: spool.id }),
    });
    if (result?.needs_filament_top_up || need > have) {
      notify('Катушка привязана. На катушке не хватает пластика — догрузите во время печати', 'warning', 'Печать', 'filament_short');
    } else {
      notify('Черновик подтверждён, пластик зарезервирован', 'success', 'Печать');
    }
    await loadData();
  } catch (error) {
    notify(error.message || 'Не удалось привязать катушку', 'error', 'Печать');
  }
}

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

async function loadAppConfig() {
  try {
    const cfg = await fetchJSON('/api/config');
    if (cfg && typeof cfg === 'object') {
      state.config = { ...state.config, ...cfg };
    }
  } catch (_error) {
    // keep defaults
  }
}

function showLoginOverlay(visible) {
  const overlay = document.getElementById('loginOverlay');
  if (!overlay) return;
  overlay.classList.toggle('hidden', !visible);
  overlay.setAttribute('aria-hidden', visible ? 'false' : 'true');
  const page = document.querySelector('.page-shell');
  if (page) page.style.visibility = visible ? 'hidden' : '';
}

function updateAuthChrome() {
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

async function ensureAuthenticated() {
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

async function submitLogin(event) {
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

async function logout() {
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

function init() {
  bindEvents();
  document.getElementById('loginForm')?.addEventListener('submit', submitLogin);
  document.getElementById('logoutBtn')?.addEventListener('click', logout);
  ensureAuthenticated().then((ok) => {
    if (!ok) return;
    connectEvents();
    refreshCloudAccountBadge();
    loadFilamentCatalog();
    loadNotificationHistory();
    loadData();
  });
}

init();
