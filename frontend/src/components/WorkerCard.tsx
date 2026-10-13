import { Card, CardContent } from "@/components/ui/card";
import { Loader2, HardDrive, Wifi, Activity, AlertTriangle, Gauge } from "lucide-react";
import { useTranslation } from "react-i18next";
import { WorkerIcon } from "./ui/WorkerIcon";
import { QBittorrentIcon } from "./ui/QBittorrentIcon";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "./ui/tooltip";
import { WorkerErrorBadge } from "./WorkerErrorDisplay";
import { useWorkerLiveStatus } from "@/hooks/useWorkerLiveStatus";
import type { WorkerLiveStatus } from "@/hooks/useWorkerLiveStatus";
import type { Worker } from "@/types/worker";

function formatBytes(bytes: number) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

interface WorkerCardProps {
  worker: Worker;
  statusRefreshToken: number;
  onShowDetails: (worker: Worker) => void;
  liveStatus?: WorkerLiveStatus;
}

export function WorkerCard({ worker, statusRefreshToken, onShowDetails, liveStatus }: Readonly<WorkerCardProps>) {
  const { t } = useTranslation();
  const independentLive = useWorkerLiveStatus(worker.uuid, statusRefreshToken, liveStatus === undefined);
  const live = liveStatus ?? independentLive;

  // Merge DB-only fields (name/address/icon/color) with the lazily-loaded
  // live status, so downstream consumers (WorkerErrorBadge, details modal)
  // keep working with a normal Worker shape.
  const merged: Worker = {
    ...worker,
    status: live.status,
    error: live.error,
    error_code: live.error_code,
    permanent: live.permanent,
    instance: live.instance ?? worker.instance,
  };

  return (
    <Card
      className={`relative cursor-pointer hover:container-content-background/50 transition-colors ${merged.status === 'ERRORED' ? 'border border-red-500/30' : ''
        } ${merged.status === 'INITIALIZING' ? 'border border-yellow-500/30' : ''}`}
      onClick={() => onShowDetails(merged)}
    >
      <CardContent className="p-0">
        <div className="flex">
          <div className="flex-1 p-4">
            <div className="flex gap-3 items-center">
              <WorkerIcon
                iconName={worker.icon}
                color={worker.color}
                size="lg"
                className="w-16 h-16 rounded-lg"
              />

              <div className="flex-1 min-w-0 space-y-1.5">
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    {merged.status === 'ACTIVE' && (
                      <div className="w-2 h-2 rounded-full bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.6)] ml-0.5"></div>
                    )}
                    {merged.status === 'ERRORED' && (
                      <div className="w-2 h-2 rounded-full bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.6)] ml-0.5"></div>
                    )}
                    {merged.status === 'INITIALIZING' && (
                      <div className="w-2 h-2 rounded-full bg-yellow-500 shadow-[0_0_8px_rgba(234,179,8,0.6)] ml-0.5 animate-pulse"></div>
                    )}
                    {merged.status === 'PENDING' && (
                      <div className="w-2 h-2 rounded-full bg-muted-foreground/50 ml-0.5 animate-pulse"></div>
                    )}
                    <h3 className="font-semibold text-base truncate">{worker.name}</h3>
                    {merged.status === 'ERRORED' && (
                      <AlertTriangle className="h-4 w-4 text-red-500 flex-shrink-0" />
                    )}
                    {(merged.status === 'INITIALIZING' || live.checking) && (
                      <Loader2 className="h-4 w-4 text-muted-foreground flex-shrink-0 animate-spin" />
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground flex items-center gap-2">
                    <Wifi className="h-3 w-3" />
                    <span className="truncate">{worker.address}</span>
                  </p>
                </div>

                {!live.checking && <WorkerErrorBadge worker={merged} />}

                {worker.bandwidth_schedule_status?.active && (
                  <div className="flex items-center gap-1 text-xs text-primary" title={worker.bandwidth_schedule_status.name}>
                    <Gauge className="h-3 w-3" />
                    <span>{t('workers.schedules.activeBadge', 'Scheduled limit')}{worker.bandwidth_schedule_status.name ? `: ${worker.bandwidth_schedule_status.name}` : ''}</span>
                  </div>
                )}

                {merged.instance && merged.status === 'ACTIVE' && (
                  <div className="flex items-center gap-3 text-xs text-muted-foreground">
                    <div className="flex items-center gap-1">
                      <HardDrive className="h-3 w-3" />
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className="cursor-help">{formatBytes(merged.instance.server.free_space_on_disk)}</span>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p>{t('workers.freeSpaceOnDisk', 'Free Space on Disk')}</p>
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </div>
                    <div className="h-3 w-px bg-border"></div>
                    <div className="flex items-center gap-1">
                      <Activity className="h-3 w-3" />
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span className="cursor-help">{merged.instance.transfer.global_ratio.toFixed(2)}</span>
                          </TooltipTrigger>
                          <TooltipContent>
                            <p>{t('workers.globalRatio', 'Global Ratio')}</p>
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                    </div>
                  </div>
                )}
              </div>

              <div className="flex-shrink-0 flex items-center gap-2">
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <div className="cursor-help">
                        <QBittorrentIcon
                          size="lg"
                          className="w-8 h-8 text-muted-foreground/60"
                        />
                      </div>
                    </TooltipTrigger>
                    <TooltipContent>
                      <p>qBittorrent</p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </div>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

export default WorkerCard;
