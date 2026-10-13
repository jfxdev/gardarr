import { useEffect, useRef, useState } from "react";
import { workerService } from "@/services/workers";
import type { Worker, WorkerOverview } from "@/types/worker";
import type { WorkerLiveStatus } from "./useWorkerLiveStatus";

export interface WorkerOverviewState {
  overview?: WorkerOverview;
  checking: boolean;
  error?: string;
}

export type WorkerOverviews = Partial<Record<string, WorkerOverviewState>>;

// One queue for the full page, independent of the search filter. Changing the
// roster cancels obsolete work; each result is published as soon as it arrives.
export function useWorkerOverviews(workers: Worker[]): WorkerOverviews {
  const [states, setStates] = useState<WorkerOverviews>({});
  const generation = useRef(0);

  useEffect(() => {
    const current = ++generation.current;
    const controller = new AbortController();
    const obsolete = () => controller.signal.aborted || generation.current !== current;
    setStates(previous => Object.fromEntries(workers.map(worker => [worker.uuid, {
      ...previous[worker.uuid], checking: true,
    }])));
    let cursor = 0;
    const failQueue = (error: unknown) => {
      if (obsolete()) return;
      const message = error instanceof Error ? error.message : String(error);
      setStates(previous => Object.fromEntries(Object.entries(previous).map(([id, state]) => [id,
        state?.checking ? { error: message, checking: false } : state,
      ])));
    };

    async function consume() {
      try {
        while (!obsolete() && cursor < workers.length) {
          const worker = workers[cursor++];
          let state: WorkerOverviewState;
          try {
            const response = await workerService.getWorkerOverview(worker.uuid, controller.signal);
            state = response.data && !response.error
              ? { overview: response.data, checking: false }
              : { error: response.error, checking: false };
          } catch (error) {
            state = { error: error instanceof Error ? error.message : String(error), checking: false };
          }
          if (obsolete()) return;
          setStates(previous => ({ ...previous, [worker.uuid]: state }));
        }
      } catch (error) {
        failQueue(error);
      }
    }

    void Promise.all(Array.from({ length: Math.min(4, workers.length) }, consume)).catch(failQueue);
    return () => { controller.abort(); };
  }, [workers]);

  return states;
}

export function overviewLiveStatus(state?: WorkerOverviewState): WorkerLiveStatus {
  const worker = state?.overview?.worker;
  if (!worker) return {
    status: state && !state.checking ? "ERRORED" : "PENDING",
    error: state?.error,
    checking: state?.checking ?? true,
  };
  return {
    status: worker.status,
    error: worker.error,
    error_code: worker.error_code,
    permanent: worker.permanent,
    instance: worker.instance,
    checking: state.checking,
  };
}
