'use client';

import React from 'react';

export function useMatchChat() {
  const [chatMessages, setChatMessages] = React.useState<{ sender: 'white' | 'black'; text: string }[]>([]);
  const [chatInput,    setChatInput]    = React.useState('');
  const chatRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    chatRef.current?.scrollTo({ top: chatRef.current.scrollHeight });
  }, [chatMessages]);

  const resetChat = React.useCallback(() => {
    setChatMessages([]);
    setChatInput('');
  }, []);

  return {
    chatMessages, setChatMessages,
    chatInput, setChatInput,
    chatRef,
    resetChat,
  };
}
