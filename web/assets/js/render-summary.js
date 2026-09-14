import { state, refs } from './state.js';
import { getActiveJobs } from './jobs.js';
import { updateClock } from './utils.js';
import { refreshDerivedState } from './derived.js';
import { updatePendingAlertCount } from './toasts.js';

export function renderSummary() {
  refreshDerivedState();
  const lowCount = state.spools.filter((spool) => spool.status === 'low' || spool.status === 'empty').length;
  const activeCount = getActiveJobs().length;

  refs.totalSpools.textContent = String(state.spools.length);
  refs.lowStockCount.textContent = String(lowCount);
  refs.activePrints.textContent = String(activeCount);
  refs.printerCount.textContent = String(state.printers.length);
  updatePendingAlertCount();
  updateClock();
}
