"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { type FormEvent, useState } from "react";
import { ApiError } from "@/lib/auth-api";
import { useAuth } from "@/components/auth-provider";

type AuthFormProps = {
  mode: "login" | "register";
};

export function AuthForm({ mode }: AuthFormProps) {
  const router = useRouter();
  const { login, register } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const isRegistration = mode === "register";
  const title = isRegistration ? "Создайте аккаунт" : "С возвращением";
  const submitLabel = isRegistration ? "Зарегистрироваться" : "Войти";

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);

    if (!email.includes("@")) {
      setError("Введите корректный email.");
      return;
    }
    if (password.length < 8) {
      setError("Пароль должен содержать не менее 8 символов.");
      return;
    }

    setIsSubmitting(true);
    try {
      if (isRegistration) {
        await register(email, password);
      } else {
        await login(email, password);
      }
      router.replace("/dashboard");
    } catch (requestError) {
      if (requestError instanceof ApiError) {
        setError(requestError.message);
      } else {
        setError("Не удалось выполнить запрос. Повторите попытку.");
      }
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-card" aria-labelledby="auth-title">
        <Link className="brand" href="/dashboard">Uptime</Link>
        <p className="eyebrow">Мониторинг доступности</p>
        <h1 id="auth-title">{title}</h1>
        <p className="auth-copy">
          {isRegistration ? "Начните следить за своими сервисами." : "Войдите, чтобы продолжить работу с сервисами."}
        </p>
        <form className="auth-form" onSubmit={handleSubmit} noValidate>
          <label htmlFor="email">Email</label>
          <input
            autoComplete="email"
            aria-describedby={error ? "auth-error" : undefined}
            aria-invalid={Boolean(error)}
            id="email"
            inputMode="email"
            onChange={(event) => setEmail(event.target.value)}
            placeholder="you@example.com"
            required
            type="email"
            value={email}
          />
          <label htmlFor="password">Пароль</label>
          <input
            autoComplete={isRegistration ? "new-password" : "current-password"}
            aria-describedby={error ? "auth-error" : undefined}
            aria-invalid={Boolean(error)}
            id="password"
            minLength={8}
            onChange={(event) => setPassword(event.target.value)}
            placeholder="Не менее 8 символов"
            required
            type="password"
            value={password}
          />
          {error ? <p className="form-error" id="auth-error" role="alert">{error}</p> : null}
          <button disabled={isSubmitting} type="submit">
            {isSubmitting ? "Подождите…" : submitLabel}
          </button>
        </form>
        <p className="auth-switch">
          {isRegistration ? "Уже есть аккаунт?" : "Нет аккаунта?"}{" "}
          <Link href={isRegistration ? "/login" : "/register"}>
            {isRegistration ? "Войти" : "Зарегистрироваться"}
          </Link>
        </p>
      </section>
    </main>
  );
}
