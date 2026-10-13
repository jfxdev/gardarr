import { describe, it, expect, vi, beforeEach } from 'vitest';
import { TorrentService, convertMagnetUriToTaskMagnetLink } from './torrents';
import { api } from '../lib/api';
import type { Task, CreateTaskRequest } from '../types/torrent';

// Mock the api module
vi.mock('../lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}));

describe('convertMagnetUriToTaskMagnetLink', () => {
  const hexHash = '5a0cdacbd5571efcdd5e279fcb0e913b6c36cb36';
  const base32Hash = 'lignvs6vk4ppzxk6e6p4wdurhnwdnszw';

  it.each([base32Hash, base32Hash.toUpperCase(), hexHash, hexHash.toUpperCase()])(
    'normalizes %s to the same hexadecimal torrent hash', (hash) => {
      expect(convertMagnetUriToTaskMagnetLink(`magnet:?xt=urn:btih:${hash}`)?.hash).toBe(hexHash);
    },
  );

  it('parses a synthetic Base32 magnet and preserves its metadata', () => {
    const uri = `magnet:?xt=urn:btih:${base32Hash}&dn=Gardarr Test Fixture.2026.1080p.WEB-DL.x264&tr=udp://tracker.example.invalid:1337/announce`;
    expect(convertMagnetUriToTaskMagnetLink(uri)).toEqual({
      hash: hexHash,
      display_name: 'Gardarr Test Fixture.2026.1080p.WEB-DL.x264',
      trackers: ['udp://tracker.example.invalid:1337/announce'],
      exact_length: '',
      exact_source: '',
    });
  });

  it.each([base32Hash.slice(1), `${base32Hash}a`, `${hexHash}a`, '0'.repeat(32), `prefix-urn:btih:${hexHash}`])(
    'does not accept a malformed hash %s', (hash) => {
      expect(convertMagnetUriToTaskMagnetLink(`magnet:?xt=urn:btih:${hash}`)?.hash).toBe('');
    },
  );

  it('rejects a different URI scheme', () => {
    expect(convertMagnetUriToTaskMagnetLink(`https://example.com?xt=urn:btih:${hexHash}`)).toBeNull();
  });
});

describe('TorrentService', () => {
  let torrentService: TorrentService;

  beforeEach(() => {
    torrentService = new TorrentService();
    vi.clearAllMocks();
  });

  describe('listTasks', () => {
    it('should call api.get with correct endpoint', async () => {
      const mockTasks: Task[] = [];
      vi.mocked(api.get).mockResolvedValue({ data: mockTasks, error: undefined });

      await torrentService.listTasks();

      expect(api.get).toHaveBeenCalledWith('/workers/tasks');
    });

    it('should return tasks from API', async () => {
      const mockTasks: Task[] = [
        {
          id: '1',
          name: 'Test Torrent',
          hash: 'abc123',
          size: 1000,
          progress: 50,
          state: 'DOWNLOADING',
          ratio: 0.5,
          category: 'movies',
          tags: ['test'],
          priority: 1,
          popularity: 10,
          magnet_uri: 'magnet:?xt=urn:btih:abc123',
          magnet_link: {
            hash: 'abc123',
            display_name: 'Test Torrent',
            trackers: ['http://tracker.example.com'],
            exact_length: '1000',
            exact_source: 'magnet:?xt=urn:btih:abc123'
          },
          path: '/downloads/test',
          network: {
            download: { speed: 1000, amount: 500 },
            upload: { speed: 500, amount: 250 },
          },
          pairs: {
            seeders: 10,
            leechers: 5,
            swarm_seeders: 20,
            swarm_leechers: 10,
          },
        },
      ];
      vi.mocked(api.get).mockResolvedValue({ data: mockTasks, error: undefined });

      const result = await torrentService.listTasks();

      expect(result.data).toEqual(mockTasks);
      expect(result.error).toBeUndefined();
    });
  });

  describe('createTask', () => {
    it('should call api.post with correct endpoint and data', async () => {
      const workerId = 'worker-123';
      const taskData: CreateTaskRequest = {
        magnet_uri: 'magnet:?xt=urn:btih:abc123',
        category: 'movies',
        tags: ['test'],
      };
      vi.mocked(api.post).mockResolvedValue({ data: {} as Task, error: undefined });

      await torrentService.createTask(workerId, taskData);

      expect(api.post).toHaveBeenCalledWith(`/worker/${workerId}/task`, taskData);
    });

    it('should return response from API', async () => {
      const workerId = 'worker-123';
      const taskData: CreateTaskRequest = {
        magnet_uri: 'magnet:?xt=urn:btih:abc123',
        category: 'movies',
        tags: [],
      };
      const createdTask = {
        id: 'abc123',
        name: 'Test Torrent',
        hash: 'abc123',
        created_at: new Date().toISOString(),
        state: 'DOWNLOADING',
        category: 'movies',
        path: '/downloads/test',
        priority: 0,
        ratio: 0,
        size: 1000,
        progress: 0,
        magnet_uri: taskData.magnet_uri,
        magnet_link: {
          hash: 'abc123',
          display_name: 'Test Torrent',
          trackers: [],
          exact_length: '1000',
          exact_source: ''
        },
        popularity: 0,
        pairs: {
          seeders: 0,
          leechers: 0,
          swarm_seeders: 0,
          swarm_leechers: 0,
        },
        network: {
          download: { speed: 0, amount: 0 },
          upload: { speed: 0, amount: 0 },
        },
        tags: []
      } as Task;
      vi.mocked(api.post).mockResolvedValue({ data: createdTask, error: undefined });

      const result = await torrentService.createTask(workerId, taskData);

      expect(result.data).toEqual(createdTask);
      expect(result.error).toBeUndefined();
    });
  });

  describe('deleteTask', () => {
    it('should call api.delete with correct endpoint without purge', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.delete).mockResolvedValue({ data: null, error: undefined });

      await torrentService.deleteTask(workerId, taskId);

      expect(api.delete).toHaveBeenCalledWith(`/worker/${workerId}/task/${taskId}`);
    });

    it('should call api.delete with correct endpoint with purge', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.delete).mockResolvedValue({ data: null, error: undefined });

      await torrentService.deleteTask(workerId, taskId, true);

      expect(api.delete).toHaveBeenCalledWith(`/worker/${workerId}/task/${taskId}?purge=true`);
    });

    it('should return response from API', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.delete).mockResolvedValue({ data: null, error: undefined });

      const result = await torrentService.deleteTask(workerId, taskId);

      expect(result.data).toBeNull();
      expect(result.error).toBeUndefined();
    });
  });

  describe('listTaskTrackers', () => {
    it('should call api.get with correct endpoint', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.get).mockResolvedValue({ data: [], error: undefined });

      await torrentService.listTaskTrackers(workerId, taskId);

      expect(api.get).toHaveBeenCalledWith(`/worker/${workerId}/task/${taskId}/trackers`);
    });
  });

  describe('addTaskTrackers', () => {
    it('should call api.post with correct endpoint and urls', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      const urls = ['http://tracker.example.com/announce'];
      vi.mocked(api.post).mockResolvedValue({ data: null, error: undefined });

      await torrentService.addTaskTrackers(workerId, taskId, urls);

      expect(api.post).toHaveBeenCalledWith(`/worker/${workerId}/task/${taskId}/trackers`, { urls });
    });
  });

  describe('removeTaskTrackers', () => {
    it('should call api.delete with urls as repeated query params', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      const urls = ['http://tracker1.example.com', 'http://tracker2.example.com'];
      vi.mocked(api.delete).mockResolvedValue({ data: null, error: undefined });

      await torrentService.removeTaskTrackers(workerId, taskId, urls);

      expect(api.delete).toHaveBeenCalledWith(
        `/worker/${workerId}/task/${taskId}/trackers?url=${encodeURIComponent(urls[0])}&url=${encodeURIComponent(urls[1])}`
      );
    });
  });

  describe('editTaskTracker', () => {
    it('should call api.put with the original and new tracker urls', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      const origUrl = 'http://old.example.com';
      const newUrl = 'http://new.example.com';
      vi.mocked(api.put).mockResolvedValue({ data: null, error: undefined });

      await torrentService.editTaskTracker(workerId, taskId, origUrl, newUrl);

      expect(api.put).toHaveBeenCalledWith(`/worker/${workerId}/task/${taskId}/trackers`, {
        orig_url: origUrl,
        new_url: newUrl,
      });
    });
  });

  describe('setTaskQueuePriority', () => {
    it('should call api.put with the priority endpoint and action', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.put).mockResolvedValue({ data: null, error: undefined });

      await torrentService.setTaskQueuePriority(workerId, taskId, 'top');

      expect(api.put).toHaveBeenCalledWith(`/worker/${workerId}/task/${taskId}/priority`, { action: 'top' });
    });
  });

  describe('listTaskPeers', () => {
    it('should call api.get with the peers endpoint', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.get).mockResolvedValue({ data: [], error: undefined });

      await torrentService.listTaskPeers(workerId, taskId);

      expect(api.get).toHaveBeenCalledWith(`/worker/${workerId}/task/${taskId}/peers`);
    });
  });

  describe('banPeer', () => {
    it('should call api.post with ip and port on the worker-scoped ban endpoint', async () => {
      const workerId = 'worker-123';
      vi.mocked(api.post).mockResolvedValue({ data: null, error: undefined });

      await torrentService.banPeer(workerId, '203.0.113.5', 51413);

      expect(api.post).toHaveBeenCalledWith(`/worker/${workerId}/peers/ban`, { ip: '203.0.113.5', port: 51413 });
    });
  });

  describe('pauseTask', () => {
    it('should call api.post with correct stop endpoint', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.post).mockResolvedValue({ data: null, error: undefined });

      await torrentService.pauseTask(workerId, taskId);

      expect(api.post).toHaveBeenCalledWith(`/worker/${workerId}/task/${taskId}/stop`);
    });

    it('should return response from API', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.post).mockResolvedValue({ data: null, error: undefined });

      const result = await torrentService.pauseTask(workerId, taskId);

      expect(result.data).toBeNull();
      expect(result.error).toBeUndefined();
    });
  });

  describe('resumeTask', () => {
    it('should call api.post with correct start endpoint', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.post).mockResolvedValue({ data: null, error: undefined });

      await torrentService.resumeTask(workerId, taskId);

      expect(api.post).toHaveBeenCalledWith(`/worker/${workerId}/task/${taskId}/start`);
    });

    it('should return response from API', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.post).mockResolvedValue({ data: null, error: undefined });

      const result = await torrentService.resumeTask(workerId, taskId);

      expect(result.data).toBeNull();
      expect(result.error).toBeUndefined();
    });
  });

  describe('forceDownloadTask', () => {
    it('should call api.post with correct force_download endpoint', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.post).mockResolvedValue({ data: null, error: undefined });

      await torrentService.forceDownloadTask(workerId, taskId);

      expect(api.post).toHaveBeenCalledWith(`/worker/${workerId}/task/${taskId}/force_download`);
    });

    it('should return response from API', async () => {
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.post).mockResolvedValue({ data: null, error: undefined });

      const result = await torrentService.forceDownloadTask(workerId, taskId);

      expect(result.data).toBeNull();
      expect(result.error).toBeUndefined();
    });
  });

  describe('error handling', () => {
    it('should handle API errors in listTasks', async () => {
      const errorMessage = 'Network error';
      vi.mocked(api.get).mockResolvedValue({ data: null, error: errorMessage });

      const result = await torrentService.listTasks();

      expect(result.data).toBeNull();
      expect(result.error).toBe(errorMessage);
    });

    it('should handle API errors in createTask', async () => {
      const errorMessage = 'Invalid request';
      const workerId = 'worker-123';
      const taskData: CreateTaskRequest = {
        magnet_uri: 'invalid-url',
        category: 'movies',
        tags: [],
      };
      vi.mocked(api.post).mockResolvedValue({ data: undefined, error: errorMessage });

      const result = await torrentService.createTask(workerId, taskData);

      expect(result.data).toBeUndefined();
      expect(result.error).toBe(errorMessage);
    });

    it('should handle API errors in deleteTask', async () => {
      const errorMessage = 'Not found';
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.delete).mockResolvedValue({ data: null, error: errorMessage });

      const result = await torrentService.deleteTask(workerId, taskId);

      expect(result.data).toBeNull();
      expect(result.error).toBe(errorMessage);
    });

    it('should handle API errors in pauseTask', async () => {
      const errorMessage = 'Task already paused';
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.post).mockResolvedValue({ data: null, error: errorMessage });

      const result = await torrentService.pauseTask(workerId, taskId);

      expect(result.data).toBeNull();
      expect(result.error).toBe(errorMessage);
    });

    it('should handle API errors in resumeTask', async () => {
      const errorMessage = 'Task already running';
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.post).mockResolvedValue({ data: null, error: errorMessage });

      const result = await torrentService.resumeTask(workerId, taskId);

      expect(result.data).toBeNull();
      expect(result.error).toBe(errorMessage);
    });

    it('should handle API errors in forceDownloadTask', async () => {
      const errorMessage = 'Unable to force download';
      const workerId = 'worker-123';
      const taskId = 'task-456';
      vi.mocked(api.post).mockResolvedValue({ data: null, error: errorMessage });

      const result = await torrentService.forceDownloadTask(workerId, taskId);

      expect(result.data).toBeNull();
      expect(result.error).toBe(errorMessage);
    });
  });
});
