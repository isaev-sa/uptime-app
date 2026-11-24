'use client';

import { useEffect, useRef, useState } from 'react';

/** Render and manage the signed-in user’s monitor list. */
export default function Monitors({ apiUrl, token }) {
  const [items, setItems] = useState(null);
  const [open, setOpen] = useState(false);
  const [url, setUrl] = useState('');
  const [amount, setAmount] = useState('5');
  const [unit, setUnit] = useState('60');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [retry, setRetry] = useState(0);
  const [hasMore, setHasMore] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const tokenRef = useRef(token);
  tokenRef.current = token;

  useEffect(() => {
    const abort = new AbortController();
    fetch(`${apiUrl}/monitors`, { headers: { Authorization: `Bearer ${tokenRef.current}` }, signal: abort.signal })
      .then((response) => { if (!response.ok) throw new Error(); return response.json(); })
      .then((data) => { setItems(data.items); setHasMore(data.has_more); setError(''); })
      .catch((err) => { if (err.name !== 'AbortError') setError('Не удалось загрузить сайты. Попробуйте ещё раз.'); });
    return () => abort.abort();
  }, [apiUrl, retry]);

  async function loadMore() {
    if (loadingMore || !items?.length) return;
    const last = items[items.length - 1];
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
      setHasMore(data.has_more);
    } catch { setError('Не удалось загрузить остальные сайты. Попробуйте ещё раз.'); }
    finally { setLoadingMore(false); }
  }

  async function create(event) {
    event.preventDefault();
    const interval = Number(amount) * Number(unit);
    if (!Number.isSafeInteger(interval) || interval < 1 || interval > 2147483647) {
      setError('Укажите целый интервал от 1 секунды до 2147483647 секунд.'); return;
    }
    setBusy(true); setError('');
    try {
      const response = await fetch(`${apiUrl}/monitors`, { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` }, body: JSON.stringify({ url: url.trim(), interval_seconds: interval }) });
      if (response.status === 409) throw new Error('Достигнут лимит: 1000 сайтов.');
      if (response.status === 400) throw new Error('Укажите корректный URL сайта с http:// или https:// и интервал.');
      if (!response.ok) throw new Error('Не удалось создать точку. Попробуйте ещё раз.');
      const item = await response.json();
      setItems((current) => current ? [item, ...current] : [item]);
      setUrl(''); setOpen(false);
    } catch (err) { setError(err.message || 'Не удалось подключиться к серверу.'); }
    finally { setBusy(false); }
  }

  return <>
    <div className="monitor-actions"><button className="primary-button add-monitor" type="button" onClick={() => { setOpen(!open); setError(''); }} aria-expanded={open} disabled={items === null}>{open ? 'Отмена' : 'Добавить сайт'}</button></div>
    {open && <form className="monitor-form" onSubmit={create}>
      <h2>Новая точка мониторинга</h2>
      <label htmlFor="monitor-url">URL сайта</label><input id="monitor-url" type="url" placeholder="https://example.com" value={url} onChange={(event) => setUrl(event.target.value)} required maxLength={2048} />
      <label htmlFor="monitor-interval">Частота опроса</label>
      <div className="interval-fields"><input id="monitor-interval" type="number" min="1" step="1" value={amount} onChange={(event) => setAmount(event.target.value)} required /><select aria-label="Единица времени" value={unit} onChange={(event) => setUnit(event.target.value)}><option value="1">секунд</option><option value="60">минут</option><option value="3600">часов</option></select></div>
      <button className="primary-button" type="submit" disabled={busy}>{busy ? 'Создаём…' : 'Создать'}</button>
    </form>}
    {error && <div className="notice monitor-notice" role="alert">{error}{items === null && <button type="button" onClick={() => setRetry((value) => value + 1)}>Повторить</button>}</div>}
    {items === null ? !error && <p>Загружаем сайты…</p> : items.length === 0 ? <div className="dashboard-empty"><div className="dashboard-empty-icon">◈</div><h2>Всё начинается здесь</h2><p>Добавьте сайт, чтобы создать первую точку мониторинга.</p></div> : <div className="monitor-list">{items.map((item) => <div className="saved-monitor" key={item.id}><div><strong>{item.url}</strong><p>Интервал: {item.interval_seconds % 3600 === 0 ? `${item.interval_seconds / 3600} ч` : item.interval_seconds % 60 === 0 ? `${item.interval_seconds / 60} мин` : `${item.interval_seconds} сек`}</p></div><span>Создана</span></div>)}</div>}
    {hasMore && <button className="primary-button add-monitor" type="button" onClick={loadMore} disabled={loadingMore}>{loadingMore ? 'Загружаем…' : 'Показать ещё'}</button>}
  </>;
}
