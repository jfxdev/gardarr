import { useCallback, useEffect, useState } from 'react'
import { ArrowLeft, ArrowRightLeft, BarChart3, Bell, CheckCircle, CheckCircle2, Plus, PlusCircle, Send, Trash2, Wifi, WifiOff, XCircle, type LucideIcon } from 'lucide-react'
import { useNavigate } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Toggle } from '@/components/ui/toggle'
import { Badge } from '@/components/ui/badge'
import { EVENT_TYPE_LABELS, EVENT_TYPES_BY_GROUP, type EventGroup, type EventType } from '@/constants/eventTypes'
import { discordService } from '@/services/discord'
import type { DiscordIntegration, DiscordIntegrationInput } from '@/types/discord'
import { toast } from 'sonner'

const blank = (): Required<DiscordIntegrationInput> => ({
  name: '', webhook_url: '', enabled: true, all_events: true, event_types: [], status_filter: [], category_filter: [], name_terms: [],
})

const toTerms = (value: string) => value.split(',').map((item) => item.trim()).filter(Boolean)

const EVENT_TYPE_GROUPS: readonly { group: Exclude<EventGroup, 'all'>; labelKey: string; defaultLabel: string }[] = [
  { group: 'torrent', labelKey: 'discord.eventTypeGroup.torrents', defaultLabel: 'Torrents' },
  { group: 'worker', labelKey: 'discord.eventTypeGroup.workers', defaultLabel: 'Workers' },
  { group: 'schedule', labelKey: 'discord.eventTypeGroup.schedule', defaultLabel: 'Bandwidth schedule' },
  { group: 'report', labelKey: 'discord.eventTypeGroup.reports', defaultLabel: 'Transfer reports' },
]

const EVENT_TYPE_ICONS: Record<EventType, LucideIcon> = {
  'torrent.state_change': ArrowRightLeft,
  'torrent.added': PlusCircle,
  'torrent.removed': XCircle,
  'torrent.completed': CheckCircle,
  'bandwidth.schedule_applied': ArrowRightLeft,
  'worker.offline': WifiOff,
  'worker.recovered': Wifi,
  'report.transfer.daily': BarChart3,
  'report.transfer.weekly': BarChart3,
}

const EVENT_TYPE_ICON_COLORS: Record<EventType, string> = {
  'torrent.state_change': 'text-amber-600 dark:text-amber-400',
  'torrent.added': 'text-blue-600 dark:text-blue-400',
  'torrent.removed': 'text-red-600 dark:text-red-400',
  'torrent.completed': 'text-emerald-600 dark:text-emerald-400',
  'bandwidth.schedule_applied': 'text-violet-600 dark:text-violet-400',
  'worker.offline': 'text-red-600 dark:text-red-400',
  'worker.recovered': 'text-emerald-600 dark:text-emerald-400',
  'report.transfer.daily': 'text-orange-600 dark:text-orange-400',
  'report.transfer.weekly': 'text-orange-600 dark:text-orange-400',
}

