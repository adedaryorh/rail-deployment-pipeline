import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { getAccounts, getFXQuote, createPayment } from '../services/api';

const CURRENCIES = ['USD', 'EUR', 'GBP'];
const COUNTRIES = ['United States', 'United Kingdom', 'Germany', 'France', 'China', 'Japan', 'Canada', 'Australia'];

export default function NewPayment() {
  const navigate = useNavigate();
  const [accounts, setAccounts] = useState([]);
  const [form, setForm] = useState({
    sender_account_id: '',
    recipient_name: '',
    recipient_country: '',
    destination_currency: 'USD',
    amount: '',
  });
  const [quote, setQuote] = useState(null);
  const [quoteLoading, setQuoteLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  useEffect(() => {
    getAccounts().then(res => {
      const ngn = res.data.accounts?.filter(a => a.currency === 'NGN') || [];
      setAccounts(ngn);
      if (ngn.length > 0) setForm(f => ({ ...f, sender_account_id: ngn[0].id }));
    }).catch(() => {});
  }, []);

  const fetchQuote = useCallback(async () => {
    if (!form.amount || !form.sender_account_id || parseFloat(form.amount) <= 0) {
      setQuote(null);
      return;
    }
    setQuoteLoading(true);
    try {
      const res = await getFXQuote('NGN', form.destination_currency, form.amount);
      setQuote(res.data);
    } catch (e) {
      setQuote(null);
    } finally {
      setQuoteLoading(false);
    }
  }, [form.amount, form.destination_currency, form.sender_account_id]);

  useEffect(() => {
    const timer = setTimeout(fetchQuote, 600);
    return () => clearTimeout(timer);
  }, [fetchQuote]);

  const handleChange = (e) => {
    const { name, value } = e.target;
    setForm(f => ({ ...f, [name]: value }));
    setError('');
  };

  const selectedAccount = accounts.find(a => a.id === form.sender_account_id);

  const handleSubmit = async (e) => {
    e.preventDefault();
    if (!form.amount || parseFloat(form.amount) <= 0) {
      setError('Please enter a valid amount');
      return;
    }
    if (selectedAccount && parseFloat(form.amount) > selectedAccount.balance) {
      setError('Amount exceeds account balance');
      return;
    }

    setSubmitting(true);
    setError('');
    try {
      const res = await createPayment({
        ...form,
        amount: parseFloat(form.amount),
      });
      setSuccess('Payment initiated successfully!');
      setTimeout(() => navigate(`/payments/${res.data.transaction.id}`), 1200);
    } catch (e) {
      setError(e.response?.data?.error || e.message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div style={{ maxWidth: 600, margin: '0 auto' }}>
      <div className="page-header">
        <h1 className="page-title">New Payment</h1>
        <p className="page-sub">Send money to international suppliers</p>
      </div>

      {error && <div className="alert alert-error">{error}</div>}
      {success && <div className="alert alert-success">{success}</div>}

      <div className="card">
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Source Account</label>
            <select name="sender_account_id" value={form.sender_account_id} onChange={handleChange} required>
              {accounts.map(a => (
                <option key={a.id} value={a.id}>
                  {a.business_name} — {a.currency} (Balance: {Number(a.balance).toLocaleString('en-NG')})
                </option>
              ))}
            </select>
            {selectedAccount && (
              <div style={{ marginTop: 6, fontSize: 12, color: 'var(--text-muted)' }}>
                Available: ₦{Number(selectedAccount.balance).toLocaleString('en-NG')}
              </div>
            )}
          </div>

          <div className="form-grid">
            <div className="form-group">
              <label>Recipient Name</label>
              <input
                name="recipient_name"
                value={form.recipient_name}
                onChange={handleChange}
                placeholder="Acme Corp Ltd"
                required
              />
            </div>
            <div className="form-group">
              <label>Recipient Country</label>
              <select name="recipient_country" value={form.recipient_country} onChange={handleChange} required>
                <option value="">Select country</option>
                {COUNTRIES.map(c => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
          </div>

          <div className="form-grid">
            <div className="form-group">
              <label>Amount (NGN)</label>
              <input
                name="amount"
                type="number"
                value={form.amount}
                onChange={handleChange}
                placeholder="500000"
                min="1"
                step="any"
                required
              />
            </div>
            <div className="form-group">
              <label>Destination Currency</label>
              <select name="destination_currency" value={form.destination_currency} onChange={handleChange}>
                {CURRENCIES.map(c => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
          </div>

          {/* FX Preview */}
          {form.amount && parseFloat(form.amount) > 0 && (
            <div className="fx-preview">
              {quoteLoading ? (
                <div style={{ textAlign: 'center', color: 'var(--text-muted)', fontSize: 13 }}>
                  <span className="loading-spinner" />Fetching live rate...
                </div>
              ) : quote ? (
                <>
                  <div className="fx-row">
                    <span className="fx-label">Exchange Rate</span>
                    <span className="fx-value">
                      1 NGN = {quote.quote?.rate?.toFixed(6)} {form.destination_currency}
                    </span>
                  </div>
                  <div className="fx-row">
                    <span className="fx-label">You Send</span>
                    <span className="fx-value">₦{Number(form.amount).toLocaleString('en-NG')}</span>
                  </div>
                  <div style={{ borderTop: '1px solid var(--border)', margin: '10px 0' }} />
                  <div className="fx-row">
                    <span className="fx-label">Recipient Gets</span>
                    <span className="fx-value fx-big">
                      {Number(quote.destination_amount).toFixed(2)} {form.destination_currency}
                    </span>
                  </div>
                  <div style={{ marginTop: 8, fontSize: 11, color: 'var(--text-muted)' }}>
                    Quote expires in 5 minutes
                  </div>
                </>
              ) : null}
            </div>
          )}

          <div style={{ display: 'flex', gap: 12, marginTop: 8 }}>
            <button type="button" className="btn btn-secondary" onClick={() => navigate('/')}>
              Cancel
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={submitting || !form.recipient_name || !form.recipient_country || !form.amount}
              style={{ flex: 1 }}
            >
              {submitting ? <><span className="loading-spinner" />Processing...</> : '→ Send Payment'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
