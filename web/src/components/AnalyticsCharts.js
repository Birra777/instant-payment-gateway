import React, { useMemo } from 'react';
import {
  BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
  PieChart, Pie, Cell, Legend,
} from 'recharts';

const COLORS = {
  pending: '#f59e0b',
  authorized: '#3b82f6',
  settled: '#22c55e',
  failed: '#ef4444',
  refunded: '#6366f1',
};

function AnalyticsCharts({ analytics, transactions }) {
  const statusData = useMemo(() => {
    if (!transactions || transactions.length === 0) return [];
    const counts = {};
    transactions.forEach((t) => {
      counts[t.status] = (counts[t.status] || 0) + 1;
    });
    return Object.entries(counts).map(([name, value]) => ({ name, value }));
  }, [transactions]);

  const volumeData = useMemo(() => {
    if (!transactions || transactions.length === 0) return [];
    const daily = {};
    transactions.forEach((t) => {
      const date = new Date(t.created_at).toLocaleDateString();
      daily[date] = (daily[date] || 0) + t.amount;
    });
    return Object.entries(daily)
      .map(([date, amount]) => ({ date, amount: amount / 100 }))
      .sort((a, b) => new Date(a.date) - new Date(b.date));
  }, [transactions]);

  if (!analytics && (!transactions || transactions.length === 0)) {
    return <p>No data available yet.</p>;
  }

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '24px' }}>
      <div>
        <h3 style={{ fontSize: '14px', color: '#888', marginBottom: '12px' }}>
          Transaction Volume
        </h3>
        <div className="chart-container">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={volumeData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" fontSize={12} />
              <YAxis fontSize={12} />
              <Tooltip formatter={(value) => [`$${value.toFixed(2)}`, 'Volume']} />
              <Bar dataKey="amount" fill="#3b82f6" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      <div>
        <h3 style={{ fontSize: '14px', color: '#888', marginBottom: '12px' }}>
          Status Distribution
        </h3>
        <div className="chart-container">
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={statusData}
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={100}
                paddingAngle={5}
                dataKey="value"
              >
                {statusData.map((entry) => (
                  <Cell key={entry.name} fill={COLORS[entry.name] || '#ccc'} />
                ))}
              </Pie>
              <Tooltip />
              <Legend />
            </PieChart>
          </ResponsiveContainer>
        </div>
      </div>
    </div>
  );
}

export default AnalyticsCharts;
