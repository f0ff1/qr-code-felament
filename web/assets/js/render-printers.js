import { state, refs } from './state.js';
import { getEffectivePrinterStatus } from './jobs.js';
import { translateStatus } from './utils.js';

export function renderPrinters() {
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
