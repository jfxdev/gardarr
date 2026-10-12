import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { createInstance } from "i18next";
import { I18nextProvider } from "react-i18next";
import en from "@/locales/en-US.json";
import pt from "@/locales/pt-BR.json";
import { makeOverview, makeWorker } from "@/test/workerOverviewFixtures";
import { WorkerOverviewWidgets } from "../WorkerOverviewWidgets";

const i18n = createInstance();
await i18n.init({ lng: "en-US", resources: { "en-US": { translation: en }, "pt-BR": { translation: pt } }, interpolation: { escapeValue: false } });

describe("WorkerOverviewWidgets", () => {
  it("divides each bar by the sum of known values and keeps unavailable workers in the legend", () => {
    const workers = [makeWorker("low", "Small"), makeWorker("high", "Large"), makeWorker("offline", "Offline")];
    render(<I18nextProvider i18n={i18n}><WorkerOverviewWidgets workers={workers} states={{
      low: { overview: makeOverview(workers[0], 1024), checking: false },
      high: { overview: makeOverview(workers[1], 4096), checking: false },
      offline: { checking: false, error: "offline" },
    }} filtered refreshing={false} /></I18nextProvider>);
    expect(screen.getAllByLabelText("Partial — data from 2/3 workers").length).toBeGreaterThan(0);
    expect(screen.getAllByText("5.00 KB").length).toBeGreaterThan(0);
    expect(screen.getByText("Showing 3 filtered workers")).toBeInTheDocument();
    const comparison = screen.getByRole("heading", { name: "Stored content by node" }).parentElement!.parentElement!;
    const segments = within(screen.getByRole("group", { name: "Stored content by node" })).getAllByRole("button");
    expect(segments.map(segment => segment.getAttribute("aria-label"))).toEqual(["Small: 1.00 KB", "Large: 4.00 KB"]);
    expect(segments[0]).toHaveStyle({ width: "20%" });
    expect(segments[1]).toHaveStyle({ width: "80%" });
    expect(within(comparison).getByRole("button", { name: "Offline: Unavailable" })).toBeInTheDocument();
    const freeSegments = within(screen.getByRole("group", { name: "Free space by node" })).getAllByRole("button");
    expect(freeSegments.map(segment => segment.getAttribute("aria-label"))).toEqual(["Small: 4.88 KB", "Large: 4.88 KB"]);
    freeSegments.forEach(segment => expect(segment).toHaveStyle({ width: "50%" }));
  });

  it("renders an empty selection with zero totals and no comparison rows", () => {
    render(<I18nextProvider i18n={i18n}><WorkerOverviewWidgets workers={[]} states={{}} filtered refreshing={false} /></I18nextProvider>);
    const stored = screen.getByRole("heading", { name: "Total stored" }).parentElement!.parentElement!;
    expect(within(stored).getByText("0 B")).toBeInTheDocument();
    expect(screen.getAllByText("No workers to compare")).toHaveLength(2);
  });

  it("keeps every worker keyboard-accessible in the legend", () => {
    const workers = Array.from({ length: 8 }, (_, i) => makeWorker(String(i)));
    render(<I18nextProvider i18n={i18n}><WorkerOverviewWidgets workers={workers} states={Object.fromEntries(workers.map(w => [w.uuid, { overview: makeOverview(w), checking: false }]))} filtered={false} refreshing={false} /></I18nextProvider>);
    const chart = screen.getByRole("heading", { name: "Stored content by node" }).parentElement!.parentElement!;
    workers.forEach(worker => {
      const buttons = within(chart).getAllByRole("button", { name: `${worker.name}: 2.93 KB` });
      expect(buttons).toHaveLength(2);
      expect(buttons[0]).toHaveAttribute("tabindex", "-1");
      expect(buttons[1]).not.toHaveAttribute("tabindex", "-1");
    });
  });

  it("does not invent segments for known zero or missing values", () => {
    const workers = [makeWorker("zero"), makeWorker("missing")];
    const overview = makeOverview(workers[0], 0);
    overview.worker.instance.server.free_space_on_disk = 0;
    render(<I18nextProvider i18n={i18n}><WorkerOverviewWidgets workers={workers} states={{ zero: { overview, checking: false }, missing: { checking: false } }} filtered={false} refreshing={false} /></I18nextProvider>);
    for (const title of ["Free space by node", "Stored content by node"]) {
      expect(within(screen.getByRole("group", { name: title })).queryAllByRole("button")).toHaveLength(0);
      const chart = screen.getByRole("heading", { name: title }).parentElement!.parentElement!;
      expect(within(chart).getByRole("button", { name: "zero: 0 B" })).toBeInTheDocument();
      expect(within(chart).getByRole("button", { name: "missing: Unavailable" })).toBeInTheDocument();
    }
  });
});
