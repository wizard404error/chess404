'use client';

import React from 'react';
import { submitPlayerReport } from '../../lib/platform-service';

interface GamePanelProps {
  chatMessages: Array<{ sender: string; text: string; senderAccountId?: string }>;
  onSendMessage: (text: string) => void;
  isChatDisabled: boolean;
  movHist: { n: string | number; w?: string; b?: string }[];
  engineNode: React.ReactNode;
  accountId?: string;
  sessionToken?: string;
  currentSender?: string;
}

export function GamePanel({
  chatMessages,
  onSendMessage,
  isChatDisabled,
  movHist,
  engineNode,
  accountId,
  sessionToken,
  currentSender,
}: GamePanelProps) {
  const [activeTab, setActiveTab] = React.useState<'chat' | 'moves' | 'engine'>('moves');
  const [input, setInput] = React.useState('');
  const [reportingMsgIdx, setReportingMsgIdx] = React.useState<number | null>(null);
  const [reportCategory, setReportCategory] = React.useState('abuse');
  const [reportDetails, setReportDetails] = React.useState('');
  const [reportSubmitted, setReportSubmitted] = React.useState(false);
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
                    <React.Fragment key={i}>
                      <div className="chat-message" style={{ display: 'flex', gap: '6px', marginBottom: '4px', alignItems: 'baseline' }}>
                        <span className={`chat-message__sender chat-message__sender--${msg.sender}`} style={{ fontWeight: 'bold', color: msg.sender === 'white' ? '#ffe8a0' : '#b090f0', fontSize: '12px' }}>
                          {msg.sender === 'white' ? '⚪ W:' : '⚫ B:'}
                        </span>
                        <span className="chat-message__text" style={{ color: 'rgba(240,220,180,0.9)', fontSize: '12px' }}>{msg.text}</span>
                        {ts && <span className="chat-message__time" style={{ color: 'rgba(160,184,216,0.6)', fontSize: '9px', marginLeft: 'auto', whiteSpace: 'nowrap' }}>{ts}</span>}
                        {msg.sender !== currentSender && (
                          <button onClick={() => { setReportingMsgIdx(reportingMsgIdx === i ? null : i); setReportSubmitted(false); setReportDetails(''); }} style={{ background:'none', border:'none', color:'rgba(255,255,255,0.3)', cursor:'pointer', fontSize:'10px', padding:'0 2px', marginLeft:'4px' }} aria-label="Report this message" title="Report">⚑</button>
                        )}
                      </div>
                      {reportingMsgIdx === i && (
                        <div style={{ display:'flex', flexDirection:'column', gap:'4px', marginTop:'4px', padding:'4px', background:'rgba(255,255,255,0.05)', borderRadius:'4px', fontSize:'11px' }}>
                          {reportSubmitted ? (
                            <span style={{ color:'#8f8' }}>Report submitted</span>
                          ) : (
                            <>
                              <select value={reportCategory} onChange={e=>setReportCategory(e.target.value)} style={{ fontSize:'11px', padding:'2px', background:'#1a1f2e', color:'#d0d8e8', border:'1px solid #2a3040', borderRadius:'2px' }}>
                                <option value="abuse">Abuse</option>
                                <option value="harassment">Harassment</option>
                                <option value="spam">Spam</option>
                                <option value="other">Other</option>
                              </select>
                              <textarea value={reportDetails} onChange={e=>setReportDetails(e.target.value)} placeholder="Details (optional)" rows={2} style={{ fontSize:'11px', padding:'2px', background:'#1a1f2e', color:'#d0d8e8', border:'1px solid #2a3040', borderRadius:'2px', resize:'none' }}/>
                              <div style={{ display:'flex', gap:'4px' }}>
                                <button onClick={async () => {
                                  if (accountId && sessionToken) {
                                    await submitPlayerReport({ accountId, sessionToken, targetAccountId: msg.sender, category: reportCategory, details: reportDetails });
                                    setReportSubmitted(true);
                                    setTimeout(() => { setReportingMsgIdx(null); setReportSubmitted(false); }, 2000);
                                  }
                                }} style={{ fontSize:'10px', padding:'2px 6px', background:'#5a3040', color:'#f0d0d0', border:'none', borderRadius:'2px', cursor:'pointer' }}>
                                  Submit
                                </button>
                                <button onClick={() => { setReportingMsgIdx(null); setReportDetails(''); setReportCategory('abuse'); }} style={{ fontSize:'10px', padding:'2px 6px', background:'#2a3040', color:'#a0a8b0', border:'none', borderRadius:'2px', cursor:'pointer' }}>
                                  Cancel
                                </button>
                              </div>
                            </>
                          )}
                        </div>
                      )}
                    </React.Fragment>
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
