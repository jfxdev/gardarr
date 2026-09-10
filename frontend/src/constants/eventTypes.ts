/**
 * Event types available in the system
 * These correspond to the event types defined in the backend
 */

export type EventType =
  | 'torrent.state_change'
  | 'torrent.added'
  | 'torrent.removed'
  | 'torrent.completed'
  | 'bandwidth.schedule_applied'
  | 'worker.offline'
  | 'worker.recovered'
  | 'report.transfer.daily'
  | 'report.transfer.weekly';

export const EVENT_TYPES: readonly EventType[] = [
  'torrent.state_change',
  'torrent.added',
  'torrent.removed',
  'torrent.completed',
  'bandwidth.schedule_applied',
  'worker.offline',
  'worker.recovered',
  'report.transfer.daily',
  'report.transfer.weekly',
] as const;

export const EVENT_TYPE_LABELS: Record<EventType, string> = {
  'torrent.state_change': 'State Change',
  'torrent.added': 'Added',
  'torrent.removed': 'Removed',
  'torrent.completed': 'Completed',
  'bandwidth.schedule_applied': 'Bandwidth schedule applied',
  'worker.offline': 'Worker offline',
  'worker.recovered': 'Worker recovered',
  'report.transfer.daily': 'Daily transfer report',
  'report.transfer.weekly': 'Weekly transfer report',
};

export const EVENT_TYPE_DESCRIPTIONS: Record<EventType, string> = {
  'torrent.state_change': 'Triggered when a torrent changes its state',
  'torrent.added': 'Triggered when a new torrent is added',
  'torrent.removed': 'Triggered when a torrent is removed',
  'torrent.completed': 'Triggered when a torrent completes downloading',
  'bandwidth.schedule_applied': 'Triggered when a scheduled global speed limit is applied',
  'worker.offline': 'Triggered when a worker is confirmed unreachable',
  'worker.recovered': 'Triggered when a previously offline worker becomes reachable again',
  'report.transfer.daily': 'Generated daily upload and download ranking',
  'report.transfer.weekly': 'Generated weekly upload and download ranking',
};

/**
 * Event group a table/filter is scoped to. Matches the backend `group` query
 * param, except 'all' which is frontend-only shorthand for "no group filter"
 * (a combined, mixed-type feed - e.g. the Integrations page's events drawer).
 */
export type EventGroup = 'torrent' | 'worker' | 'schedule' | 'report' | 'all';

export const WORKER_EVENT_TYPES: readonly EventType[] = ['worker.offline', 'worker.recovered'];

export const SCHEDULE_EVENT_TYPES: readonly EventType[] = ['bandwidth.schedule_applied'];
export const REPORT_EVENT_TYPES: readonly EventType[] = ['report.transfer.daily', 'report.transfer.weekly'];

export const TORRENT_EVENT_TYPES: readonly EventType[] = EVENT_TYPES.filter(
  (type) => !WORKER_EVENT_TYPES.includes(type) && !SCHEDULE_EVENT_TYPES.includes(type)
);

export const EVENT_TYPES_BY_GROUP: Record<EventGroup, readonly EventType[]> = {
  torrent: TORRENT_EVENT_TYPES,
  worker: WORKER_EVENT_TYPES,
  schedule: SCHEDULE_EVENT_TYPES,
  report: REPORT_EVENT_TYPES,
  all: EVENT_TYPES,
};
