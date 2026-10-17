"use client";

import { type FormEvent, useEffect, useRef, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { createMonitor, listMonitors, type Monitor } from "@/lib/monitor-api";
import { formatInterval, intervalInSeconds, intervalUnits, maxIntervalSeconds, type IntervalUnit } from "./monitor-config";

export function MonitorList() {
  const { authorizedFetch } = useAuth();
  const [monitors, setMonitors] = useState<Monitor[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [url, setUrl] = useState("");
  const [interval, setInterval] = useState("5");
  const [unit, setUnit] = useState<IntervalUnit>("minutes");
  const [isSaving, setIsSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const saving = useRef(false);

  useEffect(() => {
    let cancelled = false;
    listMonitors(authorizedFetch).then((points) => {
      if (!cancelled) setMonitors(points);
    }).catch((error: unknown) => {
      if (!cancelled) setLoadError(error instanceof Error ? error.message : "Не удалось загрузить сайты.");
    }).finally(() => {
      if (!cancelled) setIsLoading(false);
    });
    return () => { cancelled = true; };
  }, [authorizedFetch, loadAttempt]);

  async function handleCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving.current) return;
    setFormError(null);
    let seconds: number;
    try {
      seconds = intervalInSeconds(interval, unit);
      const parsed = new URL(url.trim());
      if (!["http:", "https:"].includes(parsed.protocol) || parsed.username || parsed.password) {
        throw new Error("Укажите URL с протоколом http:// или https:// без логина и пароля.");
      }
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Проверьте данные сайта.");
      return;
    }
    saving.current = true;
    setIsSaving(true);
    try {
      const point = await createMonitor(authorizedFetch, url.trim(), seconds);
      setMonitors((current) => [point, ...current]);
      setSuccess(`Сайт ${point.url} добавлен.`);
      dialog.current?.close();
      setUrl("");
      setInterval("5");
      setUnit("minutes");
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Не удалось создать точку мониторинга.");
    } finally {
      saving.current = false;
      setIsSaving(false);
    }
  }

  return (
    <section className="monitors" aria-labelledby="monitors-title">
      <div className="monitors-heading">
        <div><h1 id="monitors-title">Мои сайты</h1><p className="dashboard-copy">Добавьте сайты и задайте частоту будущих проверок.</p></div>
        <button className="primary-button" type="button" disabled={isLoading || !!loadError} onClick={() => {
          setFormError(null);
          setSuccess(null);
          dialog.current?.showModal();
        }}>Добавить сайт</button>
      </div>
      {success && <p className="form-success" role="status">{success}</p>}
      {isLoading && <p className="loading" role="status">Загружаем сайты…</p>}
      {loadError && <div className="form-error" role="alert">{loadError} <button className="secondary-button" type="button" onClick={() => {
        setLoadError(null);
        setIsLoading(true);
        setLoadAttempt((attempt) => attempt + 1);
      }}>Повторить</button></div>}
      {!isLoading && !loadError && monitors.length === 0 && <div className="monitors-empty"><h2>Пока нет сайтов</h2><p>Нажмите «Добавить сайт», чтобы создать первую точку мониторинга.</p></div>}
      {monitors.length > 0 && <ul className="monitor-list">{monitors.map((point) => (
        <li className="monitor-card" key={point.id}>
          <a href={point.url} target="_blank" rel="noopener noreferrer">{point.url}</a>
          <p>Интервал: {formatInterval(point.intervalSeconds)}</p>
          <span className="monitor-status">Опрос ещё не запущен</span>
        </li>
      ))}</ul>}
      <dialog className="monitor-dialog" ref={dialog} aria-labelledby="create-monitor-title" onCancel={(event) => { if (saving.current) event.preventDefault(); }}>
        <h2 id="create-monitor-title">Добавить сайт</h2>
        <form className="monitor-form" onSubmit={handleCreate}>
          <label htmlFor="monitor-url">URL сайта</label>
          <input id="monitor-url" name="url" type="url" placeholder="https://example.com" required autoFocus value={url} onChange={(event) => setUrl(event.target.value)} disabled={isSaving} />
          <label htmlFor="monitor-interval">Частота опроса</label>
          <div className="monitor-interval">
            <input id="monitor-interval" name="interval" type="number" min="1" max={Math.floor(maxIntervalSeconds / intervalUnits[unit])} step="1" required value={interval} onChange={(event) => setInterval(event.target.value)} disabled={isSaving} />
            <select aria-label="Единица интервала" name="unit" value={unit} onChange={(event) => setUnit(event.target.value as IntervalUnit)} disabled={isSaving}>
              <option value="seconds">Секунды</option><option value="minutes">Минуты</option><option value="hours">Часы</option>
            </select>
          </div>
          <p className="monitor-hint">Сохраним настройки точки. Проверки сайтов пока не выполняются.</p>
          {formError && <p className="form-error" role="alert">{formError}</p>}
          <div className="monitor-actions">
            <button className="secondary-button" type="button" disabled={isSaving} onClick={() => dialog.current?.close()}>Отмена</button>
            <button className="primary-button" type="submit" disabled={isSaving}>{isSaving ? "Создаём…" : "Создать"}</button>
          </div>
        </form>
      </dialog>
    </section>
  );
}
