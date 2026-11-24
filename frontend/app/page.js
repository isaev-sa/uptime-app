'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { usePathname } from 'next/navigation';
import Monitors from './monitors';

const API_URL = (process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080').replace(/\/$/, '');
let pendingRefresh;

async function request(path, body) {
  let response;
  try {
    response = await fetch(`${API_URL}/auth/${path}`, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  } catch {
    throw new Error('Не удалось подключиться к серверу. Проверьте соединение и попробуйте ещё раз.');
  }

  if (!response.ok) {
    const error = new Error();
    error.status = response.status;
    throw error;
  }

  const data = await response.json();
  if (typeof data.access_token !== 'string' || !data.access_token || !Number.isFinite(Date.parse(data.expires_at))) {
    throw new Error('Сервер вернул некорректный ответ. Попробуйте ещё раз.');
  }
  return data;
}

function refreshSession() {
  if (!pendingRefresh) {
    const refresh = () => request('refresh', {});
    pendingRefresh = (navigator.locks ? navigator.locks.request('uptime-refresh', refresh) : refresh())
      .finally(() => { pendingRefresh = null; });
  }
  return pendingRefresh;
}

async function logoutSession() {
  const logout = async () => {
    const response = await fetch(`${API_URL}/auth/refresh/logout`, {
      method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: '{}',
    });
    if (!response.ok) throw new Error('Не удалось выйти из аккаунта. Попробуйте ещё раз.');
  };
  return navigator.locks ? navigator.locks.request('uptime-refresh', logout) : logout();
}

function Brand() {
  return <a className="brand" href="/" aria-label="Uptime, на главную"><span className="brand-mark"><span /></span><span>uptime<span className="brand-dot">.</span></span></a>;
}

function MonitorGraphic() {
  return (
    <div className="monitor-card" aria-hidden="true">
      <div className="monitor-top"><div className="monitor-icon"><span /></div><div><strong>Ваши сервисы</strong><small>Обзор состояния</small></div><span className="monitor-menu">···</span></div>
      <div className="monitor-divider" />
      <div className="monitor-row"><span className="service-icon purple">✳</span><span><strong>Основной сайт</strong><small>Проверка доступности</small></span><span className="status"><i /> Работает</span></div>
      <div className="monitor-row"><span className="service-icon blue">◇</span><span><strong>API сервер</strong><small>Проверка доступности</small></span><span className="status"><i /> Работает</span></div>
      <div className="monitor-row"><span className="service-icon peach">▧</span><span><strong>База данных</strong><small>Проверка доступности</small></span><span className="status"><i /> Работает</span></div>
      <div className="monitor-footer"><span><i /> Все системы работают</span><span>Обновлено только что</span></div>
    </div>
  );
}

/** Render authentication, the dashboard, or the profile for the current session. */
export default function Home() {
  const isProfile = usePathname() === '/profile';
  const [profile, setProfile] = useState(null);
  const [profileName, setProfileName] = useState('');
  const [avatarFile, setAvatarFile] = useState(null);
  const [avatarPreview, setAvatarPreview] = useState('');
  const [profileError, setProfileError] = useState('');
  const [profileLoading, setProfileLoading] = useState(false);
  const [profileSaved, setProfileSaved] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [mode, setMode] = useState('login');
  const [session, setSession] = useState(null);
  const [checking, setChecking] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [connectionError, setConnectionError] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [password, setPassword] = useState('');
  const [retry, setRetry] = useState(0);
  const [loggingOut, setLoggingOut] = useState(false);
  const mounted = useRef(false);
  const loggingOutRef = useRef(false);

  const restore = useCallback(async () => {
    try {
      const tokens = await refreshSession();
      if (mounted.current) { setSession(tokens); setConnectionError(false); setError(''); }
    } catch (err) {
      if (!mounted.current) return;
      if (err.status === 401) { setSession(null); setConnectionError(false); }
      else { setConnectionError(true); setError(err.message || 'Не удалось обновить сессию. Попробуйте ещё раз.'); }
    } finally {
      if (mounted.current) setChecking(false);
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    restore();
    return () => { mounted.current = false; };
  }, [restore]);

  useEffect(() => {
    if (!session) return undefined;
    const remaining = Date.parse(session.expires_at) - Date.now();
    // Refresh before expiry while leaving short-lived tokens time to be used.
    const delay = retry ? 30000 : Math.max(1000, remaining - Math.min(60000, remaining / 2));
    const timer = setTimeout(async () => {
      try {
        const tokens = await refreshSession();
        if (mounted.current && !loggingOutRef.current) { setSession(tokens); setRetry(0); setConnectionError(false); setError(''); }
      } catch (err) {
        if (!mounted.current || loggingOutRef.current) return;
        if (err.status === 401) { setSession(null); setRetry(0); setError('Сессия завершилась. Войдите снова.'); }
        else { setConnectionError(true); setError('Не удалось обновить сессию. Повторяем попытку.'); setRetry((value) => value + 1); }
      }
    }, delay);
    return () => clearTimeout(timer);
  }, [session, retry]);

  useEffect(() => {
    if (!isProfile || !session) return undefined;
    const abort = new AbortController();
    setProfileLoading(true);
    setProfileError('');
    fetch(`${API_URL}/profile`, { headers: { Authorization: `Bearer ${session.access_token}` }, signal: abort.signal })
      .then((response) => { if (!response.ok) throw new Error(); return response.json(); })
      .then((data) => { setProfile(data); setProfileName(data.name); })
      .catch((err) => { if (err.name !== 'AbortError') setProfileError('Не удалось загрузить профиль. Обновите страницу и попробуйте ещё раз.'); })
      .finally(() => { if (!abort.signal.aborted) setProfileLoading(false); });
    return () => abort.abort();
  }, [isProfile, Boolean(session)]);

  useEffect(() => {
    if (!avatarFile) { setAvatarPreview(''); return undefined; }
    const url = URL.createObjectURL(avatarFile);
    setAvatarPreview(url);
    return () => URL.revokeObjectURL(url);
  }, [avatarFile]);

  function chooseAvatar(event) {
    const file = event.target.files?.[0];
    if (!file) return;
    if (!['image/jpeg', 'image/png'].includes(file.type) || file.size > 5 * 1024 * 1024) {
      setProfileError('Выберите изображение JPEG или PNG размером до 5 МБ.');
      event.target.value = '';
      return;
    }
    setAvatarFile(file);
    setProfileSaved(false);
    setProfileError('');
    event.target.value = '';
  }

  async function saveProfile(event) {
    event.preventDefault();
    const trimmed = profileName.trim();
    setProfileError('');
    setProfileSaved(false);
    if (!trimmed || Array.from(trimmed).length > 100) { setProfileError('Укажите имя длиной до 100 символов.'); return; }
    setBusy(true);
    try {
      const response = await fetch(`${API_URL}/profile`, { method: 'PATCH', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${session.access_token}` }, body: JSON.stringify({ name: trimmed }) });
      if (!response.ok) throw new Error();
      const data = await response.json();
      setProfile(data);
      setProfileName(data.name);
      setSession((current) => current ? { ...current, name: data.name, email: data.email, avatar_url: data.avatar_url } : current);
      if (avatarFile) {
        const form = new FormData();
        form.append('avatar', avatarFile);
        const upload = await fetch(`${API_URL}/profile/avatar`, { method: 'POST', headers: { Authorization: `Bearer ${session.access_token}` }, body: form });
        if (!upload.ok) throw new Error('Не удалось загрузить аватар. Проверьте изображение и попробуйте ещё раз.');
        const result = await upload.json();
        setProfile((current) => ({ ...current, avatar_url: result.avatar_url }));
        setSession((current) => current ? { ...current, avatar_url: result.avatar_url } : current);
        setAvatarFile(null);
      }
      setProfileSaved(true);
    } catch (err) { setProfileError(err.message || 'Не удалось сохранить профиль. Попробуйте ещё раз.'); }
    finally { setBusy(false); }
  }

  async function submit(event) {
    event.preventDefault();
    setError('');
    if (mode === 'register' && (!name.trim() || Array.from(name.trim()).length > 100)) { setError('Укажите имя длиной до 100 символов.'); return; }
    const passwordBytes = new TextEncoder().encode(password).length;
    if (passwordBytes < 8 || passwordBytes > 72) { setError('Пароль должен содержать от 8 до 72 байт.'); return; }
    setBusy(true);
    try {
      const tokens = await request(mode === 'login' ? 'login' : 'register', mode === 'login' ? { email, password } : { name: name.trim(), email, password });
      setSession(tokens);
      setPassword('');
      setConnectionError(false);
    } catch (err) {
      if (err.status === 409) setError('Аккаунт с таким email уже существует. Попробуйте войти.');
      else if (err.status === 401) setError('Неверный email или пароль. Проверьте данные и попробуйте ещё раз.');
      else if (err.status === 400) setError('Проверьте email и пароль и попробуйте ещё раз.');
      else setError(err.message || 'Что-то пошло не так. Попробуйте ещё раз.');
    } finally { setBusy(false); }
  }

  function switchMode(next) { setMode(next); setError(''); setShowPassword(false); setPassword(''); }

  async function logout() {
    loggingOutRef.current = true;
    setLoggingOut(true);
    setError('');
    try {
      await logoutSession();
      setSession(null);
      setProfile(null);
      setAvatarFile(null);
      setMode('login');
      setPassword('');
      setEmail('');
      setName('');
      setRetry(0);
      setConnectionError(false);
    } catch {
      setError('Не удалось выйти из аккаунта. Проверьте соединение и попробуйте ещё раз.');
      setRetry((value) => value + 1);
    } finally {
      loggingOutRef.current = false;
      setLoggingOut(false);
    }
  }

  if (!checking && session) {
    const userName = (isProfile && profile ? profileName.trim() : session.name) || session.email?.split('@')[0] || 'Пользователь';
    return (
      <main className="dashboard">
        <header className="dashboard-header">
          <Brand />
          <div className="profile-menu">
            <button className="profile-trigger" type="button" aria-expanded={menuOpen} aria-controls="profile-dropdown" onClick={() => setMenuOpen(!menuOpen)}>
              <span className="profile-avatar">{session.avatar_url ? <img src={`${API_URL}${session.avatar_url}`} alt="" /> : Array.from(userName)[0].toUpperCase()}</span>
              <span className="profile-label"><strong>{userName}</strong><small>{session.email || 'Аккаунт'}</small></span><span aria-hidden="true">⌄</span>
            </button>
            {menuOpen && <div className="profile-dropdown" id="profile-dropdown"><a href="/profile">Мой профиль</a><button className="profile-logout" type="button" onClick={logout} disabled={loggingOut}>{loggingOut ? 'Выходим…' : 'Выйти'}</button></div>}
          </div>
        </header>
        <section className="dashboard-content">
          {isProfile ? <>
            <div className="eyebrow"><span /> ВАШ АККАУНТ</div>
            <h1>Мой профиль<span className="accent">.</span></h1>
            <p className="dashboard-description">Просматривайте и обновляйте данные аккаунта.</p>
            <div className="profile-card">
              {profileLoading && !profile ? <p>Загружаем профиль…</p> : profile && <form onSubmit={saveProfile}>
                <div className="avatar-editor">
                  <span className="profile-avatar avatar-large">{avatarPreview || profile.avatar_url ? <img src={avatarPreview || `${API_URL}${profile.avatar_url}`} alt="Предпросмотр аватара" /> : Array.from(profile.name || 'П')[0].toUpperCase()}</span>
                  <div><label className="avatar-upload" htmlFor="avatar-file">Загрузить аватар</label><input className="avatar-input" id="avatar-file" type="file" accept="image/jpeg,image/png" onChange={chooseAvatar} /><p className="hint">JPEG или PNG, до 5 МБ.</p></div>
                </div>
                <label htmlFor="profile-name">Имя</label><input id="profile-name" value={profileName} onChange={(event) => { setProfileName(event.target.value); setProfileSaved(false); }} required />
                <label htmlFor="profile-email">Email</label><input id="profile-email" type="email" value={profile.email} readOnly />
                <p className="hint">Email изменить нельзя.</p>
                {profileSaved && <p className="profile-success" role="status">Изменения сохранены.</p>}
                {profileError && <div className="error" role="alert">{profileError}</div>}
                <button className="primary-button" disabled={busy || profileLoading} type="submit">{busy ? 'Сохраняем…' : 'Сохранить изменения'}</button>
              </form>}
              {!profile && profileError && <div className="error" role="alert">{profileError}</div>}
            </div>
          </> : <>
            <div className="eyebrow"><span /> ВАШЕ ПРОСТРАНСТВО</div>
            <h1>Добро пожаловать<span className="accent">.</span></h1>
            <p className="dashboard-description">Ваши точки мониторинга сайтов.</p>
            {error && <div className="notice" role="alert">{error}</div>}
            <Monitors apiUrl={API_URL} token={session.access_token} />
          </>}
        </section>
      </main>
    );
  }

  return (
    <main className="shell">
      <section className="auth-side">
        <div className="mobile-brand"><Brand /></div>
        <div className="auth-content">
          <div className="eyebrow"><span /> МОНИТОРИНГ БЕЗ ЛИШНЕГО</div>
          {checking ? (
            <div className="session-state" aria-live="polite"><h1>Одну минуту<span className="accent">.</span></h1><p>Проверяем вашу сессию.</p><div className="loader" /></div>
          ) : connectionError ? (
            <div className="session-state" aria-live="polite"><h1>Нет соединения<span className="accent">.</span></h1><p>{error}</p><button className="primary-button" onClick={() => { setChecking(true); restore(); }}>Попробовать снова</button></div>
          ) : (
            <>
              <h1>{mode === 'login' ? 'С возвращением' : 'Давайте начнём'}<span className="accent">.</span></h1>
              <p className="intro">{mode === 'login' ? 'Рады видеть вас снова. Войдите, чтобы продолжить.' : 'Создайте аккаунт и держите всё под контролем.'}</p>
              <div className="tabs" aria-label="Авторизация"><button type="button" className={mode === 'login' ? 'active' : ''} onClick={() => switchMode('login')}>Войти</button><button type="button" className={mode === 'register' ? 'active' : ''} onClick={() => switchMode('register')}>Создать аккаунт</button></div>
              <form onSubmit={submit}>
                {mode === 'register' && <><label htmlFor="name">Имя пользователя</label><input id="name" type="text" placeholder="Как к вам обращаться" autoComplete="name" required value={name} onChange={(event) => setName(event.target.value)} /></>}
                <label htmlFor="email">Email</label><input id="email" type="email" placeholder="you@example.com" autoComplete="email" required value={email} onChange={(event) => setEmail(event.target.value)} />
                <label htmlFor="password">Пароль</label><div className="password-field"><input id="password" type={showPassword ? 'text' : 'password'} placeholder={mode === 'login' ? 'Введите пароль' : 'Не менее 8 символов'} autoComplete={mode === 'login' ? 'current-password' : 'new-password'} required value={password} onChange={(event) => setPassword(event.target.value)} /><button type="button" className="show-password" onClick={() => setShowPassword(!showPassword)} aria-label={showPassword ? 'Скрыть пароль' : 'Показать пароль'}>{showPassword ? 'Скрыть' : 'Показать'}</button></div>
                {mode === 'register' && <p className="hint">От 8 до 72 байт. Используйте надёжный пароль.</p>}
                {error && <div className="error" role="alert">{error}</div>}
                <button className="primary-button" type="submit" disabled={busy}>{busy ? 'Подождите…' : mode === 'login' ? 'Войти в аккаунт' : 'Создать аккаунт'}</button>
              </form>
              <p className="switch-copy">{mode === 'login' ? 'Ещё нет аккаунта?' : 'Уже есть аккаунт?'} <button type="button" onClick={() => switchMode(mode === 'login' ? 'register' : 'login')}>{mode === 'login' ? 'Зарегистрироваться' : 'Войти'}</button></p>
            </>
          )}
        </div>
        <div className="auth-footer"><span>© {new Date().getFullYear()} uptime</span><span>Создано для спокойной работы</span></div>
      </section>
      <aside className="visual-side"><div className="visual-inner"><Brand /><div className="visual-main"><div className="visual-label"><span /> ВСЁ ПОД КОНТРОЛЕМ</div><h2>Спокойствие начинается с <em>uptime.</em></h2><p>Место, где состояние ваших сервисов всегда перед глазами.</p><MonitorGraphic /></div><div className="visual-bottom"><span>Меньше неизвестности.<br />Больше уверенности.</span><span className="visual-symbol">✳</span></div></div></aside>
    </main>
  );
}
