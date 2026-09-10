// AppLayout.tsx
import { Button } from "@/components/ui/button";
import { Settings, Users, ArrowDownUp, Menu, Sun, Moon, Info, LogOut, FolderOpen, UserCircle, Server, Plug, History, Tag, Rss, BarChart3 } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router";
import PageTransition from "@/components/PageTransition";
import { useTranslation } from "react-i18next";
import { useAuth } from "@/contexts/auth-hooks";
import { AddTorrentProvider } from "@/contexts/AddTorrentContext";
import { useAddTorrent } from "@/contexts/add-torrent-hooks";
import { TagColorsProvider } from "@/contexts/TagColorsContext";
import { AddTorrentModal } from "@/components/AddTorrentModal";
import { AddTorrentButton } from "@/components/AddTorrentButton";
import VariantColorSelectButton from "@/components/VariantColorSelectButton";
import DisplaySettingsButton from "@/components/DisplaySettingsButton";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import logoImage from "@/assets/img/logo/logo.png";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <TagColorsProvider>
      <AddTorrentProvider>
        <AppLayoutInner>{children}</AppLayoutInner>
        <AddTorrentModal />
      </AddTorrentProvider>
    </TagColorsProvider>
  );
}

function AppLayoutInner({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation();
  const { user, logout } = useAuth();
  const { openAddModal } = useAddTorrent();
  const navigate = useNavigate();
  const [sidebarOpen, setSidebarOpen] = useState(() => {
    if (window.innerWidth < 768) return false;
    return localStorage.getItem("sidebarOpen") === "true";
  });
  const [isDark, setIsDark] = useState<boolean>(false);
  const [isMobile, setIsMobile] = useState(() => window.innerWidth < 768);
  const location = useLocation();

  useEffect(() => {
    const root = document.documentElement;
    const stored = localStorage.getItem("theme");
    // Default to dark theme if no preference is stored
    const dark = stored ? stored === "dark" : true;
    if (dark) {
      root.classList.add('dark');
    } else {
      root.classList.remove('dark');
    }
    setIsDark(root.classList.contains('dark'));
  }, []);

  useEffect(() => {
    const checkIsMobile = () => {
      const nowMobile = window.innerWidth < 768;
      setIsMobile((wasMobile) => {
        if (wasMobile && !nowMobile) {
          // Resizing from mobile to desktop: sidebarOpen was forced closed
          // for mobile, so restore the stored desktop preference instead of
          // letting the persistence effect below overwrite it with false.
          setSidebarOpen(localStorage.getItem("sidebarOpen") === "true");
        }
        return nowMobile;
      });
    };

    checkIsMobile();
    window.addEventListener('resize', checkIsMobile);

    return () => window.removeEventListener('resize', checkIsMobile);
  }, []);

  // Open the Add Torrent modal when launched via the PWA "Add Torrent" shortcut (?add=1)
  useEffect(() => {
    const params = new URLSearchParams(location.search);
    if (params.get("add") === "1") {
      openAddModal();
      params.delete("add");
      navigate({ pathname: location.pathname, search: params.toString() }, { replace: true });
    }
  }, [location.search, location.pathname, openAddModal, navigate]);

  function toggleTheme() {
    const root = document.documentElement;
    const nextDark = !root.classList.contains('dark');
    if (nextDark) {
      root.classList.add('dark');
      localStorage.setItem("theme", "dark");
    } else {
      root.classList.remove('dark');
      localStorage.setItem("theme", "light");
    }
    setIsDark(nextDark);
  }

  const handleLogout = async () => {
    await logout();
    navigate("/login");
  };

  const menuSections = [
    {
      key: "library",
      label: t("navigation.sections.library"),
      items: [
        { href: "/torrents", icon: ArrowDownUp, label: t("navigation.torrents"), key: "torrents" },
        { href: "/history", icon: History, label: t("navigation.history"), key: "history" },
        { href: "/reports", icon: BarChart3, label: t("navigation.reports", "Reports"), key: "reports" },
      ],
    },
    {
      key: "management",
      label: t("navigation.sections.management"),
      items: [
        { href: "/categories", icon: FolderOpen, label: t("navigation.categories"), key: "categories" },
        { href: "/tags", icon: Tag, label: t("navigation.tags"), key: "tags" },
        { href: "/rss", icon: Rss, label: t("navigation.rss"), key: "rss" },
      ],
    },
    {
      key: "system",
      label: t("navigation.sections.system"),
      items: [
        { href: "/workers", icon: Server, label: t("navigation.workers"), key: "workers" },
        { href: "/integrations", icon: Plug, label: t("navigation.integrations"), key: "integrations" },
      ],
    },
    {
      key: "administration",
      label: t("navigation.sections.administration"),
      items: user?.role === "admin"
        ? [
            { href: "/users", icon: Users, label: t("navigation.users"), key: "users" },
            { href: "/settings", icon: Settings, label: t("navigation.settings"), key: "settings" },
          ]
        : [],
    },
  ].filter((section) => section.items.length > 0);

  return (
    <div className="flex h-dvh bg-background text-foreground overflow-hidden max-h-dvh safe-left safe-right">
      {/* Mobile Sidebar Overlay */}
      <div
        className={`fixed inset-0 bg-black/50 z-40 md:hidden transition-opacity duration-300 ${sidebarOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'
          }`}
        onClick={() => setSidebarOpen(false)}
      />

      {/* Sidebar Esquerda - Responsiva para mobile */}
      <div
        data-testid="app-sidebar"
        className={`bg-sidebar border-r border-border flex-col transition-all duration-300 ease-in-out overflow-hidden safe-top safe-bottom max-md:safe-left
          ${sidebarOpen
            ? 'flex fixed inset-y-0 left-0 z-50 w-52 translate-x-0 md:relative md:z-auto md:w-52'
            : 'fixed inset-y-0 left-0 z-50 w-52 -translate-x-full md:flex md:relative md:translate-x-0 md:w-14'
          }`}
      >
        {/* Header da Sidebar */}
        <div className="border-b border-border px-3 py-2.5">
          <div className="flex items-center gap-2">
            <div className="h-7 w-7 rounded-md overflow-hidden flex items-center justify-center flex-shrink-0">
              <img
                src={logoImage}
                alt="Gardarr Logo"
                className="h-full w-full object-contain"
              />
            </div>
            <span className={`font-semibold text-sm whitespace-nowrap transition-all duration-200 overflow-hidden ${sidebarOpen || isMobile ? 'opacity-100 max-w-[180px]' : 'opacity-0 max-w-0'
              }`}>
              Gardarr
            </span>
          </div>
        </div>

        {/* Menu de Navegação */}
        <nav className="p-1.5 flex-1 overflow-auto">
          {(() => {
            const isCollapsed = !sidebarOpen && !isMobile;

            const addButton = (
              <AddTorrentButton
                type="button"
                iconOnly={isCollapsed}
                aria-label={t("torrents.addTorrent")}
                onClick={() => {
                  openAddModal();
                  if (isMobile) {
                    setSidebarOpen(false);
                  }
                }}
                className="mb-1.5 h-8 w-full text-xs"
              >
                {t("torrents.addTorrent")}
              </AddTorrentButton>
            );

            return isCollapsed ? (
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    {addButton}
                  </TooltipTrigger>
                  <TooltipContent side="right">
                    <p>{t("torrents.addTorrent")}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            ) : (
              addButton
            );
          })()}
          <div className="space-y-4">
            {menuSections.map((section) => {
              const isCollapsed = !sidebarOpen && !isMobile;

              return (
                <section key={section.key} aria-label={section.label}>
                  <h2
                    className={`px-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-sidebar-foreground/45 transition-all duration-200 ${
                      isCollapsed ? "h-0 overflow-hidden p-0 opacity-0" : "opacity-100"
                    }`}
                  >
                    {section.label}
                  </h2>
                  <ul className="space-y-0.5">
                    {section.items.map((item) => {
                      const IconComponent = item.icon;
                      const isActive = location.pathname === item.href || location.pathname.startsWith(item.href + "/");

                      const menuItem = (
                        <Link
                          to={item.href}
                          onClick={() => isMobile && setSidebarOpen(false)}
                          className={`flex items-center px-2 py-1.5 text-xs rounded-md transition-all ${isActive
                            ? "bg-primary text-primary-foreground"
                            : "text-sidebar-foreground hover:bg-primary/90 hover:text-primary-foreground"
                            } ${isCollapsed ? 'justify-center' : 'gap-2'}`}
                        >
                          <IconComponent className="h-3.5 w-3.5 flex-shrink-0" />
                          <span className={`whitespace-nowrap transition-all duration-200 overflow-hidden ${sidebarOpen || isMobile ? 'opacity-100 max-w-[180px]' : 'opacity-0 max-w-0'
                            }`}>
                            {item.label}
                          </span>
                        </Link>
                      );

                      return (
                        <li key={item.key}>
                          {isCollapsed ? (
                            <TooltipProvider>
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  {menuItem}
                                </TooltipTrigger>
                                <TooltipContent side="right">
                                  <p>{item.label}</p>
                                </TooltipContent>
                              </Tooltip>
                            </TooltipProvider>
                          ) : (
                            menuItem
                          )}
                        </li>
                      );
                    })}
                  </ul>
                </section>
              );
            })}
          </div>
        </nav>

        {/* Footer da Sidebar */}
        <div className="border-t border-border p-1.5 mt-auto space-y-0.5">
          {(() => {
            const isCollapsed = !sidebarOpen && !isMobile;

            const aboutItem = (
              <Link
                to="/about"
                onClick={() => isMobile && setSidebarOpen(false)}
                className={`flex items-center px-2 py-1.5 text-xs rounded-md transition-all ${location.pathname === "/about"
                  ? "bg-primary text-primary-foreground"
                  : "text-sidebar-foreground hover:bg-primary/90 hover:text-primary-foreground"
                  } ${isCollapsed ? 'justify-center' : 'gap-2'}`}
              >
                <Info className="h-3.5 w-3.5 flex-shrink-0" />
                <span className={`whitespace-nowrap transition-all duration-200 overflow-hidden ${sidebarOpen || isMobile ? 'opacity-100 max-w-[180px]' : 'opacity-0 max-w-0'
                  }`}>
                  {t("navigation.about")}
                </span>
              </Link>
            );

            const logoutItem = (
              <button
                type="button"
                onClick={() => {
                  handleLogout();
                  if (isMobile) {
                    setSidebarOpen(false);
                  }
                }}
                className={`flex items-center px-2 py-1.5 text-xs rounded-md transition-all text-sidebar-foreground hover:bg-primary/90 hover:text-primary-foreground w-full ${isCollapsed ? 'justify-center' : 'gap-2'}`}
              >
                <LogOut className="h-3.5 w-3.5 flex-shrink-0" />
                <span className={`whitespace-nowrap transition-all duration-200 overflow-hidden ${sidebarOpen || isMobile ? 'opacity-100 max-w-[180px]' : 'opacity-0 max-w-0'
                  }`}>
                  {t("auth.logout")}
                </span>
              </button>
            );

            return (
              <>
                {isCollapsed ? (
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        {aboutItem}
                      </TooltipTrigger>
                      <TooltipContent side="right">
                        <p>{t("navigation.about")}</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                ) : (
                  aboutItem
                )}

                {isCollapsed ? (
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        {logoutItem}
                      </TooltipTrigger>
                      <TooltipContent side="right">
                        <p>{t("auth.logout")}</p>
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                ) : (
                  logoutItem
                )}
              </>
            );
          })()}
        </div>
      </div>

      {/* Conteúdo Principal - Ocupa todo o espaço restante */}
      <div className="flex-1 flex flex-col min-w-0 min-h-0 overflow-hidden max-h-screen">
        {/* Header */}
        <header className="min-h-12 border-b border-border bg-card flex items-center justify-between px-3 md:px-4 flex-shrink-0 safe-top">
          <div className="flex items-center">
            <Button
              variant="ghost"
              size="icon"
              aria-label={t("navigation.toggleSidebar")}
              onClick={() => {
                const next = !sidebarOpen;
                setSidebarOpen(next);
                if (!isMobile) {
                  localStorage.setItem("sidebarOpen", String(next));
                }
              }}
              className="h-8 w-8 flex items-center justify-center"
            >
              <Menu className="h-4 w-4" />
            </Button>
          </div>

          <div className="flex items-center gap-1 md:gap-2">
            <Button variant="ghost" size="icon" aria-label={t("theme.toggle")} onClick={toggleTheme} className="h-8 w-8 flex items-center justify-center">
              {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </Button>
            <VariantColorSelectButton />
            <DisplaySettingsButton />
            <Button variant="ghost" size="icon" aria-label={t("navigation.profile")} asChild className="h-8 w-8 flex items-center justify-center">
              <Link to="/profile">
                <UserCircle className="h-4 w-4" />
              </Link>
            </Button>
            <Button variant="ghost" size="icon" aria-label={t("auth.logout")} className="h-8 w-8" onClick={handleLogout}>
              <LogOut className="h-4 w-4" />
            </Button>
          </div>
        </header>

        {/* Conteúdo da Rota */}
        <main className="flex-1 min-h-0 overflow-y-auto overflow-x-hidden">
          <div className="w-full max-w-7xl mx-auto p-4 md:p-6 pb-[max(1rem,env(safe-area-inset-bottom))] md:pb-[max(1.5rem,env(safe-area-inset-bottom))]">
            <PageTransition>
              {children}
            </PageTransition>
          </div>
        </main>
      </div>
    </div>
  );
}
