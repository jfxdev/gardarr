import type { ReactNode } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { TorrentDetailsModal } from "@/components/torrents/TorrentDetailsModal";
import type { Task } from "@/types/torrent";

const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
const mockApiPut = vi.fn();
const mockUpdateTaskCategory = vi.fn();
const mockUpdateTaskTags = vi.fn();
const mockListCategories = vi.fn();
const mockPreferencesLoad = vi.fn();
let dialogContentProps: {
  className?: string;
  onOpenAutoFocus?: (event: Event) => void;
} = {};

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? key,
  }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: (...args: unknown[]) => mockToastSuccess(...args),
    error: (...args: unknown[]) => mockToastError(...args),
  },
}));

vi.mock("@/lib/api", () => ({
  api: {
    put: (...args: unknown[]) => mockApiPut(...args),
  },
}));

vi.mock("@/services/torrents", () => ({
  torrentService: {
    updateTaskCategory: (...args: unknown[]) => mockUpdateTaskCategory(...args),
    updateTaskTags: (...args: unknown[]) => mockUpdateTaskTags(...args),
  },
}));

vi.mock("@/services/categories", () => ({
  categoryService: {
    listCategories: (...args: unknown[]) => mockListCategories(...args),
  },
}));

vi.mock("@/services/preferences", () => ({
  preferencesService: {
    load: (...args: unknown[]) => mockPreferencesLoad(...args),
  },
}));

vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({ children, open }: { children: ReactNode; open?: boolean }) =>
    open === false ? null : <div>{children}</div>,
  DialogContent: ({ children, className, onOpenAutoFocus }: {
    children: ReactNode;
    className?: string;
    onOpenAutoFocus?: (event: Event) => void;
  }) => {
    dialogContentProps = { className, onOpenAutoFocus };
    return <div>{children}</div>;
  },
  DialogDescription: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogFooter: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/ui/popover", () => ({
  Popover: ({ children }: { children: ReactNode }) => <>{children}</>,
  PopoverTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
  PopoverContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/ui/tabs", () => ({
  Tabs: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  TabsList: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  TabsTrigger: ({ children }: { children: ReactNode }) => <button type="button">{children}</button>,
  TabsContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/ui/custom-scroll-area", () => ({
  CustomScrollArea: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("@/components/ui/button-group", () => ({
  ButtonGroup: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/ui/separator", () => ({
  Separator: () => <hr />,
}));

vi.mock("@/components/DeleteTorrentModal", () => ({
  DeleteTorrentModal: () => null,
}));

vi.mock("@/components/torrents/TorrentFilesList", () => ({
  TorrentFilesList: () => null,
}));

vi.mock("@/components/widgets/TorrentRatioWidget", () => ({
  TorrentRatioWidget: ({ onShare }: { onShare?: () => void }) => (
    <button type="button" onClick={onShare}>Abrir compartilhamento pelo ratio</button>
  ),
}));

vi.mock("@/components/widgets/TorrentLifetimeWidget", () => ({
  TorrentLifetimeWidget: () => null,
}));

vi.mock("@/components/torrents/TorrentImageEditor", () => ({
  TorrentImageEditor: () => null,
}));

vi.mock("@/components/torrents/share/ShareableTorrentCardDialog", () => ({
  ShareableTorrentCardDialog: ({ open }: { open: boolean }) => open ? <div>Dialog de compartilhamento aberto</div> : null,
}));

vi.mock("@/components/ui/WorkerIcon", () => ({
  WorkerIcon: () => null,
}));

vi.mock("@/components/ui/ProgressBar", () => ({
  ProgressBar: () => null,
}));

vi.mock("@/components/ui/TagBadge", () => ({
  TagBadge: ({ tag }: { tag: string }) => <span>{tag}</span>,
}));

