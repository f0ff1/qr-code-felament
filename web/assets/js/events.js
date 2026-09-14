import { state } from './state.js';
import { fetchJSON } from './api.js';
import { formatEventTime } from './utils.js';
import {
  appendActivity,
  mapServerEvent,
  markToastSeen,
  shouldToastEvent,
  showToast,
  updatePendingAlertCount,
  renderActivity,
  ONESHOT_EVENT_TYPES,
  clearAlertToasts,
} from './toasts.js';
import { loadData } from './data.js';

let eventSource = null;

export function handleServerEvent(data, { allowToast = true } = {}) {
  if (!state.authenticated) return;
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
    markToastSeen(evt);
  }
  updatePendingAlertCount();
}

export async function loadNotificationHistory() {
  if (!state.authenticated) return;
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

export function disconnectEvents() {
  if (eventSource) {
    try {
      eventSource.close();
    } catch (_error) {
      // ignore
    }
    eventSource = null;
  }
}

export function connectEvents() {
  if (!window.EventSource || !state.authenticated) return;
  disconnectEvents();

  let reconnectToastAt = 0;
  const source = new EventSource('/events');
  eventSource = source;

  source.onmessage = (event) => {
    if (!state.authenticated || eventSource !== source) return;
    try {
      const data = JSON.parse(event.data);
      handleServerEvent(data, { allowToast: true });
    } catch (_error) {
      // ignore malformed payloads
    }
    loadData({ soft: true });
  };

  source.onerror = () => {
    if (!state.authenticated || eventSource !== source) return;
    const now = Date.now();
    if (now - reconnectToastAt > 15000) {
      reconnectToastAt = now;
      showToast('Связь в реальном времени переподключается…', 'warning', 'Сеть');
    }
  };
}

export function clearSessionRuntime() {
  disconnectEvents();
  clearAlertToasts();
  state.spools = [];
  state.printers = [];
  state.products = [];
  state.jobs = [];
  state.events = [];
  state.serverEvents = [];
  state.sites = [];
  state.adminUsersCache = [];
  state.adminPasswordResets = [];
  state.pendingAlertCount = 0;
  state.completedNotified = new Set();
  state.toastSessionShown = {};
}
