import { FileText, Image } from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { ProgressBar } from "@/components/ui/ProgressBar";
import { WorkerIcon } from "@/components/ui/WorkerIcon";
import { getStatusIcon, getStatusColor, getStatusBackgroundColor, type TorrentStatus } from "../TorrentStatusIcon";
import { getStatusTranslationKey } from "@/utils/statusUtils";
import { useTorrentBlurIntensity, getBlurPixels } from "../hooks";
import { formatBytes } from "@/utils/bytes";
import { AutoSaveField } from "./AutoSaveField";
import { TorrentProgressIndicator } from "../shared";
import { QueueRankBadge } from "@/components/QueueRankBadge";
import type { Task, TaskMetadata } from "@/types/torrent";

interface TorrentHeroHeaderProps {
  torrent: Task;
  onUpdate?: (metadata?: TaskMetadata) => Promise<void> | void;
}

export function TorrentHeroHeader({ torrent, onUpdate }: TorrentHeroHeaderProps) {
  const { t } = useTranslation();
  const blurIntensity = useTorrentBlurIntensity();
  const blurPixels = getBlurPixels(blurIntensity);
  const hasImage = Boolean(torrent.metadata?.image_url);
  const StatusIcon = getStatusIcon(torrent.state as TorrentStatus);

  const handleSaveName = async (name: string) => {
    try {
      const response = await api.put<TaskMetadata>(`/tasks/metadata/${torrent.hash}/name`, { name });
      if (onUpdate) {
        await onUpdate(response.data);
      }
      toast.success(t("torrentDetails.toasts.nameUpdateSuccess", { defaultValue: "Nome atualizado com sucesso" }));
    } catch {
      toast.error(t("torrentDetails.toasts.nameUpdateError", { defaultValue: "Erro ao atualizar nome" }));
      return false;
    }
  };

  return (
    <div className="relative rounded-lg border overflow-hidden">
      {/* Cover background */}
      {hasImage ? (
        <>
          <div
            className="absolute inset-0 bg-cover pointer-events-none z-0"
            style={{
              backgroundImage: `url(${encodeURI(torrent.metadata!.image_url!)})`,
              filter: `blur(${blurPixels}px)`,
              backgroundPosition: `center ${torrent.metadata?.image_position_y ?? 50}%`,
              opacity: Math.max(0.15, Math.min(0.85, (torrent.metadata?.image_brightness ?? 65) / 100)),
            }}
            aria-hidden
          />
          <div className="absolute inset-0 bg-white/60 dark:bg-black/40 z-0 pointer-events-none" aria-hidden />
        </>
      ) : (
        <div
          className="absolute inset-0 pointer-events-none z-0 bg-neutral-200/50 dark:bg-neutral-900/60"
          aria-hidden
        />
      )}

      <div className="relative z-10 p-3 sm:p-4 flex gap-3 sm:gap-4">
        {/* Thumbnail */}
        <div className="flex-shrink-0 hidden sm:block">
          {torrent.metadata?.thumbnail_url ? (
            <img
              src={torrent.metadata.thumbnail_url}
              alt={torrent.name}
              className="w-24 h-24 rounded-lg border object-cover"
            />
          ) : (
            <div className="w-24 h-24 rounded-lg border border-neutral-300 bg-neutral-200 flex items-center justify-center dark:border-neutral-700 dark:bg-neutral-800">
              <Image className="h-10 w-10 text-neutral-500 dark:text-neutral-400" />
            </div>
          )}
        </div>

        <div className="flex-1 min-w-0 space-y-2 sm:space-y-3">
          {/* Editable display name — click to edit, autosaves on blur */}
          <AutoSaveField
            icon={FileText}
            value={torrent.metadata?.name || torrent.name}
            ariaLabel={t("torrentDetails.name.edit", { defaultValue: "Editar nome" })}
            copyText={torrent.metadata?.name || torrent.name}
            inputClassName="font-semibold"
            onSave={handleSaveName}
            display={
              <div>
                <span
                  className="block truncate text-xs font-semibold leading-relaxed sm:overflow-visible sm:text-clip sm:whitespace-normal sm:text-sm sm:break-words"
                  title={torrent.metadata?.name || torrent.name}
                >
                  {torrent.metadata?.name || torrent.name}
                  {torrent.metadata?.release_date && (
                    <span className="text-muted-foreground font-normal ml-2">({torrent.metadata.release_date})</span>
                  )}
                </span>
                {torrent.metadata?.name && (
                  <div className="text-[10px] text-muted-foreground mt-1 truncate" title={torrent.name}>
                    {t("torrentDetails.name.file", { defaultValue: "Arquivo" })}: {torrent.name}
                  </div>
                )}
              </div>
            }
          />

          {/* Status + worker */}
          <div className="flex flex-wrap items-center gap-2">
            <div className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs sm:text-sm font-medium border ${getStatusBackgroundColor(torrent.state as TorrentStatus)}`}>
              <div className={`flex items-center gap-1.5 ${getStatusColor(torrent.state as TorrentStatus)}`}>
                <StatusIcon className="h-3.5 w-3.5" />
                <span>{t(getStatusTranslationKey(torrent.state as TorrentStatus))}</span>
              </div>
            </div>
            <QueueRankBadge priority={torrent.priority} />
            {torrent.worker && (
              <div className="inline-flex items-center gap-1.5 px-2 py-1 rounded-md container-content-background/60 border">
                <WorkerIcon iconName={torrent.worker.icon} color={torrent.worker.color} size="sm" />
                <span className="text-xs sm:text-sm text-muted-foreground">{torrent.worker.name}</span>
              </div>
            )}
          </div>

          {/* Progress */}
          <div className="flex items-center gap-2">
            <ProgressBar progress={torrent.progress} height="sm" className="flex-1" />
            <TorrentProgressIndicator
              progress={torrent.progress}
              precision={1}
              className="text-xs sm:text-sm"
            />
            <span className="text-xs text-muted-foreground whitespace-nowrap hidden sm:inline">
              {formatBytes(torrent.network?.download?.amount || (torrent.progress / 100) * torrent.size)} / {formatBytes(torrent.size)}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}
