import { api } from '../lib/api';
import type { ApiResponse } from '../lib/api';
import type {
  Worker,
  WorkerOverview,
  CreateWorkerRequest,
  UpdateWorkerRequest,
  Version,
  TaskStats,
  WorkerPreferences,
  SpeedLimitsSchema,
  ActiveLimitsSchema,
  LogEntry,
  BandwidthSchedule,
  BandwidthScheduleInput,
  BandwidthSchedulePreview
} from '../types/worker';

// Worker routes remain the public contract, but each worker now represents a
// direct qBittorrent connection managed by the central Gardarr server.
export class WorkerService {
  private readonly baseEndpoint = '/workers/'; 

  /**
   * Lists all workers
   */
  async listWorkers(): Promise<ApiResponse<Worker[]>> {
    return api.get<Worker[]>(`/workers/`);
  }

  /**
   * Lists all workers straight from the database, skipping the live
   * qBittorrent connectivity/instance check (fast path for an initial
   * render). Callers load status separately via getWorker() or the shared
   * overview used by the Workers page.
   */
  async listWorkersBasic(signal?: AbortSignal): Promise<ApiResponse<Worker[]>> {
    return api.get<Worker[]>(`/workers/?live=false`, { signal });
  }

  /**
   * Gets a specific worker by ID
   */
  async getWorker(workerId: string): Promise<ApiResponse<Worker>> {
    return api.get<Worker>(`/worker/${workerId}/`);
  }

  async getWorkerOverview(workerId: string, signal?: AbortSignal): Promise<ApiResponse<WorkerOverview>> {
    return api.get<WorkerOverview>(`/worker/${workerId}/overview`, { signal });
  }

  /**
   * Creates a new worker
   */
  async createWorker(workerData: CreateWorkerRequest): Promise<ApiResponse<Worker>> {
    return api.post<Worker>('/worker', workerData);
  }

  /**
   * Updates an existing worker
   */
  async updateWorker(workerId: string, workerData: UpdateWorkerRequest): Promise<ApiResponse<Worker>> {
    return api.put<Worker>(`/worker/${workerId}`, workerData);
  }

  /**
   * Deletes a worker
   */
  async deleteWorker(workerId: string): Promise<ApiResponse<null>> {
    return api.delete<null>(`/worker/${workerId}`);
  }

  /**
   * Lists all tasks from all workers
   */
  async listWorkersTasks(): Promise<ApiResponse<unknown[]>> {
    return api.get<unknown[]>(`${this.baseEndpoint}/tasks/`);
  }

  /**
   * Lists tasks for a specific worker
   */
  async listWorkerTasks(workerId: string): Promise<ApiResponse<unknown[]>> {
    return api.get<unknown[]>(`/worker/${workerId}/tasks/`);
  }

  /**
   * Creates a task for a specific worker
   */
  async createWorkerTask(workerId: string, taskData: unknown): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task`, taskData);
  }

  /**
   * Gets the qBittorrent version information for a specific worker connection.
   */
  async getWorkerVersion(workerId: string): Promise<ApiResponse<Version>> {
    return api.get<Version>(`/worker/${workerId}/version/`);
  }

  /**
   * Gets the task statistics for a specific worker
   */
  async getWorkerTaskStats(workerId: string): Promise<ApiResponse<TaskStats>> {
    return api.get<TaskStats>(`/worker/${workerId}/tasks/stats/`);
  }

  /**
   * Gets the preferences for a specific worker
   */
  async getWorkerPreferences(workerId: string): Promise<ApiResponse<WorkerPreferences>> {
    return api.get<WorkerPreferences>(`/worker/${workerId}/preferences`);
  }

  /**
   * Sets the speed limits for a specific worker
   */
  async setWorkerSpeedLimits(workerId: string, limits: SpeedLimitsSchema): Promise<ApiResponse<{ message: string }>> {
    return api.post<{ message: string }>(`/worker/${workerId}/speed/limits`, limits);
  }

  /**
   * Sets the active torrent limits for a specific worker
   */
  async setWorkerActiveLimits(workerId: string, limits: ActiveLimitsSchema): Promise<ApiResponse<{ message: string }>> {
    return api.post<{ message: string }>(`/worker/${workerId}/active/limits`, limits);
  }

  async listSchedules(workerId: string): Promise<ApiResponse<BandwidthSchedule[]>> {
    return api.get<BandwidthSchedule[]>(`/worker/${workerId}/schedules`);
  }

  async getSchedulePreview(workerId: string): Promise<ApiResponse<BandwidthSchedulePreview>> {
    return api.get<BandwidthSchedulePreview>(`/worker/${workerId}/schedules/preview`);
  }

  async createSchedule(workerId: string, schedule: BandwidthScheduleInput): Promise<ApiResponse<BandwidthSchedule>> {
    return api.post<BandwidthSchedule>(`/worker/${workerId}/schedules`, schedule);
  }

  async updateSchedule(workerId: string, scheduleId: string, schedule: BandwidthScheduleInput): Promise<ApiResponse<BandwidthSchedule>> {
    return api.put<BandwidthSchedule>(`/worker/${workerId}/schedules/${scheduleId}`, schedule);
  }

  async deleteSchedule(workerId: string, scheduleId: string): Promise<ApiResponse<null>> {
    return api.delete<null>(`/worker/${workerId}/schedules/${scheduleId}`);
  }

  async reorderSchedules(workerId: string, scheduleIds: string[]): Promise<ApiResponse<BandwidthSchedule[]>> {
    return api.put<BandwidthSchedule[]>(`/worker/${workerId}/schedules/order`, { schedule_ids: scheduleIds });
  }

  /**
   * Fetches logs for a specific worker
   */
  async getWorkerLogs(
    workerId: string,
    options: {
      normal?: boolean;
      info?: boolean;
      warning?: boolean;
      critical?: boolean;
      lastKnownId?: number;
    } = {}
  ): Promise<ApiResponse<LogEntry[]>> {
    const params = new URLSearchParams();
    if (options.normal !== undefined) params.append('normal', options.normal.toString());
    if (options.info !== undefined) params.append('info', options.info.toString());
    if (options.warning !== undefined) params.append('warning', options.warning.toString());
    if (options.critical !== undefined) params.append('critical', options.critical.toString());
    if (options.lastKnownId !== undefined) params.append('last_known_id', options.lastKnownId.toString());

    return api.get<LogEntry[]>(`/worker/${workerId}/logs?${params.toString()}`);
  }
}

// Default service instance
export const workerService = new WorkerService();
