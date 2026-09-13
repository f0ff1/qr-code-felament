export function normalizeSpool(spool) {
  const remaining = Number(spool.remaining_weight ?? spool.remaining ?? 0);
  const initial = Number(spool.initial_weight ?? spool.initial ?? 0);
  const currentRemaining = spool.current_remaining != null
    ? Number(spool.current_remaining)
    : null;

  return {
    id: spool.id,
    material: spool.material,
    color: spool.color,
    manufacturer: spool.manufacturer,
    remaining,
    currentRemaining,
    initial,
    price: Number(spool.price ?? 0),
    status: 'available',
    qr: spool.qr_token ?? spool.qr ?? '—',
  };
}

export function normalizePrinter(printer) {
  return {
    id: printer.id,
    name: printer.name,
    model: printer.model,
    status: String(printer.status ?? 'idle').toLowerCase(),
    lanEnabled: Boolean(printer.lan_enabled),
    lanHost: printer.lan_host ?? '',
    lanSerial: printer.lan_serial ?? '',
    cloudEnabled: Boolean(printer.cloud_enabled),
    cloudRegion: printer.cloud_region ?? 'us',
    cloudEmail: printer.cloud_email ?? '',
    cloudLinked: Boolean(printer.cloud_linked),
    needsVerification: Boolean(printer.needs_verification),
    connection: printer.connection || (printer.cloud_enabled ? 'cloud' : printer.lan_enabled ? 'lan' : 'none'),
    defaultSpoolId: printer.default_spool_id ?? null,
  };
}

export function normalizeProduct(product) {
  return {
    id: product.id,
    name: product.name,
    description: product.description,
    material: product.material,
    estimatedWeight: Number(product.estimated_weight ?? product.estimatedWeight ?? 0),
    estimatedPrintTime: product.estimated_print_time ?? product.estimatedPrintTime ?? '0s',
    price: Number(product.price ?? 0),
    priceLegal: Number(product.price_legal ?? product.priceLegal ?? 0),
    billingMode: String(product.billing_mode ?? product.billingMode ?? 'person'),
  };
}

export function normalizeJob(job) {
  const toDate = (value) => {
    if (!value) return null;
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? null : parsed;
  };

  return {
    id: job.id,
    status: String(job.status ?? 'queued').toLowerCase(),
    progress: Number(job.progress ?? 0),
    printer: job.printer_id ? String(job.printer_id).slice(0, 8) : '—',
    printerId: job.printer_id ?? null,
    product: job.product_id ? String(job.product_id).slice(0, 8) : '—',
    productId: job.product_id ?? null,
    spoolId: job.spool_id ?? null,
    startedAt: toDate(job.started_at),
    source: job.source ?? 'manual',
    fileName: job.file_name ?? '',
    isDraft: Boolean(job.is_draft),
    estimatedWeight: Number(job.estimated_weight ?? 0),
    consumedWeight: Number(job.consumed_weight ?? 0),
    needsFilamentTopUp: Boolean(job.needs_filament_top_up),
    remainingMinutes: Number(job.remaining_minutes ?? 0),
    estimatedDurationSec: Number(job.estimated_duration_sec ?? 0),
    layerCurrent: Number(job.layer_current ?? 0),
    layerTotal: Number(job.layer_total ?? 0),
  };
}
