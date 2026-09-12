import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { StatusBadge } from "@/components/StatusBadge";
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { normalizeTaskStatus } from "@/utils/statusUtils";
import { formatBytesPerSecond } from "@/utils/bytes";
import { type EventType, type EventGroup, EVENT_TYPES_BY_GROUP, REPORT_EVENT_TYPES } from "@/constants/eventTypes";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import {
  AlertCircle,
  CheckCircle,
  XCircle,
  PlusCircle,
  ArrowRightLeft,
  Filter,
  Search,
  HelpCircle,
  WifiOff,
  Wifi,
  Server,
  BarChart3,
} from "lucide-react";

export type FilterType = EventType | "all";

export interface Event {
  uuid: string;
  worker_id?: string;
  type: EventType;
  task_hash: string;
  old_value?: string;
  new_value?: string;
  metadata?: {
    name?: string;
    progress?: number;
    old_progress?: number;
    new_progress?: number;
    last_progress?: number;
    schedule_name?: string;
    download_limit?: number;
    upload_limit?: number;
  };
  created_at: string;
}

interface EventListProps {
  /** Which event group this table shows - scopes the filter dropdown/legend and row layout. */
  group: EventGroup;
  events: Event[];
  isLoading: boolean;
  total: number;
  page: number;
  limit: number;
  filterType: FilterType;
  searchQuery?: string;
  onFilterChange: (value: FilterType) => void;
  onPageChange: (page: number) => void;
  onSearchChange?: (query: string) => void;
  /** worker_id -> worker name, used to label worker.offline/worker.recovered rows */
  workerNames?: Record<string, string>;
}

