import { state, refs } from './state.js';
import { getSpoolCurrentDisplay, jobNeedsFilamentTopUp } from './remaining.js';
import { getEffectiveJobStatus, getJobProgress, getJobTimeLabel, getJobLayerLabel, isActiveJobStatus } from './jobs.js';
import { translateStatus } from './utils.js';
import { refreshDerivedState } from './derived.js';

export function renderOverview() {
  refreshDerivedState();

  const spoolList = [...state.spools]
    .filter((spool) => {
      if (state.filter === 'all') return true;
      if (state.filter === 'low') return spool.status === 'low' || spool.status === 'empty' || Number(spool.remaining ?? 0) < Number(state.config.spool_low_weight_g ?? 200);
      if (state.filter === 'available') return spool.status === 'available' || Number(spool.remaining ?? 0) > 0;
      return false;
    })
    .sort((a, b) => b.remaining - a.remaining)
    .slice(0, 6);

  const taskList = state.jobs
    .filter((job) => {
      const effectiveStatus = getEffectiveJobStatus(job);
      if (state.filter === 'completed') return effectiveStatus === 'completed';
      if (state.filter === 'printing') return isActiveJobStatus(effectiveStatus);
      if (state.filter === 'all' || state.filter === 'low' || state.filter === 'available') {
        return isActiveJobStatus(effectiveStatus);
      }
      return false;
    })
    .slice(0, 6);

  if (!refs.overviewSpoolList || !refs.overviewJobList) return;

  if (state.filter === 'printing' || state.filter === 'completed') {
    refs.overviewSpoolList.innerHTML = '<div class="empty-state compact"><h3>Катушки скрыты фильтром</h3><p>Сейчас показаны только задачи.</p></div>';
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
        const title = job.fileName || product?.name || job.product;
        const layerLabel = getJobLayerLabel(job);
        const needsTopUp = jobNeedsFilamentTopUp(job);
        return `
          <div class="overview-item">
            <div>
              <strong>${title}</strong>
              <span>${translateStatus(status)}${job.isDraft ? ' • нужна катушка' : ''} • ${getJobTimeLabel(job)}${layerLabel ? ` • ${layerLabel}` : ''}</span>
              ${needsTopUp ? '<span class="job-flag top-up">догрузите пластик</span>' : ''}
              ${job.isDraft ? '<span class="job-flag needs-spool">нужна катушка</span>' : ''}
            </div>
            <div class="overview-meta">
              <span>${status === 'completed' ? '100%' : `${Math.round(progress)}%`}</span>
              <em class="overview-status ${status}">${translateStatus(status)}</em>
            </div>
          </div>
        `;
      }).join('')
    : '<div class="empty-state compact"><h3>Активных печатей нет</h3><p>Завершённые задачи сразу исчезают из этой графы.</p></div>';
}
