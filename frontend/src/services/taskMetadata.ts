import { api } from "../lib/api";
import type { ApiResponse } from "../lib/api";
import type { MetadataProviderSearchResult, TaskMetadata } from "../types/torrent";

export interface ReleaseParseResult {
  type: "unknown" | "movie" | "series" | "os" | "game" | "book" | "music" | "software" | "audiobook" | "comic" | "course" | "dataset" | "rom" | "podcast" | "anime";
  confidence: "low" | "medium" | "high";
  title?: string;
  year?: string;
  season?: string;
  episode?: string;
  format?: string;
  platform?: string;
  version?: string;
  hdr?: string;
  region?: string;
  provider?: string;
  package?: string;
}

export interface ReleaseParseResponse {
  release: ReleaseParseResult;
  display_name: string;
  tags: string[];
}

const SEARCH_PREVIEW_TASK_HASH = "wizard-preview";

interface ProviderStatusResponse {
  provider: string;
  active: boolean;
}

interface ApplyProviderRequest {
  id: string;
  title?: string;
  release_date?: string;
  description?: string;
  image_id?: string;
}

class TaskMetadataService {
  getProviderImagePreviewUrl(provider: string, imageId?: string): string | undefined {
    const normalizedProvider = provider.trim();
    const normalizedImageId = imageId?.trim();
    if (!normalizedProvider || !normalizedImageId) {
      return undefined;
    }

    return `/v1/tasks/metadata/providers/${encodeURIComponent(normalizedProvider)}/image?image_id=${encodeURIComponent(normalizedImageId)}`;
  }

  async getProviderStatus(provider: string): Promise<ApiResponse<ProviderStatusResponse>> {
    return api.get<ProviderStatusResponse>(`/tasks/metadata/providers/${provider}/status`);
  }

  async searchProvider(provider: string, query: string, auto = false): Promise<ApiResponse<MetadataProviderSearchResult[]>> {
    const autoParam = auto ? "&auto=true" : "";
    return api.get<MetadataProviderSearchResult[]>(
      `/tasks/metadata/${SEARCH_PREVIEW_TASK_HASH}/providers/${provider}/search?q=${encodeURIComponent(query)}${autoParam}`
    );
  }

  async parseRelease(rawName: string): Promise<ApiResponse<ReleaseParseResponse>> {
    return api.post<ReleaseParseResponse>("/tasks/metadata/release-parse", { raw_name: rawName });
  }

  async parseReleaseFile(file: File): Promise<ApiResponse<ReleaseParseResponse>> {
    const formData = new FormData();
    formData.append("torrent", file);
    return api.post<ReleaseParseResponse>("/tasks/metadata/release-parse/file", formData);
  }

  async applyProvider(
    taskHash: string,
    provider: string,
    payload: ApplyProviderRequest
  ): Promise<ApiResponse<TaskMetadata>> {
    return api.post<TaskMetadata>(`/tasks/metadata/${taskHash}/providers/${provider}`, payload);
  }

  async updateName(taskHash: string, name: string): Promise<ApiResponse<TaskMetadata>> {
    return api.put<TaskMetadata>(`/tasks/metadata/${taskHash}/name`, { name });
  }

  async updateDescription(taskHash: string, description: string): Promise<ApiResponse<TaskMetadata>> {
    return api.put<TaskMetadata>(`/tasks/metadata/${taskHash}/description`, { description });
  }

  async uploadImage(taskHash: string, file: File): Promise<ApiResponse<TaskMetadata>> {
    const formData = new FormData();
    formData.append("image", file);
    return api.post<TaskMetadata>(`/tasks/metadata/${taskHash}/image`, formData);
  }
}

export const taskMetadataService = new TaskMetadataService();
