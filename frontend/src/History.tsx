import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Activity, RefreshCw, Download, Server, Gauge, BarChart3 } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { EventList } from '@/components/EventList';
import { Button } from '@/components/ui/button';
import { workerService } from '@/services/workers';
import { useEventHistory } from '@/hooks/useEventHistory';

export default function HistoryPage() {
  const { t } = useTranslation();
  const [workerNames, setWorkerNames] = useState<Record<string, string>>({});

  const torrentEvents = useEventHistory('torrent');
  const workerEvents = useEventHistory('worker');
  const scheduleEvents = useEventHistory('schedule');
  const reportEvents = useEventHistory('report');

  useEffect(() => {
    workerService.listWorkersBasic().then((response) => {
      if (!response.data) return;
      setWorkerNames(
        Object.fromEntries(response.data.map((worker) => [worker.uuid, worker.name]))
      );
    }).catch((error) => {
      console.error("Failed to load workers:", error);
    });
  }, []);

  const isRefreshing = torrentEvents.isLoading || workerEvents.isLoading || scheduleEvents.isLoading || reportEvents.isLoading;
  const refreshAll = () => {
    torrentEvents.refresh();
    workerEvents.refresh();
    scheduleEvents.refresh();
    reportEvents.refresh();
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-4">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-primary/10">
            <Activity className="h-6 w-6 text-primary" />
          </div>
          <div>
            <h1 className="text-xl sm:text-2xl font-bold tracking-tight">
              {t('history.title')}
            </h1>
            <p className="text-muted-foreground text-sm mt-1">
              {t('history.subtitle')}
            </p>
          </div>
        </div>
        <Button onClick={refreshAll} variant="outline" disabled={isRefreshing}>
          <RefreshCw className={`h-4 w-4 mr-2 ${isRefreshing ? "animate-spin" : ""}`} />
          {t('history.refresh')}
        </Button>
      </div>

      <Card>
        <CardContent className="p-4 sm:p-6">
          <Tabs defaultValue="torrent">
            <TabsList className="grid w-full grid-cols-4 max-w-2xl">
              <TabsTrigger value="torrent" className="gap-2">
                <Download className="h-4 w-4" />
                {t('history.groups.torrent') || 'Torrent events'}
              </TabsTrigger>
              <TabsTrigger value="worker" className="gap-2">
                <Server className="h-4 w-4" />
                {t('history.groups.worker') || 'Worker events'}
              </TabsTrigger>
              <TabsTrigger value="schedule" className="gap-2">
                <Gauge className="h-4 w-4" />
                {t('history.groups.schedule') || 'Schedule events'}
              </TabsTrigger>
              <TabsTrigger value="report" className="gap-2">
                <BarChart3 className="h-4 w-4" />
                {t('history.groups.report', 'Reports')}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="torrent" className="mt-4">
              <EventList group="torrent" {...torrentEvents} />
            </TabsContent>

            <TabsContent value="worker" className="mt-4">
              <EventList group="worker" {...workerEvents} workerNames={workerNames} />
            </TabsContent>

            <TabsContent value="schedule" className="mt-4">
              <EventList group="schedule" {...scheduleEvents} />
            </TabsContent>
            <TabsContent value="report" className="mt-4">
              <EventList group="report" {...reportEvents} />
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}
