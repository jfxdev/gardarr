import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { AddCategoryModal } from "@/components/AddCategoryModal";
import type { Category } from "@/types/category";

const listWorkersBasicMock = vi.fn();
vi.mock("@/services/workers", () => ({ workerService: { listWorkersBasic: (...args: unknown[]) => listWorkersBasicMock(...args) } }));
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: (key: string) => key }) }));
vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));

const workers = [
  { uuid: "worker-1", name: "Offline worker", status: "PENDING", icon: "Server", color: "#f97316" },
  { uuid: "worker-2", name: "Another worker", status: "INACTIVE", icon: "HardDrive", color: "#123456" },
];
const category: Category = { id: "movies", name: "Movies", preferred_worker_id: "worker-1", default_tags: [], default_directories: [], metadata_source: "none", created_at: "", updated_at: "" };

beforeAll(() => {
  HTMLElement.prototype.scrollIntoView = vi.fn();
  HTMLElement.prototype.hasPointerCapture = () => false;
  HTMLElement.prototype.setPointerCapture = vi.fn();
  HTMLElement.prototype.releasePointerCapture = vi.fn();
});

async function choosePreference(name: string) {
  const select = screen.getByRole("combobox", { name: "categories.fields.preferredWorker" });
  fireEvent.keyDown(select, { key: "Enter" });
  const option = await screen.findByRole("option", { name });
  fireEvent.click(option);
}

describe("AddCategoryModal preferred worker", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listWorkersBasicMock.mockReset();
    listWorkersBasicMock.mockResolvedValue({ data: workers });
  });

  it("creates a category with an offline worker preference", async () => {
    const create = vi.fn().mockResolvedValue(undefined);
    render(<AddCategoryModal open onOpenChange={vi.fn()} onCategoryCreated={create} />);
    const select = screen.getByRole("combobox", { name: "categories.fields.preferredWorker" });
    await waitFor(() => expect(select).not.toBeDisabled());
    expect(select).toHaveTextContent("categories.preferredWorker.none");
    fireEvent.change(screen.getByLabelText(/categories.fields.name/), { target: { value: "Movies" } });
    await choosePreference("Offline worker");
    expect(select).toHaveTextContent("Offline worker");
    fireEvent.click(screen.getByRole("button", { name: "categories.create" }));
    await waitFor(() => expect(create).toHaveBeenCalledWith(expect.objectContaining({ preferred_worker_id: "worker-1" })));
    expect(listWorkersBasicMock).toHaveBeenCalledTimes(1);
  });

  it("loads, changes and removes an existing preference", async () => {
    const update = vi.fn().mockResolvedValue(undefined);
    const props = { open: true, onOpenChange: vi.fn(), onCategoryCreated: vi.fn(), onCategoryUpdated: update, editingCategory: category };
    const view = render(<AddCategoryModal {...props} />);
    const select = screen.getByRole("combobox", { name: "categories.fields.preferredWorker" });
    await waitFor(() => expect(select).toHaveTextContent("Offline worker"));
    await choosePreference("Another worker");
    fireEvent.click(screen.getByRole("button", { name: "common.save" }));
    await waitFor(() => expect(update).toHaveBeenCalledWith("movies", expect.objectContaining({ preferred_worker_id: "worker-2" })));
    view.rerender(<AddCategoryModal {...props} editingCategory={{ ...category, preferred_worker_id: "worker-2" }} />);
    await waitFor(() => expect(select).toHaveTextContent("Another worker"));
    await choosePreference("categories.preferredWorker.none");
    fireEvent.click(screen.getByRole("button", { name: "common.save" }));
    await waitFor(() => expect(update).toHaveBeenLastCalledWith("movies", expect.objectContaining({ preferred_worker_id: "" })));
  });

  it("preserves the preference on loading failure and lets the user retry", async () => {
    listWorkersBasicMock.mockResolvedValueOnce({ error: "Unavailable" }).mockResolvedValueOnce({ data: workers });
    const update = vi.fn().mockResolvedValue(undefined);
    render(<AddCategoryModal open editingCategory={category} onOpenChange={vi.fn()} onCategoryCreated={vi.fn()} onCategoryUpdated={update} />);
    await screen.findByText("categories.preferredWorker.loadFailed");
    expect(screen.getByRole("combobox", { name: "categories.fields.preferredWorker" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "common.save" }));
    await waitFor(() => expect(update).toHaveBeenCalledWith("movies", expect.objectContaining({ preferred_worker_id: "worker-1" })));
  });

  it("retries loading workers without resetting an existing preference", async () => {
    listWorkersBasicMock.mockRejectedValueOnce(new Error("Network")).mockResolvedValueOnce({ data: workers });
    render(<AddCategoryModal open editingCategory={category} onOpenChange={vi.fn()} onCategoryCreated={vi.fn()} />);
    await screen.findByText("categories.preferredWorker.loadFailed");
    fireEvent.click(screen.getByRole("button", { name: "categories.preferredWorker.retry" }));
    await waitFor(() => expect(screen.getByRole("combobox", { name: "categories.fields.preferredWorker" })).toHaveTextContent("Offline worker"));
    expect(screen.queryByText("categories.preferredWorker.loadFailed")).not.toBeInTheDocument();
    expect(listWorkersBasicMock).toHaveBeenCalledTimes(2);
  });
});
