import type {
  ButtonHTMLAttributes,
  ComponentProps,
  InputHTMLAttributes,
  LabelHTMLAttributes,
  ReactNode,
} from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { TaskMetadataSearchSheet } from "@/components/TaskMetadataSearchSheet";
import { taskMetadataService } from "@/services/taskMetadata";
import type { Category } from "@/types/category";
import type { Task } from "@/types/torrent";
import { toast } from "sonner";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock("@/hooks/use-portrait-mobile-tablet", () => ({
  useIsPortraitMobileOrTablet: () => false,
}));

vi.mock("lucide-react", () => ({
  Calendar: () => <span aria-hidden="true" />,
  Check: () => <span aria-hidden="true" />,
  Image: () => <span aria-hidden="true" />,
  Loader2: () => <span aria-hidden="true" />,
  Search: () => <span aria-hidden="true" />,
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

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock("@/services/taskMetadata", () => ({
  taskMetadataService: {
    getProviderImagePreviewUrl: (provider: string, imageId?: string) =>
      imageId ? `/v1/tasks/metadata/providers/${provider}/image?image_id=${encodeURIComponent(imageId)}` : undefined,
    getProviderStatus: vi.fn(),
    searchProvider: vi.fn(),
    applyProvider: vi.fn(),
  },
}));

const searchResult = {
  id: "123",
  title: "Halo",
  release_date: "2024-01-01",
  description: "Overview",
  image_id: "front.jpg",
  image_url: "https://cdn.thegamesdb.net/images/large/front.jpg",
};

function renderSearchSheet(overrides: Partial<ComponentProps<typeof TaskMetadataSearchSheet>> = {}) {
  const props = {
    isOpen: true,
    onClose: vi.fn(),
    task: { name: "Halo", hash: "hash-1" } as Task,
    category: { metadata_source: "tgdb" } as Category,
    onApplied: vi.fn(),
    ...overrides,
  };

  render(<TaskMetadataSearchSheet {...props} />);

  return props;
}

describe("TaskMetadataSearchSheet", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(taskMetadataService.getProviderStatus).mockResolvedValue({
      data: {
        provider: "tgdb",
        active: true,
      },
    });

    vi.mocked(taskMetadataService.searchProvider).mockResolvedValue({ data: [searchResult] });
  });

  it("applies metadata using the selected id with fallback metadata", async () => {
    vi.mocked(taskMetadataService.applyProvider).mockResolvedValue({
      data: {
        uuid: "meta-1",
        task_hash: "hash-1",
        name: "Halo",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    });

    const { onApplied, onClose } = renderSearchSheet();

    await waitFor(() => expect(taskMetadataService.searchProvider).toHaveBeenCalledWith("tgdb", "Halo", true));

    fireEvent.click(screen.getByRole("button", { name: "torrents.addModal.actions.finish" }));

    await waitFor(() =>
      expect(taskMetadataService.applyProvider).toHaveBeenCalledWith("hash-1", "tgdb", {
        id: "123",
        title: "Halo",
        release_date: "2024-01-01",
        description: "Overview",
        image_id: "front.jpg",
      })
    );
    expect(toast.success).toHaveBeenCalledWith("tgdb.success.applied");
    expect(onApplied).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });

  it("loads provider previews through the same-origin Gardarr endpoint", async () => {
    renderSearchSheet();

    const preview = await screen.findByRole("img", { name: "Halo" });
    expect(preview).toHaveAttribute(
      "src",
      "/v1/tasks/metadata/providers/tgdb/image?image_id=front.jpg"
    );
  });

  it("passes a raw torrent release name to the backend parser", async () => {
    renderSearchSheet({
      task: {
        name: "Super.Mario.Galaxy.O.Filme.2026.WEB-DL.1080p.x264.DUAL.5.1",
        hash: "hash-1",
      } as Task,
    });

    await waitFor(() =>
      expect(taskMetadataService.searchProvider).toHaveBeenCalledWith(
        "tgdb",
        "Super.Mario.Galaxy.O.Filme.2026.WEB-DL.1080p.x264.DUAL.5.1",
        true
      )
    );
    expect(screen.getByDisplayValue("Super.Mario.Galaxy.O.Filme.2026.WEB-DL.1080p.x264.DUAL.5.1")).toBeInTheDocument();
  });

  it("shows a toast and resets the loading state when apply fails", async () => {
    const user = userEvent.setup();

    vi.mocked(taskMetadataService.applyProvider).mockResolvedValue({
      error: "id is required",
    });

    const { onApplied, onClose } = renderSearchSheet();

    await waitFor(() => expect(taskMetadataService.searchProvider).toHaveBeenCalledWith("tgdb", "Halo", true));

    await user.click(screen.getByRole("button", { name: "torrents.addModal.actions.finish" }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("id is required"));
    expect(onApplied).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "torrents.addModal.actions.finish" })).toBeEnabled();
  });
});
