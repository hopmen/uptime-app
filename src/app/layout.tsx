import type { Metadata } from "next";
import { AuthProvider } from "@/components/auth-provider";
import "./globals.css";

export const metadata: Metadata = {
  title: "Uptime — мониторинг доступности",
  description: "Сервис мониторинга доступности ваших систем.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="ru">
      <body><AuthProvider>{children}</AuthProvider></body>
    </html>
  );
}
