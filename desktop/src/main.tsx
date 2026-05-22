import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './styles/global.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);

// TypeScript: declare the window.piAPI type
declare global {
  interface Window {
    piAPI?: {
      getServerUrl: () => Promise<string | null>;
      startServer: () => Promise<{ url: string; port: number } | { error: string }>;
    };
  }
}
