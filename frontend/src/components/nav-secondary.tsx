import type { ComponentPropsWithoutRef } from "react"
import { Link, useLocation } from "react-router"

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

export function NavSecondary({
  items,
  ...props
}: {
  items: AppNavigationItem[]
} & ComponentPropsWithoutRef<typeof SidebarGroup>) {
  const location = useLocation()
  const { isMobile, setOpenMobile } = useSidebar()

  return (
    <SidebarGroup {...props}>
      <SidebarGroupContent>
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
