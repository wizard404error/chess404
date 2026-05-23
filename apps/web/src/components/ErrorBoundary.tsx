'use client';

import React from 'react';

interface ErrorBoundaryProps {
  children: React.ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('[ErrorBoundary] Uncaught error:', error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100vh',
          padding: '32px',
          fontFamily: "'Segoe UI', sans-serif",
          background: 'linear-gradient(160deg, rgba(8,4,20,0.98) 0%, rgba(15,6,30,0.98) 100%)',
          color: '#f4efe6',
          textAlign: 'center',
          gap: '16px',
        }}>
          <div style={{ fontSize: '48px', marginBottom: '8px' }}>⚠️</div>
          <h1 style={{ fontSize: '24px', fontWeight: 800, margin: 0, color: '#ffd700' }}>
            Something went wrong
          </h1>
          <p style={{ fontSize: '14px', color: 'rgba(244,239,230,0.7)', maxWidth: '480px', lineHeight: 1.6 }}>
            An unexpected error occurred while rendering this page. Please try refreshing the application.
          </p>
          <div style={{
            padding: '12px 16px',
            background: 'rgba(220,40,40,0.12)',
            border: '1px solid rgba(220,60,60,0.35)',
            borderRadius: '10px',
            fontSize: '12px',
            color: '#ff8a8a',
            maxWidth: '100%',
            overflow: 'auto',
            textAlign: 'left',
            fontFamily: 'monospace',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
          }}>
            {this.state.error?.message ?? 'Unknown error'}
          </div>
          <button
            onClick={() => {
              this.setState({ hasError: false, error: null });
              window.location.reload();
            }}
            style={{
              padding: '12px 24px',
              borderRadius: '10px',
              border: '1px solid rgba(255,215,0,0.4)',
              background: 'linear-gradient(180deg, #c8860a 0%, #7a5008 100%)',
              color: '#fff8e0',
              fontWeight: 700,
              fontSize: '14px',
              cursor: 'pointer',
              marginTop: '8px',
            }}
          >
            Reload Application
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
