import type { CSSProperties } from "react"
import { useCallback, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate } from "react-router"

import { AddTorrentModal } from "@/components/AddTorrentModal"
import { AppSidebar } from "@/components/app-sidebar"
import { getPageTitle } from "@/components/navigation-config"
import PageTransition from "@/components/PageTransition"
import { SiteHeader } from "@/components/site-header"
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { AddTorrentProvider } from "@/contexts/AddTorrentContext"
import { useAddTorrent } from "@/contexts/add-torrent-hooks"
import { TagColorsProvider } from "@/contexts/TagColorsContext"

const SIDEBAR_STORAGE_KEY = "sidebarOpen"
const SIDEBAR_WIDTH_STORAGE_KEY = "sidebarWidth"
const DEFAULT_SIDEBAR_WIDTH = 240
const MIN_SIDEBAR_WIDTH = 208
const MAX_SIDEBAR_WIDTH = 352

function getInitialSidebarWidth() {
  const storedWidth = Number(localStorage.getItem(SIDEBAR_WIDTH_STORAGE_KEY))
  if (!Number.isFinite(storedWidth) || storedWidth === 0) {
    return DEFAULT_SIDEBAR_WIDTH
  }
  return Math.min(MAX_SIDEBAR_WIDTH, Math.max(MIN_SIDEBAR_WIDTH, storedWidth))
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <TagColorsProvider>
      <AddTorrentProvider>
        <AppLayoutInner>{children}</AppLayoutInner>
      </AddTorrentProvider>
    </TagColorsProvider>
  )
}

function AppLayoutInner({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const { openAddModal } = useAddTorrent()
  const navigate = useNavigate()
  const location = useLocation()
  const [sidebarOpen, setSidebarOpen] = useState(() => {
    const stored = localStorage.getItem(SIDEBAR_STORAGE_KEY)
    return stored === null ? true : stored === "true"
  })
  const [sidebarWidth, setSidebarWidth] = useState(getInitialSidebarWidth)
  const [isDark, setIsDark] = useState(false)

  useEffect(() => {
    const root = document.documentElement
    const stored = localStorage.getItem("theme")
    const dark = stored ? stored === "dark" : true

    root.classList.toggle("dark", dark)
    setIsDark(dark)
  }, [])

  useEffect(() => {
    const params = new URLSearchParams(location.search)
    if (params.get("add") === "1") {
      openAddModal()
      params.delete("add")
      navigate(
        { pathname: location.pathname, search: params.toString() },
        { replace: true },
      )
    }
  }, [location.search, location.pathname, openAddModal, navigate])

  const handleSidebarOpenChange = useCallback((open: boolean) => {
    setSidebarOpen(open)
    localStorage.setItem(SIDEBAR_STORAGE_KEY, String(open))
  }, [])

  const handleSidebarWidthChange = useCallback((width: number) => {
    setSidebarWidth(width)
    localStorage.setItem(SIDEBAR_WIDTH_STORAGE_KEY, String(width))
  }, [])

  const toggleTheme = () => {
    const root = document.documentElement
    const nextDark = !root.classList.contains("dark")

    root.classList.toggle("dark", nextDark)
    localStorage.setItem("theme", nextDark ? "dark" : "light")
    setIsDark(nextDark)
  }

  return (
    <SidebarProvider
      open={sidebarOpen}
      onOpenChange={handleSidebarOpenChange}
      width={sidebarWidth}
      onWidthChange={handleSidebarWidthChange}
      minWidth={MIN_SIDEBAR_WIDTH}
      maxWidth={MAX_SIDEBAR_WIDTH}
      resizable
      className="h-dvh min-h-0 max-h-dvh overflow-hidden bg-background text-foreground safe-left safe-right"
      style={
        {
          "--header-height": "3rem",
        } as CSSProperties
      }
    >
      <AppSidebar variant="inset" />
      <SidebarInset className="min-h-0 overflow-hidden">
        <SiteHeader
          title={getPageTitle(t, location.pathname)}
          isDark={isDark}
          onToggleTheme={toggleTheme}
        />

        <div className="flex min-h-0 flex-1 flex-col">
          <div className="scrollbar flex-1 overflow-x-hidden overflow-y-auto">
            <div className="mx-auto w-full max-w-7xl p-4 pb-[max(1rem,env(safe-area-inset-bottom))] md:p-6 md:pb-[max(1.5rem,env(safe-area-inset-bottom))]">
              <PageTransition>{children}</PageTransition>
            </div>
          </div>
        </div>
      </SidebarInset>
      <AddTorrentModal />
    </SidebarProvider>
  )
}
