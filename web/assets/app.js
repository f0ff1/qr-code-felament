const state = {
  spools: [],
  printers: [],
  products: [],
  jobs: [],
  events: [],
  filter: 'all',
  query: '',
  jobRuntimeStarts: {},
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
  productForm: document.getElementById('productForm'),
  jobForm: document.getElementById('jobForm'),
  spoolFormStatus: document.getElementById('spoolFormStatus'),
  printerFormStatus: document.getElementById('printerFormStatus'),
  productFormStatus: document.getElementById('productFormStatus'),
  jobFormStatus: document.getElementById('jobFormStatus'),
  jobPrinterId: document.getElementById('jobPrinterId'),
  jobProductId: document.getElementById('jobProductId'),
  jobSpoolId: document.getElementById('jobSpoolId'),
};

async function fetchJSON(url, options = {}, timeoutMs = 10000) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const finalOptions = {
      ...options,
      signal: options.signal || controller.signal,
    };

    const response = await fetch(url, finalOptions);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || `Request failed: ${response.status}`);
    }

    const contentType = response.headers.get('content-type') || '';
    return contentType.includes('application/json') ? response.json() : null;
  } catch (error) {
    if (error?.name === 'AbortError') {
      throw new Error('Сервер долго отвечает. Проверьте соединение и попробуйте ещё раз.');
    }
    throw error;
  } finally {
    clearTimeout(timeoutId);
  }
}

function getSpoolStatus(spool) {
  const remaining = Number(spool.remaining_weight ?? spool.remaining ?? 0);
  const initial = Number(spool.initial_weight ?? spool.initial ?? 0);
  const base = String(spool.status ?? 'available').toLowerCase();

  if (remaining <= 0 || base === 'empty') return 'empty';
  if (remaining <= Math.max(50, initial * 0.2)) return 'low';
  if (base === 'in_use') return 'in_use';
  return 'available';
}

function normalizeSpool(spool) {
  const remaining = Number(spool.remaining_weight ?? spool.remaining ?? 0);
  const initial = Number(spool.initial_weight ?? spool.initial ?? 0);

  return {
    id: spool.id,
    material: spool.material,
    color: spool.color,
    manufacturer: spool.manufacturer,
    remaining,
    initial,
    status: getSpoolStatus(spool),
    qr: spool.qr_token ?? spool.qr ?? '—',
  };
}

function normalizePrinter(printer) {
  return {
    id: printer.id,
    name: printer.name,
    model: printer.model,
    status: String(printer.status ?? 'idle').toLowerCase(),
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
    product: job.product_id ? String(job.product_id).slice(0, 8) : '—',
    productId: job.product_id ?? null,
    spoolId: job.spool_id ?? null,
    startedAt: toDate(job.started_at),
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

  const progress = getJobProgress(job);
  if (['printing', 'paused'].includes(job.status) && progress >= 100) {
    return 'completed';
  }

  return job.status;
}

function getJobProgress(job) {
  if (!job) return 0;

  if (!['printing', 'paused'].includes(job.status)) {
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
  const capped = Math.min(100, Math.max(0, progress));
  return capped >= 100 ? 100 : capped;
}

function getJobRemainingEstimate(job) {
  const product = state.products.find((item) => item.id === job.productId);
  return Number(product?.estimatedWeight ?? 0);
}

function getSpoolCurrentDisplay(spool) {
  const activeJobs = state.jobs.filter((job) => job.spoolId === spool.id && ['printing', 'paused'].includes(job.status));
  const futureRemaining = Number(spool.remaining ?? 0);
  const consumedFromJobs = activeJobs.reduce((total, job) => {
    const weight = getJobRemainingEstimate(job);
    const progress = getJobProgress(job) / 100;
    return total + weight * (1 - progress);
  }, 0);
  return Math.max(0, futureRemaining + consumedFromJobs);
}

function updateClock() {
  refs.updatedAt.textContent = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function translateStatus(status) {
  const map = {
    available: 'доступно',
    low: 'мало',
    in_use: 'в работе',
    printing: 'печать',
    idle: 'простаивает',
    queued: 'в очереди',
    success: 'успешно',
    warning: 'предупреждение',
    error: 'ошибка',
    info: 'инфо',
    completed: 'завершено',
  };

  return map[status] || status;
}

async function loadData() {
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

    renderSummary();
    renderOverview();
    renderSpools();
    renderPrinters();
    renderProducts();
    renderJobs();
    renderJobOptions();
  } catch (error) {
    appendActivity('Не удалось загрузить данные панели', 'error');
  }
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

    if (state.filter === 'all') {
      return matchesQuery;
    }
    if (state.filter === 'low') {
      return matchesQuery && Number(spool.remaining ?? 0) < 200;
    }
    if (state.filter === 'available') {
      return matchesQuery && Number(spool.remaining ?? 0) > 0;
    }

    return false;
  });
}

