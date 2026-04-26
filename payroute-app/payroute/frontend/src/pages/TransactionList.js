import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { getPayments } from '../services/api';

const STATUS_OPTIONS = ['', 'initiated', 'processing', 'completed', 'failed', 'reversed'];

function StatusBadge({ status }) {
  return <span className={`badge badge-${status}`}>{status}</span>;
}

function formatAmount(amount, currency) {
  return new Intl.NumberFormat('en-NG', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(amount) + ' ' + currency;
}

function formatDate(dateStr) {
  return new Date(dateStr).toLocaleString('en-NG', {
    dateStyle: 'medium',
    timeStyle: 'short',
  });
}

export default function TransactionList() {
  const navigate = useNavigate();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [status, setStatus] = useState('');
  const [page, setPage] = useState(1);

  const fetchPayments = useCallback(async () => {
    setLoading(true);
    try {
      const res = await getPayments({ status: status || undefined, page, page_size: 10 });
      setData(res.data);
    } catch (e) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }, [status, page]);

  useEffect(() => { fetchPayments(); }, [fetchPayments]);

  const handleStatusChange = (e) => { setStatus(e.target.value); setPage(1); };

  const statusCounts = {};
  if (data?.transactions) {
    data.transactions.forEach(t => {
      statusCounts[t.status] = (statusCounts[t.status] || 0) + 1;
    });
  }

  return (
    <div>
      <div className="page-header">
        <h1 className="page-title">Transactions</h1>
        <p className="page-sub">{data?.total || 0} total payments processed</p>
      </div>

      {data && (
        <div className="stats-row">
          {['processing', 'completed', 'failed', 'reversed'].map(s => (
            <div className="stat-card" key={s}>
              <div className="stat-label">{s}</div>
              <div className={`stat-value badge-${s}`} style={{ fontSize: 28 }}>
                {data.transactions?.filter(t => t.status === s).length || 0}
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="filters">
        <select className="filter-select" value={status} onChange={handleStatusChange}>
          {STATUS_OPTIONS.map(s => (
            <option key={s} value={s}>{s || 'All statuses'}</option>
          ))}
        </select>
        <button className="btn btn-secondary" onClick={fetchPayments} style={{ marginLeft: 'auto' }}>
          ↻ Refresh
        </button>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <div className="loading"><span className="loading-spinner" />Loading transactions...</div>
      ) : (
        <>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Reference</th>
                  <th>Sender</th>
                  <th>Recipient</th>
                  <th>Source Amount</th>
                  <th>Destination</th>
                  <th>FX Rate</th>
                  <th>Status</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {!data?.transactions?.length ? (
                  <tr><td colSpan={8} style={{ textAlign: 'center', color: 'var(--text-muted)', padding: '40px' }}>
                    No transactions found
                  </td></tr>
                ) : data.transactions.map(tx => (
                  <tr key={tx.id} onClick={() => navigate(`/payments/${tx.id}`)}>
                    <td><span className="mono">{tx.reference}</span></td>
                    <td style={{ color: 'var(--text-secondary)' }}>
                      {tx.sender_account?.business?.name || tx.sender_account_id?.slice(0, 8) + '...'}
                    </td>
                    <td>
                      <div>{tx.recipient_name}</div>
                      <div style={{ color: 'var(--text-muted)', fontSize: 11 }}>{tx.recipient_country}</div>
                    </td>
                    <td className="mono">{formatAmount(tx.source_amount, 'NGN')}</td>
                    <td className="mono" style={{ color: 'var(--accent-green)' }}>
                      {formatAmount(tx.destination_amount, tx.destination_currency)}
                    </td>
                    <td className="mono" style={{ color: 'var(--text-muted)', fontSize: 11 }}>
                      {tx.fx_rate?.toFixed(6)}
                    </td>
                    <td><StatusBadge status={tx.status} /></td>
                    <td style={{ color: 'var(--text-muted)', fontSize: 12 }}>
                      {formatDate(tx.created_at)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {data?.total_pages > 1 && (
            <div className="pagination">
              <button className="page-btn" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>← Prev</button>
              {Array.from({ length: data.total_pages }, (_, i) => i + 1).map(p => (
                <button
                  key={p}
                  className={`page-btn${p === page ? ' active' : ''}`}
                  onClick={() => setPage(p)}
                >{p}</button>
              ))}
              <button className="page-btn" disabled={page >= data.total_pages} onClick={() => setPage(p => p + 1)}>Next →</button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
