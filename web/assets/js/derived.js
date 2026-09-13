import { state } from './state.js';
import { getSpoolStatus } from './remaining.js';
import { getJobProgress, isActiveJobStatus } from './jobs.js';

export function refreshDerivedState() {
  let completedNow = false;
  state.jobs.forEach((job) => {
    if (!isActiveJobStatus(job.status)) return;
    // Bambu completion comes from the printer/monitor, not client-side ETA.
    if (job.source === 'bambu') return;
    if (getJobProgress(job) < 100) return;
    job.status = 'completed';
    job.progress = 100;
    completedNow = true;
  });

  state.spools.forEach((spool) => {
    spool.status = getSpoolStatus(spool);
  });

  return completedNow;
}
