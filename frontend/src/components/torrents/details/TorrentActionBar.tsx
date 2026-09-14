import { Play, Pause, Zap, Radio, CheckCircle, Share2, Trash2, ChevronsUp, ChevronUp, ChevronDown, ChevronsDown, type LucideIcon } from "lucide-react";
import type { QueuePriorityAction } from "@/services/torrents";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { ButtonGroup } from "@/components/ui/button-group";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useActionPulse } from "@/hooks/useActionPulse";

// Queue-priority direction -> transient pulse styling shown on the clicked
// icon (see useActionPulse). Reuses Tailwind's built-in animate-bounce, no
// custom keyframes needed.
const QUEUE_PULSE_CLASSNAME: Record<QueuePriorityAction, string> = {
  top: "animate-bounce text-emerald-600 dark:text-emerald-400",
  up: "animate-bounce text-emerald-600 dark:text-emerald-400",
  down: "animate-bounce text-amber-600 dark:text-amber-400",
  bottom: "animate-bounce text-amber-600 dark:text-amber-400",
};

interface TorrentActionBarProps {
  torrentId: string;
  onPlay?: (torrentId: string) => void;
  onPause?: (torrentId: string) => void;
  onForceDownload?: (torrentId: string) => void;
  onForceReannounce?: (torrentId: string) => void;
  onForceRecheck?: (torrentId: string) => void;
  onQueuePriority?: (torrentId: string, action: QueuePriorityAction) => void;
  onShare?: () => void;
  onDelete?: () => void;
}

interface ActionDefinition {
  icon: LucideIcon;
  label: string;
  onClick: () => void;
  className?: string;
  pulseKey?: QueuePriorityAction;
}

export function TorrentActionBar({
  torrentId,
  onPlay,
  onPause,
  onForceDownload,
  onForceReannounce,
  onForceRecheck,
  onQueuePriority,
  onShare,
  onDelete,
}: TorrentActionBarProps) {
  const { t } = useTranslation();
  const { pulsedAction, pulseToken, trigger: triggerPulse } = useActionPulse<QueuePriorityAction>();

  const actions: ActionDefinition[] = [];

  if (onPlay) {
    actions.push({
      icon: Play,
      label: t("torrents.actionButtons.play"),
      onClick: () => onPlay(torrentId),
    });
  }
  if (onPause) {
    actions.push({
      icon: Pause,
      label: t("torrents.actionButtons.pause"),
      onClick: () => onPause(torrentId),
    });
  }
  if (onForceDownload) {
    actions.push({
      icon: Zap,
      label: t("torrentDetails.actions.forceDownload", { defaultValue: "Force Download" }),
      onClick: () => onForceDownload(torrentId),
      className: "text-orange-600 hover:text-orange-700 hover:bg-orange-50 dark:hover:bg-orange-950 dark:text-orange-400 dark:hover:text-orange-300",
    });
  }
  if (onForceReannounce) {
    actions.push({
      icon: Radio,
      label: t("torrentDetails.actions.forceReannounce", { defaultValue: "Force Reannounce" }),
      onClick: () => onForceReannounce(torrentId),
      className: "text-blue-600 hover:text-blue-700 hover:bg-blue-50 dark:hover:bg-blue-950 dark:text-blue-400 dark:hover:text-blue-300",
    });
  }
  if (onForceRecheck) {
    actions.push({
      icon: CheckCircle,
      label: t("torrentDetails.actions.forceRecheck", { defaultValue: "Force Recheck" }),
      onClick: () => onForceRecheck(torrentId),
      className: "text-purple-600 hover:text-purple-700 hover:bg-purple-50 dark:hover:bg-purple-950 dark:text-purple-400 dark:hover:text-purple-300",
    });
  }
  if (onQueuePriority) {
    actions.push({
      icon: ChevronsUp,
      label: t("torrentDetails.actions.queueTop", { defaultValue: "Move to top of queue" }),
      onClick: () => {
        triggerPulse("top");
        onQueuePriority(torrentId, "top");
      },
      pulseKey: "top",
    });
    actions.push({
      icon: ChevronUp,
      label: t("torrentDetails.actions.queueUp", { defaultValue: "Move up in queue" }),
      onClick: () => {
        triggerPulse("up");
        onQueuePriority(torrentId, "up");
      },
      pulseKey: "up",
    });
    actions.push({
      icon: ChevronDown,
      label: t("torrentDetails.actions.queueDown", { defaultValue: "Move down in queue" }),
      onClick: () => {
        triggerPulse("down");
        onQueuePriority(torrentId, "down");
      },
      pulseKey: "down",
    });
    actions.push({
      icon: ChevronsDown,
      label: t("torrentDetails.actions.queueBottom", { defaultValue: "Move to bottom of queue" }),
      onClick: () => {
        triggerPulse("bottom");
        onQueuePriority(torrentId, "bottom");
      },
      pulseKey: "bottom",
    });
  }
  if (onShare) {
    actions.push({
      icon: Share2,
      label: t("torrentDetails.actions.share", { defaultValue: "Share card" }),
      onClick: onShare,
    });
  }
  if (onDelete) {
    actions.push({
      icon: Trash2,
      label: t("torrents.actionButtons.delete"),
      onClick: onDelete,
      className: "text-red-600 hover:text-red-700 hover:bg-red-50 dark:hover:bg-red-950 dark:text-red-400 dark:hover:text-red-300",
    });
  }

  if (actions.length === 0) return null;

  return (
    <div className="flex overflow-x-auto scrollbar-hide sm:overflow-visible" style={{ WebkitOverflowScrolling: "touch" }}>
      <ButtonGroup className="flex-shrink-0 sm:!w-full">
        {actions.map(({ icon: Icon, label, onClick, className, pulseKey }) => {
          const isPulsing = pulseKey !== undefined && pulsedAction === pulseKey;
          return (
            <Tooltip key={label}>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="icon"
                  onClick={onClick}
                  className={`h-10 w-10 flex-shrink-0 sm:!w-auto sm:!flex-1 sm:!shrink ${className ?? ""}`}
                  aria-label={label}
                >
                  <Icon
                    key={isPulsing ? pulseToken : "idle"}
                    className={`h-5 w-5 ${isPulsing ? QUEUE_PULSE_CLASSNAME[pulseKey] : ""}`}
                  />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{label}</TooltipContent>
            </Tooltip>
          );
        })}
      </ButtonGroup>
    </div>
  );
}
