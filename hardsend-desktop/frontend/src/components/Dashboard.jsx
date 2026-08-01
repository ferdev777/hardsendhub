import React, { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { useWebSocket } from '../hooks/useWebSocket'
import { API_BASE } from '../config/api'
import StatsRow from './StatsRow'
import ProgressBar from './ProgressBar'
import Dropzone from './Dropzone'
import ErrorDatagrid from './ErrorDatagrid'
import AnalyticsSection from './AnalyticsSection'
import Footer from './Footer'
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
    Zap,
    LayoutDashboard,
    Clock,
    Calendar as CalendarIcon,
    Radio,
    Mail,
    User,
    MailX,
    HardDrive,
    Trash2,
} from 'lucide-react'

function Dashboard({ onNavigateHistory, onNavigateMissingEmails, onNavigateCampaign }) {
    const { user, logout, token } = useAuth()
    const { metrics, events, connected } = useWebSocket(token)
    const [metricsHistory, setMetricsHistory] = useState([])
    const [currentTime, setCurrentTime] = useState(new Date())

    // Update clock every second
    useEffect(() => {
        const timer = setInterval(() => setCurrentTime(new Date()), 1000)
        return () => clearInterval(timer)
    }, [])

    // Build metrics history for the chart
    useEffect(() => {
        if (metrics && metrics.processed_count > 0) {
            setMetricsHistory(prev => {
                const now = new Date()
                const timeLabel = now.toLocaleTimeString('es-AR', {
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                })
                const newEntry = {
                    time: timeLabel,
                    exitosos: metrics.success_count,
                    errores: metrics.error_validation_count + metrics.error_network_count,
                    aperturas: metrics.opened_count || 0,
                    procesados: metrics.processed_count,
                }
                const updated = [...prev, newEntry]
                // Keep last 60 data points (1 minute of data)
                return updated.slice(-60)
            })
        }
    }, [metrics])

    const handleResetSystem = async () => {
        if (!window.confirm("¡ATENCIÓN! Esto borrará todo el historial de campañas, facturas, mails rebotados, faltantes y registros diarios. Todos los contadores volverán a 0. ¿Estás absolutamente seguro de que querés empezar de cero?")) {
            return
        }
        
        try {
            const res = await fetch(`${API_BASE}/api/system/reset`, {
                method: 'DELETE',
                headers: { 
                    Authorization: `Bearer ${token}` 
                }
            })
            if (res.ok) {
                alert("El sistema se ha reiniciado por completo. Todos los datos fueron borrados extiosamente.")
                window.location.reload()
            } else {
                alert("Hubo un error al intentar limpiar la base de datos.")
            }
        } catch (error) {
            console.error(error)
            alert("Error de conexión al servidor.")
        }
    }


    const handleUploadComplete = (data) => {
        console.log('[Dashboard] Upload complete:', data)
    }

    const CustomTooltip = ({ active, payload, label }) => {
        if (active && payload && payload.length) {
            return (
                <div className="glass-card p-3 text-xs border-surface-600/50">
                    <p className="text-surface-300 font-medium mb-1">{label}</p>
                    {payload.map((entry, index) => (
                        <p key={index} style={{ color: entry.color }} className="font-mono">
                            {entry.name}: {entry.value.toLocaleString()}
                        </p>
                    ))}
                </div>
            )
        }
        return null
    }

    return (
        <div className="min-h-screen">
            <div className="animated-bg" />

            {/* Floating Premium Navbar */}
            <div className="fixed top-4 left-1/2 -translate-x-1/2 z-50 w-[96%] max-w-7xl">
                <header className="glass-card rounded-2xl px-4 sm:px-5 h-16 flex items-center justify-between shadow-[0_8px_32px_rgba(0,0,0,0.4)] border border-surface-700/50 bg-surface-900/90">
                    {/* Logo Area */}
                    <div className="flex items-center gap-3">
                        <div className="w-10 h-10 rounded-[12px] overflow-hidden shadow-[0_0_20px_rgba(99,102,241,0.2)] border border-white/10 flex-shrink-0 bg-surface-950">
                            <img src="/favicon.png" alt="Hardsend" className="w-full h-full object-cover" />
                        </div>
                        <div>
                            <h1 className="text-xl font-black text-white tracking-tight leading-none mb-0.5">
                                Hard<span className="text-transparent bg-clip-text bg-gradient-to-r from-indigo-400 to-cyan-400">send</span>
                            </h1>
                            <div className="flex items-center gap-1.5">
                                <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.8)]"></span>
                                <span className="text-[9px] uppercase tracking-wider text-surface-400 font-bold">Server Edition</span>
                            </div>
                        </div>
                    </div>

                    {/* Right Side Actions */}
                    <div className="flex items-center gap-2 sm:gap-4">
                        <div className="hidden lg:flex items-center gap-2 text-surface-500 text-xs px-3 py-1.5 rounded-lg bg-surface-950/30 border border-surface-800/50">
                            <Clock className="w-3.5 h-3.5" />
                            <span className="font-mono tabular-nums font-medium">
                                {currentTime.toLocaleTimeString('es-AR', {
                                    hour: '2-digit',
                                    minute: '2-digit',
                                    second: '2-digit',
                                })}
                            </span>
                        </div>

                        {/* Navigation Pills */}
                        <div className="flex items-center gap-1 sm:gap-2 bg-surface-950/30 p-1 rounded-xl border border-surface-800/50">
                            <button
                                onClick={onNavigateCampaign}
                                className="flex items-center gap-2 px-3 py-1.5 rounded-lg hover:bg-surface-800 text-surface-400 hover:text-white transition-all duration-200 text-sm font-medium"
                                title="Carpeta Local"
                            >
                                <HardDrive className="w-4 h-4 text-emerald-400" />
                                <span className="hidden md:inline">Carpeta</span>
                            </button>
                            <button
                                onClick={onNavigateMissingEmails}
                                className="flex items-center gap-2 px-3 py-1.5 rounded-lg hover:bg-surface-800 text-surface-400 hover:text-white transition-all duration-200 text-sm font-medium"
                                title="Emails Faltantes"
                            >
                                <MailX className="w-4 h-4 text-amber-400" />
                                <span className="hidden md:inline">Faltantes</span>
                            </button>
                            <button
                                onClick={onNavigateHistory}
                                className="flex items-center gap-2 px-3 py-1.5 rounded-lg hover:bg-surface-800 text-surface-400 hover:text-white transition-all duration-200 text-sm font-medium"
                                title="Registro de Envíos"
                            >
                                <CalendarIcon className="w-4 h-4 text-indigo-400" />
                                <span className="hidden md:inline">Registro</span>
                            </button>
                        </div>
                    </div>
                </header>
            </div>

            {/* Main Content */}
            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-28 pb-6 space-y-6">
                {/* Stats Row */}
                <StatsRow metrics={metrics} connected={connected} />

                {/* Progress Bar */}
                <ProgressBar metrics={metrics} />

                {/* Two Column Layout */}
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                    {/* Dropzone */}
                    <div className="space-y-6">
                        <Dropzone onUploadComplete={handleUploadComplete} />

                        {/* Live Activity Feed */}
                        <div className="glass-card p-6 animate-slide-up" style={{ animationDelay: '400ms' }}>
                            <div className="flex items-center justify-between mb-5">
                                <div className="flex items-center gap-3">
                                    <div className="p-2 rounded-lg bg-cyan-500/20">
                                        <Radio className="w-5 h-5 text-cyan-400" />
                                    </div>
                                    <div>
                                        <h3 className="text-white font-semibold text-sm">Actividad en Vivo</h3>
                                        <p className="text-surface-500 text-xs">Eventos de Resend detectados</p>
                                    </div>
                                </div>
                                <span className="badge badge-processing animate-pulse">Live</span>
                            </div>

                            <div className="space-y-3 max-h-[300px] overflow-y-auto pr-2 custom-scrollbar">
                                {events.length > 0 ? (
                                    events.map((event, idx) => (
                                        <div key={idx} className="flex items-start gap-3 p-3 rounded-xl bg-surface-800/40 border border-surface-700/30 animate-fade-in">
                                            <div className={`p-2 rounded-lg ${event.event_type === 'email.opened' ? 'bg-cyan-500/10 text-cyan-400' :
                                                event.event_type === 'email.bounced' ? 'bg-orange-500/10 text-orange-400' :
                                                    'bg-surface-700 text-surface-400'
                                                }`}>
                                                {event.event_type === 'email.opened' ? <Mail className="w-4 h-4" /> : <User className="w-4 h-4" />}
                                            </div>
                                            <div className="flex-1 min-w-0">
                                                <p className="text-surface-200 text-sm truncate">
                                                    <span className="font-semibold text-white">Factura #{event.invoice_number || '---'}</span>
                                                    {event.event_type === 'email.opened' ? ' abierta por ' : ' procesada para '}
                                                    <span className="text-hardsend-300">{event.recipient || event.email || 'usuario'}</span>
                                                </p>
                                                <p className="text-surface-500 text-[10px] mt-1">
                                                    {new Date(event.created_at || Date.now()).toLocaleTimeString()} • {event.event_type || 'evento'}
                                                </p>
                                            </div>
                                        </div>
                                    ))
                                ) : (
                                    <div className="py-10 text-center">
                                        <Radio className="w-8 h-8 text-surface-700 mx-auto mb-2 opacity-20" />
                                        <p className="text-surface-600 text-sm">Esperando actividad...</p>
                                    </div>
                                )}
                            </div>
                        </div>
                    </div>

                    {/* Live Chart */}
                    <div className="glass-card p-6 animate-slide-up" style={{ animationDelay: '350ms' }}>
                        <div className="flex items-center gap-3 mb-5">
                            <div className="p-2 rounded-lg bg-hardsend-500/20">
                                <LayoutDashboard className="w-5 h-5 text-hardsend-400" />
                            </div>
                            <div>
                                <h3 className="text-white font-semibold text-sm">Monitor en Tiempo Real</h3>
                                <p className="text-surface-500 text-xs">Actividad del último minuto</p>
                            </div>
                        </div>

                        {metricsHistory.length > 1 ? (
                            <ResponsiveContainer width="100%" height={200}>
                                <AreaChart data={metricsHistory}>
                                    <defs>
                                        <linearGradient id="colorExitosos" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                                            <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                                        </linearGradient>
                                        <linearGradient id="colorErrores" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="5%" stopColor="#ef4444" stopOpacity={0.3} />
                                            <stop offset="95%" stopColor="#ef4444" stopOpacity={0} />
                                        </linearGradient>
                                        <linearGradient id="colorAperturas" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="5%" stopColor="#22d3ee" stopOpacity={0.3} />
                                            <stop offset="95%" stopColor="#22d3ee" stopOpacity={0} />
                                        </linearGradient>
                                    </defs>
                                    <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
                                    <XAxis
                                        dataKey="time"
                                        stroke="#475569"
                                        tick={{ fill: '#64748b', fontSize: 10 }}
                                        interval="preserveStartEnd"
                                    />
                                    <YAxis
                                        stroke="#475569"
                                        tick={{ fill: '#64748b', fontSize: 10 }}
                                    />
                                    <Tooltip content={<CustomTooltip />} />
                                    <Area
                                        type="monotone"
                                        dataKey="exitosos"
                                        name="Exitosos"
                                        stroke="#10b981"
                                        fillOpacity={1}
                                        fill="url(#colorExitosos)"
                                        strokeWidth={2}
                                    />
                                    <Area
                                        type="monotone"
                                        dataKey="errores"
                                        name="Errores"
                                        stroke="#ef4444"
                                        fillOpacity={1}
                                        fill="url(#colorErrores)"
                                        strokeWidth={2}
                                    />
                                    <Area
                                        type="monotone"
                                        dataKey="aperturas"
                                        name="Aperturas"
                                        stroke="#22d3ee"
                                        fillOpacity={1}
                                        fill="url(#colorAperturas)"
                                        strokeWidth={2}
                                    />
                                    <Legend verticalAlign="top" height={36} iconType="circle" />
                                </AreaChart>
                            </ResponsiveContainer>
                        ) : (
                            <div className="h-[200px] flex items-center justify-center">
                                <div className="text-center">
                                    <LayoutDashboard className="w-10 h-10 mx-auto mb-3 text-surface-700" />
                                    <p className="text-surface-500 text-sm">
                                        El gráfico se mostrará cuando haya datos
                                    </p>
                                    <p className="text-surface-600 text-xs mt-1">
                                        Inicia un procesamiento para ver métricas en vivo
                                    </p>
                                </div>
                            </div>
                        )}
                    </div>
                </div>

                {/* Historical Analytics & TimeSeries Chart */}
                <AnalyticsSection token={token} />

                {/* Error Datagrid */}
                <ErrorDatagrid />

                {/* System Actions */}
                <div className="py-6 border-t border-surface-800/50 flex flex-col items-center justify-center">
                    <button
                        onClick={handleResetSystem}
                        className="flex items-center gap-2 px-4 py-2 rounded-lg
                            bg-danger-500/10 text-danger-400 hover:bg-danger-500/20 hover:text-danger-300
                            border border-danger-500/20 hover:border-danger-500/40
                            transition-all duration-200 text-sm font-medium"
                        title="Borrar toda la base de datos (campañas, registro, historial, etc.)"
                    >
                        <Trash2 className="w-4 h-4" />
                        Resetear Sistema (Borrar Todo)
                    </button>
                </div>

                <Footer />
            </main>
        </div>
    )
}

export default Dashboard
