'use client';

import React, { useState, useMemo } from 'react';

type Rarity   = 'trash' | 'common' | 'rare' | 'epic' | 'legendary';
type CardType = 'spell' | 'trap';
interface Card { name:string; mechanic:string; type:CardType; rarity:Rarity; icon:string; desc:string; dropRate:number; }
interface CardsPageProps {
  onNavigate:(page:string)=>void;
  embedded?: boolean;
}

const RS: Record<Rarity,{accent:string;glow:string;label:string;dark:string;mid:string;artTop:string;artBot:string}> = {
  trash:     {accent:'#a8b4c0',glow:'rgba(168,180,192,0.6)',label:'TRASH',    dark:'#1c1e24',mid:'#2a2d38',artTop:'#3a3d4a',artBot:'#1c1e24'},
  common:    {accent:'#4ade80',glow:'rgba(74,222,128,0.7)', label:'COMMON',   dark:'#0d2818',mid:'#1a4a2a',artTop:'#1e6b35',artBot:'#0a2015'},
  rare:      {accent:'#60a5fa',glow:'rgba(96,165,250,0.8)', label:'RARE',     dark:'#0a1a3a',mid:'#122860',artTop:'#1a3a8a',artBot:'#081428'},
  epic:      {accent:'#c084fc',glow:'rgba(192,132,252,0.9)',label:'EPIC',     dark:'#180a30',mid:'#2e1060',artTop:'#4a1a90',artBot:'#140828'},
  legendary: {accent:'#fbbf24',glow:'rgba(251,191,36,1.0)', label:'LEGENDARY',dark:'#2a1400',mid:'#4a2800',artTop:'#8a4e00',artBot:'#2a1400'},
};

const DROP_RATES:Record<Rarity,number> = {legendary:5,epic:20,rare:30,common:40,trash:5};
const RARITY_ORDER:Rarity[] = ['legendary','epic','rare','common','trash'];

