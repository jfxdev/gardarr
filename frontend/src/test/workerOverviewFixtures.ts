import type { Worker, WorkerOverview } from "@/types/worker";

export function makeWorker(uuid: string, name = uuid): Worker {
  return {
    uuid, name, address: `https://${uuid}.example.test`, status: "PENDING", color: "#10b981",
    instance: {
      application: { version: "5.1", api_version: "2" },
      server: { free_space_on_disk: 5000 },
      transfer: { all_time_downloaded: 0, all_time_uploaded: 0, global_ratio: 1, last_external_address_v4: "", last_external_address_v6: "" },
    },
  };
}

export function makeOverview(worker: Worker, stored = 3000): WorkerOverview {
  return {
    worker: { ...worker, status: "ACTIVE" }, collected_at: "2026-10-11T20:00:00Z",
    metrics: { stored_bytes: stored, total_torrents: 3, active_torrents: 2, download_speed: 100, upload_speed: 200 },
  };
}