function renderSummary() {
  refs.totalSpools.textContent = String(state.spools.length);
  refs.lowStockCount.textContent = String(state.spools.filter((spool) => spool.status === 'low' || spool.status === 'empty').length);
  refs.activePrints.textContent = String(state.jobs.filter((job) => ['printing', 'paused', 'queued'].includes(job.status)).length);
  refs.printerCount.textContent = String(state.printers.length);
  refs.alertCount.textContent = `${Math.max(0, state.spools.filter((spool) => spool.status === 'low' || spool.status === 'empty').length)} уведомлений`;
  updateClock();
}

function getEffectivePrinterStatus(printerId) {
  const hasActiveJob = state.jobs.some((job) => String(job.printerId) === String(printerId) && ['printing', 'paused'].includes(job.status));
  return hasActiveJob ? 'printing' : 'idle';
}

function renderOverview() {
  state.jobs.forEach((job) => {
    if (['printing', 'paused'].includes(job.status) && getJobProgress(job) >= 100) {
      job.status = 'completed';
    }
  });

  const spoolList = [...state.spools]
    .filter((spool) => {
      if (state.filter === 'all') return true;
      if (state.filter === 'low') return Number(spool.remaining ?? 0) < 200;
      if (state.filter === 'available') return Number(spool.remaining ?? 0) > 0;
      return false;
    })
    .sort((a, b) => b.remaining - a.remaining)
    .slice(0, 4);

  const taskList = state.jobs.filter((job) => {
    const effectiveStatus = getEffectiveJobStatus(job);
    if (state.filter === 'all') return true;
    if (state.filter === 'printing') return ['printing', 'paused', 'queued'].includes(effectiveStatus);
    if (state.filter === 'completed') return effectiveStatus === 'completed';
    return false;
  }).slice(0, 4);

  if (!refs.overviewSpoolList || !refs.overviewJobList) return;

  if (state.filter === 'printing' || state.filter === 'completed' || state.filter === 'low' || state.filter === 'available') {
    refs.overviewSpoolList.innerHTML = state.filter === 'low' || state.filter === 'available'
      ? (spoolList.length
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
        : '<div class="empty-state compact"><h3>Катушки не найдены</h3><p>Нет катушек по этому фильтру.</p></div>')
      : '<div class="empty-state compact"><h3>Катушки скрыты фильтром</h3><p>Сейчас показаны только задачи.</p></div>';
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
        const estimated = product?.estimatedPrintTime || '0m';
        return `
          <div class="overview-item">
            <div>
              <strong>Задача ${job.id.slice(0, 8)}</strong>
              <span>Печать • ${product?.name || job.product} • ${formatDuration(parseDurationToMs(estimated))}</span>
            </div>
            <div class="overview-meta">
              <span>${status === 'completed' ? '100%' : `${Math.round(progress)}%`}</span>
              <em class="overview-status ${status}">${translateStatus(status)}</em>
            </div>
          </div>
        `;
      }).join('')
    : '<div class="empty-state compact"><h3>Задач нет</h3><p>Новые задачи появятся здесь сразу после запуска.</p></div>';
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
      const qrUrl = `/public/spools/qr/${spool.qr}`;

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
              <div class="percent-track"><span style="width:${Math.max(0, Math.min(100, bar.percent))}%; background:${bar.color};"></span></div>
            </div>
          </td>
          <td>${futureRemaining}g</td>
          <td>
            <a class="qr-link" href="${qrUrl}" target="_blank" rel="noopener noreferrer" download>
              <img class="qr-thumb" src="${qrUrl}" alt="QR-${spool.qr}" title="Открыть QR-код и скачать" />
            </a>
          </td>
          <td>
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
      return `
        <div class="printer-card">
          <div class="printer-copy">
            <strong>${printer.name}</strong>
            <span>${printer.model}</span>
          </div>
          <div class="card-actions">
            <span class="printer-tag ${status}">${translateStatus(status)}</span>
            <button class="mini-btn delete" data-delete-type="printer" data-delete-id="${printer.id}" aria-label="Удалить принтер" title="Удалить">×</button>
          </div>
        </div>
      `;
    })
    .join('');
}

