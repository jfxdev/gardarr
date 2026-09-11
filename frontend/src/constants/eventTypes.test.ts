import { describe, expect, it } from 'vitest'
import { EVENT_TYPES_BY_GROUP, REPORT_EVENT_TYPES, TORRENT_EVENT_TYPES } from './eventTypes'

describe('event type groups', () => {
  it('keeps transfer reports out of the torrent group', () => {
    for (const type of REPORT_EVENT_TYPES) {
      expect(TORRENT_EVENT_TYPES).not.toContain(type)
    }
    expect(EVENT_TYPES_BY_GROUP.report).toEqual(REPORT_EVENT_TYPES)
  })
})
