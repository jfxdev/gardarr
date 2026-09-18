import { describe, expect, it } from "vitest";
import { taskMetadataService } from "../taskMetadata";

describe("TaskMetadataService", () => {
  it("builds a same-origin provider preview URL and escapes the image id", () => {
    expect(
      taskMetadataService.getProviderImagePreviewUrl("tgdb", "boxart/front/123 1.jpg")
    ).toBe(
      "/v1/tasks/metadata/providers/tgdb/image?image_id=boxart%2Ffront%2F123%201.jpg"
    );
    expect(taskMetadataService.getProviderImagePreviewUrl("tmdb", "poster.jpg")).toBe(
      "/v1/tasks/metadata/providers/tmdb/image?image_id=poster.jpg"
    );
  });

  it("does not build a preview URL without an image id", () => {
    expect(taskMetadataService.getProviderImagePreviewUrl("tmdb", " ")).toBeUndefined();
  });
});
