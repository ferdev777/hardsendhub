import React, { useState, useEffect, useCallback } from 'react'
import { useAuth } from '../context/AuthContext'
import {
    ArrowLeft,
    Download,
    CheckCircle2,
    Check,
    AlertTriangle,
    Search,
    Filter,
    Mail,
    UserX,
    Loader2,
    MailX,
    ArrowUpDown,
} from 'lucide-react'

const PERIODS = [
    { value: 'day', label: 'Hoy' },
    { value: 'week', label: 'Semana' },
    { value: 'month', label: 'Mes' },
    { value: 'year', label: 'Año' },
    { value: 'all', label: 'Total' },
]

function MissingEmails({ onBack }) {
    const { token } = useAuth()
    const [items, setItems] = useState([])
    const [summary, setSummary] = useState(null)
    const [period, setPeriod] = useState('month')
    const [showResolved, setShowResolved] = useState(false)
    const [loading, setLoading] = useState(true)
    const [selectedIds, setSelectedIds] = useState(new Set())
    const [searchTerm, setSearchTerm] = useState('')
    const [resolving, setResolving] = useState(false)
    const [message, setMessage] = useState(null)
    const [reasonFilter, setReasonFilter] = useState('all') // 'all', 'no_email', 'bounced'
    const [sortNewest, setSortNewest] = useState(true)

    const fetchData = useCallback(async () => {
        setLoading(true)
        try {
            const res = await fetch(
                `/api/missing-emails?period=${period}&show_resolved=${showResolved}`,
                { headers: { Authorization: `Bearer ${token}` } }
            )
            if (res.ok) {
                const data = await res.json()
                setItems(data.items || [])
                setSummary(data.summary)
            }
        } catch (err) {
            console.error('Failed to fetch missing emails', err)
        } finally {
            setLoading(false)
        }
    }, [token, period, showResolved])

    useEffect(() => {
        fetchData()
    }, [fetchData])

    const filteredItems = items
        .filter(item => {
            if (reasonFilter !== 'all' && item.reason !== reasonFilter) return false
            if (!searchTerm) return true
            const lower = searchTerm.toLowerCase()
            return (
                item.invoice_number.toLowerCase().includes(lower) ||
                item.client_name.toLowerCase().includes(lower) ||
                (item.email && item.email.toLowerCase().includes(lower))
            )
        })
        .sort((a, b) => {
            const dateA = new Date(a.created_at)
            const dateB = new Date(b.created_at)
            return sortNewest ? dateB - dateA : dateA - dateB
        })

    const toggleSelect = (id) => {
        setSelectedIds(prev => {
            const next = new Set(prev)
            if (next.has(id)) next.delete(id)
            else next.add(id)
            return next
        })
    }

    const toggleSelectAll = () => {
        if (selectedIds.size === filteredItems.length) {
            setSelectedIds(new Set())
        } else {
            setSelectedIds(new Set(filteredItems.map(i => i.id)))
        }
    }

    const handleResolve = async (ids, all = false) => {
        setResolving(true)
        setMessage(null)
        try {
            const body = all
                ? { all: true, period }
                : { ids: Array.from(ids) }

            const res = await fetch('/api/missing-emails/resolve', {
                method: 'POST',
                headers: {
                    Authorization: `Bearer ${token}`,
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(body),
            })

            if (res.ok) {
                const data = await res.json()
                setMessage({ type: 'success', text: data.message })
                setSelectedIds(new Set())
                fetchData()
            } else {
                setMessage({ type: 'error', text: 'Error al resolver' })
            }
        } catch (err) {
            setMessage({ type: 'error', text: 'Error de conexión' })
        } finally {
            setResolving(false)
        }
    }

    const handleExport = () => {
        const url = `/api/missing-emails/export?period=${period}&show_resolved=${showResolved}`
        const link = document.createElement('a')
        link.href = url
        link.setAttribute('download', '')
        // Add auth header via fetch
        fetch(url, { headers: { Authorization: `Bearer ${token}` } })
            .then(res => res.blob())
            .then(blob => {
                const url = window.URL.createObjectURL(blob)
                link.href = url
                link.click()
                window.URL.revokeObjectURL(url)
            })
    }

    return (
        <div className="min-h-screen">
            <div className="animated-bg" />

            {/* Header */}
            <header className="sticky top-0 z-50 backdrop-blur-xl bg-surface-950/80 border-b border-surface-800/50">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div className="flex items-center justify-between h-16">
                        <div className="flex items-center gap-3">
                            <button
                                onClick={onBack}
                                className="flex items-center gap-2 px-3 py-2 rounded-lg
                                    text-surface-400 hover:text-white hover:bg-surface-800/50
                                    transition-all duration-200 text-sm"
                                id="back-button"
                            >
                                <ArrowLeft className="w-4 h-4" />
                                <span className="hidden sm:inline">Panel</span>
                            </button>
                            <div className="h-6 w-px bg-surface-700" />
                            <div className="flex items-center gap-2">
                                <div className="p-2 rounded-lg bg-amber-500/20">
                                    <MailX className="w-5 h-5 text-amber-400" />
                                </div>
                                <div>
                                    <h1 className="text-lg font-bold text-white leading-tight">
                                        Emails <span className="text-amber-400">Faltantes</span>
                                    </h1>
                                    <p className="text-[10px] text-surface-500 font-medium -mt-0.5">
                                        Sin email / Emails rebotados
                                    </p>
                                </div>
                            </div>
                        </div>

                        {/* Export Button */}
                        <button
                            onClick={handleExport}
                            className="flex items-center gap-2 px-4 py-2 rounded-lg
                                bg-hardsend-500/10 text-hardsend-400 hover:bg-hardsend-500/20 hover:text-hardsend-300
                                border border-hardsend-500/20 hover:border-hardsend-500/40
                                transition-all duration-200 text-sm font-medium"
                            id="export-csv-button"
                        >
                            <Download className="w-4 h-4" />
                            <span className="hidden sm:inline">Descargar CSV</span>
                        </button>
                    </div>
                </div>
            </header>

            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6">
                {/* Summary Cards */}
                {summary && (
                    <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 animate-slide-up">
                        <div className="glass-card p-5">
                            <div className="flex items-center gap-3">
                                <div className="p-2.5 rounded-xl bg-amber-500/10">
                                    <UserX className="w-5 h-5 text-amber-400" />
                                </div>
                                <div>
                                    <p className="text-surface-500 text-xs font-medium uppercase tracking-wider">Total</p>
                                    <p className="text-2xl font-bold text-white tabular-nums">{summary.total.toLocaleString()}</p>
                                </div>
                            </div>
                        </div>
                        <div className="glass-card p-5">
                            <div className="flex items-center gap-3">
                                <div className="p-2.5 rounded-xl bg-orange-500/10">
                                    <AlertTriangle className="w-5 h-5 text-orange-400" />
                                </div>
                                <div>
                                    <p className="text-surface-500 text-xs font-medium uppercase tracking-wider">Pendientes</p>
                                    <p className="text-2xl font-bold text-orange-400 tabular-nums">{summary.pending.toLocaleString()}</p>
                                </div>
                            </div>
                        </div>
                        <div className="glass-card p-5">
                            <div className="flex items-center gap-3">
                                <div className="p-2.5 rounded-xl bg-success/10">
                                    <CheckCircle2 className="w-5 h-5 text-success" />
                                </div>
                                <div>
                                    <p className="text-surface-500 text-xs font-medium uppercase tracking-wider">Resueltos</p>
                                    <p className="text-2xl font-bold text-success tabular-nums">{summary.resolved.toLocaleString()}</p>
                                </div>
                            </div>
                        </div>
                    </div>
                )}

                {/* Filters & Actions Bar */}
                <div className="glass-card p-4 animate-slide-up" style={{ animationDelay: '100ms' }}>
                    <div className="flex flex-col sm:flex-row items-start sm:items-center gap-4">
                        {/* Period Filter */}
                        <div className="flex items-center gap-2">
                            <Filter className="w-4 h-4 text-surface-500" />
                            <div className="flex gap-1">
                                {PERIODS.map(p => (
                                    <button
                                        key={p.value}
                                        onClick={() => setPeriod(p.value)}
                                        className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all duration-200
                                            ${period === p.value
                                                ? 'bg-hardsend-500/20 text-hardsend-300 border border-hardsend-500/40'
                                                : 'text-surface-400 hover:text-surface-200 hover:bg-surface-800/50 border border-transparent'
                                            }`}
                                        id={`filter-${p.value}`}
                                    >
                                        {p.label}
                                    </button>
                                ))}
                            </div>
                        </div>

                        <div className="h-6 w-px bg-surface-700 hidden sm:block" />

                        {/* Reason Filter */}
                        <div className="flex gap-1">
                            {[
                                { value: 'all', label: 'Todos' },
                                { value: 'no_email', label: 'Sin email' },
                                { value: 'bounced', label: 'Rebotados' },
                                { value: 'invalid_email', label: 'Email inválido' },
                            ].map(f => (
                                <button
                                    key={f.value}
                                    onClick={() => setReasonFilter(f.value)}
                                    className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-all duration-200
                                        ${reasonFilter === f.value
                                            ? f.value === 'bounced'
                                                ? 'bg-red-500/20 text-red-300 border border-red-500/40'
                                                : f.value === 'no_email'
                                                    ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40'
                                                    : f.value === 'invalid_email'
                                                        ? 'bg-violet-500/20 text-violet-300 border border-violet-500/40'
                                                        : 'bg-hardsend-500/20 text-hardsend-300 border border-hardsend-500/40'
                                            : 'text-surface-400 hover:text-surface-200 hover:bg-surface-800/50 border border-transparent'
                                        }`}
                                    id={`reason-filter-${f.value}`}
                                >
                                    {f.label}
                                </button>
                            ))}
                        </div>

                        <div className="h-6 w-px bg-surface-700 hidden sm:block" />

                        {/* Sort Toggle */}
                        <button
                            onClick={() => setSortNewest(!sortNewest)}
                            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium
                                text-surface-400 hover:text-surface-200 hover:bg-surface-800/50
                                border border-transparent hover:border-surface-700 transition-all duration-200"
                            id="sort-toggle"
                        >
                            <ArrowUpDown className="w-3.5 h-3.5" />
                            {sortNewest ? 'Más nuevos' : 'Más viejos'}
                        </button>

                        <div className="h-6 w-px bg-surface-700 hidden sm:block" />

                        {/* Show Resolved Toggle */}
                        <button
                            onClick={() => setShowResolved(!showResolved)}
                            className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium transition-all duration-200 border
                                ${showResolved
                                    ? 'bg-surface-700/50 text-surface-200 border-surface-600'
                                    : 'text-surface-500 hover:text-surface-300 border-transparent hover:border-surface-700'
                                }`}
                            id="toggle-resolved"
                        >
                            <Check className="w-3.5 h-3.5" />
                            Mostrar resueltos
                        </button>

                        <div className="flex-1" />

                        {/* Search */}
                        <div className="relative w-full sm:w-64">
                            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-surface-500" />
                            <input
                                type="text"
                                value={searchTerm}
                                onChange={(e) => setSearchTerm(e.target.value)}
                                placeholder="Buscar factura o cliente..."
                                className="w-full pl-9 pr-4 py-2 rounded-lg bg-surface-800/50 border border-surface-700/50
                                    text-surface-200 text-sm placeholder:text-surface-600
                                    focus:outline-none focus:border-hardsend-500/50 focus:ring-1 focus:ring-hardsend-500/20
                                    transition-all duration-200"
                                id="search-missing"
                            />
                        </div>
                    </div>
                </div>

                {/* Bulk Actions */}
                {selectedIds.size > 0 && (
                    <div className="glass-card p-3 flex items-center justify-between animate-slide-down border-hardsend-500/30">
                        <span className="text-sm text-surface-300">
                            <span className="text-hardsend-300 font-bold">{selectedIds.size}</span> seleccionado{selectedIds.size !== 1 ? 's' : ''}
                        </span>
                        <div className="flex gap-2">
                            <button
                                onClick={() => handleResolve(selectedIds)}
                                disabled={resolving}
                                className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium
                                    bg-success/20 text-success hover:bg-success/30
                                    border border-success/30 transition-all duration-200"
                                id="resolve-selected"
                            >
                                {resolving ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Check className="w-3.5 h-3.5" />}
                                Resolver seleccionados
                            </button>
                            <button
                                onClick={() => handleResolve(null, true)}
                                disabled={resolving}
                                className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium
                                    bg-amber-500/20 text-amber-300 hover:bg-amber-500/30
                                    border border-amber-500/30 transition-all duration-200"
                                id="resolve-all"
                            >
                                Resolver todos
                            </button>
                        </div>
                    </div>
                )}

                {/* Message */}
                {message && (
                    <div className={`p-3 rounded-lg border animate-slide-down text-sm ${message.type === 'success'
                        ? 'bg-success/10 border-success/30 text-success'
                        : 'bg-danger/10 border-danger/30 text-danger-light'
                        }`}>
                        {message.text}
                    </div>
                )}

                {/* Table */}
                <div className="glass-card overflow-hidden animate-slide-up" style={{ animationDelay: '200ms' }}>
                    {loading ? (
                        <div className="flex items-center justify-center py-20">
                            <Loader2 className="w-8 h-8 text-hardsend-500 animate-spin" />
                        </div>
                    ) : filteredItems.length === 0 ? (
                        <div className="flex flex-col items-center justify-center py-20 text-center">
                            <Mail className="w-12 h-12 text-surface-700 mb-3" />
                            <p className="text-surface-400 font-medium">No hay emails faltantes</p>
                            <p className="text-surface-600 text-sm mt-1">
                                {searchTerm ? 'Probá con otro término de búsqueda' : 'Todos los clientes tienen email asignado'}
                            </p>
                        </div>
                    ) : (
                        <div className="overflow-x-auto">
                            <table className="w-full">
                                <thead>
                                    <tr className="border-b border-surface-800/50">
                                        <th className="py-3 px-4 text-left w-10">
                                            <button
                                                onClick={toggleSelectAll}
                                                className={`w-5 h-5 rounded border-2 flex items-center justify-center transition-all duration-200
                                                    ${selectedIds.size === filteredItems.length && filteredItems.length > 0
                                                        ? 'bg-hardsend-500 border-hardsend-500'
                                                        : 'border-surface-600 hover:border-surface-400'
                                                    }`}
                                                id="select-all-checkbox"
                                            >
                                                {selectedIds.size === filteredItems.length && filteredItems.length > 0 && (
                                                    <Check className="w-3 h-3 text-white" />
                                                )}
                                            </button>
                                        </th>
                                        <th className="py-3 px-4 text-left text-xs font-semibold text-surface-400 uppercase tracking-wider">
                                            Nro Factura
                                        </th>
                                        <th className="py-3 px-4 text-left text-xs font-semibold text-surface-400 uppercase tracking-wider">
                                            Cliente
                                        </th>
                                        <th className="py-3 px-4 text-left text-xs font-semibold text-surface-400 uppercase tracking-wider">
                                            Email
                                        </th>
                                        <th className="py-3 px-4 text-left text-xs font-semibold text-surface-400 uppercase tracking-wider">
                                            Razón
                                        </th>
                                        <th className="py-3 px-4 text-left text-xs font-semibold text-surface-400 uppercase tracking-wider">
                                            Fecha
                                        </th>
                                        <th className="py-3 px-4 text-left text-xs font-semibold text-surface-400 uppercase tracking-wider">
                                            Estado
                                        </th>
                                        <th className="py-3 px-4 text-center text-xs font-semibold text-surface-400 uppercase tracking-wider">
                                            Acción
                                        </th>
                                    </tr>
                                </thead>
                                <tbody className="divide-y divide-surface-800/30">
                                    {filteredItems.map((item) => (
                                        <tr
                                            key={item.id}
                                            className={`transition-colors duration-150 ${selectedIds.has(item.id)
                                                ? 'bg-hardsend-500/5'
                                                : 'hover:bg-surface-800/20'
                                                }`}
                                        >
                                            <td className="py-3 px-4">
                                                <button
                                                    onClick={() => toggleSelect(item.id)}
                                                    className={`w-5 h-5 rounded border-2 flex items-center justify-center transition-all duration-200
                                                        ${selectedIds.has(item.id)
                                                            ? 'bg-hardsend-500 border-hardsend-500'
                                                            : 'border-surface-600 hover:border-surface-400'
                                                        }`}
                                                >
                                                    {selectedIds.has(item.id) && (
                                                        <Check className="w-3 h-3 text-white" />
                                                    )}
                                                </button>
                                            </td>
                                            <td className="py-3 px-4">
                                                <span className="text-sm font-mono text-hardsend-300">
                                                    {item.invoice_number}
                                                </span>
                                            </td>
                                            <td className="py-3 px-4">
                                                <span className="text-sm text-surface-200">
                                                    {item.client_name || '—'}
                                                </span>
                                            </td>
                                            <td className="py-3 px-4">
                                                <span className="text-sm text-surface-300 font-mono">
                                                    {item.email || '—'}
                                                </span>
                                            </td>
                                            <td className="py-3 px-4">
                                                {item.reason === 'bounced' ? (
                                                    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-red-500/10 text-red-400 border border-red-500/20">
                                                        Rebotó
                                                    </span>
                                                ) : item.reason === 'invalid_email' ? (
                                                    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-violet-500/10 text-violet-400 border border-violet-500/20">
                                                        Email inválido
                                                    </span>
                                                ) : (
                                                    <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">
                                                        Sin email
                                                    </span>
                                                )}
                                            </td>
                                            <td className="py-3 px-4">
                                                <span className="text-sm text-surface-400 tabular-nums">
                                                    {new Date(item.created_at).toLocaleDateString('es-AR', {
                                                        day: '2-digit',
                                                        month: '2-digit',
                                                        year: 'numeric',
                                                        hour: '2-digit',
                                                        minute: '2-digit',
                                                    })}
                                                </span>
                                            </td>
                                            <td className="py-3 px-4">
                                                {item.resolved ? (
                                                    <span className="badge badge-success">Resuelto</span>
                                                ) : (
                                                    <span className="badge badge-error">Pendiente</span>
                                                )}
                                            </td>
                                            <td className="py-3 px-4 text-center">
                                                {!item.resolved && (
                                                    <button
                                                        onClick={() => handleResolve(new Set([item.id]))}
                                                        disabled={resolving}
                                                        className="p-1.5 rounded-lg hover:bg-success/20 text-surface-500 hover:text-success
                                                            transition-all duration-200"
                                                        title="Marcar como resuelto"
                                                    >
                                                        <CheckCircle2 className="w-4 h-4" />
                                                    </button>
                                                )}
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    )}

                    {/* Table Footer */}
                    {!loading && filteredItems.length > 0 && (
                        <div className="px-4 py-3 border-t border-surface-800/50 flex items-center justify-between">
                            <span className="text-xs text-surface-500">
                                Mostrando {filteredItems.length} de {items.length} registro{items.length !== 1 ? 's' : ''}
                            </span>
                            {selectedIds.size > 0 && (
                                <span className="text-xs text-hardsend-400">
                                    {selectedIds.size} seleccionado{selectedIds.size !== 1 ? 's' : ''}
                                </span>
                            )}
                        </div>
                    )}
                </div>

                {/* Footer */}
                <footer className="text-center py-6 border-t border-surface-800/50">
                    <p className="text-surface-600 text-xs">
                        © 2026 Fernando Hirschfeld & Devrow. All rights reserved. Closed Source.
                    </p>
                </footer>
            </main>
        </div>
    )
}

export default MissingEmails
