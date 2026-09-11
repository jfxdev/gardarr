import { useCallback, useEffect, useState } from 'react'
import { AlertTriangle, BarChart3, Camera, Download, Save, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { transferReportsService } from '@/services/transferReports'
import { settingsService } from '@/services/settings'
import type { LatestTransferReports, TransferRankItem, TransferReport, TransferReportSettings } from '@/types/transferReports'
import { formatBytes } from '@/utils/bytes'
import { toast } from 'sonner'

const initialSettings: TransferReportSettings = {
  enabled: true, snapshots_per_day: 4, daily_report_time: '00:05', weekly_report_day: 1, weekly_report_time: '00:10', top_n: 10,
}
const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

function Ranking({ title, icon: Icon, items, empty }: { title: string; icon: typeof Upload; items: TransferRankItem[]; empty: string }) {
  const { t } = useTranslation()
  return (
    <div className="space-y-3 rounded-lg border p-4">
      <div className="flex items-center gap-2 font-medium"><Icon className="h-4 w-4 text-primary" />{title}</div>
      {items.length === 0 ? <p className="text-sm text-muted-foreground">{empty}</p> : (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <caption className="sr-only">{title}</caption>
            <thead className="border-b text-xs text-muted-foreground">
              <tr><th scope="col" className="pb-2 pr-2 font-medium">{t('reports.rank', 'Rank')}</th><th scope="col" className="pb-2 font-medium">{t('reports.name', 'Torrent')}</th><th scope="col" className="pb-2 text-right font-medium">{t('reports.transferred', 'Transferred')}</th></tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.hash} className="border-b last:border-0">
                  <td className="py-2 pr-2 text-muted-foreground">{item.rank}.</td>
                  <td className="min-w-0 py-2" title={item.name}><span className="block max-w-[16rem] truncate">{item.name || item.hash}</span></td>
                  <td className="py-2 text-right font-medium whitespace-nowrap">{formatBytes(item.bytes)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

function discordBytes(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let amount = bytes
  let index = 0
  while (amount >= 1024 && index < units.length - 1) {
    amount /= 1024
    index += 1
  }
  return `${amount.toFixed(amount >= 100 ? 0 : 1)} ${units[index]}`
}

function discordTimestamp(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toISOString().replace('.000Z', 'Z')
}

function discordRanking(items: TransferRankItem[], empty: string): string {
  if (items.length === 0) return empty
  return items.map((item) => `${item.rank}. ${item.name.slice(0, 120)} — ${discordBytes(item.bytes)}`).join('\n')
}

function DiscordReportPreview({ report, language, inProgress = false }: { report: TransferReport; language: string; inProgress?: boolean }) {
  const { t } = useTranslation()
  const portuguese = language === 'pt-BR'
  const daily = report.period_type === 'daily'
  const title = daily
    ? (portuguese ? 'Gardarr · Relatório diário' : 'Gardarr · Daily transfer report')
    : (portuguese ? 'Gardarr · Relatório semanal' : 'Gardarr · Weekly transfer report')
  const description = daily
    ? (inProgress ? (portuguese ? 'Ranking de transferências até agora hoje.' : 'Top transfer activity so far today.') : (portuguese ? 'Ranking de transferências do dia encerrado.' : 'Top transfer activity for the completed day.'))
    : (inProgress ? (portuguese ? 'Ranking de transferências até agora nesta semana.' : 'Top transfer activity so far this week.') : (portuguese ? 'Ranking de transferências da semana encerrada.' : 'Top transfer activity for the completed week.'))
  const empty = portuguese ? 'Nenhuma movimentação registrada.' : 'No movement recorded.'

  return (
    <section data-testid={`discord-preview-${inProgress ? 'current-' : ''}${report.period_type}`} className="rounded-md bg-[#313338] p-3 text-[#dbdee1] shadow-sm">
      <p className="mb-2 text-xs font-medium uppercase tracking-wide text-[#b5bac1]">{inProgress ? t('reports.discordPreviewCurrent', 'Discord preview if sent now') : t('reports.discordPreview', 'Discord preview')}</p>
      <div className="border-l-4 border-[#5865f2] pl-3">
        <p className="font-semibold text-white">{title}</p>
        <p className="mt-1 text-sm text-[#b5bac1]">{description}</p>
        <dl className="mt-3 grid gap-3 text-sm">
          <div><dt className="font-semibold text-white">Period</dt><dd className="mt-1 break-all text-[#dbdee1]">{discordTimestamp(report.period_start)} → {discordTimestamp(report.period_end)}</dd></div>
          <div><dt className="font-semibold text-white">Coverage</dt><dd className="mt-1 text-[#dbdee1]">{report.coverage}</dd></div>
          <div><dt className="font-semibold text-white">Upload</dt><dd className="mt-1 whitespace-pre-line text-[#dbdee1]">{discordRanking(report.upload, empty)}</dd></div>
          <div><dt className="font-semibold text-white">Download</dt><dd className="mt-1 whitespace-pre-line text-[#dbdee1]">{discordRanking(report.download, empty)}</dd></div>
          {report.unavailable_workers.length > 0 && <div><dt className="font-semibold text-white">Unavailable workers</dt><dd className="mt-1 break-all text-[#dbdee1]">[{report.unavailable_workers.join(' ')}]</dd></div>}
        </dl>
        <p className="mt-3 text-xs text-[#b5bac1]">{discordTimestamp(report.generated_at)}</p>
      </div>
    </section>
  )
}

function ReportCard({ title, report, discordLanguage, showDiscordPreview = true, discordPreviewInProgress = false }: { title: string; report: TransferReport | null; discordLanguage: string; showDiscordPreview?: boolean; discordPreviewInProgress?: boolean }) {
  const { t, i18n } = useTranslation()
  if (!report) {
    return <Card><CardHeader><CardTitle>{title}</CardTitle><CardDescription>{t('reports.noReport', 'No report generated yet.')}</CardDescription></CardHeader></Card>
  }
  const formatter = new Intl.DateTimeFormat(i18n.language, {
    timeZone: report.timezone,
    year: 'numeric', month: 'numeric', day: 'numeric',
  })
  const period = `${formatter.format(new Date(report.period_start))} – ${formatter.format(new Date(report.period_end))}`

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between gap-3">
          <div><CardTitle>{title}</CardTitle><CardDescription>{period} · {report.timezone}</CardDescription></div>
          <Badge variant={report.coverage === 'complete' ? 'secondary' : 'outline'}>{report.coverage}</Badge>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        {report.unavailable_workers.length > 0 && <p className="flex items-center gap-2 text-sm text-amber-600 dark:text-amber-400"><AlertTriangle className="h-4 w-4" />{t('reports.partial', 'Some workers were unavailable.')}</p>}
        <div className="grid gap-3 lg:grid-cols-2">
          <Ranking title={t('reports.upload', 'Upload')} icon={Upload} items={report.upload} empty={t('reports.noUpload', 'No upload movement available for this period.')} />
          <Ranking title={t('reports.download', 'Download')} icon={Download} items={report.download} empty={t('reports.noDownload', 'No download movement available for this period.')} />
        </div>
        {showDiscordPreview && <DiscordReportPreview report={report} language={discordLanguage} inProgress={discordPreviewInProgress} />}
      </CardContent>
    </Card>
  )
}

export default function ReportsPage() {
  const { t, i18n } = useTranslation()
  const [settings, setSettings] = useState(initialSettings)
  const [reports, setReports] = useState<LatestTransferReports>({ daily: null, weekly: null })
  const [currentReports, setCurrentReports] = useState<LatestTransferReports>({ daily: null, weekly: null })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [capturing, setCapturing] = useState(false)
  const [discordLanguage, setDiscordLanguage] = useState(i18n.language)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [settingsResult, reportsResult, currentResult, languageResult] = await Promise.all([transferReportsService.getSettings(), transferReportsService.getLatest(), transferReportsService.getCurrent(), settingsService.getLanguage()])
      if (settingsResult.data) setSettings(settingsResult.data)
      if (reportsResult.data) setReports(reportsResult.data)
      if (currentResult.data) setCurrentReports(currentResult.data)
      if (languageResult.data?.default_language) setDiscordLanguage(languageResult.data.default_language)
      if (settingsResult.error || reportsResult.error || currentResult.error) toast.error(t('reports.loadFailed', 'Could not load transfer reports.'))
    } catch {
      toast.error(t('reports.loadFailed', 'Could not load transfer reports.'))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => { void load() }, [load])

  const save = async () => {
    setSaving(true)
    try {
      const result = await transferReportsService.updateSettings(settings)
      if (result.data) {
        setSettings(result.data)
        toast.success(t('reports.saved', 'Report settings saved.'))
      } else toast.error(result.error || t('reports.saveFailed', 'Could not save report settings.'))
    } catch {
      toast.error(t('reports.saveFailed', 'Could not save report settings.'))
    } finally {
      setSaving(false)
    }
  }

  const captureSnapshot = async () => {
    setCapturing(true)
    try {
      const result = await transferReportsService.captureSnapshot()
      if (result.error) {
        toast.error(result.error || t('reports.snapshotFailed', 'Could not capture a transfer snapshot.'))
      } else {
        toast.success(t('reports.snapshotCaptured', 'Transfer snapshot captured.'))
        void load()
      }
    } catch {
      toast.error(t('reports.snapshotFailed', 'Could not capture a transfer snapshot.'))
    } finally {
      setCapturing(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="rounded-lg bg-primary/10 p-2"><BarChart3 className="h-6 w-6 text-primary" /></div>
        <div className="flex-1"><h1 className="text-2xl font-bold tracking-tight">{t('reports.title', 'Transfer reports')}</h1><p className="text-sm text-muted-foreground">{t('reports.subtitle', 'Periodic upload and download rankings from qBittorrent counters.')}</p></div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => { void captureSnapshot() }} disabled={capturing || loading}><Camera className="mr-2 h-4 w-4" />{capturing ? t('reports.capturingSnapshot', 'Capturing…') : t('reports.captureSnapshot', 'Capture snapshot now')}</Button>
        </div>
      </div>
      <Card>
        <CardHeader><CardTitle>{t('reports.settings', 'Schedule')}</CardTitle><CardDescription>{t('reports.settingsDescription', 'Times use the Gardarr timezone configured in Settings.')}</CardDescription></CardHeader>
        <CardContent className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          <div className="flex items-center justify-between rounded-lg border p-3"><Label htmlFor="reports-enabled">{t('reports.enabled', 'Enable reports')}</Label><Switch id="reports-enabled" checked={settings.enabled} onCheckedChange={(enabled) => { setSettings((value) => ({ ...value, enabled })) }} /></div>
          <div className="space-y-2"><Label>{t('reports.snapshots', 'Snapshots per day')}</Label><select className="h-9 w-full rounded-md border bg-background px-3 text-sm" value={settings.snapshots_per_day} onChange={(event) => { setSettings((value) => ({ ...value, snapshots_per_day: Number(event.target.value) })) }}>{[1, 2, 3, 4, 6, 8, 12, 24].map((value) => <option key={value} value={value}>{value}</option>)}</select></div>
          <div className="space-y-2"><Label>{t('reports.topN', 'Ranking size')}</Label><Input type="number" min={1} max={50} value={settings.top_n} onChange={(event) => { setSettings((value) => ({ ...value, top_n: Number(event.target.value) })) }} /></div>
          <div className="space-y-2"><Label>{t('reports.dailyTime', 'Daily report time')}</Label><Input type="time" value={settings.daily_report_time} onChange={(event) => { setSettings((value) => ({ ...value, daily_report_time: event.target.value })) }} /></div>
          <div className="space-y-2"><Label>{t('reports.weeklyDay', 'Weekly report day')}</Label><select className="h-9 w-full rounded-md border bg-background px-3 text-sm" value={settings.weekly_report_day} onChange={(event) => { setSettings((value) => ({ ...value, weekly_report_day: Number(event.target.value) })) }}>{weekdays.map((day, index) => <option key={day} value={index}>{t(`reports.weekdays.${index}`, day)}</option>)}</select></div>
          <div className="space-y-2"><Label>{t('reports.weeklyTime', 'Weekly report time')}</Label><Input type="time" value={settings.weekly_report_time} onChange={(event) => { setSettings((value) => ({ ...value, weekly_report_time: event.target.value })) }} /></div>
          <div className="md:col-span-2 xl:col-span-3"><Button onClick={() => { void save() }} disabled={saving || loading}><Save className="mr-2 h-4 w-4" />{saving ? t('common.saving', 'Saving…') : t('reports.save', 'Save schedule')}</Button></div>
        </CardContent>
      </Card>
      {loading ? <p className="text-sm text-muted-foreground">{t('common.loading', 'Loading…')}</p> : (
        <div className="space-y-8">
          <section className="space-y-4">
            <div><h2 className="text-lg font-semibold">{t('reports.current', 'Current snapshot rankings')}</h2><p className="text-sm text-muted-foreground">{t('reports.currentDescription', 'Live values calculated from stored snapshots. Discord is not required.')}</p><p className="mt-1 text-sm text-muted-foreground">{t('reports.baselineHint', 'The first snapshot establishes a baseline; capture another after transfer activity to calculate movement.')}</p></div>
            <div className="grid gap-6 xl:grid-cols-2"><ReportCard title={t('reports.today', 'Today')} report={currentReports.daily} discordLanguage={discordLanguage} discordPreviewInProgress /><ReportCard title={t('reports.currentWeek', 'This week')} report={currentReports.weekly} discordLanguage={discordLanguage} discordPreviewInProgress /></div>
          </section>
          <section className="space-y-4">
            <div><h2 className="text-lg font-semibold">{t('reports.completed', 'Last completed reports')}</h2><p className="text-sm text-muted-foreground">{t('reports.completedDescription', 'These are the reports retained in history and sent to matching Discord destinations.')}</p></div>
            <div className="grid gap-6 xl:grid-cols-2"><ReportCard title={t('reports.daily', 'Daily report')} report={reports.daily} discordLanguage={discordLanguage} /><ReportCard title={t('reports.weekly', 'Weekly report')} report={reports.weekly} discordLanguage={discordLanguage} /></div>
          </section>
        </div>
      )}
    </div>
  )
}
