import { useCallback, useEffect, useState } from 'react'
import { ArrowLeft, Bell, CheckCircle2, Plus, Send, Trash2 } from 'lucide-react'
import { useNavigate } from 'react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { EVENT_TYPE_OPTIONS, type EventType } from '@/constants/eventTypes'
import { discordService } from '@/services/discord'
import type { DiscordIntegration, DiscordIntegrationInput } from '@/types/discord'
import { toast } from 'sonner'

const blank = (): Required<DiscordIntegrationInput> => ({
  name: '', webhook_url: '', enabled: true, all_events: true, event_types: [], status_filter: [], category_filter: [], name_terms: [],
})

const toTerms = (value: string) => value.split(',').map((item) => item.trim()).filter(Boolean)

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
            <Card key={item.uuid}>
              <CardHeader className="pb-3">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <CardTitle className="flex items-center gap-2"><Bell className="h-5 w-5 text-primary" />{item.name}</CardTitle>
                    <CardDescription>{item.all_events ? t('discord.allEvents', 'All events') : t('discord.selectedEvents', '{{count}} selected events', { count: item.event_types.length })}</CardDescription>
                  </div>
                  <Badge variant={item.enabled ? 'secondary' : 'outline'}>{item.enabled ? t('discord.enabled', 'Enabled') : t('discord.disabled', 'Disabled')}</Badge>
                </div>
              </CardHeader>
              <CardContent className="flex flex-wrap gap-2">
                <Button size="sm" variant="outline" onClick={() => { void test(item) }}><Send className="mr-2 h-4 w-4" />{t('discord.test', 'Test')}</Button>
                <Button size="sm" variant="outline" onClick={() => { openEdit(item) }}>{t('common.edit', 'Edit')}</Button>
                <Button size="sm" variant="ghost" className="text-destructive" onClick={() => { void remove(item) }}><Trash2 className="mr-2 h-4 w-4" />{t('common.delete', 'Delete')}</Button>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-h-[90dvh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editing ? t('discord.edit', 'Edit Discord destination') : t('discord.create', 'Add Discord destination')}</DialogTitle>
            <DialogDescription>{t('discord.secretHelp', 'The webhook URL is encrypted and is never shown again after saving.')}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2"><Label>{t('discord.name', 'Name')}</Label><Input value={form.name} onChange={(event) => { setForm((value) => ({ ...value, name: event.target.value })) }} /></div>
            <div className="space-y-2"><Label>{t('discord.webhookUrl', 'Discord webhook URL')}</Label><Input type="password" placeholder={editing ? t('discord.keepUrl', 'Leave blank to keep the current URL') : 'https://discord.com/api/webhooks/...'} value={form.webhook_url} onChange={(event) => { setForm((value) => ({ ...value, webhook_url: event.target.value })) }} /></div>
            <div className="flex items-center justify-between rounded-lg border p-3"><Label>{t('discord.enabled', 'Enabled')}</Label><Switch checked={form.enabled} onCheckedChange={(enabled) => { setForm((value) => ({ ...value, enabled })) }} /></div>
            <div className="flex items-center justify-between rounded-lg border p-3">
              <div><Label>{t('discord.allEvents', 'All events')}</Label><p className="text-xs text-muted-foreground">{t('discord.allEventsHelp', 'Includes future Gardarr event types.')}</p></div>
              <Switch checked={form.all_events} onCheckedChange={(all_events) => { setForm((value) => ({ ...value, all_events })) }} />
            </div>
            {!form.all_events && (
              <div className="space-y-2">
                <Label>{t('discord.eventTypes', 'Event types')}</Label>
                <div className="grid gap-2 sm:grid-cols-2">
                  {EVENT_TYPE_OPTIONS.map(({ type, label }) => (
                    <label key={type} className="flex items-center gap-2 rounded border p-2 text-sm">
                      <input type="checkbox" checked={form.event_types.includes(type)} onChange={() => { toggleType(type) }} />
                      {label}
                    </label>
                  ))}
                </div>
              </div>
            )}
            <div className="grid gap-3 sm:grid-cols-3">
              <div className="space-y-2"><Label>{t('discord.statusFilter', 'Statuses')}</Label><Input placeholder="UPLOADING, ERROR" value={form.status_filter.join(', ')} onChange={(event) => { setForm((value) => ({ ...value, status_filter: toTerms(event.target.value) })) }} /></div>
              <div className="space-y-2"><Label>{t('discord.categoryFilter', 'Categories')}</Label><Input placeholder="movies, shows" value={form.category_filter.join(', ')} onChange={(event) => { setForm((value) => ({ ...value, category_filter: toTerms(event.target.value) })) }} /></div>
              <div className="space-y-2"><Label>{t('discord.nameFilter', 'Name terms')}</Label><Input placeholder="1080p, linux" value={form.name_terms.join(', ')} onChange={(event) => { setForm((value) => ({ ...value, name_terms: toTerms(event.target.value) })) }} /></div>
            </div>
            <p className="text-xs text-muted-foreground">{t('discord.filterHelp', 'Torrent filters only apply to torrent events; reports and worker events still follow their event selection.')}</p>
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
