import { useEffect, useState } from "react";
import { workerService } from "@/services/workers";
import type { Instance, Worker, WorkerErrorCode, WorkerStatus } from "@/types/worker";

export interface WorkerLiveStatus {
  status: WorkerStatus;
  error?: string;
  error_code?: WorkerErrorCode;
  permanent?: boolean;
  instance?: Instance;
  checking: boolean;
}

const PENDING_STATUS: WorkerLiveStatus = { status: "PENDING", checking: true };

// Fetches a single worker's live qBittorrent connectivity/instance status,
// independently of whatever list it was rendered from. Re-fetches whenever
// workerId or refreshToken change, so a parent can force every row to
// re-check by bumping a shared refreshToken.
export function useWorkerLiveStatus(workerId: string, refreshToken = 0, enabled = true): WorkerLiveStatus {
  const [live, setLive] = useState<WorkerLiveStatus>(PENDING_STATUS);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    setLive(PENDING_STATUS);

    workerService.getWorker(workerId).then((response) => {
      if (cancelled) return;

      if (response.error || !response.data) {
        setLive({ status: "ERRORED", error: response.error, checking: false });
        return;
      }

      const worker: Worker = response.data;
      setLive({
        status: worker.status,
        error: worker.error,
        error_code: worker.error_code,
        permanent: worker.permanent,
        instance: worker.instance,
        checking: false,
      });
    });

    return () => {
      cancelled = true;
    };
  }, [workerId, refreshToken, enabled]);

  return live;
}
