"use client";

/* eslint-disable @next/next/no-img-element -- avatar files are served by the configured API origin. */

import Link from "next/link";
import { useRouter } from "next/navigation";
import { type ChangeEvent, type FormEvent, useEffect, useRef, useState } from "react";
import { ApiError, avatarUrl } from "@/lib/auth-api";
import { useAuth } from "@/components/auth-provider";

export default function ProfilePage() {
  const router = useRouter();
  const { isLoading, updateProfile, uploadAvatar, user } = useAuth();
  const [editedName, setEditedName] = useState<string | null>(null);
  const [selectedAvatar, setSelectedAvatar] = useState<File | null>(null);
  const [avatarPreview, setAvatarPreview] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);
  const previewRef = useRef("");

  useEffect(() => () => {
    if (previewRef.current) {
      URL.revokeObjectURL(previewRef.current);
    }
  }, []);

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
      if (selectedAvatar) {
        await uploadAvatar(selectedAvatar);
        URL.revokeObjectURL(previewRef.current);
        previewRef.current = "";
        setAvatarPreview("");
        setSelectedAvatar(null);
      }
      setEditedName(null);
      setSaved(true);
    } catch (caughtError) {
      setError(caughtError instanceof ApiError ? caughtError.message : "Не удалось сохранить профиль. Повторите попытку.");
    } finally {
      setIsSaving(false);
    }
  }

  function handleAvatarChange(event: ChangeEvent<HTMLInputElement>) {
    const avatar = event.target.files?.[0];
    if (!avatar) {
      return;
    }
    if (!avatar.type.match(/^image\/(jpeg|png|gif|webp)$/)) {
      setError("Выберите изображение в формате JPEG, PNG, GIF или WebP.");
      event.target.value = "";
      return;
    }
    if (avatar.size > 5 * 1024 * 1024) {
      setError("Размер изображения не должен превышать 5 МБ.");
      event.target.value = "";
      return;
    }
    setError(null);
    if (previewRef.current) {
      URL.revokeObjectURL(previewRef.current);
    }
    const preview = URL.createObjectURL(avatar);
    previewRef.current = preview;
    setAvatarPreview(preview);
    setSelectedAvatar(avatar);
  }

  return (
    <main className="profile-page">
      <section className="profile-card" aria-labelledby="profile-title">
        <Link className="brand" href="/dashboard">Uptime</Link>
        <p className="eyebrow">Аккаунт</p>
        <h1 id="profile-title">Профиль</h1>
        <p className="profile-copy">Измените отображаемое имя. Email пока доступен только для просмотра.</p>
        <form className="profile-form" onSubmit={handleSubmit}>
          <div className="avatar-editor">
            {avatarPreview || user.avatarUrl ? <img className="profile-avatar profile-avatar-large" src={avatarPreview || avatarUrl(user.avatarUrl)} alt="Аватар пользователя" /> : <span className="profile-avatar profile-avatar-large" aria-hidden="true">{(user.name || user.email).charAt(0).toUpperCase()}</span>}
            <div>
              <label className="avatar-upload-button" htmlFor="avatar">Загрузить аватар</label>
              <input id="avatar" className="avatar-input" name="avatar" type="file" accept="image/jpeg,image/png,image/gif,image/webp" onChange={handleAvatarChange} />
              <p className="avatar-hint">JPEG, PNG, GIF или WebP, до 5 МБ.</p>
              {selectedAvatar && <p className="avatar-selected">Выбрано: {selectedAvatar.name}</p>}
            </div>
          </div>
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
