import React, { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { useWebSocket } from '../hooks/useWebSocket'
import StatsRow from './StatsRow'
import ProgressBar from './ProgressBar'
import Dropzone from './Dropzone'
import ErrorDatagrid from './ErrorDatagrid'
import {
    AreaChart,
    Area,
    XAxis,
    YAxis,
    CartesianGrid,
    Tooltip,
    ResponsiveContainer,
} from 'recharts'
import {
    Zap,
    LogOut,
    LayoutDashboard,
    Clock,
    Calendar as CalendarIcon,
} from 'lucide-react'

function Dashboard({ onNavigateHistory }) {
    const { user, logout, token } = useAuth()
    const { metrics, connected } = useWebSocket(token)
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
                    procesados: metrics.processed_count,
                }
                const updated = [...prev, newEntry]
                // Keep last 60 data points (1 minute of data)
                return updated.slice(-60)
            })
        }
    }, [metrics])

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

            {/* Top Header */}
            <header className="sticky top-0 z-50 backdrop-blur-xl bg-surface-950/80 border-b border-surface-800/50">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div className="flex items-center justify-between h-16">
                        {/* Logo */}
                        <div className="flex items-center gap-3">
                            <div className="w-9 h-9 rounded-xl flex items-center justify-center"
                                style={{ background: 'var(--gradient-primary)' }}>
                                <Zap className="w-5 h-5 text-white" strokeWidth={2.5} />
                            </div>
                            <div>
                                <h1 className="text-lg font-bold text-white leading-tight">
                                    Hard<span className="text-hardsend-400">send</span>
                                </h1>
                                <p className="text-[10px] text-surface-500 font-medium -mt-0.5">Server Edition</p>
                            </div>
                        </div>

                        {/* Center - Dashboard Title */}
                        <div className="hidden md:flex items-center gap-2 text-surface-400">
                            <LayoutDashboard className="w-4 h-4" />
                            <span className="text-sm font-medium">Panel de Control</span>
                        </div>

                        {/* Right Side */}
                        <div className="flex items-center gap-4">
                            <div className="hidden sm:flex items-center gap-2 text-surface-500 text-xs">
                                <Clock className="w-3.5 h-3.5" />
                                <span className="font-mono tabular-nums">
                                    {currentTime.toLocaleTimeString('es-AR', {
                                        hour: '2-digit',
                                        minute: '2-digit',
                                        second: '2-digit',
                                    })}
                                </span>
                            </div>

                            <div className="h-6 w-px bg-surface-700" />

                            {/* Registro Button */}
                            <button
                                onClick={onNavigateHistory}
                                className="flex items-center gap-2 px-3 py-2 rounded-lg
                                    bg-hardsend-500/10 text-hardsend-400 hover:bg-hardsend-500/20 hover:text-hardsend-300
                                    border border-hardsend-500/20 hover:border-hardsend-500/40
                                    transition-all duration-200 text-sm font-medium"
                                id="registro-button"
                            >
                                <CalendarIcon className="w-4 h-4" />
                                <span className="hidden sm:inline">Registro</span>
                            </button>

                            <div className="flex items-center gap-3">
                                <div className="hidden sm:block text-right">
                                    <p className="text-surface-300 text-xs font-medium">{user}</p>
                                    <p className="text-surface-600 text-[10px]">Superadmin</p>
                                </div>
                                <button
                                    onClick={logout}
                                    className="flex items-center gap-2 px-3 py-2 rounded-lg
                           text-surface-400 hover:text-white hover:bg-surface-800/50
                           transition-all duration-200 text-sm"
                                    id="logout-button"
                                >
                                    <LogOut className="w-4 h-4" />
                                    <span className="hidden sm:inline">Salir</span>
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </header>

            {/* Main Content */}
            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6">
                {/* Stats Row */}
                <StatsRow metrics={metrics} connected={connected} />

                {/* Progress Bar */}
                <ProgressBar metrics={metrics} />

                {/* Two Column Layout */}
                <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                    {/* Dropzone */}
                    <Dropzone onUploadComplete={handleUploadComplete} />

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

                {/* Error Datagrid */}
                <ErrorDatagrid />

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

export default Dashboard
