import { state, refs } from './state.js';

export function calculateProductCost(form) {
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

export function updateProductCostPreview() {
  if (!refs.productForm || !refs.productCostPreview) return;

  const cost = calculateProductCost(refs.productForm);
  const legalChecked = Boolean(refs.productForm.querySelector('.billing-toggle[value="legal"]')?.checked);
  const label = legalChecked ? 'Для юр. лица' : 'Для физ. лица';
  refs.productCostPreview.textContent = `${label}: ${cost.toFixed(2)} ₽`;

  const hiddenPrice = refs.productForm.querySelector('input[name="price"]');
  if (hiddenPrice) hiddenPrice.value = String(cost);
}

export function syncBillingType(event) {
  const toggles = refs.productForm ? refs.productForm.querySelectorAll('.billing-toggle') : document.querySelectorAll('.billing-toggle');
  toggles.forEach((toggle) => {
    if (toggle !== event.target) {
      toggle.checked = false;
    }
  });
  updateProductCostPreview();
}

export function bindProductCostEvents() {
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
