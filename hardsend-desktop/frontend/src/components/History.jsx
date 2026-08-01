import React, { useState, useEffect, useCallback } from 'react'
import { useAuth } from '../context/AuthContext'
import { API_BASE } from '../config/api'
import Footer from './Footer'
import {
    Calendar,
    Clock,
    FileText,
    CheckCircle2,
    XCircle,
    TrendingUp,
    ChevronLeft,
    Filter,
} from 'lucide-react'

const PERIODS = [
    { key: 'day', label: 'Hoy' },
    { key: 'week', label: 'Semana' },
    { key: 'month', label: 'Mes' },
    { key: 'year', label: 'Año' },
    { key: 'all', label: 'Todo' },
]

function History({ onBack }) {
    const { token } = useAuth()
    const [period, setPeriod] = useState('month')
    const [data, setData] = useState(null)
    const [loading, setLoading] = useState(true)

    const fetchHistory = useCallback(async () => {
        setLoading(true)
        try {
            const res = await fetch(`${API_BASE}/api/history?period=${period}`, {
                headers: { Authorization: `Bearer ${token}` },
            })
            if (res.ok) {
                const json = await res.json()
                setData(json)
            }
        } catch (err) {
            console.error('Failed to fetch history:', err)
        } finally {
            setLoading(false)
        }
    }, [period, token])

    useEffect(() => {
        fetchHistory()
    }, [fetchHistory])

    const formatDate = (dateStr) => {
        const d = new Date(dateStr)
        return d.toLocaleDateString('es-AR', {
            day: '2-digit',
            month: '2-digit',
            year: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
        })
    }

    const SummaryCard = ({ icon: Icon, label, value, color, subtext }) => (
        <div className="glass-card p-5 text-center animate-slide-up">
            <div className={`inline-flex p-3 rounded-xl mb-3`} style={{ backgroundColor: `${color}20` }}>
                <Icon className="w-6 h-6" style={{ color }} />
            </div>
            <p className="text-2xl font-bold text-white font-mono tabular-nums">{value?.toLocaleString() || '0'}</p>
            <p className="text-surface-400 text-xs mt-1 font-medium">{label}</p>
            {subtext && <p className="text-surface-600 text-[10px] mt-0.5">{subtext}</p>}
        </div>
    )

    return (
        <div className="min-h-screen">
            <div className="animated-bg" />

            {/* Floating Premium Navbar */}
            <div className="fixed top-4 left-1/2 -translate-x-1/2 z-50 w-[96%] max-w-7xl">
                <header className="glass-card rounded-2xl px-3 sm:px-5 h-16 flex items-center justify-between shadow-[0_8px_32px_rgba(0,0,0,0.4)] border border-surface-700/50 bg-surface-900/90">
                    <div className="flex items-center gap-2 sm:gap-4">
                        <button
                            onClick={onBack}
                            className="flex items-center gap-1 sm:gap-2 px-2 sm:px-3 py-1.5 rounded-lg text-surface-400 hover:text-white hover:bg-surface-800/50 transition-all duration-200 text-sm font-medium"
                            id="back-to-dashboard"
                        >
                            <ChevronLeft className="w-4 h-4" />
                            <span className="hidden sm:inline">Panel</span>
                        </button>
                        <div className="h-6 w-px bg-surface-800" />
                        <div className="flex items-center gap-2 sm:gap-3">
                            <div className="w-8 h-8 rounded-lg flex items-center justify-center shadow-lg border border-white/10" style={{ background: 'var(--gradient-primary)' }}>
                                <Calendar className="w-4 h-4 text-white" />
                            </div>
                            <div className="hidden sm:block">
                                <h1 className="text-sm font-bold text-white leading-none mb-0.5">Registro Histórico</h1>
                                <p className="text-[10px] text-surface-400 font-medium">Estadísticas</p>
                            </div>
                        </div>
                    </div>

                    {/* Period filters */}
                    <div className="flex items-center gap-1 bg-surface-950/40 border border-surface-800/50 rounded-xl p-1 overflow-x-auto">
                        <Filter className="w-3.5 h-3.5 text-surface-500 ml-2 hidden sm:block" />
                        {PERIODS.map((p) => (
                            <button
                                key={p.key}
                                onClick={() => setPeriod(p.key)}
                                className={`px-2 sm:px-3 py-1.5 rounded-lg text-xs font-medium transition-all duration-200 whitespace-nowrap ${period === p.key
                                    ? 'bg-hardsend-500 text-white shadow-lg shadow-hardsend-500/25'
                                    : 'text-surface-400 hover:text-white hover:bg-surface-800/50'
                                    }`}
                                id={`filter-${p.key}`}
                            >
                                {p.label}
                            </button>
                        ))}
                    </div>
                </header>
            </div>

            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-28 pb-6 space-y-6">
                {loading ? (
                    <div className="flex items-center justify-center py-20">
                        <div className="w-10 h-10 border-4 border-hardsend-500/30 border-t-hardsend-500 rounded-full animate-spin" />
                    </div>
                ) : data ? (
                    <>
                        {/* Summary Cards */}
                        <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
                            <SummaryCard icon={FileText} label="Envíos" value={data.summary?.total_jobs} color="#6366f1" />
                            <SummaryCard icon={Clock} label="Facturas" value={data.summary?.total_invoices} color="#8b5cf6" />
                            <SummaryCard icon={CheckCircle2} label="Exitosas" value={data.summary?.total_success} color="#10b981" />
                            <SummaryCard icon={XCircle} label="Errores" value={data.summary?.total_errors} color="#ef4444" />
                            <SummaryCard
                                icon={TrendingUp}
                                label="Tasa de Éxito"
                                value={`${(data.summary?.success_rate || 0).toFixed(1)}%`}
                                color="#f59e0b"
                            />
                        </div>

                        {/* Jobs Table */}
                        <div className="glass-card overflow-hidden animate-slide-up" style={{ animationDelay: '200ms' }}>
                            <div className="p-5 border-b border-surface-700/50">
                                <h3 className="text-white font-semibold text-sm flex items-center gap-2">
                                    <FileText className="w-4 h-4 text-hardsend-400" />
                                    Historial de Envíos
                                    <span className="text-surface-500 font-normal ml-1">
                                        ({data.jobs?.length || 0} registros)
                                    </span>
                                </h3>
                            </div>

                            {data.jobs?.length > 0 ? (
                                <div className="overflow-x-auto">
                                    <table className="w-full">
                                        <thead>
                                            <tr className="border-b border-surface-700/50">
                                                <th className="text-left py-3 px-5 text-surface-400 text-xs font-semibold uppercase tracking-wider">Fecha</th>
                                                <th className="text-left py-3 px-5 text-surface-400 text-xs font-semibold uppercase tracking-wider">Archivo</th>
                                                <th className="text-center py-3 px-5 text-surface-400 text-xs font-semibold uppercase tracking-wider">Total</th>
                                                <th className="text-center py-3 px-5 text-surface-400 text-xs font-semibold uppercase tracking-wider">Exitosas</th>
                                                <th className="text-center py-3 px-5 text-surface-400 text-xs font-semibold uppercase tracking-wider">Errores</th>
                                                <th className="text-center py-3 px-5 text-surface-400 text-xs font-semibold uppercase tracking-wider">Estado</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {data.jobs.map((job, idx) => (
                                                <tr
                                                    key={job.id}
                                                    className="border-b border-surface-800/30 hover:bg-surface-800/30 transition-colors"
                                                    style={{ animationDelay: `${idx * 30}ms` }}
                                                >
                                                    <td className="py-3 px-5 text-surface-300 text-xs font-mono tabular-nums">
                                                        {formatDate(job.created_at)}
                                                    </td>
                                                    <td className="py-3 px-5 text-white text-xs font-medium max-w-[200px] truncate">
                                                        {job.filename}
                                                    </td>
                                                    <td className="py-3 px-5 text-center text-surface-300 text-xs font-mono tabular-nums">
                                                        {job.total_files}
                                                    </td>
                                                    <td className="py-3 px-5 text-center">
                                                        <span className="text-emerald-400 text-xs font-bold font-mono tabular-nums">
                                                            {job.success_count}
                                                        </span>
                                                    </td>
                                                    <td className="py-3 px-5 text-center">
                                                        <span className={`text-xs font-bold font-mono tabular-nums ${job.error_count > 0 ? 'text-red-400' : 'text-surface-600'
                                                            }`}>
                                                            {job.error_count}
                                                        </span>
                                                    </td>
                                                    <td className="py-3 px-5 text-center">
                                                        <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-wider ${job.status === 'COMPLETED'
                                                            ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                                                            : job.status === 'PROCESSING'
                                                                ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                                                                : 'bg-red-500/10 text-red-400 border border-red-500/20'
                                                            }`}>
                                                            {job.status === 'COMPLETED' ? 'Completado' :
                                                                job.status === 'PROCESSING' ? 'Procesando' : 'Error'}
                                                        </span>
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            ) : (
                                <div className="py-16 text-center">
                                    <Calendar className="w-12 h-12 mx-auto mb-4 text-surface-700" />
                                    <p className="text-surface-500 text-sm">No hay registros en este período</p>
                                    <p className="text-surface-600 text-xs mt-1">
                                        Seleccioná otro filtro o realizá un envío primero
                                    </p>
                                </div>
                            )}
                        </div>
                    </>
                ) : null}

                {/* Footer */}
                <Footer />
            </main>
        </div>
    )
}

export default History
