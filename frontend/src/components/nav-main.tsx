import { Link, useLocation } from "react-router"

import { AddTorrentButton } from "@/components/AddTorrentButton"
import type { AppNavigationItem } from "@/components/navigation-config"
import { isNavigationItemActive } from "@/components/navigation-config"
import {
  SidebarGroup,
  SidebarGroupContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { useSidebar } from "@/components/ui/sidebar-hooks"

export function NavMain({
  items,
  addLabel,
  onAdd,
}: {
  items: AppNavigationItem[]
  addLabel: string
  onAdd: () => void
}) {
  const location = useLocation()
  const { isMobile, setOpenMobile } = useSidebar()

  const handleAdd = () => {
    onAdd()
    if (isMobile) setOpenMobile(false)
  }

  return (
    <SidebarGroup>
      <SidebarGroupContent className="flex flex-col gap-2">
        <SidebarMenu>
          <SidebarMenuItem>
            <AddTorrentButton
              type="button"
              aria-label={addLabel}
              onClick={handleAdd}
              className="h-8 w-full justify-start text-sm"
            >
              {addLabel}
            </AddTorrentButton>
          </SidebarMenuItem>
        </SidebarMenu>
        <SidebarMenu>
          {items.map((item) => {
            const isActive = isNavigationItemActive(item, location.pathname)
            const Icon = item.icon

            return (
              <SidebarMenuItem key={item.href}>
                <SidebarMenuButton asChild isActive={isActive} tooltip={item.label}>
                  <Link
                    to={item.href}
                    aria-current={isActive ? "page" : undefined}
                    onClick={() => isMobile && setOpenMobile(false)}
                  >
                    <Icon />
                    <span>{item.label}</span>
                  </Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
            )
          })}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}
