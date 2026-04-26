import React from 'react';
import { BrowserRouter as Router, Routes, Route, NavLink } from 'react-router-dom';
import TransactionList from './pages/TransactionList';
import NewPayment from './pages/NewPayment';
import TransactionDetail from './pages/TransactionDetail';
import './App.css';

function App() {
  return (
    <Router>
      <div className="app">
        <nav className="navbar">
          <div className="nav-brand">
            <span className="nav-logo">₦</span>
            <span className="nav-title">PayRoute</span>
            <span className="nav-sub">Cross-Border Payments</span>
          </div>
          <div className="nav-links">
            <NavLink to="/" end className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
              Transactions
            </NavLink>
            <NavLink to="/new" className={({ isActive }) => isActive ? 'nav-link active' : 'nav-link'}>
              + New Payment
            </NavLink>
          </div>
        </nav>
        <main className="main-content">
          <Routes>
            <Route path="/" element={<TransactionList />} />
            <Route path="/new" element={<NewPayment />} />
            <Route path="/payments/:id" element={<TransactionDetail />} />
          </Routes>
        </main>
      </div>
    </Router>
  );
}

export default App;
