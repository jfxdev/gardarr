import type { ComponentProps } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import logoImage from "@/assets/img/logo/logo.png"
import { NavMain } from "@/components/nav-main"
import { NavManagement } from "@/components/nav-management"
import { NavSecondary } from "@/components/nav-secondary"
import { NavUser } from "@/components/nav-user"
import {
  getManagementNavigation,
  getPrimaryNavigation,
  getSecondaryNavigation,
} from "@/components/navigation-config"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar"
import { useSidebar } from "@/components/ui/sidebar-hooks"
import { useAddTorrent } from "@/contexts/add-torrent-hooks"
import { useAuth } from "@/contexts/auth-hooks"
import { cn } from "@/lib/utils"

export function AppSidebar({ className, ...props }: ComponentProps<typeof Sidebar>) {
  const { t } = useTranslation()
  const { user, logout } = useAuth()
  const { openAddModal } = useAddTorrent()
  const { isMobile, setOpenMobile } = useSidebar()
  const isAdmin = user?.role === "admin"
  const closeMobile = () => isMobile && setOpenMobile(false)

  return (
    <Sidebar
      data-testid="app-sidebar"
      collapsible="offcanvas"
      className={cn(
        "max-md:pt-[env(safe-area-inset-top)] max-md:pb-[env(safe-area-inset-bottom)] max-md:pl-[env(safe-area-inset-left)]",
        className,
      )}
      {...props}
    >
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton asChild className="data-[slot=sidebar-menu-button]:p-1.5!">
              <Link to="/" onClick={closeMobile}>
                <span className="flex size-6 shrink-0 items-center justify-center overflow-hidden rounded-md">
                  <img src={logoImage} alt="" className="size-full object-contain" />
                </span>
                <span className="text-base font-semibold">Gardarr</span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        <NavMain
          items={getPrimaryNavigation(t)}
          addLabel={t("torrents.addTorrent")}
          onAdd={openAddModal}
        />
        <NavManagement
          items={getManagementNavigation(t)}
          label={t("navigation.sections.management")}
        />
        <NavSecondary
          items={getSecondaryNavigation(t, isAdmin)}
          className="mt-auto"
        />
      </SidebarContent>

      <SidebarFooter>
        <NavUser
          email={user?.email ?? "Gardarr"}
          isAdmin={isAdmin}
          labels={{
            profile: t("navigation.profile"),
            about: t("navigation.about"),
            settings: t("navigation.settings"),
            logout: t("auth.logout"),
          }}
          onLogout={logout}
        />
      </SidebarFooter>
      <SidebarRail
        aria-label={t("navigation.resizeSidebar")}
        title={t("navigation.resizeSidebar")}
      />
    </Sidebar>
  )
}
