'use client';

import { useEffect, useState } from 'react';

const dateFormat = new Intl.DateTimeFormat('ru', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
const percent = new Intl.NumberFormat('ru', { style: 'percent', maximumFractionDigits: 1 });

export function useMonitorHistory(items, period, apiUrl, token) {
  const [state, setState] = useState(null);
  useEffect(() => {
    if (!items?.length) return;
    const controller = new AbortController();
    let timer;
    async function refresh() {
      try {
        const pages = [];
        for (let offset = 0; offset < items.length; offset += 50) {
          const ids = items.slice(offset, offset + 50).map((item) => item.id).join(',');
          pages.push(fetch(`${apiUrl}/monitors/history?${new URLSearchParams({ ids, period })}`, {
            headers: { Authorization: `Bearer ${token}` }, signal: controller.signal,
          }).then(async (response) => {
            if (!response.ok) throw new Error();
            const page = await response.json();
            const start = Date.parse(page.start);
            const step = page.step_seconds * 1000;
            const charts = Object.fromEntries(ids.split(',').map((id) => [id, {
              start, step, bars: Array(page.count).fill(null),
            }]));
            for (const bucket of page.buckets) {
              charts[bucket.monitor_id].bars[(Date.parse(bucket.start) - start) / step] = bucket;
            }
            return charts;
          }));
        }
        const charts = Object.assign({}, ...await Promise.all(pages));
        if (!controller.signal.aborted) setState({ items, period, charts });
      } catch {
        if (!controller.signal.aborted) setState({ items, period, error: true });
      } finally {
        if (!controller.signal.aborted) timer = setTimeout(refresh, 60000);
      }
    }
    refresh();
    return () => { controller.abort(); clearTimeout(timer); };
  }, [items, period, apiUrl, token]);
  return state?.items === items && state?.period === period ? state : null;
}

export function HistoryChart({ chart, error }) {
  const [selected, setSelected] = useState(null);
  if (!chart) return <p className="history-chart">{error ? 'Не удалось загрузить историю' : 'Загружаем историю…'}</p>;
  const { bars, start, step } = chart;
  const index = selected === null ? null : Math.min(selected, bars.length - 1);
  const visible = index === null ? bars : [bars[index]];
  let good = 0, bad = 0;
  for (const bar of visible) {
    good += bar?.successes || 0;
    bad += bar?.failures || 0;
  }
  const total = good + bad;
  const caption = total ? `Успешно ${percent.format(good / total)} · Ошибки ${percent.format(bad / total)} · Проверок: ${total}` : 'Нет данных';
  function navigate(event) {
    const last = bars.length - 1;
    const positions = { ArrowLeft: Math.max(0, (index ?? last) - 1), ArrowRight: Math.min(last, (index ?? last) + 1), Home: 0, End: last };
    if (Object.hasOwn(positions, event.key)) { event.preventDefault(); setSelected(positions[event.key]); }
  }
  return <div className="history-chart">
    <div className="history-bars" role="slider" tabIndex={0} aria-label="История проверок" aria-valuemin={1}
      aria-valuemax={bars.length} aria-valuenow={(index ?? bars.length - 1) + 1}
      aria-valuetext={index === null ? caption : `${dateFormat.format(start + index * step)}. ${caption}`}
      onFocus={() => setSelected(bars.length - 1)} onBlur={() => setSelected(null)} onKeyDown={navigate}
      onPointerMove={(event) => {
        const rect = event.currentTarget.getBoundingClientRect();
        setSelected(Math.max(0, Math.min(bars.length - 1, Math.floor((event.clientX - rect.left) / rect.width * bars.length))));
      }} onPointerLeave={() => setSelected(null)}>
      {bars.map((bar, i) => <span key={i} className={index === i ? 'selected' : ''} style={bar ? {
        background: `linear-gradient(to top, #288052 ${100 * bar.successes / (bar.successes + bar.failures)}%, #bf4e45 0)`,
      } : undefined} />)}
    </div>
    <p>{index !== null && dateFormat.format(start + index * step) + ' · '}{caption}</p>
  </div>;
}
