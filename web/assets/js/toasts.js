import { state, refs } from './state.js';
import { formatEventTime } from './utils.js';
import { renderSummary } from './render-summary.js';

const TOAST_STORAGE_KEY = 'filament.toastState';
export const ONESHOT_EVENT_TYPES = new Set([
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

export function eventToastKey(evt) {
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

export function markToastSeen(evt) {
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

export function shouldToastEvent(evt, { fromHistory = false } = {}) {
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

export function showToast(message, type = 'info', title = '') {
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

export function renderActivity() {
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

export function appendActivity(message, type = 'info', id = '', createdAt = '') {
  const item = {
    id: id || (globalThis.crypto?.randomUUID ? globalThis.crypto.randomUUID() : String(Date.now())),
    type,
    label: message,
    time: createdAt ? formatEventTime(createdAt) : 'только что',
  };

  state.events = [item, ...state.events.filter((entry) => entry.id !== item.id)].slice(0, 12);
  renderActivity();
}

export function notify(message, type = 'info', title = '', eventType = '') {
  appendActivity(message, type);
  showToast(message, type, title);
  if (eventType) {
    state.localNotifyCooldown = state.localNotifyCooldown || {};
    state.localNotifyCooldown[eventType] = Date.now();
  }
}

export function updatePendingAlertCount() {
  const undismissed = (state.serverEvents || []).filter((evt) => !isToastDismissed(evt));
  state.pendingAlertCount = undismissed.length;
  if (refs.alertCount) {
    refs.alertCount.textContent = `${state.pendingAlertCount} уведомлений`;
  }
}

export function clearAlertToasts() {
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

export function mapServerEvent(type, message) {
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
