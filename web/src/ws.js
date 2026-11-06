export async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts
  })
  const text = await res.text()
  try {
    return text ? JSON.parse(text) : {}
  } catch {
    return { ok: false, msg: text }
  }
}

export const get = (p) => api(p)
export const post = (p, body) => api(p, { method: 'POST', body: JSON.stringify(body ?? {}) })

export function connectWs(role, onMsg) {
  let ws = null
  let retry = 0
  let closed = false

  const open = () => {
    if (closed) return
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    ws = new WebSocket(`${proto}//${location.host}/ws/client?role=${role}`)
    ws.onopen = () => {
      retry = 0
      onMsg({ kind: '__open' })
    }
    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data)
        if (msg.kind !== 'ping') onMsg(msg)
      } catch {
        /* ignore */
      }
    }
    ws.onclose = () => {
      if (closed) return
      onMsg({ kind: '__close' })
      retry = Math.min(retry + 1, 5)
      setTimeout(open, 1000 * retry)
    }
  }
  open()

  return {
    close() {
      closed = true
      ws?.close()
    },
    send(data) {
      ws?.readyState === 1 && ws.send(JSON.stringify(data))
    }
  }
}
