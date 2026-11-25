import React, { useState, useEffect } from 'react'
import {
    Activity,
    ArrowLeft,
    CheckCircle2,
    XCircle,
    AlertTriangle,
    Search,
    RefreshCw,
    Download,
    Eye,
    ShieldAlert,
    Clock,
    Mail,
    FileText,
    Server,
    Filter
} from 'lucide-react'
import { useAuth } from '../context/AuthContext'

function LogsViewer({ onBack }) {
    const { token } = useAuth()
    const [events, setEvents] = useState([])
    const [loading, setLoading] = useState(true)
    const [searchTerm, setSearchTerm] = useState('')
    const [filterType, setFilterType] = useState('ALL')
    const [selectedEvent, setSelectedEvent] = useState(null)

    const fetchEvents = async () => {
        setLoading(true)
        try {
            const res = await fetch('/api/logs?limit=100', {
                headers: { Authorization: `Bearer ${token}` }
            })
            if (res.ok) {
                const data = await res.json()
                setEvents(data || [])
            } else {
                // Fallback to recent job errors if /api/logs is populating
                const fallbackRes = await fetch('/api/errors', {
                    headers: { Authorization: `Bearer ${token}` }
                })
                if (fallbackRes.ok) {
                    const fallbackData = await fallbackRes.json()
                    const mapped = (fallbackData || []).map(err => ({
                        id: err.id || Math.random().toString(),
                        invoice_id: err.invoice_number,
                        recipient: err.recipient_email || 'desconocido',
                        event_type: err.status === 'ERROR_VALIDATION' ? 'bounced' : 'failed',
                        bounce_type: 'hard',
                        smtp_code: '550',
                        smtp_response: err.error_reason || 'Rechazado',
                        created_at: err.created_at || new Date().toISOString()
                    }))
                    setEvents(mapped)
                }
            }
        } catch (err) {
            console.error('Error fetching logs:', err)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchEvents()
    }, [token])

    const filteredEvents = events.filter(e => {
        const matchesSearch = 
            (e.recipient && e.recipient.toLowerCase().includes(searchTerm.toLowerCase())) ||
            (e.invoice_id && e.invoice_id.toLowerCase().includes(searchTerm.toLowerCase())) ||
            (e.message_id && e.message_id.toLowerCase().includes(searchTerm.toLowerCase()))
        
        if (!matchesSearch) return false
        if (filterType === 'ALL') return true
        if (filterType === 'DELIVERED') return e.event_type === 'delivered' || e.event_type === 'accepted'
        if (filterType === 'BOUNCED') return e.event_type === 'bounced' || e.event_type === 'deferred'
        if (filterType === 'COMPLAINED') return e.event_type === 'complained'
        if (filterType === 'FAILED') return e.event_type === 'failed' || e.event_type === 'rejected'
        return true
    })

    const getEventBadge = (type, bounceType) => {
        switch (type) {
            case 'delivered':
                return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20"><CheckCircle2 className="w-3 h-3" /> Entregado</span>
            case 'accepted':
                return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-sky-500/10 text-sky-400 border border-sky-500/20"><CheckCircle2 className="w-3 h-3" /> Aceptado</span>
            case 'bounced':
                return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-rose-500/10 text-rose-400 border border-rose-500/20"><XCircle className="w-3 h-3" /> Rebote {bounceType === 'hard' ? 'Duro' : 'Suave'}</span>
            case 'deferred':
                return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20"><AlertTriangle className="w-3 h-3" /> Pospuesto (Reintento)</span>
            case 'complained':
                return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-purple-500/10 text-purple-400 border border-purple-500/20"><ShieldAlert className="w-3 h-3" /> Queja de Spam</span>
            case 'rejected':
                return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-orange-500/10 text-orange-400 border border-orange-500/20"><XCircle className="w-3 h-3" /> Suprimido</span>
            default:
                return <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium bg-surface-700 text-surface-300">{type}</span>
        }
    }

    return (
        <div className="min-h-screen pb-12 pt-6 px-4 sm:px-6 lg:px-8 max-w-7xl mx-auto">
            {/* Header */}
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
                <div className="flex items-center gap-3">
                    <button 
                        onClick={onBack}
                        className="p-2 rounded-lg bg-surface-800/80 hover:bg-surface-700 text-surface-300 hover:text-white transition-colors border border-surface-700/50"
                        title="Volver al panel"
                    >
                        <ArrowLeft className="w-5 h-5" />
                    </button>
                    <div>
                        <h1 className="text-xl font-bold text-white flex items-center gap-2">
                            <Activity className="w-5 h-5 text-hardsend-400" />
                            Auditoría y Logs de Entrega
                        </h1>
                        <p className="text-surface-400 text-xs">
                            Registro de eventos de Cloudflare Email Sending y servidores de correo de destino
                        </p>
                    </div>
                </div>

                <div className="flex items-center gap-2">
                    <button 
                        onClick={fetchEvents}
                        disabled={loading}
                        className="btn-secondary text-xs flex items-center gap-2 py-2"
                    >
                        <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
                        Actualizar
                    </button>
                </div>
            </div>

            {/* Filters Bar */}
            <div className="glass-card p-4 mb-6 flex flex-col md:flex-row items-stretch md:items-center justify-between gap-4">
                <div className="relative flex-1">
                    <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-surface-500" />
                    <input 
                        type="text"
                        placeholder="Buscar por email, factura o Message-ID..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="input-field text-sm pl-9 py-2 w-full"
                    />
                </div>

                <div className="flex items-center gap-2 overflow-x-auto pb-1 md:pb-0">
                    <span className="text-surface-500 text-xs flex items-center gap-1 font-medium mr-1">
                        <Filter className="w-3.5 h-3.5" /> Filtro:
                    </span>
                    {['ALL', 'DELIVERED', 'BOUNCED', 'COMPLAINED', 'FAILED'].map((type) => (
                        <button
                            key={type}
                            onClick={() => setFilterType(type)}
                            className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all ${
                                filterType === type 
                                    ? 'bg-hardsend-500 text-white shadow-lg shadow-hardsend-500/20' 
                                    : 'bg-surface-800 text-surface-400 hover:text-white hover:bg-surface-700'
                            }`}
                        >
                            {type === 'ALL' && 'Todos'}
                            {type === 'DELIVERED' && 'Entregados'}
                            {type === 'BOUNCED' && 'Rebotes'}
                            {type === 'COMPLAINED' && 'Quejas'}
                            {type === 'FAILED' && 'Fallidos'}
                        </button>
                    ))}
                </div>
            </div>

            {/* Logs Table */}
            <div className="glass-card overflow-hidden">
                <div className="overflow-x-auto">
                    <table className="w-full text-left text-sm">
                        <thead className="bg-surface-900/60 text-surface-400 text-xs uppercase tracking-wider border-b border-surface-700/50">
                            <tr>
                                <th className="py-3 px-4">Fecha y Hora</th>
                                <th className="py-3 px-4">Destinatario</th>
                                <th className="py-3 px-4">Factura</th>
                                <th className="py-3 px-4">Estado del Evento</th>
                                <th className="py-3 px-4">Respuesta Servidor / SMTP</th>
                                <th className="py-3 px-4 text-right">Detalle</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-surface-800/60 font-mono text-xs">
                            {loading ? (
                                <tr>
                                    <td colSpan="6" className="py-12 text-center text-surface-500">
                                        <div className="flex flex-col items-center gap-2">
                                            <div className="w-6 h-6 border-2 border-hardsend-500 border-t-transparent rounded-full animate-spin" />
                                            <span>Consultando registros de auditoría...</span>
                                        </div>
                                    </td>
                                </tr>
                            ) : filteredEvents.length === 0 ? (
                                <tr>
                                    <td colSpan="6" className="py-12 text-center text-surface-500 font-sans">
                                        No se encontraron eventos con los filtros seleccionados.
                                    </td>
                                </tr>
                            ) : (
                                filteredEvents.map((evt, idx) => (
                                    <tr key={evt.id || idx} className="hover:bg-surface-800/40 transition-colors">
                                        <td className="py-3 px-4 text-surface-400 whitespace-nowrap">
                                            {new Date(evt.created_at).toLocaleString()}
                                        </td>
                                        <td className="py-3 px-4 text-white font-medium">
                                            {evt.recipient}
                                        </td>
                                        <td className="py-3 px-4 text-hardsend-300">
                                            #{evt.invoice_id || '---'}
                                        </td>
                                        <td className="py-3 px-4 font-sans whitespace-nowrap">
                                            {getEventBadge(evt.event_type, evt.bounce_type)}
                                        </td>
                                        <td className="py-3 px-4 text-surface-400 truncate max-w-xs">
                                            {evt.smtp_response || evt.bounce_reason || '250 OK - Aceptado'}
                                        </td>
                                        <td className="py-3 px-4 text-right font-sans">
                                            <button 
                                                onClick={() => setSelectedEvent(evt)}
                                                className="p-1.5 rounded-lg bg-surface-800 hover:bg-surface-700 text-surface-400 hover:text-white transition-colors"
                                                title="Ver detalle del evento"
                                            >
                                                <Eye className="w-3.5 h-3.5" />
                                            </button>
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
            </div>

            {/* Event Detail Modal */}
            {selectedEvent && (
                <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
                    <div className="glass-card max-w-lg w-full p-6 animate-scale-up border border-surface-700/80 shadow-2xl">
                        <div className="flex items-center justify-between pb-4 border-b border-surface-700/50 mb-4">
                            <h3 className="text-white font-semibold text-base flex items-center gap-2">
                                <FileText className="w-4 h-4 text-hardsend-400" />
                                Detalle Técnico del Evento
                            </h3>
                            <button 
                                onClick={() => setSelectedEvent(null)}
                                className="text-surface-400 hover:text-white p-1 rounded-lg hover:bg-surface-800"
                            >
                                <XCircle className="w-5 h-5" />
                            </button>
                        </div>

                        <div className="space-y-3 text-xs">
                            <div className="flex justify-between py-1.5 border-b border-surface-800">
                                <span className="text-surface-400">Tipo de Evento:</span>
                                <span>{getEventBadge(selectedEvent.event_type, selectedEvent.bounce_type)}</span>
                            </div>
                            <div className="flex justify-between py-1.5 border-b border-surface-800">
                                <span className="text-surface-400">Destinatario:</span>
                                <span className="text-white font-medium">{selectedEvent.recipient}</span>
                            </div>
                            <div className="flex justify-between py-1.5 border-b border-surface-800">
                                <span className="text-surface-400">Número de Factura:</span>
                                <span className="text-hardsend-300 font-mono">#{selectedEvent.invoice_id || '---'}</span>
                            </div>
                            <div className="flex justify-between py-1.5 border-b border-surface-800">
                                <span className="text-surface-400">Timestamp:</span>
                                <span className="text-surface-300 font-mono">{new Date(selectedEvent.created_at).toISOString()}</span>
                            </div>
                            {selectedEvent.smtp_code && (
                                <div className="flex justify-between py-1.5 border-b border-surface-800">
                                    <span className="text-surface-400">Código SMTP:</span>
                                    <span className="text-amber-400 font-mono">{selectedEvent.smtp_code}</span>
                                </div>
                            )}
                            <div>
                                <span className="text-surface-400 block mb-1">Respuesta Completa del Servidor:</span>
                                <pre className="bg-surface-950 p-3 rounded-lg border border-surface-800 text-surface-300 font-mono text-[11px] overflow-x-auto whitespace-pre-wrap">
                                    {selectedEvent.smtp_response || selectedEvent.bounce_reason || 'Sin diagnóstico adicional'}
                                </pre>
                            </div>
                        </div>

                        <div className="mt-6 flex justify-end">
                            <button 
                                onClick={() => setSelectedEvent(null)}
                                className="btn-secondary text-xs px-4 py-2"
                            >
                                Cerrar
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}

export default LogsViewer

