import type { TFunction } from "i18next"
import type { LucideIcon } from "lucide-react"
import {
  ArrowDownUp,
  BarChart3,
  FolderOpen,
  History,
  Plug,
  Rss,
  Server,
  Tag,
  Users,
} from "lucide-react"

export type AppNavigationItem = {
  href: string
  icon: LucideIcon
  label: string
  activePrefixes?: string[]
}

type Translate = TFunction

export function getPrimaryNavigation(t: Translate): AppNavigationItem[] {
  return [
    { href: "/torrents", icon: ArrowDownUp, label: t("navigation.torrents") },
    { href: "/workers", icon: Server, label: t("navigation.workers") },
    { href: "/history", icon: History, label: t("navigation.history") },
    { href: "/reports", icon: BarChart3, label: t("navigation.reports", { defaultValue: "Reports" }) },
  ]
}

export function getManagementNavigation(t: Translate): AppNavigationItem[] {
  return [
    { href: "/categories", icon: FolderOpen, label: t("navigation.categories") },
    { href: "/tags", icon: Tag, label: t("navigation.tags") },
    { href: "/rss", icon: Rss, label: t("navigation.rss") },
  ]
}

export function getSecondaryNavigation(t: Translate, isAdmin: boolean): AppNavigationItem[] {
  const items: AppNavigationItem[] = [
    {
      href: "/integrations",
      icon: Plug,
      label: t("navigation.integrations"),
      activePrefixes: ["/integrations/"],
    },
  ]

  if (isAdmin) {
    items.push({ href: "/users", icon: Users, label: t("navigation.users") })
  }

  return items
}

export function isNavigationItemActive(item: AppNavigationItem, pathname: string): boolean {
  if (item.href === "/") {
    return pathname === "/" || item.activePrefixes?.some((prefix) => pathname.startsWith(prefix)) === true
  }

  return (
    pathname === item.href ||
    pathname.startsWith(`${item.href}/`) ||
    item.activePrefixes?.some((prefix) => pathname.startsWith(prefix)) === true
  )
}

export function getPageTitle(t: Translate, pathname: string): string {
  const allItems = [
    ...getPrimaryNavigation(t),
    ...getManagementNavigation(t),
    ...getSecondaryNavigation(t, true),
  ]
  const activeItem = allItems.find((item) => isNavigationItemActive(item, pathname))

  if (activeItem) return activeItem.label
  if (pathname.startsWith("/profile")) return t("navigation.profile")
  if (pathname.startsWith("/settings")) return t("navigation.settings")
  if (pathname.startsWith("/about")) return t("navigation.about")

  return t("navigation.dashboard")
}