vi.mock("@/components/SelectCategory", () => ({
  SelectCategory: ({ onCategoryChange }: { onCategoryChange: (categoryId: string, category?: { id: string; name: string }) => void }) => (
    <button
      type="button"
      onClick={() => onCategoryChange("cat-2", { id: "cat-2", name: "Series" })}
    >
      Escolher categoria
    </button>
  ),
}));

vi.mock("@/components/SelectTags", () => ({
  SelectTags: ({ onTagsChange }: { onTagsChange: (tags: string[]) => void }) => (
    <button
      type="button"
      onClick={() => onTagsChange(["b", "c"])}
    >
      Trocar tags
    </button>
  ),
}));

const baseTask: Task = {
  id: "hash-1",
  hash: "hash-1",
  name: "Original Name",
  created_at: "2026-05-09T00:00:00Z",
  completed_at: null,
  state: "DOWNLOADING",
  category: "Movies",
  path: "/downloads",
  priority: 1,
  ratio: 0,
  size: 100,
  progress: 50,
  magnet_uri: "magnet:?xt=urn:btih:hash-1",
  magnet_link: {
    hash: "hash-1",
    display_name: "Original Name",
    trackers: [],
    exact_length: "",
    exact_source: "",
  },
  popularity: 0,
  pairs: {
    swarm_seeders: 0,
    swarm_leechers: 0,
    seeders: 0,
    leechers: 0,
  },
  network: {
    download: {
      speed: 0,
      amount: 0,
    },
    upload: {
      speed: 0,
      amount: 0,
    },
  },
  worker: {
    uuid: "worker-1",
    name: "Worker 1",
    address: "http://worker.test",
    icon: "server",
    color: "blue",
    status: "ACTIVE",
    instance: {
      application: {
        version: "1.0.0",
        api_version: "2.0.0",
      },
      server: {
        free_space_on_disk: 1024,
      },
      transfer: {
        all_time_downloaded: 0,
        all_time_uploaded: 0,
        global_ratio: 0,
        last_external_address_v4: "",
        last_external_address_v6: "",
      },
    },
  },
  tags: ["a", "b"],
  metadata: {
    uuid: "meta-1",
    task_hash: "hash-1",
    name: "Friendly Name",
    created_at: "2026-05-09T00:00:00Z",
    updated_at: "2026-05-09T00:00:00Z",
  },
};

