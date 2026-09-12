import { useEffect, useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Webhook, Bell, Plug, Monitor, BookOpen, Joystick, Activity, Gamepad2, Clapperboard, MessageCircle } from 'lucide-react';
import { api } from '@/lib/api';
import { EventList } from '@/components/EventList';
import { useEventHistory } from '@/hooks/useEventHistory';
import { toast } from 'sonner';

interface ProviderIntegrationResponse {
  provider: string;
  enabled: boolean;
  api_key_configured: boolean;
  updated_at?: string;
}

type ProviderConfigStatus = {
  isLoading: boolean;
  configured: boolean;
  enabled: boolean;
};

function getProviderStatusText(
  t: (key: string) => string,
  integrationId: string,
  providerConfig: ProviderConfigStatus
): string {
  if (providerConfig.isLoading) {
    return t('common.loading');
  }
  if (!providerConfig.configured) {
    return t(`integrations.${integrationId}.status.notConfigured`);
  }
  return providerConfig.enabled
    ? t(`integrations.${integrationId}.status.enabled`)
    : t(`integrations.${integrationId}.status.disabled`);
}

export default function IntegrationsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [showEvents, setShowEvents] = useState(false);
  const eventHistory = useEventHistory('all', 10, showEvents);
  const [showTGDBConfig, setShowTGDBConfig] = useState(false);
  const [isLoadingTGDB, setIsLoadingTGDB] = useState(false);
  const [isSavingTGDB, setIsSavingTGDB] = useState(false);
  const [tgdbEnabled, setTGDBEnabled] = useState(false);
  const [tgdbConfigured, setTGDBConfigured] = useState(false);
  const [tgdbApiKey, setTGDBApiKey] = useState("");
  const [showTMDBConfig, setShowTMDBConfig] = useState(false);
  const [isLoadingTMDB, setIsLoadingTMDB] = useState(false);
  const [isSavingTMDB, setIsSavingTMDB] = useState(false);
  const [tmdbEnabled, setTMDBEnabled] = useState(false);
  const [tmdbConfigured, setTMDBConfigured] = useState(false);
  const [tmdbApiKey, setTMDBApiKey] = useState("");

  const loadTGDBConfig = useCallback(async () => {
    setIsLoadingTGDB(true);
    try {
      const response = await api.get<ProviderIntegrationResponse>("/integrations/tgdb");
      if (response.data) {
        setTGDBEnabled(response.data.enabled);
        setTGDBConfigured(response.data.api_key_configured);
      }
    } catch {
      toast.error(t("integrations.tgdb.errors.loadFailed"));
    } finally {
      setIsLoadingTGDB(false);
    }
  }, [t]);

  useEffect(() => {
    loadTGDBConfig();
  }, [loadTGDBConfig]);

  const handleSaveTGDB = async () => {
    setIsSavingTGDB(true);
    try {
      const response = await api.put<ProviderIntegrationResponse>("/integrations/tgdb", {
        enabled: tgdbEnabled,
        api_key: tgdbApiKey,
      });

      if (response.data) {
        setTGDBConfigured(response.data.api_key_configured);
      }

      setTGDBApiKey("");
      toast.success(t("integrations.tgdb.success.saved"));
      setShowTGDBConfig(false);
    } catch {
      toast.error(t("integrations.tgdb.errors.saveFailed"));
    } finally {
      setIsSavingTGDB(false);
    }
  };

  const loadTMDBConfig = useCallback(async () => {
    setIsLoadingTMDB(true);
    try {
      const response = await api.get<ProviderIntegrationResponse>("/integrations/tmdb");
      if (response.data) {
        setTMDBEnabled(response.data.enabled);
        setTMDBConfigured(response.data.api_key_configured);
      }
    } catch {
      toast.error(t("integrations.tmdb.errors.loadFailed"));
    } finally {
      setIsLoadingTMDB(false);
    }
  }, [t]);

  useEffect(() => {
    loadTMDBConfig();
  }, [loadTMDBConfig]);

  const handleSaveTMDB = async () => {
    setIsSavingTMDB(true);
    try {
      const response = await api.put<ProviderIntegrationResponse>("/integrations/tmdb", {
        enabled: tmdbEnabled,
        api_key: tmdbApiKey,
      });

      if (response.data) {
        setTMDBConfigured(response.data.api_key_configured);
      }

      setTMDBApiKey("");
      toast.success(t("integrations.tmdb.success.saved"));
      setShowTMDBConfig(false);
    } catch {
      toast.error(t("integrations.tmdb.errors.saveFailed"));
    } finally {
      setIsSavingTMDB(false);
    }
  };

  const integrations = [
    {
      id: 'webhook',
      name: t('integrations.webhook.name'),
      description: t('integrations.webhook.description'),
      icon: Webhook,
      category: 'notifications',
      status: 'available'
    },
    {
      id: 'ntfy',
      name: t('integrations.ntfy.name'),
      description: t('integrations.ntfy.description'),
      icon: Bell,
      category: 'notifications',
      status: 'available'
    },
    {
      id: 'discord',
      name: 'Discord',
      description: t('integrations.discord.description', 'Send Gardarr events and transfer reports to Discord channels'),
      icon: MessageCircle,
      category: 'notifications',
      status: 'available'
    },
    {
      id: 'jellyfin',
      name: t('integrations.jellyfin.name'),
      description: t('integrations.jellyfin.description'),
      icon: Monitor,
      category: 'synchronizations',
      status: 'available'
    },
    {
      id: 'kavita',
      name: t('integrations.kavita.name'),
      description: t('integrations.kavita.description'),
      icon: BookOpen,
      category: 'synchronizations',
      status: 'available'
    },
    {
      id: 'romm',
      name: t('integrations.romm.name'),
      description: t('integrations.romm.description'),
      icon: Joystick,
      category: 'synchronizations',
      status: 'available'
    },
    {
      id: 'tgdb',
      name: t('integrations.tgdb.name'),
      subtitle: t('integrations.tgdb.subtitle'),
      description: t('integrations.tgdb.description'),
      icon: Gamepad2,
      category: 'others',
      status: 'available'
    },
    {
      id: 'tmdb',
      name: t('integrations.tmdb.name'),
      subtitle: t('integrations.tmdb.subtitle'),
      description: t('integrations.tmdb.description'),
      icon: Clapperboard,
      category: 'others',
      status: 'available'
    }
  ];

  const providerConfigById: Record<string, {
    isLoading: boolean;
    configured: boolean;
    enabled: boolean;
    onConfigure: () => void;
  }> = {
    tgdb: {
      isLoading: isLoadingTGDB,
      configured: tgdbConfigured,
      enabled: tgdbEnabled,
      onConfigure: () => setShowTGDBConfig(true),
    },
    tmdb: {
      isLoading: isLoadingTMDB,
      configured: tmdbConfigured,
      enabled: tmdbEnabled,
      onConfigure: () => setShowTMDBConfig(true),
    },
  };

  const categories = {
    notifications: {
      title: t('integrations.categories.notifications'),
      description: t('integrations.categories.notificationsDescription')
    },
    synchronizations: {
      title: t('integrations.categories.synchronizations'),
      description: t('integrations.categories.synchronizationsDescription')
    },
    others: {
      title: t('integrations.categories.others'),
      description: t('integrations.categories.othersDescription')
    }
  };

  const getIntegrationsByCategory = (category: string) => {
    return integrations.filter(integration => integration.category === category);
  };

  return (
    <div className="space-y-8">
      <div>
        <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-primary/10">
            <Plug className="h-6 w-6 text-primary" />
          </div>
          <h1 className="text-2xl font-bold tracking-tight">{t('integrations.title')}</h1>
          </div>
          <Button 
            onClick={() => setShowEvents(true)}
          >
            <Activity />
            {t('history.showEvents')}
          </Button>
        </div>
        <p className="mt-2 text-sm text-muted-foreground">
          {t('integrations.subtitle')}
        </p>
      </div>

      {/* Notifications Category */}
      <div className="space-y-4">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">{categories.notifications.title}</h2>
          <p className="text-muted-foreground">
            {categories.notifications.description}
          </p>
        </div>
        
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {getIntegrationsByCategory('notifications').map((integration) => {
            const IconComponent = integration.icon;
            return (
              <Card key={integration.id} className="hover:shadow-md transition-shadow">
                <CardHeader className="pb-3">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-lg bg-primary/10">
                      <IconComponent className="h-6 w-6 text-primary" />
                    </div>
                    <div>
                      <CardTitle className="text-lg">{integration.name}</CardTitle>
                      {"subtitle" in integration && integration.subtitle ? (
                        <p className="text-sm text-muted-foreground">{integration.subtitle}</p>
                      ) : null}
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <CardDescription className="mb-4">
                    {integration.description}
                  </CardDescription>
                  <Button 
                    variant="outline" 
                    className="w-full"
                    disabled={integration.id !== 'webhook' && integration.id !== 'discord'}
                    onClick={() => {
                     if (integration.id === 'webhook') {
                      navigate('/integrations/webhooks');
                     }
                     if (integration.id === 'discord') {
                      navigate('/integrations/discord');
                     }
                  }}
                  >
                    {integration.id === 'webhook' || integration.id === 'discord'
                      ? t('integrations.configure') 
                      : t('integrations.comingSoon')
                    }
                  </Button>
                </CardContent>
              </Card>
            );
          })}
        </div>
      </div>

      <div className="space-y-4">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">{categories.others.title}</h2>
          <p className="text-muted-foreground">
            {categories.others.description}
          </p>
        </div>

        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {getIntegrationsByCategory('others').map((integration) => {
            const IconComponent = integration.icon;
            const providerConfig = providerConfigById[integration.id];
            return (
              <Card key={integration.id} className="hover:shadow-md transition-shadow">
                <CardHeader className="pb-3">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-lg bg-primary/10">
                      <IconComponent className="h-6 w-6 text-primary" />
                    </div>
                    <div>
                      <CardTitle className="text-lg">{integration.name}</CardTitle>
                      {"subtitle" in integration && integration.subtitle ? (
                        <p className="text-sm text-muted-foreground">{integration.subtitle}</p>
                      ) : null}
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <CardDescription className="mb-2">
                    {integration.description}
                  </CardDescription>
                  <p className="mb-4 text-sm text-muted-foreground">
                    {getProviderStatusText(t, integration.id, providerConfig)}
                  </p>
                  <Button
                    variant="outline"
                    className="w-full"
                    onClick={providerConfig.onConfigure}
                  >
                    {t('integrations.configure')}
                  </Button>
                </CardContent>
              </Card>
            );
          })}
        </div>
      </div>

      {/* Synchronizations Category */}
      <div className="space-y-4">
        <div>
          <h2 className="text-2xl font-semibold tracking-tight">{categories.synchronizations.title}</h2>
          <p className="text-muted-foreground">
            {categories.synchronizations.description}
          </p>
        </div>
        
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {getIntegrationsByCategory('synchronizations').map((integration) => {
            const IconComponent = integration.icon;
            return (
              <Card key={integration.id} className="hover:shadow-md transition-shadow">
                <CardHeader className="pb-3">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-lg bg-primary/10">
                      <IconComponent className="h-6 w-6 text-primary" />
                    </div>
                    <div>
                      <CardTitle className="text-lg">{integration.name}</CardTitle>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <CardDescription className="mb-4">
                    {integration.description}
                  </CardDescription>
                  <Button 
                    variant="outline" 
                    className="w-full"
                    disabled={integration.id !== 'webhook'}
                    onClick={() => integration.id === 'webhook' && navigate('/integrations/webhooks')}
                  >
                    {integration.id === 'webhook' 
                      ? t('integrations.configure') 
                      : t('integrations.comingSoon')
                    }
                  </Button>
                </CardContent>
              </Card>
            );
          })}
        </div>
      </div>

      {/* Events Drawer */}
      <Sheet open={showEvents} onOpenChange={setShowEvents}>
        <SheetContent side="right" className="w-full sm:max-w-2xl overflow-y-auto">
          <SheetHeader className="space-y-1">
            <SheetTitle className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-primary/10">
                <Activity className="h-6 w-6 text-primary" />
              </div>
              {t('history.title')}
            </SheetTitle>
            <SheetDescription>
              {t('history.subtitle')}
            </SheetDescription>
          </SheetHeader>
          <EventList group="all" {...eventHistory} />
        </SheetContent>
      </Sheet>

      <Sheet open={showTGDBConfig} onOpenChange={setShowTGDBConfig}>
        <SheetContent side="right" className="w-full sm:max-w-xl overflow-y-auto">
          <SheetHeader className="space-y-1">
            <SheetTitle>{t("integrations.tgdb.sheetTitle")}</SheetTitle>
            <p className="text-sm text-muted-foreground">{t("integrations.tgdb.subtitle")}</p>
            <SheetDescription>{t("integrations.tgdb.sheetDescription")}</SheetDescription>
          </SheetHeader>

          <div className="space-y-6 py-6">
            <div className="space-y-2">
              <Label htmlFor="tgdb-enabled">{t("integrations.tgdb.enabledLabel")}</Label>
              <Button
                id="tgdb-enabled"
                type="button"
                variant={tgdbEnabled ? "default" : "outline"}
                className="w-full"
                onClick={() => setTGDBEnabled((value) => !value)}
              >
                {tgdbEnabled ? t("integrations.tgdb.enabled") : t("integrations.tgdb.disabled")}
              </Button>
            </div>

            <div className="space-y-2">
              <Label htmlFor="tgdb-api-key">{t("integrations.tgdb.apiKeyLabel")}</Label>
              <Input
                id="tgdb-api-key"
                type="password"
                value={tgdbApiKey}
                onChange={(event) => setTGDBApiKey(event.target.value)}
                placeholder={t("integrations.tgdb.apiKeyPlaceholder")}
              />
              <p className="text-xs text-muted-foreground">
                {tgdbConfigured
                  ? t("integrations.tgdb.apiKeyConfigured")
                  : t("integrations.tgdb.apiKeyNotConfigured")}
              </p>
            </div>

            <Button className="w-full" onClick={handleSaveTGDB} disabled={isSavingTGDB}>
              {isSavingTGDB ? t("common.loading") : t("common.save")}
            </Button>
          </div>
        </SheetContent>
      </Sheet>

      <Sheet open={showTMDBConfig} onOpenChange={setShowTMDBConfig}>
        <SheetContent side="right" className="w-full sm:max-w-xl overflow-y-auto">
          <SheetHeader className="space-y-1">
            <SheetTitle>{t("integrations.tmdb.sheetTitle")}</SheetTitle>
            <p className="text-sm text-muted-foreground">{t("integrations.tmdb.subtitle")}</p>
            <SheetDescription>{t("integrations.tmdb.sheetDescription")}</SheetDescription>
          </SheetHeader>

          <div className="space-y-6 py-6">
            <div className="space-y-2">
              <Label htmlFor="tmdb-enabled">{t("integrations.tmdb.enabledLabel")}</Label>
              <Button
                id="tmdb-enabled"
                type="button"
                variant={tmdbEnabled ? "default" : "outline"}
                className="w-full"
                onClick={() => setTMDBEnabled((value) => !value)}
              >
                {tmdbEnabled ? t("integrations.tmdb.enabled") : t("integrations.tmdb.disabled")}
              </Button>
            </div>

            <div className="space-y-2">
              <Label htmlFor="tmdb-api-key">{t("integrations.tmdb.apiKeyLabel")}</Label>
              <Input
                id="tmdb-api-key"
                type="password"
                value={tmdbApiKey}
                onChange={(event) => setTMDBApiKey(event.target.value)}
                placeholder={t("integrations.tmdb.apiKeyPlaceholder")}
              />
              <p className="text-xs text-muted-foreground">
                {tmdbConfigured
                  ? t("integrations.tmdb.apiKeyConfigured")
                  : t("integrations.tmdb.apiKeyNotConfigured")}
              </p>
            </div>

            <Button className="w-full" onClick={handleSaveTMDB} disabled={isSavingTMDB}>
              {isSavingTMDB ? t("common.loading") : t("common.save")}
            </Button>
          </div>
        </SheetContent>
      </Sheet>
    </div>
  );
}
