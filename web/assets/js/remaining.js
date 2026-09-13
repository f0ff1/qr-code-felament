import { state } from './state.js';
import { getEffectiveJobStatus, isActiveJobStatus } from './jobs.js';

/** Prefer API `current_remaining`; fall back to thin live math when jobs poll ahead of spool sync. */
export function getSpoolCurrentDisplay(spool) {
  const hasActiveReserve = state.jobs.some((job) => jobReservesSpool(job, spool.id));
  if (!hasActiveReserve && spool.currentRemaining != null && Number.isFinite(Number(spool.currentRemaining))) {
    return Math.max(0, Number(spool.currentRemaining));
  }

  // Mirrors server LiveRemaining so 2s job polls keep the bar fresh between full spool syncs.
  const futureRemaining = Number(spool.remaining ?? 0);
  const notYetUsed = state.jobs.reduce((total, job) => {
    if (!jobReservesSpool(job, spool.id)) return total;
    const weight = getJobRemainingEstimate(job);
    if (weight <= 0) return total;
    const progress = getJobPrintProgressPercent(job) / 100;
    return total + weight * (1 - progress);
  }, 0);
  return Math.max(0, futureRemaining + notYetUsed);
}

export function getJobRemainingEstimate(job) {
  if (Number(job?.estimatedWeight) > 0) {
    return Number(job.estimatedWeight);
  }
  if (Number(job?.consumedWeight) > 0) {
    return Number(job.consumedWeight);
  }
  const product = state.products.find((item) => item.id === job.productId);
  return Number(product?.estimatedWeight ?? 0);
}

export function jobReservesSpool(job, spoolId) {
  if (!job || String(job.spoolId ?? '') !== String(spoolId ?? '')) return false;
  // Use raw status: getEffectiveJobStatus() may mark a live print as completed at 100%.
  const status = String(job.status ?? '').toLowerCase();
  return ['printing', 'paused', 'preparing', 'calibrating', 'queued'].includes(status);
}

export function getJobPrintProgressPercent(job) {
  // Filament burn-down must follow printer/job %, same for manual and Bambu.
  let progress = Number(job?.progress ?? 0);
  if (!Number.isFinite(progress) || progress < 0) progress = 0;
  if (progress > 100) progress = 100;
  const status = String(job?.status ?? '').toLowerCase();
  const live = ['printing', 'paused', 'preparing', 'calibrating', 'queued'].includes(status);
  // Stale Cloud MQTT can report 100% while the job is still live with ETA left.
  if (live && Number(job?.remainingMinutes) > 0 && progress >= 100) {
    progress = 99;
  }
  return progress;
}

export function getSpoolStatus(spool) {
  const remaining = Number(spool.remaining_weight ?? spool.remaining ?? 0);
  const initial = Number(spool.initial_weight ?? spool.initial ?? 0);
  const spoolId = String(spool.id ?? '');
  const hasActiveJob = state.jobs.some((job) => {
    if (String(job.spoolId) !== spoolId) return false;
    return isActiveJobStatus(getEffectiveJobStatus(job));
  });

  if (hasActiveJob) return 'in_use';
  if (remaining <= 0) return 'empty';
  const lowThreshold = Number(state.config.spool_low_weight_g ?? 200);
  if (remaining < lowThreshold || (initial > 0 && remaining <= Math.max(50, initial * 0.2))) return 'low';
  return 'available';
}

export function jobNeedsFilamentTopUp(job) {
  if (!job || job.isDraft) return false;
  if (!isActiveJobStatus(getEffectiveJobStatus(job))) return false;
  if (job.needsFilamentTopUp) return true;
  if (job.estimatedWeight > 0 && job.consumedWeight < job.estimatedWeight) return true;
  if (!job.spoolId) return false;

  const spool = state.spools.find((item) => String(item.id) === String(job.spoolId));
  if (!spool) return false;

  const stillNeeded = Math.ceil(getJobRemainingEstimate(job) * (1 - getJobPrintProgressPercent(job) / 100));
  if (stillNeeded <= 0) return false;

  const current = getSpoolCurrentDisplay(spool);
  if (Number(spool.remaining ?? 0) <= 0 && stillNeeded > 0) return true;
  if (stillNeeded > current + 1) return true;

  const totalStillNeeded = state.jobs.reduce((sum, item) => {
    if (!jobReservesSpool(item, spool.id)) return sum;
    const need = Math.ceil(getJobRemainingEstimate(item) * (1 - getJobPrintProgressPercent(item) / 100));
    return sum + Math.max(0, need);
  }, 0);
  return totalStillNeeded > current + 1;
}