const CARDS:Card[] = [
  {name:'Bad Sniper',      mechanic:'badsniper',       type:'spell',rarity:'trash',    icon:'🔫',dropRate:5, desc:'Remove one of YOUR own pieces (not king).'},
  {name:'Demote',          mechanic:'demote',          type:'spell',rarity:'trash',    icon:'⬇️', dropRate:5, desc:'Lower one of YOUR pieces to a weaker type. Not king.'},
  {name:'Gambler',         mechanic:'gambler',         type:'spell',rarity:'trash',    icon:'🎲',dropRate:5, desc:'50% steal a card from opponent. 50% give one of yours away.'},
  {name:'Promote Him',     mechanic:'promotehim',      type:'spell',rarity:'trash',    icon:'📈',dropRate:5, desc:'Promote enemy piece to higher value. Not king.'},
  {name:'Half Fuse',       mechanic:'halffuse',        type:'spell',rarity:'common',   icon:'⚗️',dropRate:40,desc:'Fuse two adjacent pieces (value ≤5) into one. Not king.'},
  {name:'Swap Me',         mechanic:'swapme',          type:'spell',rarity:'common',   icon:'🔄',dropRate:40,desc:'Exchange positions of two of YOUR pieces. No check. No king.'},
  {name:'Jump',            mechanic:'jump',            type:'spell',rarity:'common',   icon:'🦘',dropRate:40,desc:'Move one of your pieces to any empty square in YOUR half. Not king.'},
  {name:'Small Sacrifice', mechanic:'smallsacrifice',  type:'spell',rarity:'common',   icon:'🩸',dropRate:40,desc:'Sacrifice pieces totaling 6+ pts to draw 2 cards.'},
  {name:'Freeze',          mechanic:'freeze',          type:'trap', rarity:'common',   icon:'🧊',dropRate:40,desc:'Freeze one enemy piece — cannot move for 1 turn. Not king.'},
  {name:'Promote',         mechanic:'promote',         type:'spell',rarity:'common',   icon:'⬆️', dropRate:40,desc:'Promote one of your pieces to a higher value type. Not king.'},
  {name:'Shield',          mechanic:'shield',          type:'trap', rarity:'common',   icon:'🛡️',dropRate:40,desc:'Protect one of your pieces from capture for 1 turn.'},
  {name:'Fog Village',     mechanic:'fog_village',     type:'spell',rarity:'common',   icon:'🌫️',dropRate:40,desc:'3×3 zone — opponent cannot see pieces inside.'},
  {name:'Joker',           mechanic:'joker',           type:'spell',rarity:'rare',     icon:'🃏',dropRate:30,desc:'Choose any card from the full card pool instantly.'},
  {name:'Full Fusion',     mechanic:'fullfusion',      type:'spell',rarity:'rare',     icon:'⚡',dropRate:30,desc:'Merge two adjacent pieces. Combined movement. No king.'},
  {name:'Swap Us',         mechanic:'swapus',          type:'spell',rarity:'rare',     icon:'↔️', dropRate:30,desc:'Swap one of YOUR pieces with one ENEMY piece. No kings.'},
  {name:'Swap Him',        mechanic:'swaphim',         type:'spell',rarity:'rare',     icon:'🔁',dropRate:30,desc:"Swap 2 of your opponent's pieces. No kings."},
  {name:'Twin Move',       mechanic:'doublemove_diff', type:'spell',rarity:'rare',     icon:'👥',dropRate:30,desc:'Move TWO DIFFERENT pieces this turn.'},
  {name:'Solo Move',       mechanic:'doublemove_same', type:'spell',rarity:'rare',     icon:'🏃',dropRate:30,desc:'Move the SAME piece TWICE this turn.'},
  {name:'Demote Him',      mechanic:'demotehim',       type:'spell',rarity:'rare',     icon:'📉',dropRate:30,desc:'Lower any piece (yours or enemy) to weaker type. Not king.'},
  {name:'Fake Piece',      mechanic:'fakepiece',       type:'spell',rarity:'rare',     icon:'👻',dropRate:30,desc:'Place an illusion piece — opponent cannot tell if real until captured.'},
  {name:'Teleport',        mechanic:'teleport',        type:'spell',rarity:'rare',     icon:'🌀',dropRate:30,desc:'Move one of your pieces to any empty square. No check. Not king.'},
  {name:'Lava Ground',     mechanic:'lavaground',      type:'trap', rarity:'rare',     icon:'🌋',dropRate:30,desc:'Mark 1 square. Any piece there next turn is destroyed (not king).'},
  {name:'Radar',           mechanic:'radar',           type:'spell',rarity:'rare',     icon:'📡',dropRate:30,desc:"See all opponent's cards for 1 turn."},
  {name:'Mirror',          mechanic:'mirror',          type:'trap', rarity:'rare',     icon:'🪞',dropRate:30,desc:"Copy opponent's last move with your equivalent piece, if legal."},
  {name:'Cheater',         mechanic:'cheater',         type:'spell',rarity:'rare',     icon:'💡',dropRate:30,desc:'Engine reveals the best move to you.'},
  {name:'Invisible',       mechanic:'invisible',       type:'spell',rarity:'rare',     icon:'👁️',dropRate:30,desc:'One of your pieces becomes invisible for 1 round.'},
  {name:'Sniper',          mechanic:'sniper',          type:'spell',rarity:'epic',     icon:'🎯',dropRate:20,desc:'Remove ANY piece from the board (even yours). Not king. No check.'},
  {name:'Fortress',        mechanic:'fortress',        type:'spell',rarity:'epic',     icon:'🏰',dropRate:20,desc:'2×2 zone — enemies cannot enter for 2 turns.'},
  {name:'Clone',           mechanic:'clone',           type:'spell',rarity:'epic',     icon:'🧬',dropRate:20,desc:'Copy one of your pieces onto an adjacent empty square. Not king.'},
  {name:'Borrow',          mechanic:'borrow',          type:'spell',rarity:'epic',     icon:'🤏',dropRate:20,desc:'Control one enemy piece for 1 turn. Not king. No check.'},
  {name:'Parasite',        mechanic:'parasite',        type:'spell',rarity:'epic',     icon:'🦠',dropRate:20,desc:'Link your piece to enemy. If yours dies → theirs dies too.'},
  {name:'Black Hole',      mechanic:'blackhole',       type:'spell',rarity:'epic',     icon:'🕳️',dropRate:20,desc:'Choose 2 squares. After 2 turns all adjacent pieces explode. Kings immune.'},
  {name:'Big Sacrifice',   mechanic:'bigsacrifice',    type:'spell',rarity:'epic',     icon:'💎',dropRate:20,desc:'Sacrifice pieces totaling 14+ pts to draw 3 cards.'},
  {name:'Undo',            mechanic:'undo',            type:'trap', rarity:'epic',     icon:'↩️', dropRate:20,desc:"Cancel the last card your opponent played."},
  {name:'Reverse',         mechanic:'reverse',         type:'trap', rarity:'epic',     icon:'⏪',dropRate:20,desc:"Undo opponent's last move."},
  {name:'Unabomber',       mechanic:'unabomber',       type:'spell',rarity:'epic',     icon:'💣',dropRate:20,desc:'Attach bomb to your piece. Next round it explodes destroying all adjacent pieces.'},
  {name:'Mind Control',    mechanic:'mindcontrol',     type:'spell',rarity:'legendary',icon:'🧠',dropRate:5, desc:'Permanently steal one enemy piece. Not king. No check.'},
];

