import { Activity, ArrowDown, ArrowUp, HardDrive, Info, Loader2, Server } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/card";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { formatBytes, formatBytesPerSecond } from "@/utils/bytes";
import { sumOverviewMetric, workerOverviewRows } from "@/utils/workerOverview";
import type { OverviewRow } from "@/utils/workerOverview";
import type { Worker } from "@/types/worker";
import type { WorkerOverviews } from "@/hooks/useWorkerOverviews";
import type { ReactNode } from "react";

type Summary = ReturnType<typeof sumOverviewMetric>;

function InlineSummary({ title, label = title, icon: Icon, children, description, coverage }: {
  title: string; label?: string; icon: LucideIcon; children: ReactNode; description: ReactNode; coverage: string | null;
}) {
  return <Tooltip>
    <TooltipTrigger asChild>
      <div tabIndex={0} className="flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-sm text-xs outline-offset-2">
        <Icon className="h-3 w-3 text-muted-foreground" />
        <h2 aria-label={title} className="text-[11px] text-muted-foreground">{label}</h2>
        <div className="flex items-center gap-1.5 font-semibold tabular-nums">{children}</div>
        {coverage && <Info className="h-3 w-3 text-muted-foreground" aria-label={coverage} />}
      </div>
    </TooltipTrigger>
    <TooltipContent>{description}{coverage && <p>{coverage}</p>}</TooltipContent>
  </Tooltip>;
}

function NodeComparison({ rows, metric, title, loading }: {
  rows: OverviewRow[]; metric: "stored" | "free"; title: string; loading: boolean;
}) {
  const { t, i18n } = useTranslation();
  const summary = sumOverviewMetric(rows, metric);
  const total = summary.value ?? 0;
  const partial = summary.available > 0 && summary.available < summary.total
    ? t("workers.overview.partial", { count: summary.available, total: summary.total }) : null;
  const valueLabel = (row: OverviewRow) => row[metric] === null
    ? t(row.checking ? "workers.overview.loading" : "workers.overview.unavailable") : formatBytes(row[metric]!);
  const color = (row: OverviewRow) => /^#[\da-f]{6}$/i.test(row.worker.color ?? "")
    ? row.worker.color : "var(--chart-1)";
  const details = (row: OverviewRow) => <>
    <p className="max-w-72 break-words font-medium">{row.worker.name}</p>
    <p>{title}: {valueLabel(row)}{row[metric] !== null && total > 0 && ` · ${new Intl.NumberFormat(i18n.language, { style: "percent", maximumFractionDigits: 1 }).format(row[metric]! / total)}`}</p>
    {row[metric] !== null && row.collectedAt && <p className="text-xs">{t("workers.overview.collectedAt", { time: new Date(row.collectedAt).toLocaleString(i18n.language) })}</p>}
  </>;

  return <Card className="min-w-0 gap-1 py-1.5 shadow-none" aria-busy={loading}>
    <div className="flex items-center justify-between gap-2 px-3">
      <h2 className="truncate text-xs font-medium">{title}</h2>
      <div className="flex shrink-0 items-center gap-1.5 text-xs tabular-nums text-muted-foreground">
        {loading && <Loader2 className="h-3 w-3 animate-spin" aria-label={t("workers.overview.updating")} />}
        <span>{summary.value === null ? t(loading ? "workers.overview.loading" : "workers.overview.unavailable") : formatBytes(summary.value)}</span>
        {partial && <Tooltip>
          <TooltipTrigger asChild><span tabIndex={0}><Info className="h-3 w-3" aria-label={partial} /></span></TooltipTrigger>
          <TooltipContent>{partial}</TooltipContent>
        </Tooltip>}
      </div>
    </div>
    <CardContent className="space-y-1 px-3">
      <div role="group" aria-label={title} className="flex h-2.5 overflow-hidden rounded-full bg-muted">
        {rows.filter(row => row[metric] !== null && row[metric]! > 0).map(row => <Tooltip key={row.worker.uuid}>
          <TooltipTrigger asChild>
            <button type="button" tabIndex={-1} aria-label={`${row.worker.name}: ${valueLabel(row)}`} className="h-full shrink-0 outline-none" style={{ width: `${row[metric]! / total * 100}%`, backgroundColor: color(row), boxShadow: "inset -1px 0 0 var(--card)" }} />
          </TooltipTrigger>
          <TooltipContent>{details(row)}</TooltipContent>
        </Tooltip>)}
      </div>
      <div className="flex items-center gap-3 overflow-x-auto whitespace-nowrap text-[10px] leading-4 text-muted-foreground">
        {rows.length === 0 ? <span>{t(loading ? "workers.overview.loading" : "workers.overview.empty")}</span> : rows.map(row => <Tooltip key={row.worker.uuid}>
          <TooltipTrigger asChild>
            <button type="button" aria-label={`${row.worker.name}: ${valueLabel(row)}`} className="flex shrink-0 items-center gap-1 rounded-sm outline-offset-2">
              <span className={`h-1.5 w-1.5 rounded-full ${row[metric] === null ? "border border-dashed border-muted-foreground" : ""}`} style={row[metric] === null ? undefined : { backgroundColor: color(row) }} aria-hidden="true" />
              <span className="max-w-24 truncate">{row.worker.name}</span>
              {row[metric] === null && <span aria-hidden="true">—</span>}
            </button>
          </TooltipTrigger>
          <TooltipContent>{details(row)}</TooltipContent>
        </Tooltip>)}
      </div>
    </CardContent>
  </Card>;
}

