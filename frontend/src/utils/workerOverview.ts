import type { Worker } from "@/types/worker";
import type { WorkerOverviews } from "@/hooks/useWorkerOverviews";

export interface OverviewRow {
  worker: Worker;
  stored: number | null;
  free: number | null;
  total: number | null;
  active: number | null;
  download: number | null;
  upload: number | null;
  online: number | null;
  checking: boolean;
  collectedAt?: string;
}

function metric(value?: number | null): number | null {
  return value != null && Number.isFinite(value) && value >= 0 ? value : null;
}

export function workerOverviewRows(workers: Worker[], states: WorkerOverviews): OverviewRow[] {
  return workers.map(worker => {
    const state = states[worker.uuid];
    const overview = state?.overview;
    const live = overview?.worker;
    return {
      worker,
      stored: metric(overview?.metrics.stored_bytes),
      free: live?.status === "ACTIVE" ? metric(live.instance?.server?.free_space_on_disk) : null,
      total: metric(overview?.metrics.total_torrents),
      active: metric(overview?.metrics.active_torrents),
      download: metric(overview?.metrics.download_speed),
      upload: metric(overview?.metrics.upload_speed),
      online: live && live.status !== "PENDING" && live.status !== "INITIALIZING"
        ? Number(live.status === "ACTIVE") : null,
      checking: state?.checking ?? true,
      collectedAt: overview?.collected_at,
    };
  });
}

export function sumOverviewMetric(rows: OverviewRow[], key: "stored" | "free" | "total" | "active" | "download" | "upload" | "online") {
  const known = rows.filter(row => row[key] !== null);
  return {
    value: rows.length === 0 || known.length > 0 ? known.reduce((sum, row) => sum + row[key]!, 0) : null,
    available: known.length,
    total: rows.length,
  };
}
