import { state, refs } from './state.js';
import { fetchJSON } from './api.js';
import { notify } from './toasts.js';
import { loadData } from './data.js';
import { setFormBusy } from './cloud.js';
import { calculateProductCost } from './product-cost.js';
import { buildEstimatedPrintTime } from './jobs.js';

export async function createSpool(event) {
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

export function syncPrinterConnectionFields() {
  const form = refs.printerForm;
  if (!form) return;
  const mode = form.connectionMode?.value || 'none';
  const lan = document.getElementById('printerLanFields');
  if (lan) lan.classList.toggle('hidden', mode !== 'lan');
}

export async function createPrinter(event) {
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

export async function createProduct(event) {
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

export async function createPrintJob(event) {
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

export async function deleteItem(type, id) {
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

export async function toggleJobStatus(id, action) {
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

export async function confirmDraftJob(jobId) {
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