describe("TorrentDetailsModal", () => {
  beforeEach(() => {
    mockToastSuccess.mockReset();
    mockToastError.mockReset();
    mockApiPut.mockReset();
    mockUpdateTaskCategory.mockReset();
    mockUpdateTaskTags.mockReset();
    mockListCategories.mockReset();
    mockPreferencesLoad.mockReset();
    dialogContentProps = {};

    mockApiPut.mockResolvedValue({ data: {} });
    mockUpdateTaskCategory.mockResolvedValue({ data: null });
    mockUpdateTaskTags.mockResolvedValue({ data: null });
    mockListCategories.mockResolvedValue({
      data: [
        { id: "cat-1", name: "Movies", icon: "film", color: "blue" },
        { id: "cat-2", name: "Series", icon: "tv", color: "green" },
      ],
    });
    mockPreferencesLoad.mockReturnValue({});
  });

  it("refreshes after saving friendly name", async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn().mockResolvedValue(undefined);

    render(
      <TorrentDetailsModal
        torrent={baseTask}
        isOpen={true}
        onClose={() => undefined}
        onUpdate={onUpdate}
      />
    );

    await user.click(screen.getByLabelText("Editar nome"));

    const input = screen.getByDisplayValue("Friendly Name");
    await user.clear(input);
    await user.type(input, "Updated Friendly Name");
    // Autosaves on blur (Enter blurs the field)
    await user.keyboard("{Enter}");

    await waitFor(() => {
      expect(mockApiPut).toHaveBeenCalledWith("/tasks/metadata/hash-1/name", {
        name: "Updated Friendly Name",
      });
      expect(onUpdate).toHaveBeenCalledTimes(1);
      expect(mockToastSuccess).toHaveBeenCalled();
    });
  });

  it("uses the full mobile viewport, a narrower desktop width, and does not move focus on open", async () => {
    render(
      <TorrentDetailsModal
        torrent={baseTask}
        isOpen={true}
        onClose={() => undefined}
      />
    );

    await waitFor(() => expect(mockListCategories).toHaveBeenCalled());

    const event = { preventDefault: vi.fn() } as unknown as Event;
    dialogContentProps.onOpenAutoFocus?.(event);

    expect(event.preventDefault).toHaveBeenCalledOnce();
    expect(dialogContentProps.className).toContain("h-[100dvh]");
    expect(dialogContentProps.className).toContain("!max-h-[100dvh]");
    expect(dialogContentProps.className).toContain("sm:!w-[80rem]");
    expect(dialogContentProps.className).toContain("sm:!max-w-[calc(100vw-2rem)]");
  });

  it("refreshes after saving category", async () => {
    const user = userEvent.setup();
    const onCategoryTagsUpdate = vi.fn();

    render(
      <TorrentDetailsModal
        torrent={baseTask}
        isOpen={true}
        onClose={() => undefined}
        onCategoryTagsUpdate={onCategoryTagsUpdate}
      />
    );

    // Category select is always open; picking a value autosaves immediately.
    await user.click(screen.getByRole("button", { name: "Escolher categoria" }));

    await waitFor(() => {
      expect(mockUpdateTaskCategory).toHaveBeenCalledWith("worker-1", "hash-1", "Series");
      expect(onCategoryTagsUpdate).toHaveBeenCalledWith("hash-1", { category: "Series" });
    });
  });

  it("does not refresh or close category edit when category update fails", async () => {
    const user = userEvent.setup();
    const onCategoryTagsUpdate = vi.fn();
    mockUpdateTaskCategory.mockResolvedValueOnce({ error: "worker rejected category" });

    render(
      <TorrentDetailsModal
        torrent={baseTask}
        isOpen={true}
        onClose={() => undefined}
        onCategoryTagsUpdate={onCategoryTagsUpdate}
      />
    );

    await user.click(screen.getByRole("button", { name: "Escolher categoria" }));

    await waitFor(() => {
      expect(mockUpdateTaskCategory).toHaveBeenCalledWith("worker-1", "hash-1", "Series");
      expect(mockToastError).toHaveBeenCalledWith("worker rejected category");
    });

    expect(onCategoryTagsUpdate).not.toHaveBeenCalled();
    expect(mockToastSuccess).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Escolher categoria" })).toBeInTheDocument();
  });

  it("refreshes after replacing tags", async () => {
    const user = userEvent.setup();
    const onCategoryTagsUpdate = vi.fn();

    render(
      <TorrentDetailsModal
        torrent={baseTask}
        isOpen={true}
        onClose={() => undefined}
        onCategoryTagsUpdate={onCategoryTagsUpdate}
      />
    );

    // Tags editor is always open; changing tags autosaves immediately.
    await user.click(screen.getByRole("button", { name: "Trocar tags" }));

    await waitFor(() => {
      expect(mockUpdateTaskTags).toHaveBeenCalledWith("worker-1", "hash-1", ["b", "c"]);
      expect(onCategoryTagsUpdate).toHaveBeenCalledWith("hash-1", { tags: ["b", "c"] });
    });
  });

  it("opens the share dialog from the ratio card", async () => {
    const user = userEvent.setup();

    render(
      <TorrentDetailsModal
        torrent={baseTask}
        isOpen={true}
        onClose={() => undefined}
      />
    );

    await user.click(screen.getByRole("button", { name: "Abrir compartilhamento pelo ratio" }));

    expect(screen.getByText("Dialog de compartilhamento aberto")).toBeInTheDocument();
  });
});