function CardTile({card,selected,onClick}:{card:Card;selected:boolean;onClick:()=>void}) {
  const [hov,setHov]=useState(false);
  const rs=RS[card.rarity];
  const lit=hov||selected;

  // Rich multi-stop art gradients — vivid and painterly
  const artBg: Record<Rarity,string> = {
    trash:     'linear-gradient(175deg,#9098a8 0%,#636875 35%,#42444f 70%,#2a2c36 100%)',
    common:    'linear-gradient(175deg,#30d870 0%,#1aaa52 35%,#0f7538 70%,#074020 100%)',
    rare:      'linear-gradient(175deg,#4a88f8 0%,#2255d0 35%,#1035a0 70%,#071a60 100%)',
    epic:      'linear-gradient(175deg,#b050ff 0%,#7820d8 35%,#500a9a 70%,#2e065a 100%)',
    legendary: 'linear-gradient(175deg,#ffd040 0%,#e08800 35%,#a85500 70%,#602800 100%)',
  };

  // Outer frame — the "card border" gradient, rarity-colored
  const frameBg: Record<Rarity,string> = {
    trash:     'linear-gradient(160deg,#8a9aaa 0%,#4a5060 50%,#2a2e38 100%)',
    common:    'linear-gradient(160deg,#50e888 0%,#18b050 50%,#0a6030 100%)',
    rare:      'linear-gradient(160deg,#60b8ff 0%,#1a68e0 50%,#082880 100%)',
    epic:      'linear-gradient(160deg,#d080ff 0%,#8020e0 50%,#3a0890 100%)',
    legendary: 'linear-gradient(160deg,#ffe060 0%,#e09000 50%,#804000 100%)',
  };

  // Glow intensities per rarity
  const idleGlow: Record<Rarity,string> = {
    trash:     '0 4px 16px rgba(0,0,0,0.7)',
    common:    `0 4px 16px rgba(0,0,0,0.7), 0 0 8px ${rs.glow}44`,
    rare:      `0 4px 16px rgba(0,0,0,0.7), 0 0 14px ${rs.glow}55`,
    epic:      `0 4px 20px rgba(0,0,0,0.7), 0 0 20px ${rs.glow}66`,
    legendary: `0 4px 20px rgba(0,0,0,0.7), 0 0 26px ${rs.glow}88`,
  };
  const hoverGlow: Record<Rarity,string> = {
    trash:     `0 12px 32px rgba(0,0,0,0.8), 0 0 14px ${rs.glow}66`,
    common:    `0 12px 32px rgba(0,0,0,0.8), 0 0 20px ${rs.glow}88`,
    rare:      `0 12px 36px rgba(0,0,0,0.8), 0 0 28px ${rs.glow}99`,
    epic:      `0 12px 36px rgba(0,0,0,0.8), 0 0 36px ${rs.glow}, 0 0 60px ${rs.glow}55`,
    legendary: `0 12px 40px rgba(0,0,0,0.8), 0 0 44px ${rs.glow}, 0 0 80px ${rs.glow}66`,
  };

  // Corner ornament for legendary/epic
  const showOrnaments = card.rarity==='legendary'||card.rarity==='epic';

  return (
    <div
      onClick={onClick}
      onMouseEnter={()=>setHov(true)}
      onMouseLeave={()=>setHov(false)}
      style={{
        width:170, flexShrink:0,
        cursor:'pointer',
        position:'relative',
        borderRadius:14,
        // Outer frame layer — thick rarity-colored border
        padding: card.rarity==='legendary' ? 3 : 2,
        background: frameBg[card.rarity],
        boxShadow: selected
          ? `0 0 0 2px #fff8, ${hoverGlow[card.rarity]}`
          : lit ? hoverGlow[card.rarity] : idleGlow[card.rarity],
        transform: hov
          ? 'translateY(-6px) scale(1.03) rotate(-0.4deg)'
          : selected
          ? 'translateY(-4px) scale(1.015)'
          : 'translateY(0) scale(1)',
        transition:'all 0.18s cubic-bezier(0.34,1.3,0.64,1)',
      }}
    >
      {/* Ambient hover glow bloom behind card */}
      {lit && (
        <div style={{
          position:'absolute', inset:-8, borderRadius:20, zIndex:-1, pointerEvents:'none',
          background:`radial-gradient(ellipse at 50% 60%, ${rs.accent}22 0%, transparent 70%)`,
          filter:'blur(6px)',
        }}/>
      )}

      {/* Legendary shimmer overlay */}
      {card.rarity==='legendary' && lit && (
        <div style={{
          position:'absolute', inset:0, borderRadius:14, zIndex:2, pointerEvents:'none',
          background:'linear-gradient(135deg, rgba(255,255,255,0.12) 0%, transparent 50%, rgba(255,200,0,0.08) 100%)',
        }}/>
      )}

      {/* Inner card body */}
      <div style={{
        borderRadius:12,
        overflow:'hidden',
        display:'flex',
        flexDirection:'column',
        height:240,
        position:'relative',
        // Inner card face — darker base
        background: 'linear-gradient(180deg,#0e0e20 0%,#080814 100%)',
        // Inner frame line
        boxShadow:'inset 0 0 0 1px rgba(255,255,255,0.1)',
      }}>

        {/* ── NAME BAR ── */}
        <div style={{
          padding:'8px 10px 7px',
          background:`linear-gradient(90deg, rgba(0,0,0,0.6) 0%, ${rs.accent}18 100%)`,
          borderBottom:`1px solid ${rs.accent}66`,
          display:'flex', alignItems:'center', justifyContent:'space-between', gap:6,
          flexShrink:0,
        }}>
          <div style={{
            color:'#fff', fontWeight:800, fontSize:13, letterSpacing:'0.5px',
            whiteSpace:'nowrap', overflow:'hidden', textOverflow:'ellipsis', flex:1,
            textShadow:`0 0 12px ${rs.accent}88`,
          }}>{card.name}</div>
          <span style={{
            fontSize:8, fontWeight:900, color:rs.accent,
            background:`${rs.accent}20`, border:`1px solid ${rs.accent}88`,
            padding:'2px 6px', borderRadius:4,
            letterSpacing:'0.8px', whiteSpace:'nowrap', flexShrink:0,
          }}>{rs.label}</span>
        </div>

        {/* ── ARTWORK FRAME ── */}
        <div style={{
          flex:'0 0 130px', position:'relative', overflow:'hidden',
          background: artBg[card.rarity],
          margin:'5px 5px 0',
          borderRadius:8,
          // Art frame inner border
          boxShadow:`inset 0 0 0 1px ${rs.accent}44, inset 0 -8px 20px rgba(0,0,0,0.5)`,
        }}>
          {/* Radial light behind icon */}
          <div style={{
            position:'absolute', inset:0,
            background:`radial-gradient(ellipse at 50% 55%, ${rs.accent}60 0%, ${rs.accent}20 40%, transparent 72%)`,
          }}/>

          {/* Scanline texture overlay */}
          <div style={{
            position:'absolute', inset:0, opacity:0.06,
            backgroundImage:'repeating-linear-gradient(0deg, transparent, transparent 2px, rgba(0,0,0,0.8) 2px, rgba(0,0,0,0.8) 3px)',
          }}/>

          {/* Sparkles for epic/legendary */}
          {showOrnaments && [0,1,2,3].map(i=>(
            <div key={i} style={{
              position:'absolute',
              top:`${12+i*22}%`, left:`${6+i*24}%`,
              fontSize:9, color:rs.accent,
              opacity: lit ? 0.9 : 0.35,
              animation:'sparkle 2s ease-in-out infinite',
              animationDelay:`${i*0.3}s`,
              textShadow:`0 0 6px ${rs.accent}`,
            }}>✦</div>
          ))}

          {/* Corner ornament lines — legendary only */}
          {card.rarity==='legendary' && (
            <>
              <div style={{position:'absolute',top:4,left:4,width:12,height:12,borderTop:`2px solid ${rs.accent}cc`,borderLeft:`2px solid ${rs.accent}cc`,borderRadius:'3px 0 0 0'}}/>
              <div style={{position:'absolute',top:4,right:4,width:12,height:12,borderTop:`2px solid ${rs.accent}cc`,borderRight:`2px solid ${rs.accent}cc`,borderRadius:'0 3px 0 0'}}/>
              <div style={{position:'absolute',bottom:4,left:4,width:12,height:12,borderBottom:`2px solid ${rs.accent}cc`,borderLeft:`2px solid ${rs.accent}cc`,borderRadius:'0 0 0 3px'}}/>
              <div style={{position:'absolute',bottom:4,right:4,width:12,height:12,borderBottom:`2px solid ${rs.accent}cc`,borderRight:`2px solid ${rs.accent}cc`,borderRadius:'0 0 3px 0'}}/>
            </>
          )}

          {/* Main icon */}
          <div style={{
            position:'absolute', inset:0,
            display:'flex', alignItems:'center', justifyContent:'center',
          }}>
            <div style={{
              fontSize:58, zIndex:1,
              filter:`drop-shadow(0 0 ${lit?24:10}px ${rs.glow}) drop-shadow(0 4px 8px rgba(0,0,0,0.8))`,
              transform: lit ? 'scale(1.1) translateY(-2px)' : 'scale(1)',
              transition:'all 0.2s ease',
            }}>{card.icon}</div>
          </div>
        </div>

        {/* ── BOTTOM SECTION ── */}
        <div style={{
          flex:1, display:'flex', flexDirection:'column',
          margin:'0 5px 5px', borderRadius:'0 0 8px 8px',
          background:'rgba(0,0,0,0.45)',
          boxShadow:`inset 0 0 0 1px ${rs.accent}22`,
          overflow:'hidden',
        }}>

          {/* Type badge row */}
          <div style={{
            display:'flex', alignItems:'center', justifyContent:'space-between',
            padding:'5px 8px',
            borderBottom:`1px solid ${rs.accent}33`,
            background:`linear-gradient(90deg, ${rs.accent}15 0%, transparent 100%)`,
            flexShrink:0,
          }}>
            <span style={{
              fontSize:9, fontWeight:900, color:rs.accent,
              background:`${rs.accent}22`, border:`1px solid ${rs.accent}77`,
              padding:'3px 9px', borderRadius:4,
              textTransform:'uppercase', letterSpacing:'1.2px',
            }}>{card.type==='trap'?'🪤 TRAP':'⚡ SPELL'}</span>
            <span style={{fontSize:9,color:'rgba(255,255,255,0.4)',fontWeight:700}}>{card.dropRate}%</span>
          </div>

          {/* Description */}
          <div style={{padding:'6px 8px 6px', flex:1, overflow:'hidden'}}>
            <div style={{
              color:'rgba(240,232,215,0.82)', fontSize:10, lineHeight:1.5,
              display:'-webkit-box', WebkitLineClamp:3,
              WebkitBoxOrient:'vertical' as any, overflow:'hidden',
            }}>{card.desc}</div>
          </div>
        </div>

      </div>
    </div>
  );
}

