import { state, refs } from './state.js';
import { jobNeedsFilamentTopUp } from './remaining.js';
import { getEffectiveJobStatus, getJobProgress, getJobTimeLabel, getJobLayerLabel, isActiveJobStatus } from './jobs.js';
import { translateStatus } from './utils.js';
import { updateProductCostPreview } from './product-cost.js';

export function renderJobs() {
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

export function renderJobOptions() {
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
