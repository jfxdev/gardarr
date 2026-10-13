import { api } from '../lib/api';
import type { ApiResponse } from '../lib/api';

export type BulkTaskAction =
  | 'stop'
  | 'start'
  | 'force_resume'
  | 'force_recheck'
  | 'force_reannounce'
  | 'set_category'
  | 'add_tags'
  | 'delete'
  | 'add_tracker'
  | 'remove_tracker'
  | 'queue_top'
  | 'queue_up'
  | 'queue_down'
  | 'queue_bottom';

export type QueuePriorityAction = 'top' | 'up' | 'down' | 'bottom';

export interface BulkTaskItem {
  worker_id: string;
  hash: string;
}

export interface BulkTaskActionResult {
  succeeded: number;
  failed: Record<string, string>; // workerUUID -> error message
}

export interface BulkTaskActionOptions {
  category?: string;
  tags?: string[];
  purge?: boolean;
  trackers?: string[];
}
import type {
  Task,
  CreateTaskRequest,
  TaskMagnetLink,
  TaskFile,
  TaskTracker,
  TaskPeer,
  TaskLimits,
  TaskListResponse
} from '../types/torrent';

/**
 * Serviço para comunicação com a API v1/tasks do backend
 */
export class TorrentService {
  private readonly baseEndpoint = '/workers/tasks';

  /**
   * Lista todos os torrents/tasks
   */
  async listTasks(): Promise<ApiResponse<TaskListResponse>> {
    return api.get<TaskListResponse>(this.baseEndpoint);
  }

  /**
   * Lista tasks de um worker específico
   */
  async listWorkerTasks(workerId: string): Promise<ApiResponse<Task[]>> {
    return api.get<Task[]>(`/worker/${workerId}/tasks`);
  }

  /**
   * Cria uma nova task/torrent para um worker específico
   */
  async createTask(workerId: string, taskData: CreateTaskRequest): Promise<ApiResponse<Task>> {
    return api.post<Task>(`/worker/${workerId}/task`, taskData);
  }

  /**
   * Cria uma nova task/torrent a partir de um arquivo .torrent
   */
  async createTaskFromFile(
    workerId: string,
    file: File,
    options: { category: string; directory?: string; tags?: string[] }
  ): Promise<ApiResponse<Task>> {
    const formData = new FormData();
    formData.append('torrent', file);
    formData.append('category', options.category);
    if (options.directory) {
      formData.append('directory', options.directory);
    }
    for (const tag of options.tags || []) {
      formData.append('tags', tag);
    }
    return api.post<Task>(`/worker/${workerId}/task/file`, formData);
  }

  /**
   * Remove uma task/torrent
   */
  async deleteTask(workerId: string, taskId: string, purge: boolean = false): Promise<ApiResponse<null>> {
    const endpoint = `/worker/${workerId}/task/${taskId}${purge ? '?purge=true' : ''}`;
    return api.delete<null>(endpoint);
  }

