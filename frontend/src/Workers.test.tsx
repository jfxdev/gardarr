import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createInstance } from "i18next";
import { I18nextProvider } from "react-i18next";
import en from "@/locales/en-US.json";
import Workers from "./Workers";
import type { WorkerService } from "./services/workers";
import { makeOverview, makeWorker } from "./test/workerOverviewFixtures";
import type { ApiResponse } from "./lib/api";
import type { Worker, WorkerOverview } from "./types/worker";

const serviceMock = vi.hoisted(() => ({
  listWorkersBasic: vi.fn<WorkerService["listWorkersBasic"]>(),
  getWorkerOverview: vi.fn<WorkerService["getWorkerOverview"]>(),
  getWorker: vi.fn<WorkerService["getWorker"]>(),
  deleteWorker: vi.fn<WorkerService["deleteWorker"]>(),
  updateWorker: vi.fn<WorkerService["updateWorker"]>(),
}));
vi.mock("@/services/workers", () => ({ workerService: serviceMock }));
vi.mock("./components/AddWorkerModal", () => ({ AddWorkerModal: ({ open, onSuccess, onOpenChange }: { open: boolean; onSuccess: (worker: Worker) => void; onOpenChange: (open: boolean) => void }) => open ? <button onClick={() => { onSuccess(makeWorker("new", "New worker")); onOpenChange(false); }}>Create fixture</button> : null }));
vi.mock("./components/WorkerDetailsModal", () => ({ WorkerDetailsModal: ({ isOpen, worker, onEdit, onDelete }: { isOpen: boolean; worker: Worker; onEdit: (worker: Worker) => void; onDelete: (worker: Worker) => void }) => isOpen ? <div><button onClick={() => { onEdit(worker); }}>Edit fixture</button><button onClick={() => { onDelete(worker); }}>Delete fixture</button></div> : null }));

const i18n = createInstance();
await i18n.init({ lng: "en-US", resources: { "en-US": { translation: en } }, interpolation: { escapeValue: false } });
const initialWorkers = [makeWorker("one", "Yellow"), makeWorker("two", "Green")];
const list = vi.mocked(serviceMock.listWorkersBasic);
const overview = vi.mocked(serviceMock.getWorkerOverview);
const storedCard = () => {
  const card = screen.getByRole("heading", { name: "Total stored" }).parentElement?.parentElement;
  if (!card) throw new Error("Stored summary container is missing");
  return card;
};

describe("Workers overview integration", () => {
  beforeEach(() => {
    vi.stubGlobal("ResizeObserver", class { observe() {} unobserve() {} disconnect() {} });
    vi.resetAllMocks();
    list.mockResolvedValue({ data: initialWorkers.map(worker => ({ ...worker })) });
    overview.mockImplementation(async id => ({ data: makeOverview(makeWorker(id, initialWorkers.find(worker => worker.uuid === id)?.name), 1024) }));
  });
  afterEach(() => vi.unstubAllGlobals());

  it("shares overview status with cards and filters locally without new requests", async () => {
    render(<I18nextProvider i18n={i18n}><Workers /></I18nextProvider>);
    await waitFor(() => expect(within(storedCard()).getByText("2.00 KB")).toBeInTheDocument());
    expect(overview).toHaveBeenCalledTimes(2);
    expect(serviceMock.getWorker).not.toHaveBeenCalled();
    await userEvent.type(screen.getByPlaceholderText("Search workers..."), "Yellow");
    expect(within(storedCard()).getByText("1.00 KB")).toBeInTheDocument();
    expect(screen.getByText("Showing 1 filtered workers")).toBeInTheDocument();
    expect(overview).toHaveBeenCalledTimes(2);
    await userEvent.clear(screen.getByPlaceholderText("Search workers..."));
    expect(within(storedCard()).getByText("2.00 KB")).toBeInTheDocument();
    expect(overview).toHaveBeenCalledTimes(2);
  });

  it("keeps previous data during refresh and ignores obsolete list replies after creation", async () => {
    render(<I18nextProvider i18n={i18n}><Workers /></I18nextProvider>);
    await waitFor(() => expect(within(storedCard()).getByText("2.00 KB")).toBeInTheDocument());
    let resolveList!: (response: ApiResponse<Worker[]>) => void;
    list.mockImplementationOnce(() => new Promise(resolve => { resolveList = resolve; }));
    await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
    expect(within(storedCard()).getByText("2.00 KB")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Yellow" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Add Worker" }));
    await userEvent.click(screen.getByRole("button", { name: "Create fixture" }));
    await waitFor(() => expect(within(storedCard()).getByText("3.00 KB")).toBeInTheDocument());
    await act(async () => { resolveList({ data: [] }); });
    expect(screen.getAllByText("New worker").length).toBeGreaterThan(0);
    expect(within(storedCard()).getByText("3.00 KB")).toBeInTheDocument();
  });

  it("retains data while the overview refresh is pending and publishes the new result", async () => {
    render(<I18nextProvider i18n={i18n}><Workers /></I18nextProvider>);
    await waitFor(() => expect(within(storedCard()).getByText("2.00 KB")).toBeInTheDocument());
    const completions: Array<(response: ApiResponse<WorkerOverview>) => void> = [];
    overview.mockImplementation(() => new Promise(resolve => completions.push(resolve)));
    await userEvent.click(screen.getByRole("button", { name: "Refresh" }));
    await waitFor(() => expect(completions).toHaveLength(2));
    expect(within(storedCard()).getByText("2.00 KB")).toBeInTheDocument();
    await act(async () => {
      for (const [worker, complete] of initialWorkers.map((worker, i) => [worker, completions.at(i)] as const)) {
        if (!complete) throw new Error("Overview resolver is missing");
        complete({ data: makeOverview(worker, 2048) });
      }
    });
    expect(within(storedCard()).getByText("4.00 KB")).toBeInTheDocument();
    expect(serviceMock.getWorker).not.toHaveBeenCalled();
  });

  it("updates filtered widgets after editing and removes deleted workers from totals", async () => {
    render(<I18nextProvider i18n={i18n}><Workers /></I18nextProvider>);
    await waitFor(() => expect(within(storedCard()).getByText("2.00 KB")).toBeInTheDocument());
    await userEvent.type(screen.getByPlaceholderText("Search workers..."), "Yellow");
    await userEvent.click(screen.getByRole("heading", { name: "Yellow" }));
    await userEvent.click(screen.getByRole("button", { name: "Edit fixture" }));
    const name = screen.getByLabelText("Name");
    await userEvent.clear(name);
    await userEvent.type(name, "Renamed");
    vi.mocked(serviceMock.updateWorker).mockResolvedValueOnce({ data: makeWorker("one", "Renamed") });
    await userEvent.click(screen.getByRole("button", { name: "Update Worker" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(within(storedCard()).getByText("0 B")).toBeInTheDocument();
    expect(serviceMock.updateWorker).toHaveBeenCalledWith("one", { name: "Renamed" });
    await userEvent.clear(screen.getByPlaceholderText("Search workers..."));
    await userEvent.click(screen.getByRole("heading", { name: "Renamed" }));
    await userEvent.click(screen.getByRole("button", { name: "Delete fixture" }));
    vi.mocked(serviceMock.deleteWorker).mockResolvedValueOnce({ data: null });
    await userEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    await waitFor(() => expect(within(storedCard()).getByText("1.00 KB")).toBeInTheDocument());
    expect(screen.queryByRole("heading", { name: "Renamed" })).not.toBeInTheDocument();
    expect(serviceMock.deleteWorker).toHaveBeenCalledWith("one");
  });
});