export function EventList({
  group,
  events,
  isLoading,
  total,
  page,
  limit,
  filterType,
  searchQuery = "",
  onFilterChange,
  onPageChange,
  onSearchChange,
  workerNames = {},
}: EventListProps) {
  const { t, i18n } = useTranslation();

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString(i18n.language, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  };

  const formatRelativeTime = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return t("history.time.justNow");
    if (diffMins < 60) return t("history.time.minutesAgo", { count: diffMins });
    if (diffHours < 24) return t("history.time.hoursAgo", { count: diffHours });
    return t("history.time.daysAgo", { count: diffDays });
  };

  const getEventIcon = (type: EventType) => {
    switch (type) {
      case "torrent.state_change":
        return <ArrowRightLeft className="h-5 w-5" />;
      case "torrent.added":
        return <PlusCircle className="h-5 w-5" />;
      case "torrent.removed":
        return <XCircle className="h-5 w-5" />;
      case "torrent.completed":
        return <CheckCircle className="h-5 w-5" />;
      case "bandwidth.schedule_applied":
        return <ArrowRightLeft className="h-5 w-5" />;
      case "worker.offline":
        return <WifiOff className="h-5 w-5" />;
      case "worker.recovered":
        return <Wifi className="h-5 w-5" />;
      case "report.transfer.daily":
      case "report.transfer.weekly":
        return <BarChart3 className="h-5 w-5" />;
    }
  };

  const getEventColor = (type: EventType) => {
    switch (type) {
      case "torrent.state_change":
        return "bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20";
      case "torrent.added":
        return "bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20";
      case "torrent.removed":
        return "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20";
      case "torrent.completed":
        return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20";
      case "bandwidth.schedule_applied":
        return "bg-violet-500/10 text-violet-600 dark:text-violet-400 border-violet-500/20";
      case "worker.offline":
        return "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20";
      case "worker.recovered":
        return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20";
      case "report.transfer.daily":
      case "report.transfer.weekly":
        return "bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 border-indigo-500/20";
    }
  };

  const renderStateChange = (oldValue?: string, newValue?: string) => {
    if (!oldValue || !newValue) return null;

    const oldStatus = normalizeTaskStatus(oldValue);
    const newStatus = normalizeTaskStatus(newValue);

    return (
      <div className="flex items-center gap-2 flex-wrap">
        <StatusBadge status={oldStatus} size="sm" showTooltip={false} showLabel={true} />
        <span className="text-xs text-muted-foreground">→</span>
        <StatusBadge status={newStatus} size="sm" showTooltip={false} showLabel={true} />
      </div>
    );
  };

  const isWorkerEvent = (type: EventType) => type === "worker.offline" || type === "worker.recovered";
  const isReportEvent = (type: EventType) => REPORT_EVENT_TYPES.includes(type);

  const getWorkerLabel = (event: Event) =>
    event.worker_id ? (workerNames[event.worker_id] ?? event.worker_id) : t("history.table.subject", "System");

  const renderWorkerStatus = (oldValue?: string, newValue?: string) => {
    if (!oldValue || !newValue) return null;

    const badgeClass = (value: string) =>
      value === "ONLINE"
        ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20"
        : "bg-red-500/10 text-red-600 dark:text-red-400 border-red-500/20";

    return (
      <div className="flex items-center gap-2 flex-wrap">
        <span className={`px-2 py-0.5 rounded-full text-xs border ${badgeClass(oldValue)}`}>{oldValue}</span>
        <span className="text-xs text-muted-foreground">→</span>
        <span className={`px-2 py-0.5 rounded-full text-xs border ${badgeClass(newValue)}`}>{newValue}</span>
      </div>
    );
  };

  const renderScheduleApplied = (event: Event) => {
    const { download_limit: downloadLimit, upload_limit: uploadLimit } = event.metadata ?? {};
    return (
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
        <span>{formatBytesPerSecond(downloadLimit ?? 0)} ↓</span>
        <span>{formatBytesPerSecond(uploadLimit ?? 0)} ↑</span>
      </div>
    );
  };

  const getEventBadge = (type: EventType) => {
    const typeMap: Record<EventType, string> = {
      "torrent.state_change": t("history.badge.stateChange"),
      "torrent.added": t("history.badge.added"),
      "torrent.removed": t("history.badge.removed"),
      "torrent.completed": t("history.badge.completed"),
      "bandwidth.schedule_applied": t("history.badge.bandwidthScheduleApplied", "Bandwidth schedule applied"),
      "worker.offline": t("history.badge.workerOffline", "Worker offline"),
      "worker.recovered": t("history.badge.workerRecovered", "Worker recovered"),
      "report.transfer.daily": t("history.badge.transferReportDaily", "Daily transfer report"),
      "report.transfer.weekly": t("history.badge.transferReportWeekly", "Weekly transfer report"),
    };

    return typeMap[type];
  };

  const sortedEventTypes = [...EVENT_TYPES_BY_GROUP[group]].sort((a, b) =>
    getEventBadge(a).localeCompare(getEventBadge(b), i18n.language)
  );

  // Use server-provided pagination and total (search filtering is handled by backend)
  const totalPages = Math.ceil(total / limit);

  const getPageNumbers = () => {
    const pages: (number | string)[] = [];
    const maxVisible = 5;

    if (totalPages <= maxVisible) {
      for (let i = 0; i < totalPages; i++) {
        pages.push(i);
      }
    } else {
      pages.push(0);

      let start = Math.max(1, page - 1);
      let end = Math.min(totalPages - 2, page + 1);

      if (page < 2) {
        end = 2;
      }

      if (page > totalPages - 3) {
        start = totalPages - 3;
      }

      if (start > 1) {
        pages.push('...');
      }

      for (let i = start; i <= end; i++) {
        pages.push(i);
      }

      if (end < totalPages - 2) {
        pages.push('...');
      }

      pages.push(totalPages - 1);
    }

    return pages;
  };

  return (
    <div className="space-y-4 px-4 sm:px-6">
      {/* Filters and Actions */}
      <div className="flex gap-2 flex-wrap">
        <div className="relative flex-1 min-w-[200px] max-w-xs">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            type="text"
            placeholder={t("history.search.placeholder") || "Search events..."}
            value={searchQuery}
            onChange={(e) => {
              onSearchChange?.(e.target.value);
              onPageChange(0);
            }}
            className="pl-9 h-9"
          />
        </div>

        <Select value={filterType} onValueChange={(value) => {
          onFilterChange(value as FilterType);
          onPageChange(0);
        }}>
          <SelectTrigger className="w-[160px] sm:w-[200px]">
            <Filter className="h-4 w-4 mr-2" />
            <SelectValue placeholder={t("history.filter.all")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("history.filter.all")}</SelectItem>
            {sortedEventTypes.map((eventType) => (
              <SelectItem key={eventType} value={eventType}>
                {getEventBadge(eventType)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Events List */}
      {isLoading ? (
        <Card>
          <CardContent className="px-4 sm:px-6 py-12">
            <div className="text-center">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary mx-auto"></div>
              <p className="text-sm text-muted-foreground mt-4">{t("common.loading")}</p>
            </div>
          </CardContent>
        </Card>
      ) : events.length === 0 ? (
        <Card>
          <CardContent className="px-4 sm:px-6 py-12">
            <div className="text-center">
              <AlertCircle className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
              <p className="text-muted-foreground">
                {searchQuery
                  ? t("history.events.noResults") || "No events found matching your search"
                  : t("history.events.noEvents")
                }
              </p>
            </div>
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="border rounded-md shadow-sm bg-card overflow-hidden overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/40 text-xs text-muted-foreground uppercase">
                  <th className="text-left font-semibold px-3 py-2 w-9"></th>
                  <th className="text-left font-semibold px-3 py-2">
                    {group === "worker"
                      ? t("history.table.worker") || "Worker"
                      : group === "torrent"
                        ? t("history.table.torrent") || "Torrent"
                      : group === "schedule"
                        ? t("history.table.schedule") || "Schedule"
                        : group === "report"
                          ? t("history.table.report") || "Report"
                        : t("history.table.subject") || "Torrent / Worker"}
                  </th>
                  <th className="text-left font-semibold px-3 py-2 hidden md:table-cell">{t("history.table.change") || "Change"}</th>
                  <th className="text-right font-semibold px-3 py-2 whitespace-nowrap">{t("history.table.time") || "Time"}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {events.map((event) => (
                  <tr key={event.uuid} className="hover:bg-muted/30 transition-colors">
                    <td className="px-3 py-2">
                      <Popover>
                        <PopoverTrigger asChild>
                          <button type="button" className={`h-6 w-6 rounded-full flex items-center justify-center border cursor-pointer ${getEventColor(event.type)}`}>
                            {getEventIcon(event.type)}
                          </button>
                        </PopoverTrigger>
                        <PopoverContent className="w-auto p-2 text-xs" sideOffset={4}>
                          {getEventBadge(event.type)}
                        </PopoverContent>
                      </Popover>
                    </td>
                    <td className="px-3 py-2 max-w-[220px]">
                      {isWorkerEvent(event.type) ? (
                        <p className="font-medium text-foreground truncate flex items-center gap-1.5" title={getWorkerLabel(event)}>
                          <Server className="h-3.5 w-3.5 text-muted-foreground/70 shrink-0" />
                          {getWorkerLabel(event)}
                        </p>
                      ) : event.type === "bandwidth.schedule_applied" ? (
                        <p className="font-medium text-foreground truncate" title={event.metadata?.schedule_name}>
                          {event.metadata?.schedule_name ?? t("history.badge.bandwidthScheduleApplied", "Bandwidth schedule applied")}
                        </p>
                      ) : isReportEvent(event.type) ? (
                        <p className="font-medium text-foreground truncate">
                          {getEventBadge(event.type)}
                        </p>
                      ) : event.metadata?.name ? (
                        <p className="font-medium text-foreground truncate" title={event.metadata.name}>
                          {event.metadata.name}
                        </p>
                      ) : (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <div className="flex items-center gap-1.5 text-muted-foreground cursor-help">
                              <HelpCircle className="h-3.5 w-3.5 text-muted-foreground/70 shrink-0" />
                              <span className="italic truncate">{t("history.events.unknownTorrent")}</span>
                            </div>
                          </TooltipTrigger>
                          <TooltipContent side="top" className="max-w-xs">
                            <p className="text-xs">
                              <span className="font-semibold">Hash:</span>{" "}
                              <span className="font-mono break-all">{event.task_hash}</span>
                            </p>
                          </TooltipContent>
                        </Tooltip>
                      )}
                    </td>
                    <td className="px-3 py-2 hidden md:table-cell">
                      {isWorkerEvent(event.type)
                        ? renderWorkerStatus(event.old_value, event.new_value)
                        : event.type === "torrent.state_change"
                          ? renderStateChange(event.old_value, event.new_value)
                          : event.type === "bandwidth.schedule_applied"
                            ? renderScheduleApplied(event)
                            : <span className="text-xs text-muted-foreground">—</span>}
                    </td>
                    <td className="px-3 py-2 text-right whitespace-nowrap">
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span className="text-xs text-muted-foreground cursor-help">
                            {formatRelativeTime(event.created_at)}
                          </span>
                        </TooltipTrigger>
                        <TooltipContent>
                          <p>{formatDate(event.created_at)}</p>
                        </TooltipContent>
                      </Tooltip>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Legend */}
          <div className="flex flex-wrap items-center gap-x-3 sm:gap-x-4 gap-y-2 justify-center text-xs text-muted-foreground">
            {sortedEventTypes.map((eventType) => (
              <Popover key={eventType}>
                <PopoverTrigger asChild>
                  <button type="button" className="flex items-center gap-1.5 cursor-pointer sm:cursor-default">
                    <div className={`h-6 w-6 rounded-full flex items-center justify-center border shrink-0 ${getEventColor(eventType)}`}>
                      {getEventIcon(eventType)}
                    </div>
                    <span className="hidden sm:inline">{getEventBadge(eventType)}</span>
                  </button>
                </PopoverTrigger>
                <PopoverContent className="w-auto p-2 text-xs sm:hidden" sideOffset={4}>
                  {getEventBadge(eventType)}
                </PopoverContent>
              </Popover>
            ))}
          </div>

          {/* Pagination */}
          {(totalPages > 1 || searchQuery) && events.length > 0 && (
            <div className="flex flex-col items-center gap-3 mt-4 sm:mt-6">
              <div className="text-xs sm:text-sm text-muted-foreground">
                {t("history.pagination.showing", {
                  from: Math.min(page * limit + 1, total),
                  to: Math.min((page + 1) * limit, total),
                  total: total,
                })}
              </div>
              <Pagination className="mx-0">
                <PaginationContent>
                  <PaginationItem>
                    <PaginationPrevious
                      onClick={() => onPageChange(Math.max(0, page - 1))}
                      className={page === 0 ? "pointer-events-none opacity-50" : "cursor-pointer"}
                    />
                  </PaginationItem>

                  {getPageNumbers().map((pageNum, index) => (
                    <PaginationItem key={pageNum === '...' ? `ellipsis-${index}` : pageNum}>
                      {pageNum === '...' ? (
                        <PaginationEllipsis />
                      ) : (
                        <PaginationLink
                          onClick={() => onPageChange(pageNum as number)}
                          isActive={page === pageNum}
                          className="cursor-pointer"
                        >
                          {(pageNum as number) + 1}
                        </PaginationLink>
                      )}
                    </PaginationItem>
                  ))}

                  <PaginationItem>
                    <PaginationNext
                      onClick={() => onPageChange(Math.min(totalPages - 1, page + 1))}
                      className={page >= totalPages - 1 ? "pointer-events-none opacity-50" : "cursor-pointer"}
                    />
                  </PaginationItem>
                </PaginationContent>
              </Pagination>
            </div>
          )}
        </>
      )}
    </div>
  );
}
