'use client';

import { useEffect, useRef, useState } from 'react';

import { HistoryChart, useMonitorHistory } from './monitor-history';

function SiteIcon({ url }) {
  const [sources, setSources] = useState(() => [new URL('/favicon.png', url).href, new URL('/favicon.ico', url).href]);
  const [index, setIndex] = useState(0);

  useEffect(() => {
    const abort = new AbortController();
    const site = new URL(url);
    const defaults = [new URL('/favicon.png', site).href, new URL('/favicon.ico', site).href];
    fetch(site.href, { signal: abort.signal, credentials: 'omit' })
      .then((response) => {
        if (!response.ok || !response.headers.get('content-type')?.includes('text/html')) throw new Error();
        return response.text();
      })
      .then((html) => {
        const document = new DOMParser().parseFromString(html, 'text/html');
        const icons = Array.from(document.querySelectorAll('link[rel~="icon"]'))
          .filter((link) => link.getAttribute('type') === 'image/png' || /\.png(?:[?#]|$)/i.test(link.getAttribute('href') || ''))
          .map((link) => { try { return new URL(link.getAttribute('href'), site).href; } catch { return ''; } })
          .filter((href) => /^https?:\/\//i.test(href));
        if (icons.length) { setSources([...new Set([...icons, ...defaults])]); setIndex(0); }
      })
      .catch(() => {});
    return () => abort.abort();
  }, [url]);

  return <span className="site-icon" aria-hidden="true">{sources[index] ? <img src={sources[index]} alt="" referrerPolicy="no-referrer" onError={() => setIndex((value) => value + 1)} /> : <span>◈</span>}</span>;
}

/** Render and manage the signed-in user’s monitor list. */
export default function Monitors({ apiUrl, token }) {
  const [items, setItems] = useState(null);
  const [period, setPeriod] = useState('1h');
  const history = useMonitorHistory(items, period, apiUrl, token);
  const [editing, setEditing] = useState(null);
  const [open, setOpen] = useState(false);
  const [url, setUrl] = useState('');
  const [amount, setAmount] = useState('5');
  const [unit, setUnit] = useState('60');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [retry, setRetry] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [deleting, setDeleting] = useState(null);
  const [deleteError, setDeleteError] = useState('');
  const dialogRef = useRef(null);
  const cursorRef = useRef(null);
  const tokenRef = useRef(token);
  tokenRef.current = token;

  useEffect(() => {
    const abort = new AbortController();
    fetch(`${apiUrl}/monitors`, { headers: { Authorization: `Bearer ${tokenRef.current}` }, signal: abort.signal })
      .then((response) => { if (!response.ok) throw new Error(); return response.json(); })
      .then((data) => { cursorRef.current = data.items.at(-1) || null; setItems(data.items); setHasMore(data.has_more); setError(''); })
      .catch((err) => { if (err.name !== 'AbortError') setError('Не удалось загрузить сайты. Попробуйте ещё раз.'); });
    return () => abort.abort();
  }, [apiUrl, retry]);

  useEffect(() => {
    if (deleting) dialogRef.current?.showModal();
  }, [deleting]);

  async function loadMore() {
    if (loadingMore || busy || !cursorRef.current) return;
    const last = cursorRef.current;
    const query = new URLSearchParams({ before: last.created_at, before_id: last.id });
    setLoadingMore(true); setError('');
    try {
      const response = await fetch(`${apiUrl}/monitors?${query}`, { headers: { Authorization: `Bearer ${tokenRef.current}` } });
      if (!response.ok) throw new Error();
      const data = await response.json();
      setItems((current) => {
        const ids = new Set(current.map((item) => item.id));
        return [...current, ...data.items.filter((item) => !ids.has(item.id))];
      });
      cursorRef.current = data.items.at(-1) || last;
      setHasMore(data.has_more);
    } catch { setError('Не удалось загрузить остальные сайты. Попробуйте ещё раз.'); }
    finally { setLoadingMore(false); }
  }

  function startForm(item) {
    setEditing(item);
    setUrl(item?.url || '');
    const seconds = item?.interval_seconds;
    const selectedUnit = !seconds ? 60 : seconds % 3600 === 0 ? 3600 : seconds % 60 === 0 ? 60 : 1;
    setUnit(String(selectedUnit));
    setAmount(String(seconds ? seconds / selectedUnit : 5));
    setError(''); setOpen(true);
  }

  async function save(event) {
    event.preventDefault();
    const interval = Number(amount) * Number(unit);
    if (!Number.isSafeInteger(interval) || interval < 1 || interval > 2147483647) {
      setError('Укажите целый интервал от 1 секунды до 2147483647 секунд.'); return;
    }
    setBusy(true); setError('');
    try {
      const response = await fetch(`${apiUrl}/monitors${editing ? `/${editing.id}` : ''}`, {
        method: editing ? 'PATCH' : 'POST',
        headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${tokenRef.current}` },
        body: JSON.stringify({ url: url.trim(), interval_seconds: interval }),
      });
      if (response.status === 409) throw new Error('Достигнут лимит: 1000 сайтов.');
      if (response.status === 400) throw new Error('Укажите корректный URL сайта с http:// или https:// и интервал.');
      if (response.status === 404) throw new Error('Сайт не найден. Обновите страницу.');
      if (!response.ok) throw new Error('Не удалось сохранить сайт. Попробуйте ещё раз.');
      const item = await response.json();
      setItems((current) => editing ? current.map((entry) => entry.id === item.id ? item : entry) : [item, ...current]);
      setOpen(false); setEditing(null);
    } catch (err) { setError(err.message || 'Не удалось подключиться к серверу.'); }
    finally { setBusy(false); }
  }

  async function remove() {
    if (busy || !deleting) return;
    setBusy(true); setDeleteError('');
    try {
      const response = await fetch(`${apiUrl}/monitors/${deleting.id}`, { method: 'DELETE', headers: { Authorization: `Bearer ${tokenRef.current}` } });
      if (response.status === 404) throw new Error('Сайт не найден. Обновите страницу.');
      if (!response.ok) throw new Error('Не удалось удалить сайт. Попробуйте ещё раз.');
      setItems((current) => current.filter((item) => item.id !== deleting.id));
      if (editing?.id === deleting.id) { setOpen(false); setEditing(null); }
      setDeleting(null);
    } catch (err) { setDeleteError(err.message || 'Не удалось подключиться к серверу.'); }
    finally { setBusy(false); }
  }

  return <>
    <div className="monitor-actions"><button className="primary-button add-monitor" type="button" onClick={() => open ? (setOpen(false), setEditing(null), setError('')) : startForm(null)} aria-expanded={open} disabled={items === null || busy || loadingMore}>{open ? 'Отмена' : 'Добавить сайт'}</button></div>
    {open && <form className="monitor-form" onSubmit={save}>
      <h2>{editing ? 'Редактирование сайта' : 'Новая точка мониторинга'}</h2>
      <label htmlFor="monitor-url">URL сайта</label><input id="monitor-url" type="url" placeholder="https://example.com" value={url} onChange={(event) => setUrl(event.target.value)} required maxLength={2048} />
      <label htmlFor="monitor-interval">Частота опроса</label>
      <div className="interval-fields"><input id="monitor-interval" type="number" min="1" step="1" value={amount} onChange={(event) => setAmount(event.target.value)} required /><select aria-label="Единица времени" value={unit} onChange={(event) => setUnit(event.target.value)}><option value="1">секунд</option><option value="60">минут</option><option value="3600">часов</option></select></div>
      <button className="primary-button" type="submit" disabled={busy || loadingMore}>{busy ? 'Сохраняем…' : editing ? 'Сохранить' : 'Создать'}</button>
    </form>}
    {error && <div className="notice monitor-notice" role="alert">{error}{items === null && <button type="button" onClick={() => setRetry((value) => value + 1)}>Повторить</button>}</div>}
    {!!items?.length && <label className="history-period">Период графиков
      <select value={period} onChange={(event) => setPeriod(event.target.value)}>
        <option value="1h">1 час</option><option value="24h">24 часа</option><option value="30d">30 дней</option>
      </select>
    </label>}
    {items === null ? !error && <p>Загружаем сайты…</p> : items.length === 0 && !hasMore ? <div className="dashboard-empty"><div className="dashboard-empty-icon">◈</div><h2>Всё начинается здесь</h2><p>Добавьте сайт, чтобы создать первую точку мониторинга.</p></div> : <div className="monitor-list">{items.map((item) => <div className="saved-monitor" key={item.id}><div className="saved-monitor-site"><SiteIcon key={item.url} url={item.url} /><div><strong>{item.url}</strong><p>Интервал: {item.interval_seconds % 3600 === 0 ? `${item.interval_seconds / 3600} ч` : item.interval_seconds % 60 === 0 ? `${item.interval_seconds / 60} мин` : `${item.interval_seconds} сек`}</p></div></div><div className="saved-monitor-actions"><button type="button" disabled={busy || loadingMore} onClick={() => startForm(item)}>Редактировать</button><button type="button" disabled={busy || loadingMore} onClick={() => { setDeleteError(''); setDeleting(item); }}>Удалить</button></div>
      <HistoryChart chart={history?.charts?.[item.id]} error={history?.error} />
    </div>)}</div>}
    {hasMore && <button className="primary-button add-monitor" type="button" onClick={loadMore} disabled={loadingMore || busy || !cursorRef.current}>{loadingMore ? 'Загружаем…' : 'Показать ещё'}</button>}
    {deleting && <dialog ref={dialogRef} className="delete-dialog" aria-labelledby="delete-title" aria-describedby="delete-description" onCancel={(event) => { event.preventDefault(); if (!busy) setDeleting(null); }}><h2 id="delete-title">Удалить сайт?</h2><p id="delete-description">Сайт <strong>{deleting.url}</strong> будет удалён без возможности восстановления.</p>{deleteError && <div className="notice" role="alert">{deleteError}</div>}<div className="delete-dialog-actions"><button type="button" disabled={busy} onClick={() => setDeleting(null)} autoFocus>Отмена</button><button type="button" className="delete-button" disabled={busy} onClick={remove}>{busy ? 'Удаляем…' : 'Удалить'}</button></div></dialog>}
  </>;
}
