import React from 'react';

interface GamePanelProps {
  chatMessages: Array<{ sender: string; text: string }>;
  onSendMessage: (text: string) => void;
  isChatDisabled: boolean;
  movHist: { n: string | number; w?: string; b?: string }[];
  engineNode: React.ReactNode;
}

export function GamePanel({
  chatMessages,
  onSendMessage,
  isChatDisabled,
  movHist,
  engineNode,
}: GamePanelProps) {
  const [activeTab, setActiveTab] = React.useState<'chat' | 'moves' | 'engine'>('moves');
  const [input, setInput] = React.useState('');
  const messagesEndRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages]);
  
  return (
    <div className="panel game-panel flex-col">
      <div className="game-panel__tabs">
        <button className={`game-panel__tab ${activeTab === 'chat' ? 'is-active' : ''}`} onClick={() => setActiveTab('chat')}>
          Chat {chatMessages.length > 0 && <span className="game-panel__tab-badge">{chatMessages.length}</span>}
        </button>
        <button className={`game-panel__tab ${activeTab === 'moves' ? 'is-active' : ''}`} onClick={() => setActiveTab('moves')}>
          Moves
        </button>
        <button className={`game-panel__tab ${activeTab === 'engine' ? 'is-active' : ''}`} onClick={() => setActiveTab('engine')}>
          Engine
        </button>
      </div>
      
      <div className="game-panel__content flex-1">
        {activeTab === 'chat' && (
          <div className="flex-col h-full">
            <div className="chat-log flex-1" style={{ overflowY: 'auto' }}>
              {chatMessages.length === 0 ? (
                <div className="empty-state__body text-center">No messages yet... say hi! 👋</div>
              ) : (
                chatMessages.map((msg, i) => {
                  const ts = (msg as any).timestamp ? new Date((msg as any).timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : null;
                  return (
                    <div key={i} className="chat-message" style={{ display: 'flex', gap: '6px', marginBottom: '4px', alignItems: 'baseline' }}>
                      <span className={`chat-message__sender chat-message__sender--${msg.sender}`} style={{ fontWeight: 'bold', color: msg.sender === 'white' ? '#ffe8a0' : '#b090f0', fontSize: '12px' }}>
                        {msg.sender === 'white' ? '⚪ W:' : '⚫ B:'}
                      </span>
                      <span className="chat-message__text" style={{ color: 'rgba(240,220,180,0.9)', fontSize: '12px' }}>{msg.text}</span>
                      {ts && <span className="chat-message__time" style={{ color: 'rgba(160,184,216,0.35)', fontSize: '9px', marginLeft: 'auto', whiteSpace: 'nowrap' }}>{ts}</span>}
                    </div>
                  );
                })
              )}
              <div ref={messagesEndRef} />
            </div>
            <div className="chat-input-row" style={{ display: 'flex', gap: '6px', marginTop: '8px' }}>
              <input
                className="input chat-input"
                aria-label="Chat message"
                style={{ flex: 1, padding: '8px', borderRadius: '4px', background: 'rgba(0,0,0,0.2)', border: '1px solid rgba(255,165,40,0.2)', color: '#fff' }}
                value={input}
                onChange={e => setInput(e.target.value)}
                disabled={isChatDisabled}
                placeholder={isChatDisabled ? 'Chat disabled' : 'Type a message...'}
                onKeyDown={e => {
                  if (e.key === 'Enter' && input.trim() && !isChatDisabled) {
                    onSendMessage(input.trim());
                    setInput('');
                  }
                }}
              />
              <button 
                className="btn-primary" 
                style={{ padding: '8px 12px', borderRadius: '4px', cursor: 'pointer' }}
                disabled={isChatDisabled || !input.trim()}
                onClick={() => {
                  if (input.trim() && !isChatDisabled) {
                    onSendMessage(input.trim());
                    setInput('');
                  }
                }}
              >
                Send
              </button>
            </div>
          </div>
        )}
        
        {activeTab === 'moves' && (
          <div className="moves-log flex-1" style={{ overflowY: 'auto' }}>
             {movHist.length === 0 ? (
               <div className="empty-state__body text-center">No moves played yet.</div>
             ) : (
               <div className="moves-grid">
                 {movHist.map((e, i) => (
                    <div key={i} className="move-row" style={{ display: 'flex', gap: '6px', padding: '3px 4px', fontFamily: 'monospace' }}>
                      <span className="move-number" style={{ color: '#95a5a6', width: '24px' }}>{e.n}.</span>
                      <span className="move-white mono" style={{ width: '50px', color: '#fff' }}>{e.w}</span>
                      <span className="move-black mono" style={{ width: '50px', color: '#bdc3c7' }}>{e.b || ''}</span>
                    </div>
                 ))}
               </div>
             )}
          </div>
        )}

        {activeTab === 'engine' && (
          <div className="engine-log flex-1 h-full">
            {engineNode}
          </div>
        )}
      </div>
    </div>
  );
}
