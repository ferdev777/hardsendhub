import React, { useState, useEffect } from 'react'
import {
    AreaChart,
    Area,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    ResponsiveContainer,
    Legend,
} from 'recharts'
import {
    BarChart3,
    Calendar,
    TrendingUp,
    CheckCircle2,
    Mail,
    AlertTriangle,
    RefreshCw,
} from 'lucide-react'

function AnalyticsSection({ token }) {
    const [range, setRange] = useState('day') // day, week, month, year
    const [summary, setSummary] = useState(null)
    const [timeSeries, setTimeSeries] = useState([])
    const [loading, setLoading] = useState(false)

    const fetchAnalytics = async () => {
        if (!token) return
        setLoading(true)
        try {
            const [resSummary, resTime] = await Promise.all([
                fetch(`/api/analytics/summary?period=${range}`, {
                    headers: { Authorization: `Bearer ${token}` },
                }),
                fetch(`/api/analytics/timeseries?range=${range}`, {
                    headers: { Authorization: `Bearer ${token}` },
                }),
            ])
            if (resSummary.ok) {
                const dataSum = await resSummary.json()
                setSummary(dataSum)
            }
            if (resTime.ok) {
                const dataTime = await resTime.json()
                setTimeSeries(dataTime || [])
            }
        } catch (err) {
            console.error('[Analytics] Error fetching analytics:', err)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchAnalytics()
        const timer = setInterval(fetchAnalytics, 15000)
        return () => clearInterval(timer)
    }, [token, range])

    const ranges = [
        { id: 'day', label: 'Día' },
        { id: 'week', label: 'Semana' },
        { id: 'month', label: 'Mes' },
        { id: 'year', label: 'Año' },
    ]

    const CustomTooltip = ({ active, payload, label }) => {
        if (active && payload && payload.length) {
            return (
                <div className="glass-card p-3 text-xs border border-surface-600/50 shadow-xl bg-surface-900/90 backdrop-blur-md">
                    <p className="font-semibold text-white mb-2">{label}</p>
                    <div className="space-y-1">
                        {payload.map((entry, idx) => (
                            <div key={idx} className="flex items-center justify-between gap-4">
                                <span className="flex items-center gap-1.5" style={{ color: entry.color }}>
                                    <span
                                        className="w-2 h-2 rounded-full"
                                        style={{ backgroundColor: entry.color }}
                                    />
                                    {entry.name}:
                                </span>
                                <span className="font-mono font-bold text-white">{entry.value}</span>
                            </div>
                        ))}
                    </div>
                </div>
            )
        }
        return null
    }

    return (
        <div className="glass-card p-6 animate-slide-up" style={{ animationDelay: '400ms' }}>
            {/* Header & Time Range Selector */}
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
                <div className="flex items-center gap-3">
                    <div className="p-2 rounded-lg bg-hardsend-500/20">
                        <BarChart3 className="w-5 h-5 text-hardsend-400" />
                    </div>
                    <div>
                        <h3 className="text-white font-semibold text-sm">Rendimiento y Evolución Histórica</h3>
                        <p className="text-surface-500 text-xs">Métricas acumuladas en BD por período</p>
                    </div>
                </div>

                <div className="flex items-center gap-1 bg-surface-800/80 p-1 rounded-lg border border-surface-700/50 self-start sm:self-auto">
                    {ranges.map((r) => (
                        <button
                            key={r.id}
                            onClick={() => setRange(r.id)}
                            className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all duration-200 ${
                                range === r.id
                                    ? 'bg-hardsend-600 text-white shadow-lg shadow-hardsend-600/30 font-semibold'
                                    : 'text-surface-400 hover:text-white hover:bg-surface-700/50'
                            }`}
                        >
                            {r.label}
                        </button>
                    ))}
                    <button
                        onClick={fetchAnalytics}
                        className="p-1.5 ml-1 text-surface-400 hover:text-white rounded-md hover:bg-surface-700/50 transition-colors"
                        title="Actualizar datos"
                    >
                        <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-hardsend-400' : ''}`} />
                    </button>
                </div>
            </div>

            {/* Summary Mini Cards */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-6 mb-8">
                <div className="stat-card stat-card-green">
                    <div className="flex items-center justify-between text-surface-400 mb-3">
                        <span className="text-xs font-semibold tracking-wider uppercase">Enviados</span>
                        <TrendingUp className="w-5 h-5 text-emerald-400 drop-shadow-md" />
                    </div>
                    <p className="text-2xl font-bold text-white font-mono tracking-tight">
                        {summary ? summary.total_sent.toLocaleString('es-AR') : '0'}
                    </p>
                    <p className="text-[11px] text-surface-500 mt-2 font-medium">Período: {range}</p>
                </div>

                <div className="stat-card stat-card-blue">
                    <div className="flex items-center justify-between text-surface-400 mb-3">
                        <span className="text-xs font-semibold tracking-wider uppercase">Entregados</span>
                        <CheckCircle2 className="w-5 h-5 text-blue-400 drop-shadow-md" />
                    </div>
                    <p className="text-2xl font-bold text-white font-mono tracking-tight">
                        {summary ? summary.total_delivered.toLocaleString('es-AR') : '0'}
                    </p>
                    <p className="text-[11px] text-surface-500 mt-2 font-medium">Confirmados por servidor</p>
                </div>

                <div className="stat-card stat-card-cyan">
                    <div className="flex items-center justify-between text-surface-400 mb-3">
                        <span className="text-xs font-semibold tracking-wider uppercase">Aperturas</span>
                        <Mail className="w-5 h-5 text-cyan-400 drop-shadow-md" />
                    </div>
                    <p className="text-2xl font-bold text-white font-mono tracking-tight">
                        {summary ? summary.total_opened.toLocaleString('es-AR') : '0'}
                    </p>
                    <p className="text-[11px] text-cyan-400/90 font-semibold mt-2">
                        {summary && summary.total_sent > 0
                            ? `${summary.open_rate.toFixed(1)}% tasa de apertura`
                            : '0% tasa de apertura'}
                    </p>
                </div>

                <div className="stat-card stat-card-red">
                    <div className="flex items-center justify-between text-surface-400 mb-3">
                        <span className="text-xs font-semibold tracking-wider uppercase">Rebotes</span>
                        <AlertTriangle className="w-5 h-5 text-red-400 drop-shadow-md" />
                    </div>
                    <p className="text-2xl font-bold text-white font-mono tracking-tight">
                        {summary ? summary.total_bounced.toLocaleString('es-AR') : '0'}
                    </p>
                    <p className="text-[11px] text-red-400/90 font-semibold mt-2">
                        {summary && summary.total_sent > 0
                            ? `${summary.bounce_rate.toFixed(1)}% tasa de rebote`
                            : '0% tasa de rebote'}
                    </p>
                </div>
            </div>

            {/* Historical Area Chart */}
            <div className="w-full">
                {timeSeries && timeSeries.length > 0 ? (
                    <ResponsiveContainer width="100%" height={260}>
                        <AreaChart data={timeSeries} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                            <defs>
                                <linearGradient id="gradSent" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#10b981" stopOpacity={0.35} />
                                    <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                                </linearGradient>
                                <linearGradient id="gradDelivered" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.35} />
                                    <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                                </linearGradient>
                                <linearGradient id="gradOpened" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#22d3ee" stopOpacity={0.35} />
                                    <stop offset="95%" stopColor="#22d3ee" stopOpacity={0} />
                                </linearGradient>
                                <linearGradient id="gradBounced" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="5%" stopColor="#ef4444" stopOpacity={0.35} />
                                    <stop offset="95%" stopColor="#ef4444" stopOpacity={0} />
                                </linearGradient>
                            </defs>
                            <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" vertical={false} />
                            <XAxis
                                dataKey="label"
                                stroke="#475569"
                                tick={{ fill: '#94a3b8', fontSize: 11 }}
                                tickLine={false}
                                axisLine={{ stroke: '#334155' }}
                            />
                            <YAxis
                                stroke="#475569"
                                tick={{ fill: '#94a3b8', fontSize: 11 }}
                                tickLine={false}
                                axisLine={{ stroke: '#334155' }}
                            />
                            <Tooltip content={<CustomTooltip />} />
                            <Area
                                type="monotone"
                                dataKey="sent"
                                name="Enviados"
                                stroke="#10b981"
                                fillOpacity={1}
                                fill="url(#gradSent)"
                                strokeWidth={2}
                            />
                            <Area
                                type="monotone"
                                dataKey="delivered"
                                name="Entregados"
                                stroke="#3b82f6"
                                fillOpacity={1}
                                fill="url(#gradDelivered)"
                                strokeWidth={2}
                            />
                            <Area
                                type="monotone"
                                dataKey="opened"
                                name="Aperturas"
                                stroke="#22d3ee"
                                fillOpacity={1}
                                fill="url(#gradOpened)"
                                strokeWidth={2}
                            />
                            <Area
                                type="monotone"
                                dataKey="bounced"
                                name="Rebotes"
                                stroke="#ef4444"
                                fillOpacity={1}
                                fill="url(#gradBounced)"
                                strokeWidth={2}
                            />
                            <Legend verticalAlign="top" height={36} iconType="circle" />
                        </AreaChart>
                    </ResponsiveContainer>
                ) : (
                    <div className="h-[220px] flex items-center justify-center bg-surface-900/30 rounded-xl border border-dashed border-surface-700/60">
                        <div className="text-center p-6">
                            <Calendar className="w-10 h-10 mx-auto mb-3 text-surface-600 opacity-60" />
                            <p className="text-surface-400 font-medium text-sm">
                                No hay datos de envíos para el período "{range.toUpperCase()}"
                            </p>
                            <p className="text-surface-600 text-xs mt-1">
                                En cuanto comiences a despachar correos, la serie temporal y los porcentajes se graficarán automáticamente.
                            </p>
                        </div>
                    </div>
                )}
            </div>
        </div>
    )
}

export default AnalyticsSection
