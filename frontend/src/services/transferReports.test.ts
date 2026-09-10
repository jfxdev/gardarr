import { describe, expect, it, vi } from 'vitest'

const { api } = vi.hoisted(() => ({ api: { get: vi.fn(), put: vi.fn() } }))
vi.mock('@/lib/api', () => ({ api }))

import { transferReportsService } from '@/services/transferReports'

describe('transferReportsService', () => {
  it('uses the transfer report endpoints', () => {
    const settings = { enabled: true, snapshots_per_day: 4, daily_report_time: '00:05', weekly_report_day: 1, weekly_report_time: '00:10', top_n: 10 }
    transferReportsService.getSettings()
    transferReportsService.getLatest()
    transferReportsService.updateSettings(settings)
    expect(api.get).toHaveBeenNthCalledWith(1, '/reports/transfer/settings')
    expect(api.get).toHaveBeenNthCalledWith(2, '/reports/transfer/latest')
    expect(api.put).toHaveBeenCalledWith('/reports/transfer/settings', settings)
  })
})
