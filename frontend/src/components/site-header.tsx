import { Moon, Sun } from "lucide-react"
import { useTranslation } from "react-i18next"

import DisplaySettingsButton from "@/components/DisplaySettingsButton"
import VariantColorSelectButton from "@/components/VariantColorSelectButton"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { SidebarTrigger } from "@/components/ui/sidebar"

export function SiteHeader({
  title,
  isDark,
  onToggleTheme,
}: {
  title: string
  isDark: boolean
  onToggleTheme: () => void
}) {
  const { t } = useTranslation()

  return (
    <header className="flex h-[calc(var(--header-height)+env(safe-area-inset-top))] shrink-0 items-center gap-2 border-b bg-background pt-[env(safe-area-inset-top)] transition-[width,height] ease-linear">
      <div className="flex w-full min-w-0 items-center gap-1 px-4 lg:gap-2 lg:px-6">
        <SidebarTrigger
          aria-label={t("navigation.toggleSidebar")}
          className="-ml-1 size-8"
        />
        <Separator orientation="vertical" className="mx-2 data-[orientation=vertical]:h-4" />
        <h1 className="truncate text-base font-medium">{title}</h1>
        <div className="ml-auto flex shrink-0 items-center gap-1 md:gap-2">
          <Button
            variant="ghost"
            size="icon"
            aria-label={t("theme.toggle")}
            onClick={onToggleTheme}
            className="size-8"
          >
            {isDark ? <Sun className="size-4" /> : <Moon className="size-4" />}
          </Button>
          <VariantColorSelectButton />
          <DisplaySettingsButton />
        </div>
      </div>
    </header>
  )
}
