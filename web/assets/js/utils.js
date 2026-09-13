import { refs } from './state.js';

export function translateStatus(status) {
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

export function formatProductPrice(product) {
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

export function formatEventTime(iso) {
  const ts = Date.parse(iso);
  if (!Number.isFinite(ts)) return '';
  const ageMin = Math.round((Date.now() - ts) / 60000);
  if (ageMin < 1) return 'только что';
  if (ageMin < 60) return `${ageMin} мин назад`;
  const hours = Math.round(ageMin / 60);
  if (hours < 24) return `${hours} ч назад`;
  return new Date(ts).toLocaleString();
}

export function getMaterialColor(material) {
  const palette = {
    PLA: '#58a6ff',
    PETG: '#7ee787',
    ABS: '#ffa657',
    'PLA+': '#bf7af2',
  };

  return palette[material] || '#c9d1d9';
}

export function updateClock() {
  refs.updatedAt.textContent = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

export function fillSelectOptions(select, options, preferred = '') {
  if (!select) return;
  const current = select.value || preferred;
  select.innerHTML = options.map((value) => `<option value="${value}">${value}</option>`).join('');
  if (current && options.includes(current)) {
    select.value = current;
  } else if (options.length) {
    select.value = options.includes(preferred) ? preferred : options[0];
  }
}