function BigPreview({card}:{card:Card|null}) {
  const rs=card?RS[card.rarity]:null;

  const artBg: Record<Rarity,string> = {
    trash:     'linear-gradient(170deg,#7a8090 0%,#555a68 50%,#3a3d4a 100%)',
    common:    'linear-gradient(170deg,#22c05a 0%,#18904a 50%,#0e6030 100%)',
    rare:      'linear-gradient(170deg,#2a6ae0 0%,#1a48b0 50%,#0e2e80 100%)',
    epic:      'linear-gradient(170deg,#9030f0 0%,#6018c0 50%,#4010a0 100%)',
    legendary: 'linear-gradient(170deg,#f0a800 0%,#c07800 50%,#8a5200 100%)',
  };

  const frameBg: Record<Rarity,string> = {
    trash:     'linear-gradient(180deg,#6a7080,#3a3d48)',
    common:    'linear-gradient(180deg,#2aba60,#158038)',
    rare:      'linear-gradient(180deg,#3a8af0,#1a4aa0)',
    epic:      'linear-gradient(180deg,#b060f8,#6018c0)',
    legendary: 'linear-gradient(180deg,#ffcc30,#c87800)',
  };

  if(!card||!rs) return (
    <div style={{flex:1,display:'flex',flexDirection:'column',alignItems:'center',justifyContent:'center',gap:12,opacity:0.25}}>
      <div style={{fontSize:48}}>🃏</div>
      <div style={{color:'#fff',fontSize:11,textAlign:'center',padding:'0 20px'}}>Click a card</div>
    </div>
  );

  return (
    <div style={{display:'flex',flexDirection:'column',alignItems:'center',padding:'14px 16px',gap:10,animation:'fadeUp 0.2s ease',flex:1,overflow:'hidden'}}>
      <div style={{
        width:'100%', borderRadius:16, padding:4,
        background: frameBg[card.rarity],
        boxShadow:`0 0 20px ${rs.glow}, 0 0 40px ${rs.glow}55, 0 8px 32px rgba(0,0,0,0.8)`,
        flexShrink:0,
      }}>
        <div style={{borderRadius:13, overflow:'hidden', background:artBg[card.rarity], display:'flex', flexDirection:'column', border:'1px solid rgba(255,255,255,0.15)'}}>
          <div style={{padding:'10px 14px', background:'rgba(0,0,0,0.30)', borderBottom:`1px solid ${rs.accent}55`, display:'flex', justifyContent:'space-between', alignItems:'center'}}>
            <div style={{color:'#fff', fontWeight:800, fontSize:14, letterSpacing:'0.3px', overflow:'hidden', textOverflow:'ellipsis', whiteSpace:'nowrap', flex:1}}>{card.name}</div>
            <span style={{fontSize:9, fontWeight:800, color:rs.accent, background:`${rs.accent}22`, border:`1px solid ${rs.accent}55`, padding:'3px 8px', borderRadius:5, marginLeft:8, whiteSpace:'nowrap', letterSpacing:'0.5px'}}>{rs.label}</span>
          </div>
          <div style={{height:150, position:'relative', overflow:'hidden', display:'flex', alignItems:'center', justifyContent:'center'}}>
            <div style={{position:'absolute',inset:0,background:`radial-gradient(ellipse at 50% 45%,${rs.accent}55 0%,transparent 60%)`}}/>
            {[0,1,2,3].map(i=><div key={i} style={{position:'absolute',top:`${10+i*22}%`,left:`${5+i*28}%`,fontSize:9,color:rs.accent,opacity:0.55,animation:'sparkle 2s ease-in-out infinite',animationDelay:`${i*0.3}s`}}>✦</div>)}
            <div style={{fontSize:68,filter:`drop-shadow(0 0 20px ${rs.glow})`,animation:'float 3s ease-in-out infinite',zIndex:1}}>{card.icon}</div>
          </div>
          <div style={{display:'flex',alignItems:'center',justifyContent:'space-between',padding:'7px 14px',background:'rgba(0,0,0,0.25)',borderTop:`1px solid ${rs.accent}44`}}>
            <span style={{fontSize:10,fontWeight:800,padding:'4px 12px',borderRadius:5,color:rs.accent,background:`${rs.accent}25`,border:`1px solid ${rs.accent}66`,textTransform:'uppercase',letterSpacing:'1px'}}>{card.type==='trap'?'TRAP':'SPELL'}</span>
            <span style={{fontSize:10,color:'rgba(255,255,255,0.55)',fontWeight:600}}>{card.dropRate}% drop</span>
          </div>
          <div style={{padding:'10px 14px 12px',background:'rgba(0,0,0,0.22)'}}>
            <div style={{color:'rgba(255,245,235,0.92)',fontSize:11,lineHeight:1.6}}>{card.desc}</div>
          </div>
        </div>
      </div>
      <div style={{width:'100%',display:'flex',gap:8}}>
        <div style={{flex:1,padding:'10px',background:'rgba(0,0,0,0.3)',borderRadius:8,border:`1px solid ${rs.accent}33`,textAlign:'center'}}>
          <div style={{color:'rgba(180,160,120,0.5)',fontSize:8,fontWeight:700,textTransform:'uppercase',letterSpacing:'1px',marginBottom:4}}>DROP</div>
          <div style={{color:rs.accent,fontSize:18,fontWeight:800}}>{card.dropRate}%</div>
        </div>
        <div style={{flex:1,padding:'10px',background:'rgba(0,0,0,0.3)',borderRadius:8,border:`1px solid ${rs.accent}33`,textAlign:'center'}}>
          <div style={{color:'rgba(180,160,120,0.5)',fontSize:8,fontWeight:700,textTransform:'uppercase',letterSpacing:'1px',marginBottom:4}}>TYPE</div>
          <div style={{color:rs.accent,fontSize:12,fontWeight:800,marginTop:2}}>{card.type==='trap'?'TRAP':'SPELL'}</div>
        </div>
      </div>
    </div>
  );
}

