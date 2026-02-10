import React, { useState, useEffect, useCallback } from 'react';
import axios from 'axios';
import { useWebSocket } from './hooks/useWebSocket';
import TransactionTable from './components/TransactionTable';
import AnalyticsCharts from './components/AnalyticsCharts';

const API_BASE = process.env.REACT_APP_API_URL || 'http://localhost:8080/api/v1';

function App() {
  const [transactions, setTransactions] = useState([]);
  const [analytics, setAnalytics] = useState(null);
  const [filter, setFilter] = useState({ status: '', account_id: '' });
  const [loading, setLoading] = useState(true);

  const handleWsMessage = useCallback((msg) => {
    if (msg.event && msg.event.startsWith('payment.')) {
      setTransactions((prev) => {
        const updated = prev.filter((t) => t.id !== msg.data.id);
        return [msg.data, ...updated];
      });
      fetchAnalytics();
    }
  }, []);

  const connected = useWebSocket(handleWsMessage);

  const fetchTransactions = useCallback(async () => {
    try {
      const params = {};
      if (filter.status) params.status = filter.status;
      if (filter.account_id) params.account_id = filter.account_id;
      const res = await axios.get(`${API_BASE}/payments`, { params });
      setTransactions(res.data || []);
    } catch (err) {
      console.error('Failed to fetch transactions:', err);
    } finally {
      setLoading(false);
    }
  }, [filter]);

  const fetchAnalytics = useCallback(async () => {
    try {
      const res = await axios.get(`${API_BASE}/analytics`);
      setAnalytics(res.data);
    } catch (err) {
      console.error('Failed to fetch analytics:', err);
    }
  }, []);

  useEffect(() => {
    fetchTransactions();
    fetchAnalytics();
  }, [fetchTransactions, fetchAnalytics]);

  return (
    <div className="app">
      <header>
        <h1>Payment Gateway Dashboard</h1>
        <div className="ws-status">
          <span className={`ws-dot ${connected ? 'connected' : 'disconnected'}`} />
          {connected ? 'Live' : 'Reconnecting...'}
        </div>
      </header>

      <div className="stats-grid">
        <div className="stat-card">
          <div className="label">Daily Volume</div>
          <div className="value info">
            ${analytics ? (analytics.daily_volume / 100).toLocaleString('en-US', { minimumFractionDigits: 2 }) : '0.00'}
          </div>
        </div>
        <div className="stat-card">
          <div className="label">Total Transactions</div>
          <div className="value">{analytics?.total_count || 0}</div>
        </div>
        <div className="stat-card">
          <div className="label">Success Rate</div>
          <div className="value success">
            {analytics ? `${analytics.success_rate.toFixed(1)}%` : '0%'}
          </div>
        </div>
        <div className="stat-card">
          <div className="label">Failed</div>
          <div className="value danger">{analytics?.failure_count || 0}</div>
        </div>
      </div>

      <div className="section">
        <h2>Analytics</h2>
        <AnalyticsCharts analytics={analytics} transactions={transactions} />
      </div>

      <div className="section">
        <h2>Transactions</h2>
        <div className="filters">
          <select
            value={filter.status}
            onChange={(e) => setFilter({ ...filter, status: e.target.value })}
          >
            <option value="">All Statuses</option>
            <option value="pending">Pending</option>
            <option value="authorized">Authorized</option>
            <option value="settled">Settled</option>
            <option value="failed">Failed</option>
            <option value="refunded">Refunded</option>
          </select>
          <input
            type="text"
            placeholder="Account ID"
            value={filter.account_id}
            onChange={(e) => setFilter({ ...filter, account_id: e.target.value })}
          />
        </div>
        {loading ? (
          <p>Loading transactions...</p>
        ) : (
          <TransactionTable transactions={transactions} />
        )}
      </div>
    </div>
  );
}

export default App;
