import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ReportsPage from '@/Reports'

const { getSettings, getLatest, updateSettings, toast } = vi.hoisted(() => ({ getSettings: vi.fn(), getLatest: vi.fn(), updateSettings: vi.fn(), toast: { error: vi.fn(), success: vi.fn() } }))

vi.mock('@/services/transferReports', () => ({ transferReportsService: { getSettings, getLatest, updateSettings } }))
vi.mock('sonner', () => ({ toast }))

const settings = { enabled: true, snapshots_per_day: 4, daily_report_time: '00:05', weekly_report_day: 1, weekly_report_time: '00:10', top_n: 10 }
const report = { uuid: 'daily', period_type: 'daily', period_start: '2026-09-01T00:00:00Z', period_end: '2026-09-02T00:00:00Z', timezone: 'America/Sao_Paulo', generated_at: '2026-09-02T00:05:00Z', coverage: 'partial' as const, unavailable_workers: ['worker-1'], upload: [{ rank: 1, name: 'Alpha', hash: 'a', bytes: 2048 }], download: [] }

describe('ReportsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getSettings.mockResolvedValue({ data: settings })
    getLatest.mockResolvedValue({ data: { daily: report, weekly: null } })
    updateSettings.mockResolvedValue({ data: { ...settings, snapshots_per_day: 6 } })
  })

  it('loads, renders partial rankings and persists changed settings', async () => {
    render(<ReportsPage />)
    expect(await screen.findByText('Alpha')).toBeInTheDocument()
    expect(screen.getByText('Some workers were unavailable.')).toBeInTheDocument()
    expect(screen.getByText('No download movement available for this period.')).toBeInTheDocument()
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
})
