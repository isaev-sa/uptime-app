import './globals.css';

export const metadata = {
  title: 'Uptime — вход в аккаунт',
  description: 'Вход и регистрация в Uptime.',
};

/** Wrap each page in the application document. */
export default function RootLayout({ children }) {
  return (
    <html lang="ru">
      <body>{children}</body>
    </html>
  );
}
