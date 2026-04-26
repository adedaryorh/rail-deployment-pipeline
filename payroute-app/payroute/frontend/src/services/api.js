import axios from 'axios';

const API_URL = process.env.REACT_APP_API_URL || '/app/api';

const api = axios.create({
  baseURL: API_URL,
  headers: { 'Content-Type': 'application/json' },
});

// Simple UUID v4 generator (no external dep needed)
function uuidv4() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0;
    return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
  });
}

export const getAccounts = () => api.get('/accounts');

export const getFXQuote = (from, to, amount) =>
  api.get('/fx/quote', { params: { from, to, amount } });

export const createPayment = (data) =>
  api.post('/payments', data, {
    headers: { 'Idempotency-Key': uuidv4() },
  });

export const getPayments = (params) => api.get('/payments', { params });

export const getPayment = (id) => api.get(`/payments/${id}`);

export const simulateWebhook = (providerReference, status) =>
  api.post('/webhooks/simulate', { provider_reference: providerReference, status });

export default api;