export function WorkerOverviewWidgets({ workers, states, filtered, refreshing }: {
  workers: Worker[]; states: WorkerOverviews; filtered: boolean; refreshing: boolean;
}) {
  const { t } = useTranslation();
  const rows = workerOverviewRows(workers, states);
  const stored = sumOverviewMetric(rows, "stored");
  const online = sumOverviewMetric(rows, "online");
  const total = sumOverviewMetric(rows, "total");
  const active = sumOverviewMetric(rows, "active");
  const download = sumOverviewMetric(rows, "download");
  const upload = sumOverviewMetric(rows, "upload");
  const loading = refreshing || rows.some(row => row.checking);
  const coverage = (summary: Summary) => summary.total > 0 && summary.available < summary.total
    ? t(summary.available === 0 ? (loading ? "workers.overview.loading" : "workers.overview.unavailable") : "workers.overview.partial", { count: summary.available, total: summary.total })
    : null;
  const display = (value: number | null, formatter: (value: number) => string = String) => value === null
    ? t(loading ? "workers.overview.loading" : "workers.overview.unavailable") : formatter(value);
  const transferCoverage = coverage(download.available <= upload.available ? download : upload);

  return <section className="space-y-2" aria-label={t("workers.overview.title")} aria-busy={loading}>
    <TooltipProvider>
      <div className="flex flex-wrap items-center gap-3">
        <div className="grid w-full min-w-0 gap-3 sm:w-auto sm:grid-cols-[auto_auto]">
          <Card className="min-h-12 min-w-0 justify-center gap-0 py-2 shadow-none">
            <CardContent className="flex flex-wrap items-center gap-x-3 gap-y-2 px-3">
              <InlineSummary title={t("workers.overview.stored")} label={t("workers.overview.storedShort")} icon={HardDrive} description={t("workers.overview.storedDescription")} coverage={coverage(stored)}>
                {display(stored.value, formatBytes)}
              </InlineSummary>
              <InlineSummary title={t("workers.overview.online")} label={t("workers.overview.onlineShort")} icon={Server} description={t("workers.overview.connections")} coverage={coverage(online)}>
                {display(online.value)}<span className="font-normal text-muted-foreground">/{workers.length}</span>
              </InlineSummary>
              <InlineSummary title={t("workers.overview.torrents")} icon={Activity} description={t("workers.overview.active", { value: active.value ?? "—" })} coverage={coverage(total)}>
                {display(total.value)}<span className="text-[11px] font-normal text-muted-foreground">({t("workers.overview.active", { value: active.value ?? "—" })})</span>
              </InlineSummary>
            </CardContent>
          </Card>
          <Card className="min-h-12 min-w-0 justify-center gap-0 py-2 shadow-none">
            <CardContent className="flex flex-wrap items-center gap-3 px-3 whitespace-nowrap text-xs font-semibold tabular-nums" aria-label={t("workers.overview.transfer")}>
              <span className="flex items-center gap-1"><ArrowDown aria-label={t("workers.overview.download")} className="h-3 w-3 text-muted-foreground" />{display(download.value, formatBytesPerSecond)}</span>
              <span className="flex items-center gap-1"><ArrowUp aria-label={t("workers.overview.upload")} className="h-3 w-3 text-muted-foreground" />{display(upload.value, formatBytesPerSecond)}</span>
              {transferCoverage && <Tooltip>
                <TooltipTrigger asChild><span tabIndex={0}><Info className="h-3 w-3 text-muted-foreground" aria-label={transferCoverage} /></span></TooltipTrigger>
                <TooltipContent>{transferCoverage}</TooltipContent>
              </Tooltip>}
            </CardContent>
          </Card>
        </div>
        <div className="grid min-w-0 flex-1 basis-[420px] grid-cols-2 gap-3">
          <NodeComparison rows={rows} metric="free" title={t("workers.overview.freeByNode")} loading={loading} />
          <NodeComparison rows={rows} metric="stored" title={t("workers.overview.storedByNode")} loading={loading} />
        </div>
      </div>
    </TooltipProvider>
    {(filtered || loading) && <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground" role="status">
      {filtered && <span>{t("workers.overview.filtered", { count: workers.length })}</span>}
      {loading && <span className="flex items-center gap-1.5"><Loader2 className="h-3 w-3 animate-spin" />{t("workers.overview.updating")}</span>}
    </div>}
  </section>;
}