  /**
   * Pausa uma task/torrent (stop)
   */
  async pauseTask(workerId: string, taskId: string): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/stop`);
  }

  /**
   * Retoma uma task/torrent (start)
   */
  async resumeTask(workerId: string, taskId: string): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/start`);
  }

  /**
   * Força download de uma task/torrent
   */
  async forceDownloadTask(workerId: string, taskId: string): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/force_download`);
  }

  /**
   * Força reannounce de uma task/torrent
   */
  async forceReannounceTask(workerId: string, taskId: string): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/force_reannounce`);
  }

  /**
   * Força recheck de uma task/torrent
   */
  async forceRecheckTask(workerId: string, taskId: string): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/force_recheck`);
  }

  /**
   * Ativa/desativa super seeding de uma task/torrent
   */
  async toggleSuperSeeding(workerId: string, taskId: string, enabled: boolean): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/super_seeding`, { enabled });
  }

  /**
   * Renomeia uma task/torrent
   */
  async renameTask(workerId: string, taskId: string, newName: string): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/rename`, { new_name: newName });
  }

  /**
   * Altera o caminho (location) de uma task/torrent
   */
  async setTaskLocation(workerId: string, taskId: string, location: string): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/location`, { location });
  }

  /**
   * Lista os arquivos de uma task/torrent específica
   */
  async listTaskFiles(workerId: string, taskId: string): Promise<ApiResponse<TaskFile[]>> {
    return api.get<TaskFile[]>(`/worker/${workerId}/task/${taskId}/files`);
  }

  /**
   * Lista os trackers de uma task/torrent específica
   */
  async listTaskTrackers(workerId: string, taskId: string): Promise<ApiResponse<TaskTracker[]>> {
    return api.get<TaskTracker[]>(`/worker/${workerId}/task/${taskId}/trackers`);
  }

  /**
   * Adiciona trackers a uma task/torrent
   */
  async addTaskTrackers(workerId: string, taskId: string, urls: string[]): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/trackers`, { urls });
  }

  /**
   * Remove trackers de uma task/torrent
   */
  async removeTaskTrackers(workerId: string, taskId: string, urls: string[]): Promise<ApiResponse<null>> {
    const query = urls.map((url) => `url=${encodeURIComponent(url)}`).join('&');
    return api.delete<null>(`/worker/${workerId}/task/${taskId}/trackers?${query}`);
  }

  /**
   * Edita a URL de um tracker de uma task/torrent
   */
  async editTaskTracker(
    workerId: string,
    taskId: string,
    origUrl: string,
    newUrl: string
  ): Promise<ApiResponse<null>> {
    return api.put<null>(`/worker/${workerId}/task/${taskId}/trackers`, {
      orig_url: origUrl,
      new_url: newUrl,
    });
  }

  /**
   * Atualiza as tags de uma task/torrent
   */
  async updateTaskTags(workerId: string, taskId: string, tags: string[]): Promise<ApiResponse<null>> {
    return api.put<null>(`/worker/${workerId}/task/${taskId}/tags`, { tags });
  }

  /**
   * Atualiza a categoria de uma task/torrent
   */
  async updateTaskCategory(workerId: string, taskId: string, category: string): Promise<ApiResponse<null>> {
    return api.put<null>(`/worker/${workerId}/task/${taskId}/category`, { category });
  }

  /**
   * Aplica uma ação em lote a várias tasks (possivelmente de vários workers).
   * O backend agrupa os hashes por worker e usa o batch nativo do qBittorrent.
   */
  async bulkTaskAction(
    action: BulkTaskAction,
    items: BulkTaskItem[],
    options: BulkTaskActionOptions = {}
  ): Promise<ApiResponse<BulkTaskActionResult>> {
    return api.post<BulkTaskActionResult>(`${this.baseEndpoint}/bulk`, {
      action,
      items,
      ...options,
    });
  }

  /**
   * Obtém os limites de uma task/torrent específica
   */
  async getTaskLimits(workerId: string, taskId: string): Promise<ApiResponse<TaskLimits>> {
    return api.get<TaskLimits>(`/worker/${workerId}/task/${taskId}/limits`);
  }

  /**
   * Define o limite de download de uma task/torrent
   */
  async setTaskDownloadLimit(workerId: string, taskId: string, limit: number): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/limit_download_rate`, { limit });
  }

  /**
   * Define o limite de upload de uma task/torrent
   */
  async setTaskUploadLimit(workerId: string, taskId: string, limit: number): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/task/${taskId}/limit_upload_rate`, { limit });
  }

  /**
   * Define o limite de compartilhamento (share limit) de uma task/torrent
   */
  async setTaskShareLimit(
    workerId: string,
    taskId: string,
    ratioLimit: number,
    seedingTimeLimit: number,
    inactiveSeedingTimeLimit: number
  ): Promise<ApiResponse<null>> {
    return api.put<null>(`/worker/${workerId}/task/${taskId}/share_limit`, {
      ratio_limit: ratioLimit,
      seeding_time_limit: seedingTimeLimit,
      inactive_seeding_time_limit: inactiveSeedingTimeLimit
    });
  }

  /**
   * Define a prioridade de download de um ou mais arquivos de uma task/torrent.
   * priority: 0 = não baixar, 1 = normal, 6 = alta, 7 = máxima
   */
  async setTaskFilePriority(
    workerId: string,
    taskId: string,
    indexes: number[],
    priority: number
  ): Promise<ApiResponse<null>> {
    return api.put<null>(`/worker/${workerId}/task/${taskId}/files/priority`, {
      indexes,
      priority,
    });
  }

  /**
   * Reordena um torrent na fila de download do qBittorrent
   */
  async setTaskQueuePriority(
    workerId: string,
    taskId: string,
    action: QueuePriorityAction
  ): Promise<ApiResponse<null>> {
    return api.put<null>(`/worker/${workerId}/task/${taskId}/priority`, { action });
  }

  /**
   * Lista os peers conectados de uma task/torrent específica
   */
  async listTaskPeers(workerId: string, taskId: string): Promise<ApiResponse<TaskPeer[]>> {
    return api.get<TaskPeer[]>(`/worker/${workerId}/task/${taskId}/peers`);
  }

  /**
   * Bane um peer em todo o worker (instance-wide, não apenas neste torrent)
   */
  async banPeer(workerId: string, ip: string, port: number): Promise<ApiResponse<null>> {
    return api.post<null>(`/worker/${workerId}/peers/ban`, { ip, port });
  }
}

// Instância padrão do serviço
export const torrentService = new TorrentService();

export interface CreateTorrentSubmission {
  workerId: string;
  taskData: CreateTaskRequest;
}

/**
 * Converte um magnet URI para TaskMagnetLink
 * @param magnetUri - O magnet URI a ser convertido
 * @returns TaskMagnetLink ou null se o URI for inválido
 */
export function convertMagnetUriToTaskMagnetLink(magnetUri: string): TaskMagnetLink | null {
  try {
    const url = new URL(magnetUri);

    if (url.protocol !== 'magnet:') {
      return null;
    }

    // Extrai o hash do torrent (xt parameter)
    const xtMatch = url.searchParams.get('xt')?.match(/^urn:btih:([a-f0-9]{40}|[a-z2-7]{32})$/i);
    let hash = xtMatch ? xtMatch[1].toLowerCase() : '';
    if (hash.length === 32) {
      // Base32 and hexadecimal magnets must share the same pending-task key.
      const alphabet = 'abcdefghijklmnopqrstuvwxyz234567';
      let buffer = 0;
      let bits = 0;
      let hex = '';
      for (const character of hash) {
        buffer = (buffer << 5) | alphabet.indexOf(character);
        bits += 5;
        if (bits >= 8) {
          bits -= 8;
          hex += ((buffer >>> bits) & 0xff).toString(16).padStart(2, '0');
          buffer &= (1 << bits) - 1;
        }
      }
      hash = hex;
    }

    // Extrai o nome do torrent (dn parameter)
    const displayName = url.searchParams.get('dn') || '';

    // Extrai os trackers (tr parameters)
    const trackers: string[] = [];
    url.searchParams.forEach((value, key) => {
      if (key === 'tr') {
        trackers.push(value);
      }
    });

    // Extrai o tamanho exato (xl parameter)
    const exactLength = url.searchParams.get('xl') || '';

    // Extrai a fonte exata (xs parameter)
    const exactSource = url.searchParams.get('xs') || '';

    return {
      hash,
      display_name: displayName,
      trackers,
      exact_length: exactLength,
      exact_source: exactSource
    };
  } catch (error) {
    console.error('Error parsing magnet URI:', error);
    return null;
  }
}
