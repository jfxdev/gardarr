import type { ReactNode } from "react";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi, beforeEach } from "vitest";
import AppLayout from "@/AppLayout";

const SIDEBAR_STORAGE_KEY = "sidebarOpen";

function setWindowWidth(width: number) {
  Object.defineProperty(window, "innerWidth", { writable: true, configurable: true, value: width });
}

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("lucide-react", () => {
  const Stub = () => <span />;
  return {
    Settings: Stub, Users: Stub, ArrowDownUp: Stub, Menu: Stub, Sun: Stub, Moon: Stub,
    Info: Stub, LogOut: Stub, FolderOpen: Stub, UserCircle: Stub, Server: Stub,
    Plug: Stub, History: Stub, Tag: Stub, Rss: Stub, BarChart3: Stub,
  };
});

vi.mock("@/contexts/auth-hooks", () => ({
  useAuth: () => ({ user: { role: "user" }, logout: vi.fn() }),
}));

vi.mock("@/contexts/add-torrent-hooks", () => ({
  useAddTorrent: () => ({ openAddModal: vi.fn() }),
}));

vi.mock("@/contexts/AddTorrentContext", () => ({
  AddTorrentProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("@/contexts/TagColorsContext", () => ({
  TagColorsProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("@/components/AddTorrentModal", () => ({ AddTorrentModal: () => null }));
vi.mock("@/components/AddTorrentButton", () => ({ AddTorrentButton: () => <button /> }));
vi.mock("@/components/VariantColorSelectButton", () => ({ default: () => null }));
vi.mock("@/components/DisplaySettingsButton", () => ({ default: () => null }));
vi.mock("@/components/PageTransition", () => ({ default: ({ children }: { children: ReactNode }) => <>{children}</> }));
vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

const renderLayout = () => render(
  <MemoryRouter>
    <AppLayout>
      <div>content</div>
    </AppLayout>
  </MemoryRouter>
);

describe("AppLayout sidebar persistence", () => {
  beforeEach(() => {
    localStorage.clear();
    setWindowWidth(1024);
  });

  it("defaults to collapsed on desktop when nothing is stored", () => {
    renderLayout();
    expect(screen.getByTestId("app-sidebar").className).toContain("md:w-14");
  });

  it("persists the open state to localStorage and restores it across remounts", () => {
    const { unmount } = renderLayout();

    fireEvent.click(screen.getByLabelText("navigation.toggleSidebar"));

    expect(localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe("true");
    expect(screen.getByTestId("app-sidebar").className).toContain("md:w-52");

    unmount();
    renderLayout();

    expect(screen.getByTestId("app-sidebar").className).toContain("md:w-52");
  });

  it("ignores a stored desktop preference on mobile so the overlay doesn't open on load", () => {
    localStorage.setItem(SIDEBAR_STORAGE_KEY, "true");
    setWindowWidth(500);

    renderLayout();

    expect(screen.getByTestId("app-sidebar").className).toContain("-translate-x-full");

    fireEvent.click(screen.getByLabelText("navigation.toggleSidebar"));
    fireEvent.click(screen.getByLabelText("navigation.toggleSidebar"));

    expect(localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe("true");
  });

  it("restores the stored desktop preference after a mobile-to-desktop resize instead of overwriting it", () => {
    localStorage.setItem(SIDEBAR_STORAGE_KEY, "true");
    setWindowWidth(500);

    renderLayout();
    expect(screen.getByTestId("app-sidebar").className).toContain("-translate-x-full");

    setWindowWidth(1024);
    fireEvent(window, new Event("resize"));

    expect(screen.getByTestId("app-sidebar").className).toContain("md:w-52");
    expect(localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe("true");
  });
});
