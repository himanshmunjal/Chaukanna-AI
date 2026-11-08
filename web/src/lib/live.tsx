// One WebSocket connection to the backend, shared by the whole app.
//
//   <LiveProvider>            opens /ws/dashboard and reconnects automatically
//   useLiveEvents(handler)    runs `handler` for every event
//   useLiveConnected()        true while the socket is open

import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { dashboardSocketURL, type LiveEvent } from './api'

type Listener = (event: LiveEvent) => void

interface LiveContextValue {
  connected: boolean
  subscribe: (listener: Listener) => () => void
}

const LiveContext = createContext<LiveContextValue | null>(null)

export function LiveProvider({ children }: { children: ReactNode }) {
  const listeners = useRef(new Set<Listener>())
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    let socket: WebSocket | null = null
    let retryTimer: number | undefined
    let attempt = 0
    let stopped = false

    const connect = () => {
      socket = new WebSocket(dashboardSocketURL())
      socket.onopen = () => {
        attempt = 0
        setConnected(true)
      }
      socket.onmessage = (msg) => {
        const event = JSON.parse(msg.data) as LiveEvent
        listeners.current.forEach((listener) => listener(event))
      }
      socket.onclose = () => {
        setConnected(false)
        if (!stopped) {
          // Back off: 1s, 2s, 4s ... up to 10s.
          retryTimer = window.setTimeout(connect, Math.min(1000 * 2 ** attempt++, 10_000))
        }
      }
    }

    connect()
    return () => {
      stopped = true
      window.clearTimeout(retryTimer)
      socket?.close()
    }
  }, [])

  const subscribe = useCallback((listener: Listener) => {
    listeners.current.add(listener)
    return () => {
      listeners.current.delete(listener)
    }
  }, [])

  return <LiveContext.Provider value={{ connected, subscribe }}>{children}</LiveContext.Provider>
}

/** Call `handler` for every live event. The latest handler is always used. */
export function useLiveEvents(handler: Listener) {
  const ctx = useContext(LiveContext)
  const latest = useRef(handler)
  latest.current = handler

  useEffect(() => {
    if (!ctx) return
    return ctx.subscribe((event) => latest.current(event))
  }, [ctx])
}

export function useLiveConnected(): boolean {
  return useContext(LiveContext)?.connected ?? false
}
