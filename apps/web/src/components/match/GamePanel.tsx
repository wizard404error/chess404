import React from 'react';

interface GamePanelProps {
  chatMessages: Array<{ sender: string; text: string }>;
  onSendMessage: (text: string) => void;
  isChatDisabled: boolean;
  moves: string[];
}

export function GamePanel({
  chatMessages,
  onSendMessage,
  isChatDisabled,
  moves,
}: GamePanelProps) {
  const [activeTab, setActiveTab] = React.useState<'chat' | 'moves' | 'engine'>('chat');
  const [input, setInput] = React.useState('');
  
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
            <div className="chat-log flex-1">
              {chatMessages.length === 0 ? (
                <div className="empty-state__body text-center">No messages yet... say hi! 👋</div>
              ) : (
                chatMessages.map((msg, i) => (
                  <div key={i} className="chat-message">
                    <span className={`chat-message__sender chat-message__sender--${msg.sender}`}>
                      {msg.sender === 'white' ? '⚪ W:' : '⚫ B:'}
                    </span>
                    <span className="chat-message__text">{msg.text}</span>
                  </div>
                ))
              )}
            </div>
            <div className="chat-input-row">
              <input
                className="input chat-input"
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
          <div className="moves-log">
             {moves.length === 0 ? (
               <div className="empty-state__body text-center">No moves played yet.</div>
             ) : (
               <div className="moves-grid">
                 {/* Map through moves array, chunking by 2 for white/black turns */}
                 {Array.from({ length: Math.ceil(moves.length / 2) }).map((_, i) => (
                    <div key={i} className="move-row">
                      <span className="move-number">{i + 1}.</span>
                      <span className="move-white mono">{moves[i * 2]}</span>
                      <span className="move-black mono">{moves[i * 2 + 1] || ''}</span>
                    </div>
                 ))}
               </div>
             )}
          </div>
        )}
      </div>
    </div>
  );
}
