import React, { useState, useEffect, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { getPayment, simulateWebhook } from '../services/api';

function StatusBadge({ status }) {
  return <span className={`badge badge-${status}`}>{status}</span>;
}

function formatDate(d) {
  if (!d) return '—';
  return new Date(d).toLocaleString('en-NG', { dateStyle: 'medium', timeStyle: 'short' });
}

// Real timeline from status_history table
function StatusTimeline({ history, currentStatus }) {
  if (!history || history.length === 0) {
    return <div style={{ color: 'var(--text-muted)', fontSize: 13 }}>No history available</div>;
  }

  return (
    <div className="timeline">
      {history.map((entry, i) => {
        const isLast = i === history.length - 1;
        const isFailed = entry.to_status === 'failed' || entry.to_status === 'reversed';
        const dotClass = isFailed ? 'failed' : isLast ? 'active' : 'done';

        return (
          <div className="timeline-item" key={entry.id}>
            <div className={`timeline-dot ${dotClass}`} />
            <div className="timeline-label" style={{
              color: isFailed ? 'var(--accent-red)' :
                     isLast ? 'var(--accent-blue)' :
                     'var(--accent-green)'
            }}>
              {entry.from_status
                ? `${entry.from_status} → ${entry.to_status}`
                : entry.to_status}
            </div>
            {entry.reason && (
              <div style={{ fontSize: 12, color: 'var(--text-muted)', marginTop: 2 }}>
                {entry.reason}
              </div>
            )}
            <div className="timeline-time">{formatDate(entry.created_at)}</div>
          </div>
        );
      })}
    </div>
  );
}

function LedgerEntries({ entries }) {
  if (!entries?.length) {
    return <div style={{ color: 'var(--text-muted)', padding: 16, fontSize: 13 }}>No ledger entries</div>;
  }
  return (
    <div>
      {entries.map(entry => (
        <div className="ledger-entry" key={entry.id}>
          <div>
            <div style={{ fontSize: 13 }}>
              {entry.account?.business?.name || 'Account ' + (entry.account_id?.slice(0, 8) + '...')}
            </div>
            <div style={{ fontSize: 11, color: 'var(--text-muted)', fontFamily: 'var(--mono)' }}>
              {entry.currency} · {entry.entry_type.toUpperCase()}
            </div>
          </div>
          <div className={entry.entry_type === 'debit' ? 'ledger-debit' : 'ledger-credit'}>
            {entry.entry_type === 'debit' ? '−' : '+'}
            {Number(entry.amount).toLocaleString('en-NG', { minimumFractionDigits: 2 })} {entry.currency}
          </div>
        </div>
      ))}
      {/* Double-entry balance assertion */}
      {(() => {
        const ngnEntries = entries.filter(e => e.currency === 'NGN');
        const debits = ngnEntries.filter(e => e.entry_type === 'debit').reduce((s, e) => s + Number(e.amount), 0);
        const credits = ngnEntries.filter(e => e.entry_type === 'credit').reduce((s, e) => s + Number(e.amount), 0);
        if (debits > 0 && credits > 0) {
          const balanced = Math.abs(debits - credits) < 0.01;
          return (
            <div style={{
              padding: '8px 16px',
              fontSize: 11,
              color: balanced ? 'var(--accent-green)' : 'var(--accent-red)',
              borderTop: '1px solid var(--border)',
              fontFamily: 'var(--mono)'
            }}>
              {balanced ? '✓ NGN ledger balanced' : `⚠ NGN ledger imbalance: debits=${debits.toFixed(2)} credits=${credits.toFixed(2)}`}
            </div>
          );
        }
        return null;
      })()}
    </div>
  );
}

export default function TransactionDetail() {
  const { id } = useParams();
  const [data, setData] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [simulating, setSimulating] = useState(false);
  const [simMsg, setSimMsg] = useState('');

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await getPayment(id);
      setData(res.data);
    } catch (e) {
      setError(e.response?.data?.error || e.message);
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleSimulate = async (status) => {
    if (!data?.transaction?.provider_reference) return;
    setSimulating(true);
    setSimMsg('');
    try {
      await simulateWebhook(data.transaction.provider_reference, status);
      setSimMsg(`Webhook simulated: ${status}`);
      setTimeout(fetchData, 800);
    } catch (e) {
      setSimMsg('Error: ' + (e.response?.data?.error || e.message));
    } finally {
      setSimulating(false);
    }
  };

  if (loading) return <div className="loading"><span className="loading-spinner" />Loading transaction...</div>;
  if (error) return <div className="alert alert-error">{error}</div>;
  if (!data) return null;

  const { transaction: tx, fx_quote, ledger_entries, status_history } = data;

  return (
    <div>
      <Link to="/" className="back-link">← Back to Transactions</Link>

      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16, marginBottom: 28 }}>
        <div>
          <h1 className="page-title" style={{ fontFamily: 'var(--mono)', fontSize: 18 }}>{tx.reference}</h1>
          <p className="page-sub">Created {formatDate(tx.created_at)}</p>
        </div>
        <div style={{ marginLeft: 'auto' }}>
          <StatusBadge status={tx.status} />
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 300px', gap: 20 }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>

          {/* Payment Details */}
          <div className="card">
            <div className="card-header">
              <span className="card-title">Payment Details</span>
            </div>
            <div className="meta-grid">
              {[
                ['Reference', tx.reference, true],
                ['Recipient', tx.recipient_name, false],
                ['Country', tx.recipient_country, false],
                ['Source Amount', `₦${Number(tx.source_amount).toLocaleString('en-NG', { minimumFractionDigits: 2 })}`, true],
                ['Destination Amount', `${Number(tx.destination_amount).toFixed(2)} ${tx.destination_currency}`, true],
                ['FX Rate', tx.fx_rate?.toFixed(8), true],
                ['Provider Ref', tx.provider_reference || '—', true],
                ['Completed', formatDate(tx.completed_at), false],
              ].map(([key, val, isMono]) => (
                <div className="meta-item" key={key}>
                  <div className="meta-key">{key}</div>
                  <div className={`meta-val${isMono ? ' mono' : ''}`}>{val}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Ledger */}
          <div className="card">
            <div className="card-header">
              <span className="card-title">Ledger Entries</span>
              <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>
                {ledger_entries?.length || 0} entries
              </span>
            </div>
            <LedgerEntries entries={ledger_entries} />
          </div>

          {/* Dev tool — only when processing */}
          {tx.status === 'processing' && tx.provider_reference && (
            <div className="simulate-box">
              <div className="simulate-title">⚡ Simulate Provider Webhook</div>
              <p style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 12 }}>
                Trigger a simulated provider callback to test the full lifecycle.
                Provider ref: <span style={{ fontFamily: 'var(--mono)', color: 'var(--accent-blue)' }}>{tx.provider_reference}</span>
              </p>
              {simMsg && <div className="alert alert-info" style={{ marginBottom: 10 }}>{simMsg}</div>}
              <div style={{ display: 'flex', gap: 10 }}>
                <button className="btn btn-primary" onClick={() => handleSimulate('completed')} disabled={simulating}>
                  {simulating ? <span className="loading-spinner" /> : '✓'} Simulate Completed
                </button>
                <button className="btn btn-danger" onClick={() => handleSimulate('failed')} disabled={simulating}>
                  {simulating ? <span className="loading-spinner" /> : '✗'} Simulate Failed
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Right column */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 20 }}>

          {/* Timeline from real DB history */}
          <div className="card">
            <div className="card-header">
              <span className="card-title">Status Timeline</span>
            </div>
            <StatusTimeline history={status_history} currentStatus={tx.status} />
          </div>

          {/* FX Quote */}
          {fx_quote && (
            <div className="card">
              <div className="card-header">
                <span className="card-title">FX Quote</span>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {[
                  ['Pair', `${fx_quote.from_currency} / ${fx_quote.to_currency}`],
                  ['Rate', fx_quote.rate?.toFixed(8)],
                  ['Quoted at', new Date(fx_quote.created_at).toLocaleTimeString('en-NG')],
                  ['Expires', new Date(fx_quote.expires_at).toLocaleTimeString('en-NG')],
                ].map(([label, val]) => (
                  <div className="fx-row" key={label}>
                    <span className="fx-label">{label}</span>
                    <span className="fx-value" style={{ fontSize: 13 }}>{val}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
