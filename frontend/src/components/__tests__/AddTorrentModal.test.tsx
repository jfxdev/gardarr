import type {
  ButtonHTMLAttributes,
  InputHTMLAttributes,
  LabelHTMLAttributes,
  ReactNode,
  TextareaHTMLAttributes,
} from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AddTorrentModal } from "@/components/AddTorrentModal";
import { AddTorrentContext, type AddTorrentContextValue } from "@/contexts/add-torrent-context";
import type { Task } from "@/types/torrent";
import type { Worker } from "@/types/worker";

const navigateMock = vi.fn();
let currentPathname = "/workers";

vi.mock("react-router", () => ({
  useNavigate: () => navigateMock,
  useLocation: () => ({ pathname: currentPathname }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
  },
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock("@/hooks/use-portrait-mobile-tablet", () => ({
  useIsPortraitMobileOrTablet: () => false,
}));

vi.mock("lucide-react", () => ({
  Check: () => <span aria-hidden="true" />,
  ChevronsUpDown: () => <span aria-hidden="true" />,
  Database: () => <span aria-hidden="true" />,
  Download: () => <span aria-hidden="true" />,
  FileText: () => <span aria-hidden="true" />,
  FileUp: () => <span aria-hidden="true" />,
  Folder: () => <span aria-hidden="true" />,
  Globe: () => <span aria-hidden="true" />,
  HardDrive: () => <span aria-hidden="true" />,
  Link: () => <span aria-hidden="true" />,
  Loader2: () => <span aria-hidden="true" />,
  Server: () => <span aria-hidden="true" />,
  Sparkles: () => <span aria-hidden="true" />,
  X: () => <span aria-hidden="true" />,
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({
    children,
    type = "button",
    ...props
  }: ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button type={type} {...props}>
      {children}
    </button>
  ),
}));

vi.mock("@/components/ui/input", () => ({
  Input: (props: InputHTMLAttributes<HTMLInputElement>) => <input {...props} />,
}));

vi.mock("@/components/ui/label", () => ({
  Label: ({ children, ...props }: LabelHTMLAttributes<HTMLLabelElement>) => <label {...props}>{children}</label>,
}));

vi.mock("@/components/ui/scroll-area", () => ({
  ScrollArea: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/ui/sheet", () => ({
  Sheet: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  SheetContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/ui/textarea", () => ({
  Textarea: (props: TextareaHTMLAttributes<HTMLTextAreaElement>) => <textarea {...props} />,
}));

vi.mock("@/components/ui/popover", () => ({
  Popover: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  PopoverContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  PopoverTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("@/components/ui/command", () => ({
  Command: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  CommandEmpty: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  CommandGroup: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  CommandInput: (props: InputHTMLAttributes<HTMLInputElement>) => <input {...props} />,
  CommandItem: ({ children, onSelect }: { children: ReactNode; onSelect: () => void }) => <button type="button" onClick={onSelect}>{children}</button>,
  CommandList: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/ui/WorkerIcon", () => ({
  WorkerIcon: () => <span aria-hidden="true" />,
}));

vi.mock("@/components/SelectCategory", () => ({
  SelectCategory: ({
    onCategoryChange,
    error,
  }: {
    onCategoryChange: (categoryId: string, category?: { id: string; name: string; default_tags?: string[]; default_directories?: string[] }) => void;
    error?: string;
  }) => (
    <div>
      <button
        type="button"
        onClick={() =>
          onCategoryChange("cat-1", {
            id: "cat-1",
            name: "Games",
            default_tags: ["auto-tag"],
            default_directories: ["/downloads/games", "/downloads/games-4k"],
          })
        }
      >
        Select category
      </button>
      {error && <p>{error}</p>}
    </div>
  ),
}));

vi.mock("@/components/SelectTags", () => ({
  SelectTags: ({ tags, error }: { tags: string[]; error?: string }) => (
    <div>
      <div data-testid="tags">{tags.join(",")}</div>
      {error && <p>{error}</p>}
    </div>
  ),
}));

const createTaskMock = vi.fn();
const parseReleaseMock = vi.fn();
const parseReleaseFileMock = vi.fn();
const updateNameMock = vi.fn();
const listCategoriesMock = vi.fn();

vi.mock("@/services/torrents", () => ({
  convertMagnetUriToTaskMagnetLink: (magnetUri: string) => ({
    hash: "HASH-1",
    display_name: magnetUri,
    trackers: [],
    exact_length: "",
    exact_source: "",
  }),
  torrentService: {
    createTask: (...args: unknown[]) => createTaskMock(...args),
  },
}));

const baseWorker: Worker = {
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
};

vi.mock("@/services/workers", () => ({
  workerService: {
    listWorkers: () => Promise.resolve({ data: [baseWorker] }),
  },
}));

vi.mock("@/services/categories", () => ({
  categoryService: {
    listCategories: (...args: unknown[]) => listCategoriesMock(...args),
  },
}));

vi.mock("@/services/taskMetadata", () => ({
  taskMetadataService: {
    parseRelease: (...args: unknown[]) => parseReleaseMock(...args),
    parseReleaseFile: (...args: unknown[]) => parseReleaseFileMock(...args),
    updateName: (...args: unknown[]) => updateNameMock(...args),
  },
}));

const baseTask: Task = {
  id: "task-1",
  name: "Task 1",
  hash: "hash-1",
  created_at: "2024-01-01T00:00:00Z",
  state: "queued",
  category: "Games",
  path: "/downloads/task-1",
  priority: 0,
  ratio: 0,
  size: 0,
  progress: 0,
  magnet_uri: "magnet:?xt=urn:btih:test-hash",
  magnet_link: {
    hash: "hash-1",
    display_name: "Task 1",
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
    download: { speed: 0, amount: 0 },
    upload: { speed: 0, amount: 0 },
  },
  tags: ["auto-tag"],
};

function buildContext(overrides: Partial<AddTorrentContextValue> = {}): AddTorrentContextValue {
  return {
    isAddModalOpen: true,
    addModalMode: "magnet",
    openAddModal: vi.fn(),
    closeAddModal: vi.fn(),
    pendingTorrents: [],
    addPendingTorrent: vi.fn(),
    removePendingTorrent: vi.fn(),
    ...overrides,
  };
}

function renderModal(context: AddTorrentContextValue) {
  return render(
    <AddTorrentContext.Provider value={context}>
      <AddTorrentModal />
    </AddTorrentContext.Provider>
  );
}

async function fillForm() {
  fireEvent.change(screen.getByPlaceholderText("torrents.addModal.magnetUri.placeholder"), {
    target: { value: "magnet:?xt=urn:btih:test-hash" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Select category" }));
  await waitFor(() => expect(screen.getAllByText("Worker 1")[0]).toBeInTheDocument());
}

describe("AddTorrentModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    currentPathname = "/workers";
    createTaskMock.mockResolvedValue({ data: baseTask });
    parseReleaseMock.mockResolvedValue({});
    parseReleaseFileMock.mockResolvedValue({});
    updateNameMock.mockResolvedValue({});
    listCategoriesMock.mockResolvedValue({ data: [] });
  });

  it("renders magnet and upload tabs without step navigation", async () => {
    renderModal(buildContext());

    await waitFor(() => expect(screen.getAllByText("Worker 1")[0]).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: "torrents.next" })).not.toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "torrents.addModal.source.magnet" })).toHaveAttribute("data-state", "active");
    expect(screen.getByPlaceholderText("torrents.addModal.magnetUri.placeholder")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "torrents.addModal.actions.add" })).toBeInTheDocument();
    expect(screen.getByPlaceholderText("torrents.addModal.directory.placeholder")).toBeInTheDocument();
  });

  it("switches between magnet and torrent file upload", async () => {
    const user = userEvent.setup();
    renderModal(buildContext());

    await user.type(screen.getByPlaceholderText("torrents.addModal.magnetUri.placeholder"), "magnet:?xt=urn:btih:separate-values");

    await user.click(screen.getByRole("tab", { name: "torrents.addModal.source.upload" }));

    await waitFor(() => {
      expect(screen.getByLabelText(/torrents.addModal.file.label/)).toBeInTheDocument();
      expect(screen.queryByPlaceholderText("torrents.addModal.magnetUri.placeholder")).not.toBeInTheDocument();
    });

    await user.click(screen.getByRole("tab", { name: "torrents.addModal.source.magnet" }));

    await waitFor(() => {
      expect(screen.getByPlaceholderText("torrents.addModal.magnetUri.placeholder")).toBeInTheDocument();
    });
    expect(screen.getByPlaceholderText("torrents.addModal.magnetUri.placeholder")).toHaveValue("");
  });

  it("shows the selected torrent file as an attachment and allows removing it", async () => {
    const user = userEvent.setup();
    renderModal(buildContext());
    await user.click(screen.getByRole("tab", { name: "torrents.addModal.source.upload" }));

    const file = new File(["torrent"], "release.torrent", { type: "application/x-bittorrent" });
    fireEvent.change(screen.getByLabelText(/torrents\.addModal\.file\.label/), { target: { files: [file] } });

    expect(screen.getByText("release.torrent")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "torrents.addModal.file.remove" }));
    expect(screen.queryByText("release.torrent")).not.toBeInTheDocument();
    expect(screen.getByText("torrents.addModal.file.placeholder")).toBeInTheDocument();
  });

  it("auto-fills directory and tags when a category is selected", async () => {
    renderModal(buildContext());

    fireEvent.click(screen.getByRole("button", { name: "Select category" }));

    expect(screen.getByRole("combobox", { name: /torrents\.addModal\.directory\.label/ })).toHaveTextContent("/downloads/games");
    fireEvent.click(screen.getByRole("button", { name: "/downloads/games-4k" }));
    expect(screen.getByRole("combobox", { name: /torrents\.addModal\.directory\.label/ })).toHaveTextContent("/downloads/games-4k");
    expect(screen.getByTestId("tags")).toHaveTextContent("auto-tag");
  });

  it("pre-fills high-confidence release suggestions without making them mandatory", async () => {
    listCategoriesMock.mockResolvedValue({
      data: [{ id: "movie", name: "Movies", release_type: "movie", default_tags: ["movie"], default_directories: ["/downloads/movies"] }],
    });
    parseReleaseMock.mockResolvedValue({
      data: {
        release: { type: "movie", confidence: "high", title: "The Matrix", year: "1999" },
        display_name: "The Matrix (1999)",
        tags: ["quality::2160p", "source::bluray", "codec::x265"],
      },
    });
    renderModal(buildContext());
    await waitFor(() => expect(screen.getAllByText("Worker 1")[0]).toBeInTheDocument());
    const magnet = screen.getByLabelText(/torrents\.addModal\.magnetUri\.label/);
    fireEvent.change(magnet, { target: { value: "magnet:?xt=urn:btih:test&dn=The.Matrix.1999.2160p.BluRay.x265" } });

    await waitFor(() => expect(parseReleaseMock).toHaveBeenCalled());
    expect(screen.getByDisplayValue("The Matrix (1999)")).toBeInTheDocument();
    expect(screen.getByTestId("tags")).toHaveTextContent("movie,quality::2160p,source::bluray,codec::x265");
    expect(screen.getByRole("combobox", { name: /torrents\.addModal\.directory\.label/ })).toHaveTextContent("/downloads/movies");
  });

  it("does not re-parse the release name when an unrelated field is edited", async () => {
    listCategoriesMock.mockResolvedValue({
      data: [{ id: "movie", name: "Movies", release_type: "movie", default_tags: ["movie"], default_directories: [] }],
    });
    parseReleaseMock.mockResolvedValue({
      data: {
        release: { type: "movie", confidence: "high", title: "The Matrix", year: "1999" },
        display_name: "The Matrix (1999)",
        tags: ["quality::2160p"],
      },
    });
    renderModal(buildContext());
    await waitFor(() => expect(screen.getAllByText("Worker 1")[0]).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText(/torrents\.addModal\.magnetUri\.label/), {
      target: { value: "magnet:?xt=urn:btih:test&dn=The.Matrix.1999.2160p.BluRay.x265" },
    });
    await waitFor(() => expect(parseReleaseMock).toHaveBeenCalledTimes(1));

    fireEvent.change(screen.getByLabelText(/torrents\.addModal\.directory\.label/), {
      target: { value: "/downloads/custom" },
    });

    await new Promise((resolve) => setTimeout(resolve, 1000));
    expect(parseReleaseMock).toHaveBeenCalledTimes(1);
  });

  it("clears auto-filled fields when a later parse drops to low confidence", async () => {
    listCategoriesMock.mockResolvedValue({
      data: [{ id: "movie", name: "Movies", release_type: "movie", default_tags: ["movie"], default_directories: ["/downloads/movies"] }],
    });
    parseReleaseMock.mockResolvedValueOnce({
      data: {
        release: { type: "movie", confidence: "high", title: "The Matrix", year: "1999" },
        display_name: "The Matrix (1999)",
        tags: ["quality::2160p"],
      },
    });
    renderModal(buildContext());
    await waitFor(() => expect(screen.getAllByText("Worker 1")[0]).toBeInTheDocument());
    fireEvent.change(screen.getByPlaceholderText("torrents.addModal.magnetUri.placeholder"), {
      target: { value: "magnet:?xt=urn:btih:test&dn=The.Matrix.1999.2160p.BluRay.x265" },
    });
    await screen.findByDisplayValue("The Matrix (1999)");
    expect(screen.getByRole("combobox", { name: /torrents\.addModal\.directory\.label/ })).toHaveTextContent("/downloads/movies");
    expect(screen.getByTestId("tags")).toHaveTextContent("quality::2160p");

    parseReleaseMock.mockResolvedValueOnce({
      data: {
        release: { type: "unknown", confidence: "low" },
        display_name: "",
        tags: [],
      },
    });
    fireEvent.change(screen.getByPlaceholderText("torrents.addModal.magnetUri.placeholder"), {
      target: { value: "magnet:?xt=urn:btih:test2&dn=asdf" },
    });

    await waitFor(() => expect(parseReleaseMock).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(screen.getByPlaceholderText("torrents.addModal.release.displayNamePlaceholder")).toHaveValue(""));
    expect(screen.getByPlaceholderText("torrents.addModal.directory.placeholder")).toHaveValue("");
    expect(screen.getByTestId("tags")).toHaveTextContent("");
  });

  it("applies a high-confidence category match once categories finish loading after the parse", async () => {
    let resolveCategories: (value: { data: unknown[] }) => void = () => {};
    listCategoriesMock.mockReturnValue(
      new Promise((resolve) => {
        resolveCategories = resolve;
      })
    );
    parseReleaseMock.mockResolvedValue({
      data: {
        release: { type: "movie", confidence: "high", title: "The Matrix", year: "1999" },
        display_name: "The Matrix (1999)",
        tags: ["quality::2160p"],
      },
    });
    renderModal(buildContext());
    await waitFor(() => expect(screen.getAllByText("Worker 1")[0]).toBeInTheDocument());
    fireEvent.change(screen.getByPlaceholderText("torrents.addModal.magnetUri.placeholder"), {
      target: { value: "magnet:?xt=urn:btih:test&dn=The.Matrix.1999.2160p.BluRay.x265" },
    });

    // Parse resolves (and applySuggestion runs) while categories are still loading.
    await screen.findByDisplayValue("The Matrix (1999)");
    expect(screen.queryByRole("combobox", { name: /torrents\.addModal\.directory\.label/ })).not.toBeInTheDocument();

    resolveCategories({
      data: [{ id: "movie", name: "Movies", release_type: "movie", default_tags: ["movie"], default_directories: ["/downloads/movies"] }],
    });

    await waitFor(() => expect(screen.getByRole("combobox", { name: /torrents\.addModal\.directory\.label/ })).toHaveTextContent("/downloads/movies"));
    expect(screen.getByTestId("tags")).toHaveTextContent("movie,quality::2160p");
  });

  it("suggests a matching game category for a high-confidence game release", async () => {
    listCategoriesMock.mockResolvedValue({
      data: [{ id: "game", name: "Games", release_type: "game", default_tags: ["game"], default_directories: ["/downloads/games"] }],
    });
    parseReleaseMock.mockResolvedValue({
      data: {
        release: { type: "game", confidence: "high", title: "Elden Ring" },
        display_name: "Elden Ring",
        tags: ["type::game", "platform::pc"],
      },
    });
    renderModal(buildContext());
    await waitFor(() => expect(screen.getAllByText("Worker 1")[0]).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText(/torrents\.addModal\.magnetUri\.label/), {
      target: { value: "magnet:?xt=urn:btih:test&dn=Elden.Ring.v1.12.0.PC-RUNE" },
    });

    await waitFor(() => expect(parseReleaseMock).toHaveBeenCalled());
    expect(screen.getByRole("combobox", { name: /torrents\.addModal\.directory\.label/ })).toHaveTextContent("/downloads/games");
    expect(screen.getByTestId("tags")).toHaveTextContent("game,type::game,platform::pc");
  });

  it("prefers the category with the most overlapping tags when several share a release_type", async () => {
    listCategoriesMock.mockResolvedValue({
      data: [
        { id: "generic", name: "Shows", release_type: "anime", default_tags: ["misc"], default_directories: ["/downloads/shows"] },
        { id: "anime", name: "Anime", release_type: "anime", default_tags: ["type::anime"], default_directories: ["/downloads/anime"] },
      ],
    });
    parseReleaseMock.mockResolvedValue({
      data: {
        release: { type: "anime", confidence: "high", title: "Frieren" },
        display_name: "Frieren",
        tags: ["type::anime", "season::01"],
      },
    });
    renderModal(buildContext());
    await waitFor(() => expect(screen.getAllByText("Worker 1")[0]).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText(/torrents\.addModal\.magnetUri\.label/), {
      target: { value: "magnet:?xt=urn:btih:test&dn=Frieren.S01E01" },
    });

    await waitFor(() => expect(parseReleaseMock).toHaveBeenCalled());
    expect(screen.getByRole("combobox", { name: /torrents\.addModal\.directory\.label/ })).toHaveTextContent("/downloads/anime");
  });

  it("submits optimistically: pending placeholder, close, navigate, createTask", async () => {
    const context = buildContext();
    renderModal(context);
    await fillForm();

    fireEvent.click(screen.getByRole("button", { name: "torrents.addModal.actions.add" }));

    await waitFor(() => expect(createTaskMock).toHaveBeenCalledTimes(1));
    expect(createTaskMock).toHaveBeenCalledWith("worker-1", {
      magnet_uri: "magnet:?xt=urn:btih:test-hash",
      category: "Games",
      tags: ["auto-tag"],
      directory: "/downloads/games",
    });
    expect(context.addPendingTorrent).toHaveBeenCalledWith(
      expect.objectContaining({
        hash: "hash-1",
        workerId: "worker-1",
        category: "Games",
      })
    );
    expect(context.closeAddModal).toHaveBeenCalled();
    expect(navigateMock).toHaveBeenCalledWith("/torrents");
    expect(context.removePendingTorrent).not.toHaveBeenCalled();
  });

  it("retries once and shows a localized warning when the post-create name update keeps failing", async () => {
    createTaskMock.mockResolvedValue({ data: { ...baseTask, was_created: true } });
    updateNameMock.mockResolvedValue({ error: "failed" });
    const context = buildContext();
    renderModal(context);
    await fillForm();
    fireEvent.change(screen.getByPlaceholderText("torrents.addModal.release.displayNamePlaceholder"), {
      target: { value: "Custom Name" },
    });

    fireEvent.click(screen.getByRole("button", { name: "torrents.addModal.actions.add" }));

    await waitFor(() => expect(updateNameMock).toHaveBeenCalledTimes(2));
    expect(toast.warning).toHaveBeenCalledWith("torrents.notifications.addSuccessDisplayNameFailed", { duration: Infinity });
  });

  it("does not overwrite metadata when the torrent already exists", async () => {
    createTaskMock.mockResolvedValue({ data: { ...baseTask, was_created: false } });
    parseReleaseMock.mockResolvedValue({
      data: {
        release: { type: "movie", confidence: "high" },
        display_name: "The Matrix (1999)",
        tags: [],
      },
    });
    const context = buildContext();
    renderModal(context);
    await waitFor(() => expect(screen.getAllByText("Worker 1")[0]).toBeInTheDocument());
    fireEvent.change(screen.getByPlaceholderText("torrents.addModal.magnetUri.placeholder"), {
      target: { value: "magnet:?xt=urn:btih:test-hash&dn=The.Matrix.1999.1080p" },
    });
    await screen.findByDisplayValue("The Matrix (1999)");

    fireEvent.click(screen.getByRole("button", { name: "torrents.addModal.actions.add" }));

    await waitFor(() => expect(createTaskMock).toHaveBeenCalledTimes(1));
    expect(updateNameMock).not.toHaveBeenCalled();
  });

  it("does not navigate when already on /torrents", async () => {
    currentPathname = "/torrents";
    const context = buildContext();
    renderModal(context);
    await fillForm();

    fireEvent.click(screen.getByRole("button", { name: "torrents.addModal.actions.add" }));

    await waitFor(() => expect(createTaskMock).toHaveBeenCalledTimes(1));
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("removes the pending placeholder when createTask fails", async () => {
    createTaskMock.mockResolvedValue({ error: "boom" });
    const context = buildContext();
    renderModal(context);
    await fillForm();

    fireEvent.click(screen.getByRole("button", { name: "torrents.addModal.actions.add" }));

    await waitFor(() => expect(context.removePendingTorrent).toHaveBeenCalledWith("hash-1"));
    expect(context.addPendingTorrent).toHaveBeenCalled();
  });

  it("blocks submission when the required magnet is missing", async () => {
    const context = buildContext();
    renderModal(context);
    await waitFor(() => expect(screen.getAllByText("Worker 1")[0]).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: "torrents.addModal.actions.add" }));

    expect(createTaskMock).not.toHaveBeenCalled();
    expect(context.addPendingTorrent).not.toHaveBeenCalled();
    expect(screen.getByText("torrents.addModal.errors.magnetRequired")).toBeInTheDocument();
  });
});