function RaritySection({rarity,cards,selectedId,onSelect,filterType}:{rarity:Rarity;cards:Card[];selectedId:string|null;onSelect:(c:Card)=>void;filterType:'All'|'Spell'|'Trap'}) {
  const rs=RS[rarity];
  const visible=cards.filter(c=>c.rarity===rarity&&(filterType==='All'||c.type===filterType.toLowerCase()));
  if(visible.length===0) return null;
  return (
    <div style={{marginBottom:50}}>
      <div style={{display:'flex',alignItems:'center',gap:10,marginBottom:24}}>
        <div style={{width:4,height:20,borderRadius:2,background:rs.accent,boxShadow:`0 0 10px ${rs.glow}`}}/>
        <span style={{color:rs.accent,fontWeight:800,fontSize:13,letterSpacing:'2px'}}>{rs.label}</span>
        <div style={{flex:1,height:1,background:`linear-gradient(90deg,${rs.accent}44,transparent)`}}/>
        <span style={{color:`${rs.accent}77`,fontSize:11,fontWeight:600}}>{visible.length} cards · {DROP_RATES[rarity]}% drop</span>
      </div>
      <div style={{display:'grid', gridTemplateColumns:'repeat(auto-fill, minmax(170px, 1fr))', gap:18}}>
        {visible.map(card=><CardTile key={card.mechanic} card={card} selected={selectedId===card.mechanic} onClick={()=>onSelect(card)}/>)}
      </div>
    </div>
  );
}

