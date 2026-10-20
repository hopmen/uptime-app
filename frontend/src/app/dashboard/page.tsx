"use client";

/* eslint-disable @next/next/no-img-element -- avatar files are served by the configured API origin. */

import { useRouter } from "next/navigation";
import Link from "next/link";
import { useEffect, useState } from "react";
import { avatarUrl } from "@/lib/auth-api";
import { useAuth } from "@/components/auth-provider";
import { ThemeSwitcher } from "@/components/theme-switcher";
import { MonitorList } from "./monitor-list";

export default function DashboardPage() {
  const router = useRouter();
  const { isLoading, logout, user } = useAuth();
  const [isLoggingOut, setIsLoggingOut] = useState(false);
  const [logoutError, setLogoutError] = useState<string | null>(null);

  useEffect(() => {
    if (!isLoading && !user) {
      router.replace("/login");
    }
  }, [isLoading, router, user]);

  if (isLoading) {
    return <main className="dashboard-page"><p className="loading">Восстанавливаем сессию…</p></main>;
  }

  if (!user) {
    return <main className="dashboard-page"><p className="loading">Перенаправляем на страницу входа…</p></main>;
  }

  async function handleLogout() {
    setIsLoggingOut(true);
    setLogoutError(null);

    try {
      await logout();
      router.replace("/login");
    } catch {
      setLogoutError("Не удалось завершить сессию. Повторите попытку.");
      setIsLoggingOut(false);
    }
  }

  return (
    <main className="dashboard-page">
      <header className="dashboard-header">
        <a className="brand" href="/dashboard">Uptime</a>
        <div className="dashboard-account">
          <ThemeSwitcher />
          <details className="profile-menu">
            <summary aria-label="Открыть меню профиля">
              {user.avatarUrl ? <img className="profile-avatar" src={avatarUrl(user.avatarUrl)} alt="" /> : <span className="profile-avatar" aria-hidden="true">{(user.name || user.email).charAt(0).toUpperCase()}</span>}
              <span className="profile-email">{user.name || user.email}</span>
            </summary>
            <div className="profile-menu-content">
              <p>Текущий пользователь</p>
              {user.name && <strong>{user.name}</strong>}
              <strong>{user.email}</strong>
              <Link className="profile-menu-link" href="/profile">Редактировать профиль</Link>
            </div>
          </details>
          <button className="logout-button" type="button" onClick={handleLogout} disabled={isLoggingOut}>
            {isLoggingOut ? "Выходим…" : "Выйти"}
          </button>
        </div>
        {logoutError && <p className="profile-menu-error" role="alert">{logoutError}</p>}
      </header>
      <MonitorList key={user.id} />
    </main>
  );
}
