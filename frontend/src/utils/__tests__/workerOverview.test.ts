import { describe, expect, it } from "vitest";
import { makeOverview, makeWorker } from "@/test/workerOverviewFixtures";
import { workerOverviewRows, sumOverviewMetric } from "../workerOverview";

describe("worker overview aggregation", () => {
  it("sums worker copies independently and preserves partial coverage", () => {
    const workers = [makeWorker("one"), makeWorker("two"), makeWorker("offline")];
    const offline = makeOverview(workers[2]);
    offline.worker.status = "ERRORED";
    offline.metrics = { stored_bytes: null, total_torrents: null, active_torrents: null, download_speed: null, upload_speed: null };
    const rows = workerOverviewRows(workers, {
      one: { overview: makeOverview(workers[0], 5497558138880), checking: false },
      two: { overview: makeOverview(workers[1], 5497558138880), checking: false },
      offline: { overview: offline, checking: false },
    });
    expect(sumOverviewMetric(rows, "stored")).toEqual({ value: 10995116277760, available: 2, total: 3 });
    expect(sumOverviewMetric(rows, "download").value).toBe(200);
    expect(sumOverviewMetric(rows, "upload").value).toBe(400);
    expect(sumOverviewMetric(rows, "active").value).toBe(4);
    expect(sumOverviewMetric(rows, "online")).toEqual({ value: 2, available: 3, total: 3 });
    expect(rows[2].free).toBeNull();
  });

  it("distinguishes known zero, missing metrics, invalid disk values and an empty selection", () => {
    const worker = makeWorker("one");
    const overview = makeOverview(worker, 0);
    overview.worker.instance.server.free_space_on_disk = -1;
    const rows = workerOverviewRows([worker], { one: { overview, checking: true } });
    expect(sumOverviewMetric(rows, "stored").value).toBe(0);
    expect(rows[0].free).toBeNull();
    expect(rows[0].checking).toBe(true);
    expect(sumOverviewMetric(workerOverviewRows([worker], { one: { checking: false } }), "stored").value).toBeNull();
    expect(sumOverviewMetric([], "stored")).toEqual({ value: 0, available: 0, total: 0 });
  });
});
