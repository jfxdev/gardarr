import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ReportsPage from '@/Reports'

const { getSettings, getLatest, getCurrent, updateSettings, captureSnapshot, sendDiscord, toast } = vi.hoisted(() => ({ getSettings: vi.fn(), getLatest: vi.fn(), getCurrent: vi.fn(), updateSettings: vi.fn(), captureSnapshot: vi.fn(), sendDiscord: vi.fn(), toast: { error: vi.fn(), success: vi.fn() } }))
const { getLanguage } = vi.hoisted(() => ({ getLanguage: vi.fn() }))

vi.mock('@/services/transferReports', () => ({ transferReportsService: { getSettings, getLatest, getCurrent, updateSettings, captureSnapshot, sendDiscord } }))
vi.mock('@/services/settings', () => ({ settingsService: { getLanguage } }))
vi.mock('sonner', () => ({ toast }))

const settings = { enabled: true, snapshots_per_day: 4, daily_report_time: '00:05', weekly_report_day: 1, weekly_report_time: '00:10', top_n: 10 }
const report = { uuid: 'daily', period_type: 'daily', period_start: '2026-09-01T03:00:00Z', period_end: '2026-09-02T03:00:00Z', timezone: 'America/Sao_Paulo', generated_at: '2026-09-02T00:05:00Z', coverage: 'partial' as const, unavailable_workers: ['worker-1'], upload: [{ rank: 1, name: 'Alpha', hash: 'a', bytes: 2048 }], download: [] }

describe('ReportsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getSettings.mockResolvedValue({ data: settings })
    getLatest.mockResolvedValue({ data: { daily: report, weekly: null } })
    getCurrent.mockResolvedValue({ data: { daily: { ...report, uuid: 'current-daily', coverage: 'complete', upload: [1, 2, 3, 4, 5].map((rank) => ({ rank, name: `Current ${rank}`, hash: `current-${rank}`, bytes: rank * 1024 })) }, weekly: { ...report, uuid: 'current-weekly', period_type: 'weekly', coverage: 'complete' } } })
    updateSettings.mockResolvedValue({ data: { ...settings, snapshots_per_day: 6 } })
    captureSnapshot.mockResolvedValue({ data: null })
    sendDiscord.mockResolvedValue({ data: { delivered: 2 } })
    getLanguage.mockResolvedValue({ data: { default_language: 'en-US' } })
  })

  it('loads, renders partial rankings and persists changed settings', async () => {
    render(<ReportsPage />)
    expect((await screen.findAllByText('Alpha')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('Some workers were unavailable.')).not.toHaveLength(0)
    expect(screen.getAllByText('No download movement available for this period.')).not.toHaveLength(0)
    fireEvent.click(screen.getByRole('button', { name: 'Configure' }))
    expect(screen.getByRole('dialog')).toHaveTextContent('Schedule')
    fireEvent.change(screen.getByDisplayValue('4'), { target: { value: '6' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save schedule' }))
    await waitFor(() => expect(updateSettings).toHaveBeenCalledWith(expect.objectContaining({ snapshots_per_day: 6 })))
    expect(toast.success).toHaveBeenCalled()
  })

  it('shows empty reports and a load error without blocking settings', async () => {
    getSettings.mockResolvedValue({ data: settings })
    getLatest.mockResolvedValue({ error: 'offline' })
    render(<ReportsPage />)
    expect(await screen.findAllByText('No report generated yet.')).toHaveLength(2)
    expect(toast.error).toHaveBeenCalledWith('Could not load transfer reports.')
  })

  it('formats report dates in the report timezone', async () => {
    const formatter = vi.spyOn(Intl, 'DateTimeFormat')
    render(<ReportsPage />)
    await screen.findAllByText('Alpha')
    expect(formatter.mock.calls.some(([, options]) => options?.timeZone === 'America/Sao_Paulo')).toBe(true)
    formatter.mockRestore()
  })

  it('previews the Discord embed for each available report', async () => {
    const user = userEvent.setup()
    render(<ReportsPage />)
    await user.click(await screen.findByRole('button', { name: 'Notification Preview' }))
    expect(screen.getByRole('dialog')).toHaveTextContent('Notification Preview')
    await user.click(screen.getByRole('tab', { name: 'Daily report' }))
    expect(screen.getByRole('tab', { name: 'Daily report' })).toHaveAttribute('data-state', 'active')
    const preview = await screen.findByTestId('discord-preview-daily')
    expect(preview).toHaveTextContent('Gardarr · Daily transfer report · 09/01/2026')
    expect(preview).toHaveTextContent('🥇 Alpha — 2.0 KB')
    expect(preview).not.toHaveTextContent('Period')
    expect(preview).not.toHaveTextContent('Coverage')
  })

  it('uses the server language used by Discord for the preview', async () => {
    const user = userEvent.setup()
    getLanguage.mockResolvedValue({ data: { default_language: 'pt-BR' } })
    render(<ReportsPage />)
    await user.click(await screen.findByRole('button', { name: 'Notification Preview' }))
    await user.click(screen.getByRole('tab', { name: 'Daily report' }))
    expect(await screen.findByTestId('discord-preview-daily')).toHaveTextContent('Gardarr · Relatório diário · 09/01/2026')
  })

  it('sends the selected notification to Discord', async () => {
    const user = userEvent.setup()
    render(<ReportsPage />)
    await user.click(await screen.findByRole('button', { name: 'Notification Preview' }))
    await user.click(screen.getByRole('button', { name: 'Send to Discord' }))
    await waitFor(() => expect(sendDiscord).toHaveBeenCalledWith('current', 'daily'))
    expect(toast.success).toHaveBeenCalled()
  })

  it('explains when the backend does not confirm a Discord delivery', async () => {
    const user = userEvent.setup()
    sendDiscord.mockResolvedValue({ data: null, status: 204 })
    render(<ReportsPage />)
    await user.click(await screen.findByRole('button', { name: 'Notification Preview' }))
    await user.click(screen.getByRole('button', { name: 'Send to Discord' }))
    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Discord did not confirm delivery. Reload this page and try again. (HTTP 204)'))
  })

  it('shows current rankings without requiring Discord', async () => {
    render(<ReportsPage />)
    expect(await screen.findByText('Current snapshot rankings')).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Daily' })).toHaveAttribute('data-state', 'active')
    expect(screen.getByRole('tab', { name: 'Weekly' })).toBeInTheDocument()
    expect(screen.getByText('Live values calculated from stored snapshots. Discord is not required.')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Notification Preview' }))
    expect(screen.getByTestId('discord-preview-current-daily')).toHaveTextContent('Top transfer activity so far today.')
    expect(screen.getByTestId('discord-preview-current-daily')).toHaveTextContent('🥇 Current 1 — 1.0 KB')
    expect(screen.getByTestId('current-ranking-row-1')).toHaveClass('[&>td]:bg-primary/25')
    expect(screen.getByTestId('current-ranking-row-5')).toHaveClass('[&>td]:bg-primary/5')
  })

  it('captures a snapshot and displays ranking tables', async () => {
    render(<ReportsPage />)
    await screen.findAllByText('Alpha')
    expect(screen.getAllByRole('table', { name: 'Upload' })).not.toHaveLength(0)
    fireEvent.click(screen.getByRole('button', { name: 'Capture snapshot now' }))
    await waitFor(() => expect(captureSnapshot).toHaveBeenCalledOnce())
    expect(toast.success).toHaveBeenCalledWith('Transfer snapshot captured.')
  })
})
