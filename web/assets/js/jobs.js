import { state } from './state.js';

export function isActiveJobStatus(status) {
  return ['preparing', 'calibrating', 'printing', 'paused', 'queued', 'draft'].includes(String(status ?? '').toLowerCase());
}

export function parseDurationToMs(value) {
  if (!value || typeof value !== 'string') return 0;
  const normalized = value.trim().toLowerCase();
  const hourMatch = normalized.match(/(\d+)h/);
  const minuteMatch = normalized.match(/(\d+)m/);
  const secondMatch = normalized.match(/(\d+)s/);
  const hours = Number(hourMatch ? hourMatch[1] : 0);
  const minutes = Number(minuteMatch ? minuteMatch[1] : 0);
  const seconds = Number(secondMatch ? secondMatch[1] : 0);
  return ((hours * 60 * 60) + (minutes * 60) + seconds) * 1000;
}

export function formatDuration(ms) {
  if (!Number.isFinite(ms) || ms <= 0) return '0 мин';
  const totalMinutes = Math.max(0, Math.round(ms / 60000));
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (hours > 0) {
    return `${hours}ч ${minutes}м`;
  }
  return `${minutes}м`;
}

export function buildEstimatedPrintTime(hours, minutes) {
  const totalHours = Number(hours) || 0;
  const totalMinutes = Number(minutes) || 0;
  return `${totalHours}h${totalMinutes}m`;
}

export function getJobProgress(job) {
  if (!job) return 0;

  // Trust Cloud/printer % for Bambu jobs — never invent from wall-clock.
  if (job.source === 'bambu') {
    return Math.min(100, Math.max(0, Number(job.progress ?? 0)));
  }

  if (!['printing', 'paused', 'preparing', 'calibrating'].includes(job.status)) {
    return job.status === 'completed' ? 100 : Number(job.progress ?? 0);
  }

  const product = state.products.find((item) => item.id === job.productId);
  const durationMs = parseDurationToMs(product?.estimatedPrintTime || '0m');
  const startTime = state.jobRuntimeStarts[job.id] ?? (job.startedAt ? job.startedAt.getTime() : null);

  if (!startTime || durationMs <= 0) {
    return Number(job.progress ?? 0);
  }

  const elapsed = Math.max(0, Date.now() - startTime);
  const progress = (elapsed / durationMs) * 100;
  return Math.min(100, Math.max(0, progress));
}

export function getEffectiveJobStatus(job) {
  if (!job) return 'queued';
  if (job.status === 'completed' || job.status === 'failed' || job.status === 'cancelled') {
    return job.status;
  }

  // Bambu: trust Cloud status as-is; isDraft is only "нужна катушка", not a print status.
  if (job.source === 'bambu') {
    return job.status === 'draft' ? 'printing' : job.status;
  }

  const progress = getJobProgress(job);
  if (isActiveJobStatus(job.status) && progress >= 100) {
    return 'completed';
  }

  return job.status === 'draft' ? 'printing' : job.status;
}

export function getJobTimeLabel(job) {
  if (!job) return '0 мин';
  if (Number(job.remainingMinutes) > 0 && ['printing', 'paused', 'preparing', 'calibrating'].includes(job.status)) {
    return `осталось ${formatDuration(Number(job.remainingMinutes) * 60000)}`;
  }
  if (Number(job.estimatedDurationSec) > 0) {
    return formatDuration(Number(job.estimatedDurationSec) * 1000);
  }
  const product = state.products.find((item) => item.id === job.productId);
  return formatDuration(parseDurationToMs(product?.estimatedPrintTime || '0m'));
}

export function getJobLayerLabel(job) {
  const total = Number(job?.layerTotal ?? 0);
  if (total <= 0) return '';
  const current = Math.max(0, Number(job?.layerCurrent ?? 0));
  return `слой ${current} из ${total}`;
}

export function getActiveJobs() {
  return state.jobs.filter((job) => isActiveJobStatus(getEffectiveJobStatus(job)));
}

export function getEffectivePrinterStatus(printerId) {
  const printer = state.printers.find((item) => String(item.id) === String(printerId));
  const apiStatus = String(printer?.status ?? '').toLowerCase();
  if (apiStatus && !['idle', ''].includes(apiStatus)) {
    return apiStatus;
  }
  const hasActiveJob = state.jobs.some((job) => {
    if (String(job.printerId) !== String(printerId)) return false;
    return isActiveJobStatus(getEffectiveJobStatus(job));
  });
  if (hasActiveJob) return 'printing';
  return apiStatus || 'idle';
}
