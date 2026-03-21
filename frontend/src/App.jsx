import { useState, useEffect, useCallback, useRef } from 'react'

const API = '/api'

function timeAgo(isoStr) {
  const d = new Date(isoStr + 'Z')
  const diff = Math.floor((Date.now() - d) / 1000)
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  return `${Math.floor(diff / 3600)}h ago`
}

export default function App() {
  const [screen, setScreen] = useState('setup') // 'setup' | 'live'
  const [name, setName] = useState(() => localStorage.getItem('upfor_name') || '')
  const [userId, setUserId] = useState(() => localStorage.getItem('upfor_id') || '')
  const [activity, setActivity] = useState('')
  const [radius, setRadius] = useState(5)
  const [location, setLocation] = useState(null)
  const [people, setPeople] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [lastRefresh, setLastRefresh] = useState(null)
  const locationRef = useRef(null)
  const activityRef = useRef('')
  const radiusRef = useRef(5)
  const userIdRef = useRef(userId)

  useEffect(() => { userIdRef.current = userId }, [userId])
  useEffect(() => { activityRef.current = activity }, [activity])
  useEffect(() => { radiusRef.current = radius }, [radius])

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

  // Auto-refresh every 30s when live
  useEffect(() => {
    if (screen !== 'live') return
    const id = setInterval(() => {
      fetchNearby(locationRef.current, activityRef.current, radiusRef.current, userIdRef.current)
    }, 30_000)
    return () => clearInterval(id)
  }, [screen, fetchNearby])

  const getLocation = () =>
    new Promise((resolve, reject) => {
      if (!navigator.geolocation) {
        reject(new Error('Geolocation not supported by your browser.'))
        return
      }
      navigator.geolocation.getCurrentPosition(
        (pos) => resolve({ lat: pos.coords.latitude, lng: pos.coords.longitude }),
        (err) => reject(err),
        { enableHighAccuracy: true, timeout: 10000 }
      )
    })

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
      setError(
        e.code === 1
          ? 'Location access denied. Please allow location access and try again.'
          : 'Could not get your location. Please try again.'
      )
      setLoading(false)
      return
    }

    try {
      const res = await fetch(`${API}/checkin`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: userId || undefined,
          name: trimName,
          activity: trimActivity,
          lat: loc.lat,
          lng: loc.lng,
        }),
      })
      if (!res.ok) throw new Error('Server error')
      const data = await res.json()
      const newId = data.id
      setUserId(newId)
      userIdRef.current = newId
      localStorage.setItem('upfor_id', newId)
      localStorage.setItem('upfor_name', trimName)

      locationRef.current = loc
      setLocation(loc)
      setScreen('live')
      await fetchNearby(loc, trimActivity, radius, newId)
    } catch (e) {
      setError('Failed to connect. Is the backend running on port 8080?')
    } finally {
      setLoading(false)
    }
  }

  const signOff = async () => {
    if (userId) {
      try { await fetch(`${API}/checkout/${userId}`, { method: 'DELETE' }) } catch (_) {}
    }
    setScreen('setup')
    setPeople([])
    setActivity('')
    setLastRefresh(null)
  }

  const handleRadiusChange = (val) => {
    const r = Number(val)
    setRadius(r)
    radiusRef.current = r
    if (screen === 'live') {
      fetchNearby(locationRef.current, activityRef.current, r, userIdRef.current)
    }
  }

  // --- RENDER ---
  return (
    <div className="app">
      <header>
        <div className="logo">UpFor?</div>
        <p className="tagline">Find people nearby up for the same thing right now</p>
      </header>

      {screen === 'setup' && (
        <div className="card">
          <div className="field">
            <label htmlFor="name">Your name</label>
            <input
              id="name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Alex"
              maxLength={50}
              autoFocus
            />
          </div>

          <div className="field">
            <label htmlFor="activity">What are you up for?</label>
            <input
              id="activity"
              type="text"
              value={activity}
              onChange={(e) => setActivity(e.target.value)}
              placeholder="e.g. pickleball, walking, coffee, chess…"
              maxLength={100}
              onKeyDown={(e) => e.key === 'Enter' && goLive()}
            />
          </div>

          <div className="field">
            <label>
              Search radius &nbsp;<span className="radius-value">{radius} km</span>
            </label>
            <input
              type="range"
              min={1}
              max={50}
              value={radius}
              onChange={(e) => handleRadiusChange(e.target.value)}
              className="slider"
            />
            <div className="slider-ends">
              <span>1 km</span>
              <span>50 km</span>
            </div>
          </div>

          {error && <p className="error-msg">{error}</p>}

          <button className="btn-primary" onClick={goLive} disabled={loading}>
            {loading ? (
              <span className="spinner-row"><span className="spinner" /> Getting location…</span>
            ) : (
              "I'm Up For It!"
            )}
          </button>
        </div>
      )}

      {screen === 'live' && (
        <div className="live">
          {/* Status bar */}
          <div className="card status-bar">
            <div className="status-left">
              <span className="dot-live" title="You are live" />
              <div>
                <div className="status-name">{name}</div>
                <div className="status-activity">{activity}</div>
              </div>
            </div>
            <button className="btn-signoff" onClick={signOff}>Sign Off</button>
          </div>

          {/* Radius control */}
          <div className="card radius-card">
            <label>
              Radius &nbsp;<span className="radius-value">{radius} km</span>
            </label>
            <input
              type="range"
              min={1}
              max={50}
              value={radius}
              onChange={(e) => handleRadiusChange(e.target.value)}
              className="slider"
            />
            <div className="slider-ends">
              <span>1 km</span>
              <span>50 km</span>
            </div>
          </div>

          {/* Results */}
          <div className="card results-card">
            <div className="results-header">
              <h2>
                {people.length === 0
                  ? 'Nobody nearby yet'
                  : `${people.length} ${people.length === 1 ? 'person' : 'people'} nearby`}
              </h2>
              <button
                className="btn-refresh"
                onClick={() => fetchNearby(locationRef.current, activityRef.current, radius, userId)}
                title="Refresh"
              >
                ↻
              </button>
            </div>

            {lastRefresh && (
              <p className="refresh-time">Updated {timeAgo(lastRefresh.toISOString().replace('Z', ''))}</p>
            )}

            {people.length === 0 ? (
              <div className="empty">
                <p>No one nearby is up for <em>{activity}</em> within {radius} km.</p>
                <p className="empty-sub">You are live — others will see you when they check in!</p>
              </div>
            ) : (
              <ul className="people-list">
                {people.map((p) => (
                  <li key={p.id} className="person-card">
                    <div className="person-avatar">{p.name.charAt(0).toUpperCase()}</div>
                    <div className="person-info">
                      <div className="person-name">{p.name}</div>
                      <div className="person-activity">{p.activity}</div>
                    </div>
                    <div className="person-dist">{p.distance} km</div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