export default function CardsPage({onNavigate, embedded = false}:CardsPageProps) {
  const [selected,setSelected]=useState<Card|null>(null);
  const [filterType,setFilterType]=useState<'All'|'Spell'|'Trap'>('All');
  const [filterRarity,setFilterRarity]=useState<Rarity|'All'>('All');
  const [search,setSearch]=useState('');

  const filtered=useMemo(()=>{
    let c=[...CARDS];
    if(filterType!=='All') c=c.filter(x=>x.type===filterType.toLowerCase());
    if(filterRarity!=='All') c=c.filter(x=>x.rarity===filterRarity);
    if(search.trim()) c=c.filter(x=>x.name.toLowerCase().includes(search.toLowerCase()));
    return c;
  },[filterType,filterRarity,search]);

  return (
    <div style={{height:embedded?'100%':'100vh',minHeight:0,flex:embedded?1:undefined,display:'flex',flexDirection:'column',fontFamily:"'Rajdhani','Segoe UI',sans-serif",backgroundImage:embedded?undefined:'url(/background.png)',backgroundSize:'cover',backgroundPosition:'center',backgroundAttachment:'fixed',overflow:'hidden'}}>
      <style>{`
        @import url('https://fonts.googleapis.com/css2?family=Rajdhani:wght@500;600;700&display=swap');
        @keyframes float   {0%,100%{transform:translateY(0)} 50%{transform:translateY(-8px)}}
        @keyframes sparkle {0%,100%{opacity:0.2;transform:scale(1) rotate(0deg)} 50%{opacity:1;transform:scale(1.5) rotate(20deg)}}
        @keyframes fadeUp  {from{opacity:0;transform:translateY(10px)} to{opacity:1;transform:translateY(0)}}
        @keyframes stars   {from{transform:translateY(0);opacity:0.7} to{transform:translateY(-200px);opacity:0}}
        @keyframes shimmer {0%{background-position:-200% center} 100%{background-position:200% center}}
        ::-webkit-scrollbar{width:5px;height:5px}
        ::-webkit-scrollbar-track{background:rgba(0,0,0,0.2)}
        ::-webkit-scrollbar-thumb{background:rgba(255,165,40,0.35);border-radius:3px}
      `}</style>

      {/* Background overlay */}
      {!embedded && <div style={{position:'fixed',inset:0,background:'rgba(6,3,16,0.55)',pointerEvents:'none',zIndex:0}}/>}

      {/* Floating ambient particles */}
      {!embedded && [...Array(14)].map((_,i)=>(
        <div key={i} style={{
          position:'fixed', zIndex:0, pointerEvents:'none',
          left:`${5+i*6.5}%`, bottom:`${5+((i*41)%65)}%`,
          width:i%4===0?2:1, height:i%4===0?2:1,
          borderRadius:'50%', background:'rgba(255,190,80,0.55)',
          animation:`stars ${3.2+i*0.35}s ease-in infinite`,
          animationDelay:`${i*0.45}s`,
        }}/>
      ))}

      {/* NAV */}
      {!embedded && <nav style={{position:'relative',zIndex:100,flexShrink:0,display:'flex',alignItems:'center',justifyContent:'space-between',padding:'0 48px',height:'64px',background:'linear-gradient(180deg,#1a1a3a 0%,#12122a 100%)',backdropFilter:'blur(20px)',borderBottom:'1px solid rgba(255,165,40,0.25)',boxShadow:'0 2px 24px rgba(0,0,0,0.5)'}}>
        <div style={{display:'flex',alignItems:'center',gap:14,minWidth:200}}>
          <div style={{width:40,height:40,borderRadius:10,background:'linear-gradient(135deg,#c8860a,#7a5008)',display:'flex',alignItems:'center',justifyContent:'center',fontSize:18,boxShadow:'0 0 14px rgba(200,134,10,0.6)',border:'1px solid rgba(255,180,60,0.4)'}}>♛</div>
          <span style={{fontSize:21,fontWeight:800,letterSpacing:1,background:'linear-gradient(135deg,#ffd700,#c8860a,#fff8e0)',WebkitBackgroundClip:'text',WebkitTextFillColor:'transparent'}}>CardChess</span>
        </div>
        <div style={{display:'flex',gap:8}}>
          {['Play','Queue','History','Cards','Rankings','Community','Status','Account'].map(label=>{
            const active=label==='Cards';
            return <button key={label} onClick={()=>onNavigate(label)} style={{padding:'8px 22px',fontSize:14,fontWeight:active?700:500,background:active?'linear-gradient(180deg,rgba(200,134,10,0.4),rgba(139,94,10,0.5))':'transparent',color:active?'#ffd700':'rgba(220,210,180,0.9)',border:active?'1px solid rgba(200,134,10,0.6)':'1px solid transparent',borderRadius:8,cursor:'pointer',borderBottom:active?'2px solid #c8860a':'2px solid transparent',fontFamily:'inherit'}}>{label}</button>;
          })}
        </div>
        <div style={{display:'flex',gap:12,minWidth:200,justifyContent:'flex-end'}}>
          <button style={{padding:'8px 20px',fontSize:13,background:'rgba(255,255,255,0.06)',backdropFilter:'blur(10px)',color:'rgba(230,215,175,0.95)',border:'1px solid rgba(255,255,255,0.12)',borderRadius:8,cursor:'pointer',fontFamily:'inherit'}}>Log In</button>
          <button style={{padding:'8px 20px',fontSize:13,fontWeight:700,background:'linear-gradient(180deg,#c8860a,#7a5008)',color:'#fff8e0',border:'1px solid rgba(255,180,60,0.4)',borderRadius:8,cursor:'pointer',boxShadow:'0 2px 10px rgba(200,134,10,0.4)',fontFamily:'inherit'}}>Sign Up</button>
        </div>
      </nav>}

      {/* FILTER BAR */}
      <div style={{position:'relative',zIndex:99,flexShrink:0,display:'flex',alignItems:'center',gap:10,padding:embedded?'12px 24px':'12px 48px',background:'linear-gradient(180deg,#16163a 0%,#10102e 100%)',backdropFilter:'blur(16px)',borderBottom:'1px solid rgba(255,165,40,0.15)',flexWrap:'wrap'}}>
        <div style={{display:'flex',alignItems:'center',gap:10,marginRight:16}}>
          <span style={{fontSize:20,filter:'drop-shadow(0 0 8px rgba(255,165,40,0.5))'}}>🃏</span>
          <div>
            <div style={{color:'#fff',fontWeight:700,fontSize:15}}>Cards</div>
            <div style={{color:'rgba(200,180,140,0.5)',fontSize:11}}>{filtered.length} cards</div>
          </div>
        </div>
        <div style={{width:1,height:28,background:'rgba(255,255,255,0.1)',margin:'0 4px'}}/>
        {(['All','Spell','Trap'] as const).map(t=>(
          <button key={t} onClick={()=>setFilterType(t)} style={{padding:'7px 16px',borderRadius:8,fontSize:13,fontWeight:600,background:filterType===t?(t==='Spell'?'rgba(125,211,252,0.15)':t==='Trap'?'rgba(251,146,60,0.15)':'rgba(255,165,40,0.15)'):'rgba(255,255,255,0.06)',backdropFilter:'blur(10px)',color:filterType===t?(t==='Spell'?'#7dd3fc':t==='Trap'?'#fb923c':'#ffb830'):'rgba(200,185,140,0.6)',border:filterType===t?`1px solid ${t==='Spell'?'rgba(125,211,252,0.35)':t==='Trap'?'rgba(251,146,60,0.35)':'rgba(255,165,40,0.35)'}`:'1px solid rgba(255,255,255,0.1)',cursor:'pointer',fontFamily:'inherit'}}>
            {t==='All'?`All (${CARDS.length})`:t==='Spell'?`⚡ Spell (${CARDS.filter(c=>c.type==='spell').length})`:`🪤 Trap (${CARDS.filter(c=>c.type==='trap').length})`}
          </button>
        ))}
        <div style={{width:1,height:28,background:'rgba(255,255,255,0.1)',margin:'0 4px'}}/>
        {(['All',...RARITY_ORDER] as (Rarity|'All')[]).map(r=>{
          const active=filterRarity===r; const rs2=r!=='All'?RS[r as Rarity]:null;
          const count=r==='All'?CARDS.length:CARDS.filter(c=>c.rarity===r).length;
          return <button key={r} onClick={()=>setFilterRarity(r)} style={{padding:'5px 14px',borderRadius:20,fontSize:11,fontWeight:700,letterSpacing:'0.4px',background:active?(rs2?`${rs2.accent}22`:'rgba(255,165,40,0.18)'):'rgba(255,255,255,0.06)',backdropFilter:'blur(10px)',color:active?(rs2?rs2.accent:'#ffb830'):'rgba(180,165,130,0.55)',border:active?`1px solid ${rs2?rs2.accent+'55':'rgba(255,165,40,0.4)'}`:'1px solid rgba(255,255,255,0.1)',cursor:'pointer',textTransform:'uppercase',boxShadow:active&&rs2?`0 0 10px ${rs2.glow}40`:'none',fontFamily:'inherit'}}>{r==='All'?'All':RS[r as Rarity].label} <span style={{opacity:0.6,marginLeft:3}}>({count})</span></button>;
        })}
        <div style={{flex:1}}/>
        <div style={{position:'relative'}}>
          <span style={{position:'absolute',left:11,top:'50%',transform:'translateY(-50%)',fontSize:13,opacity:0.35}}>🔍</span>
          <input aria-label="Search cards" value={search} onChange={e=>setSearch(e.target.value)} placeholder="Search..." style={{padding:'8px 14px 8px 32px',background:'rgba(255,255,255,0.05)',border:'1px solid rgba(255,255,255,0.12)',borderRadius:8,color:'#fff',fontSize:13,outline:'none',width:170,fontFamily:'inherit'}}/>
        </div>
      </div>

      {/* BODY */}
      <div style={{flex:1,display:'flex',overflow:'hidden',position:'relative',zIndex:1}}>
        <div style={{maxWidth:1600,margin:'0 auto',width:'100%',display:'flex',flex:1}}>

          {/* LEFT SIDEBAR */}
          <div style={{
            width:340, flexShrink:0, display:'flex', flexDirection:'column',
            background:'linear-gradient(180deg,#16163a 0%,#10102e 100%)',
            backdropFilter:'blur(20px)',
            borderRight:'1px solid rgba(255,165,40,0.15)',
            overflowY:'hidden',
            margin:embedded?'16px':'16px 20px 16px 16px',
            borderRadius:16,
            boxShadow:'4px 0 32px rgba(0,0,0,0.4)',
          }}>
            <BigPreview card={selected}/>
            <div style={{padding:'14px 18px',borderTop:'1px solid rgba(255,165,40,0.15)',flexShrink:0,background:'rgba(0,0,0,0.2)',borderRadius:'0 0 0 16px'}}>
              <div style={{color:'rgba(200,180,140,0.55)',fontSize:9,fontWeight:700,textTransform:'uppercase',letterSpacing:'1.5px',marginBottom:10}}>Drop Rates</div>
              {RARITY_ORDER.map(r=>{const rs2=RS[r];return(
                <div key={r} style={{display:'flex',alignItems:'center',gap:10,marginBottom:9}}>
                  <div style={{width:56,fontSize:9,fontWeight:800,color:rs2.accent,letterSpacing:'0.5px'}}>{rs2.label}</div>
                  <div style={{flex:1,height:5,borderRadius:3,background:'rgba(255,255,255,0.06)',overflow:'hidden'}}>
                    <div style={{height:'100%',width:`${DROP_RATES[r]}%`,background:`linear-gradient(90deg,${rs2.accent}88,${rs2.accent})`,boxShadow:`0 0 5px ${rs2.glow}`}}/>
                  </div>
                  <div style={{width:28,fontSize:9,color:'rgba(200,185,150,0.6)',textAlign:'right',fontWeight:700}}>{DROP_RATES[r]}%</div>
                </div>
              );})}
            </div>
            <div style={{padding:'12px 18px',borderTop:'1px solid rgba(255,165,40,0.10)',flexShrink:0}}>
              <div style={{color:'#ffb830',fontSize:11,fontWeight:700,marginBottom:5}}>⊙ Infinite Pool</div>
              <div style={{color:'rgba(200,185,150,0.65)',fontSize:10.5,lineHeight:1.55}}>Cards drawn from an infinite deck. Rarity determines drop chance each round.</div>
            </div>
            <div style={{padding:'12px 18px',borderTop:'1px solid rgba(255,165,40,0.10)',flexShrink:0}}>
              <div style={{color:'#60a5fa',fontSize:11,fontWeight:700,marginBottom:5}}>■ Strategy Tip</div>
              <div style={{color:'rgba(200,185,150,0.65)',fontSize:10.5,lineHeight:1.55}}>Traps activate on the opponent's turn. Spells activate on yours. Plan ahead!</div>
            </div>
          </div>

          {/* CARD GRID */}
          <div style={{flex:1, overflowY:'auto', padding:embedded?'28px 24px':'36px 48px'}}>
            <div style={{maxWidth:1400, margin:'0 auto'}}>
              {filterRarity!=='All'
                ? <RaritySection rarity={filterRarity} cards={filtered} selectedId={selected?.mechanic??null} onSelect={setSelected} filterType={filterType}/>
                : RARITY_ORDER.map(r=><RaritySection key={r} rarity={r} cards={CARDS} selectedId={selected?.mechanic??null} onSelect={setSelected} filterType={filterType}/>)
              }
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
