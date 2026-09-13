import { state, refs } from './state.js';
import { getSpoolCurrentDisplay } from './remaining.js';
import { getMaterialColor, translateStatus } from './utils.js';

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

function getSpoolBarStyle(remaining, initial) {
  const safeInitial = Math.max(1, Number(initial) || 1);
  const percent = Math.max(0, Math.min(100, (Number(remaining) / safeInitial) * 100));
  const hue = Math.max(0, 120 - (100 - percent) * 1.2);
  return {
    percent,
    color: `linear-gradient(90deg, hsl(${hue} 78% 55%), hsl(${Math.max(0, hue - 18)} 85% 48%))`,
  };
}

export function renderSpools() {
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
