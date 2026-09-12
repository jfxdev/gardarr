import { describe, expect, it, vi } from 'vitest'

const { api } = vi.hoisted(() => ({ api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }))
vi.mock('@/lib/api', () => ({ api }))

import { discordService } from '@/services/discord'

describe('discordService', () => {
  it('uses only the Discord integration endpoints', () => {
    const input = { name: 'Alerts', all_events: true }
    discordService.list()
    discordService.create(input)
    discordService.update('one', input)
    discordService.remove('one')
    discordService.test('one')
    discordService.history('one')
    expect(api.get).toHaveBeenNthCalledWith(1, '/integrations/discord')
    expect(api.get).toHaveBeenNthCalledWith(2, '/integrations/discord/one/history')
    expect(api.post).toHaveBeenNthCalledWith(1, '/integrations/discord', input)
    expect(api.post).toHaveBeenNthCalledWith(2, '/integrations/discord/one/test')
    expect(api.put).toHaveBeenCalledWith('/integrations/discord/one', input)
    expect(api.delete).toHaveBeenCalledWith('/integrations/discord/one')
  })
})
