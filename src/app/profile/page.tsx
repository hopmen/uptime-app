"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { type FormEvent, useEffect, useState } from "react";
import { ApiError } from "@/lib/auth-api";
import { useAuth } from "@/components/auth-provider";

export default function ProfilePage() {
  const router = useRouter();
  const { isLoading, updateProfile, user } = useAuth();
  const [editedName, setEditedName] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (!isLoading && !user) {
      router.replace("/login");
      return;
    }
  }, [isLoading, router, user]);

  if (isLoading || !user) {
    return <main className="profile-page"><p className="loading">Загружаем профиль…</p></main>;
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!user) {
      return;
    }
    setError(null);
    setSaved(false);
    setIsSaving(true);

    try {
      await updateProfile(editedName ?? user.name);
      setEditedName(null);
      setSaved(true);
    } catch (caughtError) {
      setError(caughtError instanceof ApiError ? caughtError.message : "Не удалось сохранить профиль. Повторите попытку.");
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <main className="profile-page">
      <section className="profile-card" aria-labelledby="profile-title">
        <Link className="brand" href="/dashboard">Uptime</Link>
        <p className="eyebrow">Аккаунт</p>
        <h1 id="profile-title">Профиль</h1>
        <p className="profile-copy">Измените отображаемое имя. Email пока доступен только для просмотра.</p>
        <form className="profile-form" onSubmit={handleSubmit}>
          <label htmlFor="name">Имя</label>
          <input id="name" name="name" value={editedName ?? user.name} onChange={(event) => setEditedName(event.target.value)} maxLength={100} required />
          <label htmlFor="email">Email</label>
          <input id="email" value={user.email} readOnly aria-readonly="true" />
          {error && <p className="form-error" role="alert">{error}</p>}
          {saved && <p className="form-success" role="status">Профиль сохранён.</p>}
          <div className="profile-actions">
            <Link className="secondary-button" href="/dashboard">Отмена</Link>
            <button type="submit" disabled={isSaving}>{isSaving ? "Сохраняем…" : "Сохранить"}</button>
          </div>
        </form>
      </section>
    </main>
  );
}
