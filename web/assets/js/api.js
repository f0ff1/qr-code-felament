import { state } from './state.js';

let onUnauthorized = () => {};

export function setUnauthorizedHandler(handler) {
  onUnauthorized = typeof handler === 'function' ? handler : () => {};
}

export async function fetchJSON(url, options = {}, timeoutMs = 10000) {
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
      onUnauthorized();
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

export function translateApiError(raw, status = 0) {
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
