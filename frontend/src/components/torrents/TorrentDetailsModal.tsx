import { useRef, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { CustomScrollArea } from "@/components/ui/custom-scroll-area";
import { FileText, Files, Radio, Image, Users } from "lucide-react";
import { DeleteTorrentModal } from "@/components/DeleteTorrentModal";
import { useTranslation } from "react-i18next";
import type { Task, TaskMetadata } from "@/types/torrent";
import type { Category } from "@/types/category";
import type { QueuePriorityAction } from "@/services/torrents";
import { TorrentFilesList } from "./TorrentFilesList";
import { TorrentImageEditor } from "./TorrentImageEditor";
import { TorrentHeroHeader } from "./details/TorrentHeroHeader";
import { TorrentActionBar } from "./details/TorrentActionBar";
import { OverviewTab } from "./details/OverviewTab";
import { TrackersTab } from "./details/TrackersTab";
import { PeersTab } from "./details/PeersTab";
import { TorrentPathField } from "./details/TorrentPathField";
import { ShareableTorrentCardDialog } from "./share/ShareableTorrentCardDialog";

interface TorrentDetailsModalProps {
  torrent: Task | null;
  isOpen: boolean;
  onClose: () => void;
  onPlay?: (torrentId: string) => void;
  onPause?: (torrentId: string) => void;
  onDelete?: (torrentId: string, purge: boolean) => void;
  onForceDownload?: (torrentId: string) => void;
  onForceReannounce?: (torrentId: string) => void;
  onForceRecheck?: (torrentId: string) => void;
  onQueuePriority?: (torrentId: string, action: QueuePriorityAction) => void;
  onSetLocation?: (torrentId: string, location: string) => void;
  onUpdate?: (metadata?: TaskMetadata) => Promise<void> | void;
  onCategoryTagsUpdate?: (torrentId: string, patch: { category?: string; tags?: string[] }) => void;
}

export function TorrentDetailsModal({
  torrent,
  isOpen,
  onClose,
  onPlay,
  onPause,
  onDelete,
  onForceDownload,
  onForceReannounce,
  onForceRecheck,
  onQueuePriority,
  onSetLocation,
  onUpdate,
  onCategoryTagsUpdate,
}: TorrentDetailsModalProps) {
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [isShareModalOpen, setIsShareModalOpen] = useState(false);
  const [activeTab, setActiveTab] = useState("overview");
  const [currentCategoryData, setCurrentCategoryData] = useState<Category | null>(null);
  const { t } = useTranslation();
  const contentRef = useRef<HTMLDivElement>(null);

  if (!torrent) return null;

  const handleDeleteConfirm = (purge: boolean) => {
    if (onDelete) {
      onDelete(torrent.id, purge);
    }
    setIsDeleteModalOpen(false);
  };

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent
        ref={contentRef}
        data-torrent-details-dialog
        onOpenAutoFocus={(event) => {
          event.preventDefault();
          contentRef.current?.focus();
        }}
        className="!top-0 h-[100dvh] !max-h-[100dvh] w-full !max-w-none !m-0 !translate-y-0 rounded-none border-0 px-4 pt-[calc(1rem+env(safe-area-inset-top))] pb-[calc(1rem+env(safe-area-inset-bottom))] flex flex-col overflow-hidden sm:!top-[calc(50%+(env(safe-area-inset-top)-env(safe-area-inset-bottom))/2)] sm:h-[90vh] sm:!max-h-[90vh] sm:!w-[80rem] sm:!max-w-[calc(100vw-2rem)] sm:!m-0 sm:!translate-y-[-50%] sm:rounded-lg sm:border sm:p-6"
      >
        <DialogHeader className="sr-only">
          <DialogTitle>{t("torrentDetails.title")}</DialogTitle>
          <DialogDescription>{t("torrentDetails.subtitle")}</DialogDescription>
        </DialogHeader>

        <div className="shrink-0 space-y-3">
          <TorrentHeroHeader torrent={torrent} onUpdate={onUpdate} />
          <TorrentActionBar
            torrentId={torrent.id}
            onPlay={onPlay}
            onPause={onPause}
            onForceDownload={onForceDownload}
            onForceReannounce={onForceReannounce}
            onForceRecheck={onForceRecheck}
            onQueuePriority={onQueuePriority}
            onShare={() => setIsShareModalOpen(true)}
            onDelete={onDelete ? () => setIsDeleteModalOpen(true) : undefined}
          />
        </div>

        <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full flex-1 flex flex-col min-h-0 overflow-hidden">
          <TabsList className="grid w-full grid-cols-5 shrink-0">
            <TabsTrigger value="overview" className="flex items-center gap-2">
              <FileText className="h-4 w-4 hidden sm:block" />
              {t("torrentDetails.tabs.overview", { defaultValue: "Visão Geral" })}
            </TabsTrigger>
            <TabsTrigger value="image" className="flex items-center gap-2">
              <Image className="h-4 w-4 hidden sm:block" />
              {t("torrentDetails.tabs.image", { defaultValue: "Imagem" })}
            </TabsTrigger>
            <TabsTrigger value="files" className="flex items-center gap-2">
              <Files className="h-4 w-4 hidden sm:block" />
              {t("torrentDetails.tabs.files", { defaultValue: "Arquivos" })}
            </TabsTrigger>
            <TabsTrigger value="trackers" className="flex items-center gap-2">
              <Radio className="h-4 w-4 hidden sm:block" />
              {t("torrentDetails.tabs.trackers", { defaultValue: "Trackers" })}
            </TabsTrigger>
            <TabsTrigger value="peers" className="flex items-center gap-2">
              <Users className="h-4 w-4 hidden sm:block" />
              {t("torrentDetails.tabs.peers", { defaultValue: "Peers" })}
            </TabsTrigger>
          </TabsList>

          {/* key resets scroll position when switching tabs */}
          <CustomScrollArea key={activeTab} className="w-full flex-1 min-h-0" variant="thin" mobileFallback>
            <TabsContent value="overview" className="mt-4">
              <OverviewTab
                torrent={torrent}
                onCategoryDataChange={setCurrentCategoryData}
                onCategoryTagsUpdate={onCategoryTagsUpdate}
                onShare={() => setIsShareModalOpen(true)}
              />
            </TabsContent>

            <TabsContent value="files" className="mt-4">
              <div className="space-y-4">
                <TorrentPathField torrentId={torrent.id} path={torrent.path} onSetLocation={onSetLocation} />
                <TorrentFilesList
                  workerId={torrent.worker?.uuid || ""}
                  taskId={torrent.id}
                  defaultOpen
                  title={t("torrentDetails.files.title", { defaultValue: "Lista de Arquivos" })}
                />
              </div>
            </TabsContent>

            <TabsContent value="trackers" className="mt-4">
              <TrackersTab torrent={torrent} />
            </TabsContent>

            <TabsContent value="peers" className="mt-4">
              <PeersTab torrent={torrent} />
            </TabsContent>

            <TabsContent value="image" className="mt-2">
              <TorrentImageEditor
                taskHash={torrent.hash}
                taskName={torrent.name}
                category={currentCategoryData}
                metadata={torrent.metadata}
                onUpdate={onUpdate}
              />
            </TabsContent>
          </CustomScrollArea>
        </Tabs>
      </DialogContent>

      <DeleteTorrentModal
        isOpen={isDeleteModalOpen}
        torrentName={torrent.name}
        onClose={() => setIsDeleteModalOpen(false)}
        onConfirm={handleDeleteConfirm}
      />

      <ShareableTorrentCardDialog
        torrent={torrent}
        open={isShareModalOpen}
        onOpenChange={setIsShareModalOpen}
      />
    </Dialog>
  );
}
