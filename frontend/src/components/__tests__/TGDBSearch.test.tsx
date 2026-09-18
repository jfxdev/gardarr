import type { ButtonHTMLAttributes, InputHTMLAttributes } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { TGDBSearch } from "@/components/TGDBSearch";
import { api } from "@/lib/api";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, options?: { defaultValue?: string; name?: string }) =>
      options?.defaultValue?.replace("{{name}}", options.name ?? "") ?? key,
  }),
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({ children, type = "button", ...props }: ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button type={type} {...props}>
      {children}
    </button>
  ),
}));

vi.mock("@/components/ui/input", () => ({
  Input: (props: InputHTMLAttributes<HTMLInputElement>) => <input {...props} />,
}));

vi.mock("lucide-react", () => ({
  Loader2: () => <span aria-hidden="true" />,
  Search: () => <span aria-hidden="true" />,
}));

vi.mock("sonner", () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock("@/lib/api", () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

describe("TGDBSearch", () => {
  it("exposes accessible names for the search input and icon button", () => {
    render(
      <TGDBSearch
        taskHash="task-1"
        initialQuery=""
        onSelect={() => {}}
        onCancel={() => {}}
      />
    );

    expect(screen.getByRole("textbox", { name: "Search TGDB" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Execute search" })).toBeInTheDocument();
  });

  it("posts the selected TGDB id with fallback metadata when applying", async () => {
    vi.mocked(api.get).mockResolvedValueOnce({
      data: [
        {
          id: "123",
          title: "Test Game",
          release_date: "2024-01-01",
          description: "Overview",
          image_id: "front.jpg",
          image_url: "https://cdn.thegamesdb.net/images/large/front.jpg",
        },
      ],
    });
    vi.mocked(api.post).mockResolvedValueOnce({
      data: {
        uuid: "meta-1",
        task_hash: "task-1",
        name: "Test Game",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    });

    render(
      <TGDBSearch
        taskHash="task-1"
        initialQuery="Test Game"
        onSelect={() => {}}
        onCancel={() => {}}
      />
    );

    const gameTitle = await screen.findByText("Test Game");
    expect(screen.getByRole("img", { name: "Test Game" })).toHaveAttribute(
      "src",
      "/v1/tasks/metadata/providers/tgdb/image?image_id=front.jpg"
    );
    const gameButton = gameTitle.closest("button");
    if (!gameButton) {
      throw new Error("expected TGDB result button");
    }

    fireEvent.click(gameButton);

    await waitFor(() =>
      expect(api.post).toHaveBeenCalledWith("/tasks/metadata/task-1/providers/tgdb", {
        id: "123",
        title: "Test Game",
        release_date: "2024-01-01",
        description: "Overview",
        image_id: "front.jpg",
      })
    );
  });

  it("passes the raw initial torrent name to the backend parser", () => {
    render(
      <TGDBSearch
        taskHash="task-1"
        initialQuery="Super.Mario.Galaxy.O.Filme.2026.WEB-DL.1080p.x264.DUAL.5.1"
        onSelect={() => {}}
        onCancel={() => {}}
      />
    );

    expect(screen.getByDisplayValue("Super.Mario.Galaxy.O.Filme.2026.WEB-DL.1080p.x264.DUAL.5.1")).toBeInTheDocument();
  });
});