export default function IntegrationDiscordPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [items, setItems] = useState<DiscordIntegration[]>([])
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<DiscordIntegration | null>(null)
  const [form, setForm] = useState<Required<DiscordIntegrationInput>>(blank())
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    try {
      const result = await discordService.list()
      if (result.data) setItems(result.data)
      else toast.error(result.error || t('discord.loadFailed', 'Could not load Discord integrations.'))
    } catch {
      toast.error(t('discord.loadFailed', 'Could not load Discord integrations.'))
    }
  }, [t])

  useEffect(() => { void load() }, [load])

  const openCreate = () => {
    setEditing(null)
    setForm(blank())
    setOpen(true)
  }

  const openEdit = (item: DiscordIntegration) => {
    setEditing(item)
    setForm({
      name: item.name, webhook_url: '', enabled: item.enabled, all_events: item.all_events,
      event_types: item.event_types, status_filter: item.status_filter,
      category_filter: item.category_filter, name_terms: item.name_terms,
    })
    setOpen(true)
  }

  const toggleType = (type: EventType) => {
    setForm((value) => ({
      ...value,
      event_types: value.event_types.includes(type) ? value.event_types.filter((item) => item !== type) : [...value.event_types, type],
      all_events: false,
    }))
  }

  const save = async () => {
    setSaving(true)
    try {
      const payload: DiscordIntegrationInput = { ...form }
      if (editing && !payload.webhook_url) delete payload.webhook_url
      const result = editing ? await discordService.update(editing.uuid, payload) : await discordService.create(payload)
      if (result.data) {
        toast.success(t('discord.saved', 'Discord destination saved.'))
        setOpen(false)
        void load()
      } else toast.error(result.error || t('discord.saveFailed', 'Could not save Discord destination.'))
    } catch {
      toast.error(t('discord.saveFailed', 'Could not save Discord destination.'))
    } finally {
      setSaving(false)
    }
  }

  const remove = async (item: DiscordIntegration) => {
    if (!window.confirm(t('discord.confirmDelete', 'Remove this Discord destination?'))) return
    try {
      const result = await discordService.remove(item.uuid)
      if (result.data !== undefined) {
        toast.success(t('discord.deleted', 'Discord destination removed.'))
        void load()
      } else toast.error(result.error || t('discord.deleteFailed', 'Could not remove Discord destination.'))
    } catch {
      toast.error(t('discord.deleteFailed', 'Could not remove Discord destination.'))
    }
  }

  const test = async (item: DiscordIntegration) => {
    try {
      const result = await discordService.test(item.uuid)
      if (result.data?.success) toast.success(t('discord.testSent', 'Test sent to Discord.'))
      else toast.error(result.error || t('discord.testFailed', 'Discord test failed.'))
    } catch {
      toast.error(t('discord.testFailed', 'Discord test failed.'))
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => { navigate('/integrations') }}><ArrowLeft className="h-5 w-5" /></Button>
          <div><h1 className="text-2xl font-bold tracking-tight">Discord</h1><p className="text-sm text-muted-foreground">{t('discord.subtitle', 'Receive Gardarr events and transfer reports in Discord.')}</p></div>
        </div>
        <Button onClick={openCreate}><Plus className="mr-2 h-4 w-4" />{t('discord.add', 'Add destination')}</Button>
      </div>

      {items.length === 0 ? (
        <Card><CardContent className="py-12 text-center text-muted-foreground"><Bell className="mx-auto mb-3 h-10 w-10" />{t('discord.empty', 'No Discord destinations configured yet.')}</CardContent></Card>
      ) : (
        <div className="grid gap-4">
          {items.map((item) => (
            <Card key={item.uuid} className="gap-0 py-0 transition-shadow hover:shadow-md">
              <CardContent className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between sm:p-5">
                <div className="flex min-w-0 items-start gap-3">
                  <div className="rounded-lg bg-primary/10 p-2 text-primary"><Bell className="h-5 w-5" /></div>
                  <div className="min-w-0">
                    <CardTitle className="truncate">{item.name}</CardTitle>
                    <CardDescription>{item.all_events ? t('discord.allEvents', 'All events') : t('discord.selectedEvents', '{{count}} selected events', { count: item.event_types.length })}</CardDescription>
                  </div>
                </div>
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
                  <Badge className="w-fit self-end sm:self-auto" variant={item.enabled ? 'secondary' : 'outline'}>{item.enabled ? t('discord.enabled', 'Enabled') : t('discord.disabled', 'Disabled')}</Badge>
                  <div className="grid grid-cols-2 gap-2 sm:flex">
                    <Button size="sm" variant="outline" className="w-full sm:w-auto" onClick={() => { void test(item) }}><Send className="mr-2 h-4 w-4" />{t('discord.test', 'Test')}</Button>
                    <Button size="sm" variant="outline" className="w-full sm:w-auto" onClick={() => { openEdit(item) }}>{t('common.edit', 'Edit')}</Button>
                    <Button size="sm" variant="ghost" className="col-span-2 w-full text-destructive sm:col-auto sm:w-auto" onClick={() => { void remove(item) }}><Trash2 className="mr-2 h-4 w-4" />{t('common.delete', 'Delete')}</Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="scrollbar max-h-[90dvh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editing ? t('discord.edit', 'Edit Discord destination') : t('discord.create', 'Add Discord destination')}</DialogTitle>
            <DialogDescription>{t('discord.secretHelp', 'The webhook URL is encrypted and is never shown again after saving.')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            <Card className="gap-0 py-0">
              <CardHeader className="px-4 pt-4 pb-0"><CardTitle className="text-base">{t('discord.destination', 'Destination')}</CardTitle></CardHeader>
              <CardContent className="space-y-4 p-4">
                <div className="space-y-2"><Label>{t('discord.name', 'Name')}</Label><Input value={form.name} onChange={(event) => { setForm((value) => ({ ...value, name: event.target.value })) }} /></div>
                <div className="space-y-2"><Label>{t('discord.webhookUrl', 'Discord webhook URL')}</Label><Input type="password" placeholder={editing ? t('discord.keepUrl', 'Leave blank to keep the current URL') : 'https://discord.com/api/webhooks/...'} value={form.webhook_url} onChange={(event) => { setForm((value) => ({ ...value, webhook_url: event.target.value })) }} /></div>
              </CardContent>
            </Card>
            <Card className="gap-0 py-0">
              <CardHeader className="px-4 pt-4 pb-0"><CardTitle className="text-base">{t('discord.delivery', 'Delivery')}</CardTitle></CardHeader>
              <CardContent className="space-y-4 p-4">
                <div className="grid grid-cols-2 gap-3">
                  <div className="flex items-center justify-between rounded-lg border p-3"><Label>{t('discord.enabled', 'Enabled')}</Label><Switch checked={form.enabled} onCheckedChange={(enabled) => { setForm((value) => ({ ...value, enabled })) }} /></div>
                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <Label>{t('discord.allEvents', 'All events')}</Label>
                    <Switch checked={form.all_events} onCheckedChange={(all_events) => { setForm((value) => ({ ...value, all_events })) }} />
                  </div>
                </div>
                {!form.all_events && (
                  <div className="space-y-2">
                    <Label>{t('discord.eventTypes', 'Event types')}</Label>
                    <div className="space-y-4">
                      {EVENT_TYPE_GROUPS.map(({ group, labelKey, defaultLabel }) => (
                        <fieldset key={group} className="space-y-2">
                          <legend className="text-sm font-medium text-muted-foreground/80">{t(labelKey, defaultLabel)}</legend>
                          <div className="grid gap-2 sm:grid-cols-2">
                            {EVENT_TYPES_BY_GROUP[group].map((type) => (
                              <EventTypeToggle key={type} type={type} pressed={form.event_types.includes(type)} onPressedChange={() => { toggleType(type) }} />
                            ))}
                          </div>
                        </fieldset>
                      ))}
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>
            <Card className="gap-0 py-0">
              <CardHeader className="px-4 pt-4 pb-0"><CardTitle className="text-base">{t('discord.filters', 'Filters')}</CardTitle></CardHeader>
              <CardContent className="space-y-4 p-4">
                <div className="grid gap-3 sm:grid-cols-3">
                  <div className="space-y-2"><Label>{t('discord.statusFilter', 'Statuses')}</Label><Input placeholder="UPLOADING, ERROR" value={form.status_filter.join(', ')} onChange={(event) => { setForm((value) => ({ ...value, status_filter: toTerms(event.target.value) })) }} /></div>
                  <div className="space-y-2"><Label>{t('discord.categoryFilter', 'Categories')}</Label><Input placeholder="movies, shows" value={form.category_filter.join(', ')} onChange={(event) => { setForm((value) => ({ ...value, category_filter: toTerms(event.target.value) })) }} /></div>
                  <div className="space-y-2"><Label>{t('discord.nameFilter', 'Name terms')}</Label><Input placeholder="1080p, linux" value={form.name_terms.join(', ')} onChange={(event) => { setForm((value) => ({ ...value, name_terms: toTerms(event.target.value) })) }} /></div>
                </div>
                <p className="text-xs text-muted-foreground">{t('discord.filterHelp', 'Torrent filters only apply to torrent events; reports and worker events still follow their event selection.')}</p>
              </CardContent>
            </Card>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setOpen(false) }}>{t('common.cancel', 'Cancel')}</Button>
            <Button onClick={() => { void save() }} disabled={saving}><CheckCircle2 className="mr-2 h-4 w-4" />{saving ? t('common.saving', 'Saving…') : t('common.save', 'Save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function EventTypeToggle({ type, pressed, onPressedChange }: { type: EventType; pressed: boolean; onPressedChange: () => void }) {
  const Icon = EVENT_TYPE_ICONS[type]

  return (
    <Toggle
      type="button"
      variant="outline"
      pressed={pressed}
      className="cursor-pointer justify-start gap-2 data-[state=on]:border-primary data-[state=on]:bg-primary/10 data-[state=on]:text-primary"
      onPressedChange={onPressedChange}
    >
      <Icon className={`h-4 w-4 shrink-0 ${EVENT_TYPE_ICON_COLORS[type]}`} aria-hidden="true" />
      {EVENT_TYPE_LABELS[type]}
    </Toggle>
  )
}
