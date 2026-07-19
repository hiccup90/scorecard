import { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import AdminApp from './pages/AdminApp';
import ChildApp from './pages/ChildApp';
import './styles.css';

function App() {
  const [mode, setMode] = useState(location.pathname.startsWith('/admin') ? 'admin' : 'child');

  useEffect(() => {
    history.replaceState(null, '', mode === 'admin' ? '/admin' : '/');
  }, [mode]);

  return mode === 'admin'
    ? <AdminApp onChild={() => setMode('child')} />
    : <ChildApp onAdmin={() => setMode('admin')} />;
}

createRoot(document.getElementById('root')).render(<App />);
