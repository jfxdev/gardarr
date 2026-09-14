import type { ComponentProps, ReactNode } from "react"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { MemoryRouter } from "react-router"
import { beforeEach, describe, expect, it, vi } from "vitest"

import AppLayout from "@/AppLayout"

const SIDEBAR_STORAGE_KEY = "sidebarOpen"
const SIDEBAR_WIDTH_STORAGE_KEY = "sidebarWidth"
const mocks = vi.hoisted(() => ({
  role: "user" as "admin" | "user",
  logout: vi.fn(async () => undefined),
  openAddModal: vi.fn(),
}))

function setWindowWidth(width: number) {
  Object.defineProperty(window, "innerWidth", { writable: true, configurable: true, value: width })
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: window.innerWidth < 768,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
}

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock("@/contexts/auth-hooks", () => ({
  useAuth: () => ({
    user: { email: "viewer@example.com", role: mocks.role },
    logout: mocks.logout,
  }),
}))

vi.mock("@/contexts/add-torrent-hooks", () => ({
  useAddTorrent: () => ({ openAddModal: mocks.openAddModal }),
}))

vi.mock("@/contexts/AddTorrentContext", () => ({
  AddTorrentProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}))

vi.mock("@/contexts/TagColorsContext", () => ({
  TagColorsProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}))

vi.mock("@/components/AddTorrentModal", () => ({ AddTorrentModal: () => null }))
vi.mock("@/components/AddTorrentButton", () => ({
  AddTorrentButton: ({ children, ...props }: ComponentProps<"button">) => (
    <button {...props}>{children}</button>
  ),
}))
vi.mock("@/components/VariantColorSelectButton", () => ({ default: () => null }))
vi.mock("@/components/DisplaySettingsButton", () => ({ default: () => null }))
vi.mock("@/components/PageTransition", () => ({
  default: ({ children }: { children: ReactNode }) => <>{children}</>,
}))
vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
}))
vi.mock("@/components/ui/dropdown-menu", () => ({
  DropdownMenu: ({ children }: { children: ReactNode }) => <>{children}</>,
  DropdownMenuContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DropdownMenuGroup: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DropdownMenuLabel: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  DropdownMenuSeparator: () => <hr />,
  DropdownMenuTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
  DropdownMenuItem: ({
    children,
    asChild,
    onSelect,
  }: {
    children: ReactNode
    asChild?: boolean
    onSelect?: () => void
  }) => asChild ? <>{children}</> : <button onClick={onSelect}>{children}</button>,
}))

function renderLayout(path = "/") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AppLayout>
        <div>content</div>
      </AppLayout>
    </MemoryRouter>,
  )
}

function getDesktopSidebarState() {
  return screen.getByTestId("app-sidebar").closest("[data-state]")
}

describe("AppLayout dashboard-01 shell", () => {
  beforeEach(() => {
    localStorage.clear()
    mocks.role = "user"
    mocks.logout.mockClear()
    mocks.openAddModal.mockClear()
    setWindowWidth(1024)
  })

  it("starts expanded on desktop and shows the current page title", () => {
    renderLayout("/reports")

    expect(getDesktopSidebarState()).toHaveAttribute("data-state", "expanded")
    expect(screen.getByTestId("app-sidebar").closest("[data-slot=sidebar-wrapper]")).toHaveStyle({
      "--sidebar-width": "240px",
    })
    expect(screen.getByRole("heading", { name: "navigation.reports" })).toBeInTheDocument()
  })

  it("resizes the desktop sidebar by keyboard and drag, then persists its width", () => {
    const { unmount } = renderLayout()
    const resizeHandle = screen.getByRole("button", { name: "navigation.resizeSidebar" })

    fireEvent.keyDown(resizeHandle, { key: "ArrowRight" })

    fireEvent.pointerDown(resizeHandle, { button: 0, clientX: 248 })
    fireEvent.pointerMove(window, { clientX: 280 })
    fireEvent.pointerUp(window)

    expect(localStorage.getItem(SIDEBAR_WIDTH_STORAGE_KEY)).toBe("280")
    expect(screen.getByTestId("app-sidebar").closest("[data-slot=sidebar-wrapper]")).toHaveStyle({
      "--sidebar-width": "280px",
    })

    unmount()
    renderLayout()

    expect(screen.getByTestId("app-sidebar").closest("[data-slot=sidebar-wrapper]")).toHaveStyle({
      "--sidebar-width": "280px",
    })
  })

  it("persists the desktop offcanvas state across remounts", () => {
    const { unmount } = renderLayout()

    fireEvent.click(screen.getByLabelText("navigation.toggleSidebar"))

    expect(localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe("false")
    expect(getDesktopSidebarState()).toHaveAttribute("data-collapsible", "offcanvas")

    unmount()
    renderLayout()

    expect(getDesktopSidebarState()).toHaveAttribute("data-state", "collapsed")
  })

  it("keeps the mobile sidebar closed initially and closes it after adding a torrent", async () => {
    setWindowWidth(500)
    renderLayout()

    expect(screen.queryByText("Gardarr")).not.toBeInTheDocument()

    fireEvent.click(screen.getByLabelText("navigation.toggleSidebar"))
    const addButton = await screen.findByRole("button", { name: "torrents.addTorrent" })
    expect(addButton.closest("[data-mobile=true]")).toHaveStyle({
      "--sidebar-width": "16rem",
    })
    fireEvent.click(addButton)

    expect(mocks.openAddModal).toHaveBeenCalledOnce()
    await waitFor(() => expect(screen.queryByText("Gardarr")).not.toBeInTheDocument())

    fireEvent.click(screen.getByLabelText("navigation.toggleSidebar"))
    fireEvent.click(await screen.findByRole("link", { name: "navigation.categories" }))
    await waitFor(() => expect(screen.queryByText("Gardarr")).not.toBeInTheDocument())

    expect(localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBeNull()
  })

  it("marks nested routes as active without showing a dashboard sidebar link", () => {
    const { unmount } = renderLayout("/integrations/discord")

    expect(screen.getByRole("link", { name: "navigation.integrations" })).toHaveAttribute(
      "aria-current",
      "page",
    )
    expect(screen.getByRole("heading", { name: "navigation.integrations" })).toBeInTheDocument()

    unmount()
    renderLayout("/worker/worker-1/task/task-1")

    expect(screen.queryByRole("link", { name: "navigation.dashboard" })).not.toBeInTheDocument()
    expect(screen.getByRole("link", { name: "navigation.workers" })).toHaveAttribute(
      "aria-current",
      "page",
    )
    expect(screen.getByRole("heading", { name: "navigation.workers" })).toBeInTheDocument()
  })

  it("shows account actions and gates administration links by role", () => {
    const { unmount } = renderLayout()

    expect(screen.getByRole("link", { name: "navigation.profile" })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "navigation.about" })).toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "navigation.settings" })).not.toBeInTheDocument()
    expect(screen.queryByRole("link", { name: "navigation.users" })).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole("button", { name: "auth.logout" }))
    expect(mocks.logout).toHaveBeenCalledOnce()

    unmount()
    mocks.role = "admin"
    renderLayout()

    expect(screen.getByRole("link", { name: "navigation.settings" })).toBeInTheDocument()
    expect(screen.getByRole("link", { name: "navigation.users" })).toBeInTheDocument()
  })
})
