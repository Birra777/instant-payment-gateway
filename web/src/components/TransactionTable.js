import React from 'react';

function TransactionTable({ transactions }) {
  if (!transactions || transactions.length === 0) {
    return <p>No transactions found.</p>;
  }

  const formatAmount = (amount, currency) => {
    return new Intl.NumberFormat('en-US', {
      style: 'currency',
      currency: currency || 'USD',
    }).format(amount / 100);
  };

  const formatDate = (dateStr) => {
    return new Date(dateStr).toLocaleString();
  };

  return (
    <div style={{ overflowX: 'auto' }}>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Amount</th>
            <th>Status</th>
            <th>Sender</th>
            <th>Receiver</th>
            <th>Description</th>
            <th>Created</th>
          </tr>
        </thead>
        <tbody>
          {transactions.map((txn) => (
            <tr key={txn.id}>
              <td title={txn.id}>{txn.id.substring(0, 8)}...</td>
              <td>{formatAmount(txn.amount, txn.currency)}</td>
              <td>
                <span className={`status-badge status-${txn.status}`}>
                  {txn.status}
                </span>
              </td>
              <td title={txn.sender_id}>{txn.sender_id.substring(0, 8)}...</td>
              <td title={txn.receiver_id}>{txn.receiver_id.substring(0, 8)}...</td>
              <td>{txn.description || '-'}</td>
              <td>{formatDate(txn.created_at)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default TransactionTable;
