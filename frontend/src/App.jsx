import { useState, useEffect, useCallback, useRef } from 'react'

const API = '/api'

// Reverse-geocode lat/lng → human-readable area using OpenStreetMap Nominatim (free, no key needed)
async function reverseGeocode(lat, lng) {
  try {
    const res = await fetch(
      `https://nominatim.openstreetmap.org/reverse?format=json&lat=${lat}&lon=${lng}&zoom=16&addressdetails=1`,
      { headers: { 'Accept-Language': 'en', 'User-Agent': 'UpForApp/1.0' } }
    )
    if (!res.ok) return ''
    const data = await res.json()
    const a = data.address || {}
    const area  = a.suburb || a.neighbourhood || a.quarter || a.village || a.hamlet || ''
    const city  = a.city || a.town || a.municipality || a.county || ''
    return [area, city].filter(Boolean).join(', ')
  } catch {
    return ''
  }
}

function timeAgo(isoStr) {
  const d = new Date(isoStr + 'Z')
  const diff = Math.floor((Date.now() - d) / 1000)
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  return `${Math.floor(diff / 3600)}h ago`
}

export default function App() {
  // ── Core ──────────────────────────────────────────────────────────────────
  const [screen, setScreen] = useState('setup') // 'setup' | 'live'
  const [name, setName] = useState(() => localStorage.getItem('upfor_name') || '')
  const [userId, setUserId] = useState(() => localStorage.getItem('upfor_id') || '')
  const [activity, setActivity] = useState('')
  const [radius, setRadius] = useState(5)
  const [location, setLocation] = useState(null)
  const [tab, setTab] = useState('activity') // 'activity' | 'everyone'
  const [people, setPeople] = useState([])
  const [everyonePeople, setEveryonePeople] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [lastRefresh, setLastRefresh] = useState(null)

  // ── Requests ──────────────────────────────────────────────────────────────
  // sentMap: { [toId]: { reqId, status, peerName, peerActivity } }
  const [sentMap, setSentMap] = useState({})
  const [incomingRequests, setIncomingRequests] = useState([])

  // ── Connections & Chat ────────────────────────────────────────────────────
  // connections: { [reqId]: { peerId, peerName, peerActivity } } — accepted incoming requests I responded to
  const [connections, setConnections] = useState({})
  const [chatOpen, setChatOpen] = useState(false)
  const [chatReqId, setChatReqId] = useState(null)
  const [chatPeer, setChatPeer] = useState(null) // { peerName, peerActivity }
  const [messages, setMessages] = useState([])
  const [msgInput, setMsgInput] = useState('')
  const [msgSending, setMsgSending] = useState(false)

  // ── Refs ──────────────────────────────────────────────────────────────────
  const locationRef = useRef(null)
  const activityRef = useRef('')
  const radiusRef = useRef(5)
  const userIdRef = useRef(userId)
  const nameRef = useRef(name)
  const activityStateRef = useRef(activity)
  const chatReqIdRef = useRef(null)
  const messagesEndRef = useRef(null)

  useEffect(() => { userIdRef.current = userId }, [userId])
  useEffect(() => { activityRef.current = activity; activityStateRef.current = activity }, [activity])
  useEffect(() => { radiusRef.current = radius }, [radius])
  useEffect(() => { nameRef.current = name }, [name])
  useEffect(() => { chatReqIdRef.current = chatReqId }, [chatReqId])

  // ── Fetch nearby (activity-filtered) ──────────────────────────────────────
  const fetchNearby = useCallback(async (loc, act, rad, uid) => {
    if (!loc || !act) return
    try {
      const res = await fetch(
        `${API}/nearby?activity=${encodeURIComponent(act)}&lat=${loc.lat}&lng=${loc.lng}&radius=${rad}&exclude_id=${uid}`
      )
      if (!res.ok) return
      const data = await res.json()
      setPeople(Array.isArray(data) ? data : [])
      setLastRefresh(new Date())
    } catch (e) {
      console.error('fetchNearby:', e)
    }
  }, [])

  // ── Fetch everyone (no activity filter) ───────────────────────────────────
  const fetchEveryone = useCallback(async (loc, rad, uid) => {
    if (!loc) return
    try {
      const res = await fetch(
        `${API}/nearby?activity=&lat=${loc.lat}&lng=${loc.lng}&radius=${rad}&exclude_id=${uid}`
      )
      if (!res.ok) return
      const data = await res.json()
      setEveryonePeople(Array.isArray(data) ? data : [])
    } catch (e) {
      console.error('fetchEveryone:', e)
    }
  }, [])

  // ── Fetch requests ────────────────────────────────────────────────────────
  const fetchRequests = useCallback(async (uid) => {
    if (!uid) return
    try {
      const [incRes, sentRes] = await Promise.all([
        fetch(`${API}/requests/incoming?user_id=${uid}`),
        fetch(`${API}/requests/sent?user_id=${uid}`),
      ])
      if (incRes.ok) {
        const inc = await incRes.json()
        setIncomingRequests(Array.isArray(inc) ? inc : [])
      }
      if (sentRes.ok) {
        const sent = await sentRes.json()
        if (Array.isArray(sent)) {
          setSentMap(prev => {
            const next = { ...prev }
            sent.forEach(r => {
              next[r.to_id] = {
                reqId: r.id,
                status: r.status,
                // preserve locally stored name; fall back to server-resolved to_name
                peerName: prev[r.to_id]?.peerName || r.to_name || '',
                peerActivity: prev[r.to_id]?.peerActivity || '',
              }
            })
            return next
          })
        }
      }
    } catch (e) {
      console.error('fetchRequests:', e)
    }
  }, [])

  // ── Fetch messages ────────────────────────────────────────────────────────
  const fetchMessages = useCallback(async (reqId) => {
    if (!reqId) return
    try {
      const res = await fetch(`${API}/chat/messages?request_id=${reqId}`)
      if (!res.ok) return
      const data = await res.json()
      setMessages(Array.isArray(data) ? data : [])
    } catch (e) {
      console.error('fetchMessages:', e)
    }
  }, [])

  // ── Auto-refresh every 15s ────────────────────────────────────────────────
  useEffect(() => {
    if (screen !== 'live') return
    const id = setInterval(() => {
      fetchNearby(locationRef.current, activityRef.current, radiusRef.current, userIdRef.current)
      fetchRequests(userIdRef.current)
      if (tab === 'everyone') fetchEveryone(locationRef.current, radiusRef.current, userIdRef.current)
    }, 15_000)
    return () => clearInterval(id)
  }, [screen, tab, fetchNearby, fetchRequests, fetchEveryone])

  // ── Chat polling every 5s ─────────────────────────────────────────────────
  useEffect(() => {
    if (!chatOpen || !chatReqId) return
    const id = setInterval(() => fetchMessages(chatReqIdRef.current), 5_000)
    return () => clearInterval(id)
  }, [chatOpen, chatReqId, fetchMessages])

  // ── Scroll to bottom on new messages ──────────────────────────────────────
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  // ── Fetch everyone when tab switches ──────────────────────────────────────
  useEffect(() => {
    if (tab === 'everyone' && screen === 'live') {
      fetchEveryone(locationRef.current, radiusRef.current, userIdRef.current)
    }
  }, [tab, screen, fetchEveryone])

  // ── Location helper ───────────────────────────────────────────────────────
  const getLocation = () =>
    new Promise((resolve, reject) => {
      if (!navigator.geolocation) { reject(new Error('Geolocation not supported')); return }
      navigator.geolocation.getCurrentPosition(
        pos => resolve({ lat: pos.coords.latitude, lng: pos.coords.longitude }),
        err => reject(err),
        { enableHighAccuracy: true, timeout: 10000 }
      )
    })

  // ── Go live ───────────────────────────────────────────────────────────────
  const goLive = async () => {
    const trimName = name.trim()
    const trimActivity = activity.trim()
    if (!trimName) { setError('Please enter your name.'); return }
    if (!trimActivity) { setError('Please enter what you are up for.'); return }
    setError('')
    setLoading(true)

    let loc
    try {
      loc = await getLocation()
    } catch (e) {
      setError(e.code === 1 ? 'Location access denied. Please allow location and try again.' : 'Could not get your location.')
      setLoading(false)
      return
    }

    // Geocode in background — don't block checkin if slow
    const address = await Promise.race([
      reverseGeocode(loc.lat, loc.lng),
      new Promise(r => setTimeout(() => r(''), 4000)),
    ])

    try {
      const res = await fetch(`${API}/checkin`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: userId || undefined, name: trimName, activity: trimActivity, address, lat: loc.lat, lng: loc.lng }),
      })
      if (!res.ok) throw new Error()
      const data = await res.json()
      const newId = data.id
      setUserId(newId)
      userIdRef.current = newId
      localStorage.setItem('upfor_id', newId)
      localStorage.setItem('upfor_name', trimName)

      locationRef.current = loc
      setLocation(loc)
      setScreen('live')
      await Promise.all([
        fetchNearby(loc, trimActivity, radius, newId),
        fetchRequests(newId),
      ])
    } catch {
      setError('Failed to connect. Is the backend running on port 8080?')
    } finally {
      setLoading(false)
    }
  }

  // ── Sign off ──────────────────────────────────────────────────────────────
  const signOff = async () => {
    if (userId) {
      try { await fetch(`${API}/checkout/${userId}`, { method: 'DELETE' }) } catch (_) {}
    }
    setScreen('setup')
    setPeople([])
    setEveryonePeople([])
    setActivity('')
    setLastRefresh(null)
    setSentMap({})
    setIncomingRequests([])
    setConnections({})
    setChatOpen(false)
    setChatReqId(null)
    setChatPeer(null)
  }

  const handleRadiusChange = (val) => {
    const r = Number(val)
    setRadius(r)
    radiusRef.current = r
    if (screen === 'live') {
      fetchNearby(locationRef.current, activityRef.current, r, userIdRef.current)
      if (tab === 'everyone') fetchEveryone(locationRef.current, r, userIdRef.current)
    }
  }

  // ── Send connect request ──────────────────────────────────────────────────
  const sendRequest = async (person) => {
    setSentMap(prev => ({
      ...prev,
      [person.id]: { reqId: null, status: 'pending', peerName: person.name, peerActivity: person.activity },
    }))
    try {
      const res = await fetch(`${API}/requests/send`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          from_id: userIdRef.current,
          from_name: nameRef.current,
          from_activity: activityStateRef.current,
          to_id: person.id,
        }),
      })
      if (!res.ok) throw new Error()
      const data = await res.json()
      setSentMap(prev => ({
        ...prev,
        [person.id]: { reqId: data.id, status: data.status, peerName: person.name, peerActivity: person.activity },
      }))
    } catch {
      setSentMap(prev => { const n = { ...prev }; delete n[person.id]; return n })
    }
  }

  // ── Respond to incoming request ───────────────────────────────────────────
  const respond = async (req, status) => {
    if (status === 'accepted') {
      setConnections(prev => ({
        ...prev,
        [req.id]: { peerId: req.from_id, peerName: req.from_name, peerActivity: req.from_activity },
      }))
    }
    setIncomingRequests(prev => prev.filter(r => r.id !== req.id))
    try {
      await fetch(`${API}/requests/respond`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ request_id: req.id, status }),
      })
    } catch (e) {
      console.error('respond:', e)
    }
  }

  // ── Open / close chat ─────────────────────────────────────────────────────
  const openChat = (reqId, peerName, peerActivity) => {
    setChatReqId(reqId)
    setChatPeer({ peerName, peerActivity })
    setMessages([])
    setMsgInput('')
    setChatOpen(true)
    fetchMessages(reqId)
  }

  const closeChat = () => {
    setChatOpen(false)
    setChatReqId(null)
    setChatPeer(null)
    setMessages([])
  }

  // ── Send message ──────────────────────────────────────────────────────────
  const sendMessage = async () => {
    const body = msgInput.trim()
    if (!body || !chatReqId || msgSending) return
    setMsgSending(true)
    setMsgInput('')
    try {
      await fetch(`${API}/chat/send`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          request_id: chatReqId,
          sender_id: userIdRef.current,
          sender_name: nameRef.current,
          body,
        }),
      })
      await fetchMessages(chatReqId)
    } catch (e) {
      console.error('sendMessage:', e)
      setMsgInput(body)
    } finally {
      setMsgSending(false)
    }
  }

  // ── Derive all accepted connections ───────────────────────────────────────
  const allConnections = [
    // Sent requests that were accepted
    ...Object.entries(sentMap)
      .filter(([, v]) => v.status === 'accepted' && v.reqId)
      .map(([toId, v]) => ({ reqId: v.reqId, peerId: toId, peerName: v.peerName || toId, peerActivity: v.peerActivity || '' })),
    // Incoming requests I accepted
    ...Object.entries(connections)
      .map(([reqId, v]) => ({ reqId, ...v })),
  ].filter((c, i, arr) => arr.findIndex(x => x.reqId === c.reqId) === i) // dedupe by reqId

  const connectState = (personId) => sentMap[personId]?.status || 'idle'

  const currentPeople = tab === 'activity' ? people : everyonePeople
  const pendingCount = incomingRequests.length

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <div className="app">
      <header>
        <div className="logo">UpFor?</div>
        <p className="tagline">Find people nearby up for the same thing right now</p>
      </header>

      {/* ── Setup screen ── */}
      {screen === 'setup' && (
        <div className="card">
          <div className="field">
            <label htmlFor="name">Your name</label>
            <input id="name" type="text" value={name} onChange={e => setName(e.target.value)}
              placeholder="e.g. Alex" maxLength={50} autoFocus />
          </div>

          <div className="field">
            <label htmlFor="activity">What are you up for?</label>
            <input id="activity" type="text" value={activity} onChange={e => setActivity(e.target.value)}
              placeholder="e.g. pickleball, walking, coffee…" maxLength={100}
              onKeyDown={e => e.key === 'Enter' && goLive()} />
          </div>

          <div className="field">
            <label>Search radius &nbsp;<span className="radius-value">{radius} km</span></label>
            <input type="range" min={1} max={50} value={radius}
              onChange={e => handleRadiusChange(e.target.value)} className="slider" />
            <div className="slider-ends"><span>1 km</span><span>50 km</span></div>
          </div>

          {error && <p className="error-msg">{error}</p>}

          <button className="btn-primary" onClick={goLive} disabled={loading}>
            {loading
              ? <span className="spinner-row"><span className="spinner" /> Getting location…</span>
              : "I'm Up For It!"}
          </button>
        </div>
      )}

      {/* ── Live screen ── */}
      {screen === 'live' && (
        <div className="live">

          {/* Status bar */}
          <div className="card status-bar">
            <div className="status-left">
              <span className="dot-live" />
              <div>
                <div className="status-name">{name}</div>
                <div className="status-activity">{activity}</div>
              </div>
            </div>
            <div className="status-right">
              {pendingCount > 0 && (
                <span className="req-badge" title={`${pendingCount} pending request${pendingCount > 1 ? 's' : ''}`}>
                  {pendingCount}
                </span>
              )}
              <button className="btn-signoff" onClick={signOff}>Sign Off</button>
            </div>
          </div>

          {/* Incoming requests */}
          {pendingCount > 0 && (
            <div className="card requests-card">
              <h3 className="requests-title">
                Incoming requests <span className="req-count">{pendingCount}</span>
              </h3>
              <ul className="req-list">
                {incomingRequests.map(req => (
                  <li key={req.id} className="req-item">
                    <div className="req-avatar">{req.from_name.charAt(0).toUpperCase()}</div>
                    <div className="req-info">
                      <div className="req-name">{req.from_name}</div>
                      <div className="req-activity">up for {req.from_activity}</div>
                    </div>
                    <div className="req-actions">
                      <button className="btn-accept" onClick={() => respond(req, 'accepted')}>Accept</button>
                      <button className="btn-decline" onClick={() => respond(req, 'declined')}>Decline</button>
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Connections */}
          {allConnections.length > 0 && (
            <div className="card connections-card">
              <h3 className="connections-title">
                Connected <span className="conn-count">{allConnections.length}</span>
              </h3>
              <ul className="conn-list">
                {allConnections.map(c => (
                  <li key={c.reqId} className="conn-item">
                    <div className="conn-avatar">{c.peerName.charAt(0).toUpperCase()}</div>
                    <div className="conn-info">
                      <div className="conn-name">{c.peerName}</div>
                      {c.peerActivity && <div className="conn-activity">up for {c.peerActivity}</div>}
                    </div>
                    <button className="btn-chat" onClick={() => openChat(c.reqId, c.peerName, c.peerActivity)}>
                      Chat
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {/* Radius control */}
          <div className="card radius-card">
            <label>Radius &nbsp;<span className="radius-value">{radius} km</span></label>
            <input type="range" min={1} max={50} value={radius}
              onChange={e => handleRadiusChange(e.target.value)} className="slider" />
            <div className="slider-ends"><span>1 km</span><span>50 km</span></div>
          </div>

          {/* Results */}
          <div className="card results-card">
            <div className="results-header">
              <h2>
                {currentPeople.length === 0
                  ? 'Nobody nearby yet'
                  : `${currentPeople.length} ${currentPeople.length === 1 ? 'person' : 'people'} nearby`}
              </h2>
              <button className="btn-refresh"
                onClick={() => {
                  fetchNearby(locationRef.current, activityRef.current, radius, userId)
                  fetchRequests(userId)
                  if (tab === 'everyone') fetchEveryone(locationRef.current, radius, userId)
                }}
                title="Refresh">↻</button>
            </div>

            {/* Tabs */}
            <div className="tabs">
              <button className={`tab ${tab === 'activity' ? 'tab-active' : ''}`} onClick={() => setTab('activity')}>
                Up for {activity}
              </button>
              <button className={`tab ${tab === 'everyone' ? 'tab-active' : ''}`} onClick={() => setTab('everyone')}>
                Everyone nearby
              </button>
            </div>

            {lastRefresh && (
              <p className="refresh-time">Updated {timeAgo(lastRefresh.toISOString().replace('Z', ''))}</p>
            )}

            {currentPeople.length === 0 ? (
              <div className="empty">
                {tab === 'activity'
                  ? <p>No one nearby is up for <em>{activity}</em> within {radius} km.</p>
                  : <p>Nobody within {radius} km right now.</p>}
                <p className="empty-sub">You are live — others will see you when they check in!</p>
              </div>
            ) : (
              <ul className="people-list">
                {currentPeople.map(p => {
                  const cs = connectState(p.id)
                  const conn = allConnections.find(c => c.peerId === p.id)
                  return (
                    <li key={p.id} className="person-card">
                      <div className="person-avatar">{p.name.charAt(0).toUpperCase()}</div>
                      <div className="person-info">
                        <div className="person-name">{p.name}</div>
                        <div className="person-activity">{p.activity}</div>
                        {p.address && <div className="person-address">📍 {p.address}</div>}
                        <a
                          className="btn-map"
                          href={`https://www.openstreetmap.org/?mlat=${p.lat}&mlon=${p.lng}&zoom=16`}
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          View on map
                        </a>
                      </div>
                      <div className="person-right">
                        <div className="person-dist">{p.distance} km</div>
                        {conn ? (
                          <button className="btn-chat" onClick={() => openChat(conn.reqId, conn.peerName, conn.peerActivity)}>Chat</button>
                        ) : cs === 'idle' ? (
                          <button className="btn-connect" onClick={() => sendRequest(p)}>Connect</button>
                        ) : cs === 'pending' ? (
                          <span className="connect-state pending">Pending…</span>
                        ) : cs === 'accepted' ? (
                          <span className="connect-state accepted">Connected ✓</span>
                        ) : (
                          <span className="connect-state declined">Declined</span>
                        )}
                      </div>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
        </div>
      )}

      {/* ── Chat overlay ── */}
      {chatOpen && chatPeer && (
        <div className="chat-overlay">
          <div className="chat-header">
            <button className="chat-back" onClick={closeChat}>←</button>
            <div className="chat-header-info">
              <div className="chat-peer-name">{chatPeer.peerName}</div>
              {chatPeer.peerActivity && (
                <div className="chat-peer-activity">up for {chatPeer.peerActivity}</div>
              )}
            </div>
          </div>

          <div className="chat-messages">
            {messages.length === 0 && (
              <div className="chat-empty">
                <p>No messages yet.</p>
                <p>Say hi to {chatPeer.peerName}!</p>
              </div>
            )}
            {messages.map(m => {
              const mine = m.sender_id === userId
              return (
                <div key={m.id} className={`msg-row ${mine ? 'msg-mine' : 'msg-theirs'}`}>
                  {!mine && <div className="msg-sender">{m.sender_name}</div>}
                  <div className="msg-bubble">{m.body}</div>
                  <div className="msg-time">{m.created_at}</div>
                </div>
              )
            })}
            <div ref={messagesEndRef} />
          </div>

          <div className="chat-input-bar">
            <input
              className="chat-input"
              value={msgInput}
              onChange={e => setMsgInput(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && !e.shiftKey && sendMessage()}
              placeholder="Type a message…"
              autoFocus
            />
            <button className="btn-send" onClick={sendMessage} disabled={msgSending || !msgInput.trim()}>
              Send
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
