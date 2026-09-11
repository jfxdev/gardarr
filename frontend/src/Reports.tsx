import { useCallback, useEffect, useState } from 'react'
import { AlertTriangle, BarChart3, Download, Save, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { transferReportsService } from '@/services/transferReports'
import type { LatestTransferReports, TransferRankItem, TransferReport, TransferReportSettings } from '@/types/transferReports'
import { formatBytes } from '@/utils/bytes'
import { toast } from 'sonner'

const initialSettings: TransferReportSettings = {
  enabled: true, snapshots_per_day: 4, daily_report_time: '00:05', weekly_report_day: 1, weekly_report_time: '00:10', top_n: 10,
}
const weekdays = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

function Ranking({ title, icon: Icon, items, empty }: { title: string; icon: typeof Upload; items: TransferRankItem[]; empty: string }) {
  return (
    <div className="space-y-3 rounded-lg border p-4">
      <div className="flex items-center gap-2 font-medium"><Icon className="h-4 w-4 text-primary" />{title}</div>
      {items.length === 0 ? <p className="text-sm text-muted-foreground">{empty}</p> : (
        <ol className="space-y-2">
          {items.map((item) => (
            <li key={item.hash} className="flex gap-3 text-sm">
              <span className="w-5 text-muted-foreground">{item.rank}.</span>
              <span className="min-w-0 flex-1 truncate" title={item.name}>{item.name || item.hash}</span>
              <span className="whitespace-nowrap font-medium">{formatBytes(item.bytes)}</span>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}

function ReportCard({ title, report }: { title: string; report: TransferReport | null }) {
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
      </CardContent>
    </Card>
  )
}

export default function ReportsPage() {
  const { t } = useTranslation()
  const [settings, setSettings] = useState(initialSettings)
  const [reports, setReports] = useState<LatestTransferReports>({ daily: null, weekly: null })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [settingsResult, reportsResult] = await Promise.all([transferReportsService.getSettings(), transferReportsService.getLatest()])
      if (settingsResult.data) setSettings(settingsResult.data)
      if (reportsResult.data) setReports(reportsResult.data)
      if (settingsResult.error || reportsResult.error) toast.error(t('reports.loadFailed', 'Could not load transfer reports.'))
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

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <div className="rounded-lg bg-primary/10 p-2"><BarChart3 className="h-6 w-6 text-primary" /></div>
        <div><h1 className="text-2xl font-bold tracking-tight">{t('reports.title', 'Transfer reports')}</h1><p className="text-sm text-muted-foreground">{t('reports.subtitle', 'Periodic upload and download rankings from qBittorrent counters.')}</p></div>
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
      {loading ? <p className="text-sm text-muted-foreground">{t('common.loading', 'Loading…')}</p> : <div className="grid gap-6 xl:grid-cols-2"><ReportCard title={t('reports.daily', 'Daily report')} report={reports.daily} /><ReportCard title={t('reports.weekly', 'Weekly report')} report={reports.weekly} /></div>}
    </div>
  )
}
