import type { ReactNode } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { TorrentActionBar } from "../TorrentActionBar";

vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string, opts?: { defaultValue?: string }) => opts?.defaultValue ?? key,
  }),
}));

vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("@/components/ui/button-group", () => ({
  ButtonGroup: ({ children, className }: { children: ReactNode; className?: string }) => (
    <div role="group" className={className}>{children}</div>
  ),
}));

describe("TorrentActionBar queue priority controls", () => {
  it("renders nothing when no handlers are passed", () => {
    const { container } = render(<TorrentActionBar torrentId="task-1" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("does not render queue priority buttons when onQueuePriority is omitted", () => {
    render(<TorrentActionBar torrentId="task-1" onPlay={() => {}} />);
    expect(screen.queryByLabelText("Move to top of queue")).not.toBeInTheDocument();
  });

  it("distributes the controls across the full desktop row", () => {
    render(<TorrentActionBar torrentId="task-1" onPlay={() => {}} onPause={() => {}} />);

    expect(screen.getByRole("group")).toHaveClass("sm:!w-full");
    expect(screen.getByLabelText("torrents.actionButtons.play")).toHaveClass("sm:!flex-1");
    expect(screen.getByLabelText("torrents.actionButtons.pause")).toHaveClass("sm:!flex-1");
  });

  it("calls onQueuePriority with the right action for each button", () => {
    const onQueuePriority = vi.fn();
    render(<TorrentActionBar torrentId="task-1" onQueuePriority={onQueuePriority} />);

    fireEvent.click(screen.getByLabelText("Move to top of queue"));
    fireEvent.click(screen.getByLabelText("Move up in queue"));
    fireEvent.click(screen.getByLabelText("Move down in queue"));
    fireEvent.click(screen.getByLabelText("Move to bottom of queue"));

    expect(onQueuePriority).toHaveBeenNthCalledWith(1, "task-1", "top");
    expect(onQueuePriority).toHaveBeenNthCalledWith(2, "task-1", "up");
    expect(onQueuePriority).toHaveBeenNthCalledWith(3, "task-1", "down");
    expect(onQueuePriority).toHaveBeenNthCalledWith(4, "task-1", "bottom");
  });
});

describe("TorrentActionBar queue priority pulse feedback", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("pulses the clicked button's icon and clears it after the animation window", () => {
    render(<TorrentActionBar torrentId="task-1" onQueuePriority={() => {}} />);

    const topButton = screen.getByLabelText("Move to top of queue");
    const icon = topButton.querySelector("svg");
    expect(icon).not.toHaveClass("animate-bounce");

    act(() => fireEvent.click(topButton));
    expect(topButton.querySelector("svg")).toHaveClass("animate-bounce");

    act(() => vi.advanceTimersByTime(600));
    expect(topButton.querySelector("svg")).not.toHaveClass("animate-bounce");
  });

  it("only pulses the clicked direction, not the other queue buttons", () => {
    render(<TorrentActionBar torrentId="task-1" onQueuePriority={() => {}} />);

    act(() => fireEvent.click(screen.getByLabelText("Move to top of queue")));

    expect(screen.getByLabelText("Move to top of queue").querySelector("svg")).toHaveClass("animate-bounce");
    expect(screen.getByLabelText("Move up in queue").querySelector("svg")).not.toHaveClass("animate-bounce");
    expect(screen.getByLabelText("Move down in queue").querySelector("svg")).not.toHaveClass("animate-bounce");
    expect(screen.getByLabelText("Move to bottom of queue").querySelector("svg")).not.toHaveClass("animate-bounce");
  });
});
