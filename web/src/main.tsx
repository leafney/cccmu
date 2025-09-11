import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { Toaster } from 'react-hot-toast'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
    <Toaster
      position="top-center"
      toastOptions={{
        duration: 3000,
        style: {
          background: 'rgba(0, 0, 0, 0.8)',
          color: '#fff',
          borderRadius: '8px',
          fontSize: '14px',
          backdropFilter: 'blur(10px)',
        },
      }}
    />
  </StrictMode>,
)
