// Types for communication with the API v1/categories

export type CategoryMetadataSource = "none" | "tgdb" | "tmdb";
export type CategoryReleaseType =
  | "none"
  | "movie"
  | "series"
  | "os"
  | "game"
  | "book"
  | "music"
  | "software"
  | "audiobook"
  | "comic"
  | "course"
  | "dataset"
  | "rom"
  | "podcast"
  | "anime";

export interface Category {
  id: string;
  name: string;
  default_tags: string[];
  default_directories: string[];
  metadata_source: CategoryMetadataSource;
  release_type?: CategoryReleaseType;
  color?: string;
  icon?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateCategoryRequest {
  name: string;
  default_tags?: string[];
  default_directories: string[];
  metadata_source?: CategoryMetadataSource;
  release_type?: CategoryReleaseType;
  color?: string;
  icon?: string;
}

export interface UpdateCategoryRequest {
  default_tags?: string[];
  default_directories?: string[];
  metadata_source?: CategoryMetadataSource;
  release_type?: CategoryReleaseType;
  color?: string;
  icon?: string;
}

export interface CategoryListResponse {
  data: Category[];
}

export interface CategoryResponse {
  data: Category;
}

export interface CategoryDeleteResponse {
  data: null;
}
