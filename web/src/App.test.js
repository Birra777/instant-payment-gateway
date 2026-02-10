import React from 'react';
import { render, screen } from '@testing-library/react';
import '@testing-library/jest-dom';
import App from './App';

// Mock WebSocket
global.WebSocket = class {
  constructor() {
    setTimeout(() => this.onopen && this.onopen(), 0);
  }
  close() {}
  send() {}
};

// Mock axios
jest.mock('axios', () => ({
  get: jest.fn().mockResolvedValue({ data: [] }),
}));

test('renders dashboard header', () => {
  render(<App />);
  expect(screen.getByText('Payment Gateway Dashboard')).toBeInTheDocument();
});

test('renders stats cards', () => {
  render(<App />);
  expect(screen.getByText('Daily Volume')).toBeInTheDocument();
  expect(screen.getByText('Total Transactions')).toBeInTheDocument();
  expect(screen.getByText('Success Rate')).toBeInTheDocument();
});

test('renders filter controls', () => {
  render(<App />);
  expect(screen.getByText('All Statuses')).toBeInTheDocument();
  expect(screen.getByPlaceholderText('Account ID')).toBeInTheDocument();
});
