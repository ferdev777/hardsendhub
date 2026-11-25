import React, { useState, useEffect } from 'react'
import {
    ShieldAlert,
    ArrowLeft,
    Search,
    Download,
    RefreshCw,
    XCircle,
    AlertOctagon,
    Clock,
    FileSpreadsheet,
    Mail,
    Info
} from 'lucide-react'
import { useAuth } from '../context/AuthContext'

function SuppressionsView({ onBack }) {
    const { token } = useAuth()
    const [suppressions, setSuppressions] = useState([])
    const [loading, setLoading] = useState(true)
    const [searchTerm, setSearchTerm] = useState('')

    const fetchSuppressions = async () => {
        setLoading(true)
        try {
            const res = await fetch('/api/suppressions', {
                headers: { Authorization: `Bearer ${token}` }
            })
            if (res.ok) {
                const data = await res.json()
                setSuppressions(data || [])
            } else {
                // Fallback to error datagrid
                const fallbackRes = await fetch('/api/errors', {
                    headers: { Authorization: `Bearer ${token}` }
                })
                if (fallbackRes.ok) {
                    const fallbackData = await fallbackRes.json()
                    const mapped = (fallbackData || []).map(err => ({
                        email: err.recipient_email || 'desconocido',
                        reason: err.error_reason || 'Rebote duro detectado',
                        source: 'cloudflare',
                        original_invoice: err.invoice_number,
                        created_at: err.created_at || new Date().toISOString()
                    }))
                    setSuppressions(mapped)
                }
            }
        } catch (err) {
            console.error('Error fetching suppressions:', err)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchSuppressions()
    }, [token])

    const filtered = suppressions.filter(s => 
        s.email.toLowerCase().includes(searchTerm.toLowerCase()) ||
        (s.original_invoice && s.original_invoice.toLowerCase().includes(searchTerm.toLowerCase())) ||
        (s.reason && s.reason.toLowerCase().includes(searchTerm.toLowerCase()))
    )

    const handleExportCSV = () => {
        if (filtered.length === 0) return
        const headers = ['Email', 'Motivo', 'Origen', 'Factura Original', 'Fecha de Registro', 'Caducidad']
        const rows = filtered.map(s => [
            s.email,
            `"${(s.reason || '').replace(/"/g, '""')}"`,
            s.source || 'cloudflare',
            s.original_invoice || '',
            s.created_at || '',
            s.expires_at || 'Permanente'
        ])
        const csvContent = 'data:text/csv;charset=utf-8,' + [headers.join(','), ...rows.map(e => e.join(','))].join('\n')
        const encodedUri = encodeURI(csvContent)
        const link = document.createElement('a')
        link.setAttribute('href', encodedUri)
        link.setAttribute('download', `supresiones_y_rebotes_${new Date().toISOString().slice(0, 10)}.csv`)
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
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
                            <ShieldAlert className="w-5 h-5 text-rose-400" />
                            Lista Negra y Supresiones (Anti-Bounce)
                        </h1>
                        <p className="text-surface-400 text-xs">
                            Direcciones bloqueadas automáticamente para proteger la reputación del dominio y evitar rebotes en Cloudflare
                        </p>
                    </div>
                </div>

                <div className="flex items-center gap-2">
                    <button 
                        onClick={handleExportCSV}
                        disabled={filtered.length === 0}
                        className="btn-secondary text-xs flex items-center gap-2 py-2"
                    >
                        <FileSpreadsheet className="w-3.5 h-3.5 text-emerald-400" />
                        Exportar CSV ({filtered.length})
                    </button>
                    <button 
                        onClick={fetchSuppressions}
                        disabled={loading}
                        className="btn-secondary text-xs flex items-center gap-2 py-2"
                    >
                        <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
                        Actualizar
                    </button>
                </div>
            </div>

            {/* Info Banner */}
            <div className="glass-card p-4 mb-6 border-l-4 border-l-rose-500 flex items-start gap-3">
                <Info className="w-5 h-5 text-rose-400 flex-shrink-0 mt-0.5" />
                <p className="text-surface-300 text-xs leading-relaxed">
                    Estas direcciones generaron rechazos permanentes (hard bounces) o temporales reiterados. El sistema las bloquea en <strong>Capa 1 (Pre-Vuelo)</strong> para que nunca consuman tu cuota diaria de Cloudflare. Podés exportar esta lista para que administración solicite los correos actualizados a los abonados.
                </p>
            </div>

            {/* Filter Bar */}
            <div className="glass-card p-4 mb-6 flex items-center justify-between gap-4">
                <div className="relative flex-1">
                    <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-surface-500" />
                    <input 
                        type="text"
                        placeholder="Buscar por email, factura o motivo de rebote..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                        className="input-field text-sm pl-9 py-2 w-full"
                    />
                </div>
            </div>

            {/* Table */}
            <div className="glass-card overflow-hidden">
                <div className="overflow-x-auto">
                    <table className="w-full text-left text-sm">
                        <thead className="bg-surface-900/60 text-surface-400 text-xs uppercase tracking-wider border-b border-surface-700/50">
                            <tr>
                                <th className="py-3 px-4">Correo Bloqueado</th>
                                <th className="py-3 px-4">Motivo de Supresión</th>
                                <th className="py-3 px-4">Factura Asociada</th>
                                <th className="py-3 px-4">Fecha de Detección</th>
                                <th className="py-3 px-4">Caducidad</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-surface-800/60 text-xs">
                            {loading ? (
                                <tr>
                                    <td colSpan="5" className="py-12 text-center text-surface-500">
                                        <div className="flex flex-col items-center gap-2">
                                            <div className="w-6 h-6 border-2 border-rose-500 border-t-transparent rounded-full animate-spin" />
                                            <span>Sincronizando supresiones de Cloudflare...</span>
                                        </div>
                                    </td>
                                </tr>
                            ) : filtered.length === 0 ? (
                                <tr>
                                    <td colSpan="5" className="py-12 text-center text-surface-500 font-sans">
                                        No hay direcciones en la lista de supresiones.
                                    </td>
                                </tr>
                            ) : (
                                filtered.map((item, idx) => (
                                    <tr key={idx} className="hover:bg-surface-800/40 transition-colors">
                                        <td className="py-3 px-4 font-mono text-white font-medium">
                                            <div className="flex items-center gap-2">
                                                <XCircle className="w-3.5 h-3.5 text-rose-400" />
                                                {item.email}
                                            </div>
                                        </td>
                                        <td className="py-3 px-4 text-surface-300">
                                            <span className="inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium bg-rose-500/10 text-rose-300 border border-rose-500/20">
                                                {item.reason}
                                            </span>
                                        </td>
                                        <td className="py-3 px-4 font-mono text-hardsend-300">
                                            {item.original_invoice ? `#${item.original_invoice}` : '---'}
                                        </td>
                                        <td className="py-3 px-4 text-surface-400">
                                            {new Date(item.created_at).toLocaleDateString()}
                                        </td>
                                        <td className="py-3 px-4 text-surface-500 font-mono text-[11px]">
                                            {item.expires_at ? new Date(item.expires_at).toLocaleDateString() : 'Permanente'}
                                        </td>
                                    </tr>
                                ))
                            )}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    )
}

export default SuppressionsView

