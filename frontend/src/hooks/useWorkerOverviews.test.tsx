import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ApiResponse } from "@/lib/api";
import type { WorkerOverview } from "@/types/worker";
import { workerService } from "@/services/workers";
import { makeOverview, makeWorker } from "@/test/workerOverviewFixtures";
import { useWorkerOverviews } from "./useWorkerOverviews";

vi.mock("@/services/workers", () => ({ workerService: { getWorkerOverview: vi.fn() } }));
const getOverview = vi.mocked(workerService.getWorkerOverview);

describe("useWorkerOverviews", () => {
  beforeEach(() => vi.clearAllMocks());

  it("limits the queue to four requests and publishes results independently", async () => {
    const workers = Array.from({ length: 7 }, (_, i) => makeWorker(String(i)));
    const pending: Array<() => void> = [];
    let current = 0, peak = 0;
    getOverview.mockImplementation(id => new Promise(resolve => {
      peak = Math.max(peak, ++current);
      pending.push(() => { current--; resolve({ data: makeOverview(workers.find(w => w.uuid === id)!) }); });
    }));
    const { result } = renderHook(() => useWorkerOverviews(workers));
    expect(getOverview).toHaveBeenCalledTimes(4);
    await act(async () => { pending.shift()!(); });
    expect(result.current["0"].checking).toBe(false);
    expect(result.current["1"].checking).toBe(true);
    expect(getOverview).toHaveBeenCalledTimes(5);
    while (pending.length) await act(async () => { pending.shift()!(); });
    expect(peak).toBe(4);
    expect(getOverview).toHaveBeenCalledTimes(7);
    expect(Object.values(result.current).every(state => !state.checking)).toBe(true);
  });

  it("preserves values on refresh and ignores obsolete responses and deleted workers", async () => {
    const worker = makeWorker("one");
    let resolveOld!: (value: ApiResponse<WorkerOverview>) => void;
    getOverview.mockResolvedValueOnce({ data: makeOverview(worker, 100) });
    const { result, rerender, unmount } = renderHook(({ workers }) => useWorkerOverviews(workers), { initialProps: { workers: [worker] } });
    await waitFor(() => expect(result.current.one.overview?.metrics.stored_bytes).toBe(100));
    getOverview.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve; }));
    rerender({ workers: [{ ...worker }] });
    expect(result.current.one.checking).toBe(true);
    expect(result.current.one.overview?.metrics.stored_bytes).toBe(100);
    const oldSignal = getOverview.mock.calls[1][1]!;
    getOverview.mockResolvedValueOnce({ data: makeOverview(worker, 200) });
    rerender({ workers: [{ ...worker, name: "Renamed" }] });
    await waitFor(() => expect(result.current.one.overview?.metrics.stored_bytes).toBe(200));
    expect(oldSignal.aborted).toBe(true);
    await act(async () => resolveOld({ data: makeOverview(worker, 999) }));
    expect(result.current.one.overview?.metrics.stored_bytes).toBe(200);
    rerender({ workers: [] });
    expect(result.current).toEqual({});
    unmount();
    expect(getOverview.mock.calls[2][1]!.aborted).toBe(true);
  });

  it("clears stale metrics on failure and recovers on the next refresh", async () => {
    const worker = makeWorker("one");
    getOverview.mockResolvedValueOnce({ data: makeOverview(worker) });
    const { result, rerender } = renderHook(({ workers }) => useWorkerOverviews(workers), { initialProps: { workers: [worker] } });
    await waitFor(() => expect(result.current.one.overview).toBeDefined());
    getOverview.mockRejectedValueOnce(new Error("unavailable"));
    rerender({ workers: [{ ...worker }] });
    await waitFor(() => expect(result.current.one).toEqual({ error: "unavailable", checking: false }));
    getOverview.mockResolvedValueOnce({ data: makeOverview(worker, 6000) });
    rerender({ workers: [{ ...worker }] });
    await waitFor(() => expect(result.current.one.overview?.metrics.stored_bytes).toBe(6000));
  });
});