function renderProducts() {
  if (!refs.productList) return;

  if (!state.products.length) {
    refs.productList.innerHTML = '<div class="empty-state"><h3>Продукты ещё не добавлены</h3><p>Сначала добавьте продукт в каталог.</p></div>';
    return;
  }

  refs.productList.innerHTML = state.products
    .map((product) => `
      <div class="product-card">
        <strong>${product.name}</strong>
        <span>${product.material} • ${product.estimatedWeight} г • ${product.estimatedPrintTime}</span>
        <span>${product.description}</span>
        <span>Себестоимость: ${Number(product.price ?? 0).toFixed(2)} ₽</span>
        <div class="card-actions top-gap">
          <button class="mini-btn delete" data-delete-type="product" data-delete-id="${product.id}" aria-label="Удалить продукт" title="Удалить">×</button>
        </div>
      </div>
    `)
    .join('');
}

function renderJobs() {
  if (!state.jobs.length) {
    refs.jobList.innerHTML = '<div class="empty-state"><h3>Задач нет</h3><p>Нет активных задач печати.</p></div>';
    return;
  }

  refs.jobList.innerHTML = state.jobs
    .slice(0, 4)
    .map((job) => {
      const effectiveStatus = getEffectiveJobStatus(job);
      const isCompleted = effectiveStatus === 'completed';
      const isPaused = job.status === 'paused' && !isCompleted;
      const actionLabel = isPaused ? 'Продолжить' : 'Приостановить';
      const progress = isCompleted ? 100 : Math.round(getJobProgress(job));
      const product = state.products.find((item) => item.id === job.productId);
      const estimated = product?.estimatedPrintTime || '0m';
      return `
        <div class="job-item">
          <div class="job-title">
            <strong>Задача ${job.id.slice(0, 8)}</strong>
            <span>Принтер ${job.printer} • Продукт ${product?.name || job.product} • ${formatDuration(parseDurationToMs(estimated))}</span>
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
            ${isCompleted ? '' : `<button class="mini-btn" data-job-toggle-id="${job.id}" data-job-toggle-action="${isPaused ? 'resume' : 'pause'}">${actionLabel}</button>`}
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
}

function updateProductCostPreview() {
  if (!refs.productForm || !refs.productCostPreview) return;

  const cost = calculateProductCost(refs.productForm);
  refs.productCostPreview.textContent = `${cost.toFixed(2)} ₽`;

  const hiddenPrice = refs.productForm.querySelector('input[name="price"]');
  if (hiddenPrice) hiddenPrice.value = String(cost);
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
  const electricityRate = form.querySelector('.billing-toggle[value="legal"]').checked ? 0.18381 : 0.1176;
  const electricityCost = totalHours * 0.35 * electricityRate;
  const laborCost = totalHours * 12;

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

function appendActivity(message, type = 'info') {
  const iconMap = {
    success: '✓',
    warning: '!',
    error: '×',
    info: '◌',
  };

  const item = {
    id: globalThis.crypto?.randomUUID ? globalThis.crypto.randomUUID() : String(Date.now()),
    type,
    label: message,
    time: 'только что',
  };

  state.events = [item, ...state.events].slice(0, 8);
  renderActivity();
}

function renderActivity() {
  const iconMap = {
    success: '✓',
    warning: '!',
    error: '×',
    info: '◌',
  };

  const events = state.events.length ? state.events : [{ type: 'info', label: 'Система готова', time: 'только что' }];

  refs.activityList.innerHTML = events
    .map((entry) => `
      <li class="activity-item">
        <div class="activity-icon">${iconMap[entry.type] || '◌'}</div>
        <div class="activity-copy">
          <strong>${entry.label}</strong>
          <span>${entry.time}</span>
        </div>
      </li>
    `)
    .join('');
}

function connectEvents() {
  if (!window.EventSource) return;

  const source = new EventSource('/events');

  source.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      const message = data.message || data.type || 'Получено событие';
      appendActivity(message, data.type || 'info');
    } catch (error) {
      appendActivity('Получено событие в реальном времени', 'info');
    }
  };

  source.onerror = () => {
    appendActivity('Связь в реальном времени потеряна. Повторное подключение...', 'warning');
  };
}

function setFormBusy(form, isBusy) {
  if (!form) return;
  form.dataset.busy = String(isBusy);
  const button = form.querySelector('button[type="submit"]');
  if (button) {
    button.disabled = isBusy;
    if (isBusy) {
      button.dataset.originalText = button.textContent;
      button.textContent = '…';
    } else if (button.dataset.originalText) {
      button.textContent = button.dataset.originalText;
    }
  }
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
    const qrUrl = `/public/spools/qr/${result.qr_token}`;
    refs.spoolFormStatus.innerHTML = `
      <div class="created-item">
        <strong>Катушка создана</strong>
        <a href="${qrUrl}" target="_blank" rel="noopener noreferrer" download>
          <img src="${qrUrl}" alt="QR-код" />
        </a>
      </div>
    `;
    appendActivity(`Катушка создана: ${result.qr_token}`, 'success');
    form.reset();
    await loadData();
  } catch (error) {
    refs.spoolFormStatus.textContent = error.message;
    appendActivity('Не удалось создать катушку', 'error');
  } finally {
    setFormBusy(form, false);
  }
}

async function createPrinter(event) {
  event.preventDefault();
  const form = event.currentTarget;
  if (form.dataset.busy === 'true') return;
  setFormBusy(form, true);

  try {
    const result = await fetchJSON('/api/printers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: form.name.value,
        model: form.model.value,
      }),
    });
    refs.printerFormStatus.textContent = `Принтер создан: ${result.id}`;
    appendActivity(`Принтер создан: ${form.name.value}`, 'success');
    form.reset();
    await loadData();
  } catch (error) {
    refs.printerFormStatus.textContent = error.message;
    appendActivity('Не удалось создать принтер', 'error');
  } finally {
    setFormBusy(form, false);
  }
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
    refs.productFormStatus.textContent = 'Выберите доступную катушку';
    setFormBusy(form, false);
    return;
  }

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
        price: Number(form.price.value),
      }),
    });
    refs.productFormStatus.textContent = `Продукт создан: ${result.name}`;
    appendActivity(`Продукт создан: ${result.name}`, 'success');
    form.reset();
    refs.productCostPreview.textContent = '0.00 ₽';
    await loadData();
  } catch (error) {
    refs.productFormStatus.textContent = error.message;
    appendActivity('Не удалось создать продукт', 'error');
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
    refs.jobFormStatus.textContent = 'Выберите принтер, продукт и катушку';
    setFormBusy(form, false);
    return;
  }

  try {
    const result = await fetchJSON('/api/print-jobs', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ printerId, productId, spoolId }),
    });
    state.jobRuntimeStarts[result.id] = Date.now();
    refs.jobFormStatus.textContent = `Задача создана: ${result.id}`;
    appendActivity(`Задача запускается: ${result.id}`, 'success');
    form.reset();
    await loadData();
  } catch (error) {
    refs.jobFormStatus.textContent = error.message;
    appendActivity('Не удалось создать задачу печати', 'error');
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
    appendActivity(`Удалено: ${label}`, 'success');
    await loadData();
  } catch (error) {
    appendActivity(error.message || 'Ошибка удаления', 'error');
  }
}

async function toggleJobStatus(id, action) {
  try {
    const response = await fetch(`/api/print-jobs/${id}/${action}`, { method: 'POST' });
    if (!response.ok) {
      throw new Error('Не удалось изменить состояние задачи');
    }
    appendActivity(action === 'pause' ? 'Задача приостановлена' : 'Задача продолжена', 'success');
    await loadData();
  } catch (error) {
    appendActivity(error.message || 'Ошибка изменения статуса', 'error');
  }
}

function bindEvents() {
  setInterval(() => {
    renderSummary();
    renderOverview();
    renderSpools();
    renderJobs();
  }, 5000);

  bindProductCostEvents();

  refs.searchInput.addEventListener('input', (event) => {
    state.query = event.target.value;
    renderSpools();
  });

  document.addEventListener('click', async (event) => {
    const deleteButton = event.target.closest('[data-delete-type]');
    if (deleteButton) {
      await deleteItem(deleteButton.dataset.deleteType, deleteButton.dataset.deleteId);
      return;
    }

    const jobToggle = event.target.closest('[data-job-toggle-id]');
    if (jobToggle) {
      await toggleJobStatus(jobToggle.dataset.jobToggleId, jobToggle.dataset.jobToggleAction);
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
  refs.printerForm.addEventListener('submit', createPrinter);
  refs.productForm.addEventListener('submit', createProduct);
  refs.jobForm.addEventListener('submit', createPrintJob);
}

function init() {
  bindEvents();
  connectEvents();
  appendActivity('Система готова', 'success');
  loadData();
}

init();
