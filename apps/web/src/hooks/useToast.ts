'use client';

import React from 'react';
import type { ToastMessage } from '../components/Toast';

let nextId = 0;

export function useToast() {
  const [messages, setMessages] = React.useState<ToastMessage[]>([]);

  const toast = React.useCallback((text: string, type: ToastMessage['type'] = 'info') => {
    const id = String(++nextId);
    setMessages(prev => [...prev, { id, text, type }]);
    setTimeout(() => {
      setMessages(prev => prev.filter(m => m.id !== id));
    }, 5000);
  }, []);

  const dismiss = React.useCallback((id: string) => {
    setMessages(prev => prev.filter(m => m.id !== id));
  }, []);

  return { messages, toast, dismiss };
}
