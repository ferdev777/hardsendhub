import { useState, useEffect, useRef, useCallback } from 'react'
import { WS_BASE } from '../config/api'

export function useWebSocket(token) {
    const [metrics, setMetrics] = useState(null)
    const [events, setEvents] = useState([])
    const [connected, setConnected] = useState(false)
    const wsRef = useRef(null)
    const reconnectTimeoutRef = useRef(null)
    const reconnectAttemptsRef = useRef(0)
    const maxReconnectAttempts = 10

    const connect = useCallback(() => {
        if (!token) return

        // Determine WebSocket URL for Wails desktop app
        const wsUrl = `${WS_BASE}/ws/metrics?token=${token}`

        try {
            const ws = new WebSocket(wsUrl)
            wsRef.current = ws

            ws.onopen = () => {
                console.log('[WebSocket] Connected')
                setConnected(true)
                reconnectAttemptsRef.current = 0
            }

            ws.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data)

                    // Handle metrics update
                    if (data.processed_count !== undefined) {
                        setMetrics(data)
                    }

                    // Handle individual activity event if present in the message
                    // (Assuming the backend might send event-type messages now)
                    if (data.type === 'activity_event' || data.event_type) {
                        setEvents(prev => [data, ...prev].slice(0, 50))
                    }
                } catch (err) {
                    console.error('[WebSocket] Failed to parse message:', err)
                }
            }

            ws.onclose = (event) => {
                console.log('[WebSocket] Disconnected:', event.code, event.reason)
                setConnected(false)
                wsRef.current = null

                // Auto-reconnect with exponential backoff
                if (reconnectAttemptsRef.current < maxReconnectAttempts) {
                    const delay = Math.min(1000 * Math.pow(2, reconnectAttemptsRef.current), 30000)
                    reconnectAttemptsRef.current++
                    console.log(`[WebSocket] Reconnecting in ${delay}ms (attempt ${reconnectAttemptsRef.current})`)
                    reconnectTimeoutRef.current = setTimeout(connect, delay)
                }
            }

            ws.onerror = (error) => {
                console.error('[WebSocket] Error:', error)
            }
        } catch (err) {
            console.error('[WebSocket] Connection failed:', err)
        }
    }, [token])

    useEffect(() => {
        connect()

        return () => {
            if (wsRef.current) {
                wsRef.current.close()
            }
            if (reconnectTimeoutRef.current) {
                clearTimeout(reconnectTimeoutRef.current)
            }
        }
    }, [connect])

    return { metrics, events, connected }
}
