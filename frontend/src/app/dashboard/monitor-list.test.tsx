import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MonitorList } from "./monitor-list";

const { authorizedFetch } = vi.hoisted(() => ({ authorizedFetch: vi.fn() }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ authorizedFetch }) }));

const point = { id: "point-1", url: "https://example.com", intervalSeconds: 300, createdAt: "2026-10-17T10:00:00Z" };

beforeEach(() => {
  vi.stubEnv("NEXT_PUBLIC_API_URL", "http://localhost:8080");
  authorizedFetch.mockReset();
  // jsdom does not implement the native modal methods.
  HTMLDialogElement.prototype.showModal = function () { this.open = true; };
  HTMLDialogElement.prototype.close = function () { this.open = false; };
});
afterEach(() => { cleanup(); vi.unstubAllEnvs(); });

async function openForm() {
  const user = userEvent.setup();
  render(<MonitorList />);
  await screen.findByText("Пока нет сайтов");
  await user.click(screen.getByRole("button", { name: "Добавить сайт" }));
  return user;
}

describe("monitor creation", () => {
  it.each([["seconds", "15", 15], ["minutes", "5", 300], ["hours", "2", 7200]])("creates using %s and shows the saved site", async (unit, value, seconds) => {
    authorizedFetch.mockResolvedValueOnce(Response.json([])).mockResolvedValueOnce(Response.json({ ...point, intervalSeconds: seconds }, { status: 201 }));
    const user = await openForm();
    await user.type(screen.getByLabelText("URL сайта"), point.url);
    await user.clear(screen.getByLabelText("Частота опроса"));
    await user.type(screen.getByLabelText("Частота опроса"), value as string);
    await user.selectOptions(screen.getByRole("combobox", { name: "Единица интервала" }), unit as string);
    await user.click(screen.getByRole("button", { name: "Создать" }));
    expect(await screen.findByRole("link", { name: point.url })).toBeTruthy();
    expect(authorizedFetch).toHaveBeenLastCalledWith("http://localhost:8080/api/v1/monitors", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url: point.url, intervalSeconds: seconds }),
    });
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByText("Опрос ещё не запущен")).toBeTruthy();
  });

  it("loads existing sites after remount", async () => {
    authorizedFetch.mockResolvedValue(Response.json([point]));
    render(<MonitorList />);
    expect(await screen.findByRole("link", { name: point.url })).toBeTruthy();
    expect(screen.getByText("Интервал: 5 мин")).toBeTruthy();
  });

  it("keeps form data on server failure and allows retry", async () => {
    authorizedFetch.mockResolvedValueOnce(Response.json([]))
      .mockResolvedValueOnce(Response.json({ error: "Не удалось сохранить сайт" }, { status: 500 }))
      .mockResolvedValueOnce(Response.json(point, { status: 201 }));
    const user = await openForm();
    await user.type(screen.getByLabelText("URL сайта"), point.url);
    await user.click(screen.getByRole("button", { name: "Создать" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось сохранить сайт");
    expect((screen.getByLabelText("URL сайта") as HTMLInputElement).value).toBe(point.url);
    expect(screen.queryByRole("link", { name: point.url })).toBeNull();
    await user.click(screen.getByRole("button", { name: "Создать" }));
    expect(await screen.findByRole("link", { name: point.url })).toBeTruthy();
  });

  it("blocks duplicate submissions while saving", async () => {
    let finish!: (response: Response) => void;
    authorizedFetch.mockResolvedValueOnce(Response.json([])).mockReturnValueOnce(new Promise<Response>((resolve) => { finish = resolve; }));
    const user = await openForm();
    await user.type(screen.getByLabelText("URL сайта"), point.url);
    await user.click(screen.getByRole("button", { name: "Создать" }));
    const form = screen.getByLabelText("URL сайта").closest("form")!;
    fireEvent.submit(form);
    expect(authorizedFetch).toHaveBeenCalledTimes(2);
    expect((screen.getByRole("button", { name: "Создаём…" }) as HTMLButtonElement).disabled).toBe(true);
    finish(Response.json(point, { status: 201 }));
    await screen.findByRole("link", { name: point.url });
  });

  it("cancels without sending a creation request", async () => {
    authorizedFetch.mockResolvedValueOnce(Response.json([]));
    const user = await openForm();
    await user.click(screen.getByRole("button", { name: "Отмена" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(authorizedFetch).toHaveBeenCalledTimes(1);
  });

  it("retries failed loading", async () => {
    authorizedFetch.mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce(Response.json([point]));
    const user = userEvent.setup();
    render(<MonitorList />);
    await screen.findByRole("alert");
    await user.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByRole("link", { name: point.url })).toBeTruthy();
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
  });

  it("rejects unsupported protocols before creating", async () => {
    authorizedFetch.mockResolvedValueOnce(Response.json([]));
    const user = await openForm();
    await user.type(screen.getByLabelText("URL сайта"), "ftp://example.com");
    await user.click(screen.getByRole("button", { name: "Создать" }));
    expect((await screen.findByRole("alert")).textContent).toContain("http:// или https://");
    expect(authorizedFetch).toHaveBeenCalledTimes(1);
  });
});
