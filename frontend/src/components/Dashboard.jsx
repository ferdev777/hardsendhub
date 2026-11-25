import React, { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import { useWebSocket } from '../hooks/useWebSocket'
import ProgressBar from './ProgressBar'
import Dropzone from './Dropzone'
import CampaignHub from './CampaignHub'
import Footer from './Footer'
import {
    Zap,
    LogOut,
    Clock,
    Calendar as CalendarIcon,
    MailX,
    HardDrive,
    Trash2,
    AlertTriangle,
    Activity,
    ShieldAlert,
} from 'lucide-react'

function Dashboard({ onNavigateHistory, onNavigateMissingEmails, onNavigateLogs, onNavigateSuppressions }) {
    const { user, logout, token } = useAuth()
    const { metrics, connected } = useWebSocket(token)
    const [currentTime, setCurrentTime] = useState(new Date())
    const [campaignData, setCampaignData] = useState(null)
    const [showDropzone, setShowDropzone] = useState(false)

    const fetchCampaign = async () => {
        try {
            const res = await fetch('/api/d1/campaigns/active', {
                headers: { Authorization: `Bearer ${token}` }
            })
            if (res.ok) {
                const data = await res.json()
                setCampaignData(data)
                if (!data || !data.active) {
                    setShowDropzone(true)
                }
            }
        } catch (e) {
            console.error('Failed to load active campaign', e)
        }
    }

    useEffect(() => {
        fetchCampaign()
        const interval = setInterval(fetchCampaign, 15000)
        return () => clearInterval(interval)
    }, [token])

    // Update clock every second
    useEffect(() => {
        const timer = setInterval(() => setCurrentTime(new Date()), 1000)
        return () => clearInterval(timer)
    }, [])

    const handleResetSystem = async () => {
        if (!window.confirm("¡ATENCIÓN! Esto borrará todo el historial de campañas, facturas, mails rebotados, faltantes y registros diarios. Todos los contadores volverán a 0. ¿Estás absolutamente seguro de que querés empezar de cero?")) {
            return
        }
        
        try {
            const res = await fetch('/api/system/reset', {
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
        fetchCampaign()
    }

    return (
        <div className="min-h-screen">
            <div className="animated-bg" />

            {/* Floating Premium Navbar */}
            <div className="fixed top-4 left-1/2 -translate-x-1/2 z-50 w-[96%] max-w-7xl">
                <header className="glass-card rounded-2xl px-4 sm:px-5 h-16 flex items-center justify-between shadow-[0_8px_32px_rgba(0,0,0,0.4)] border border-surface-700/50 bg-surface-900/60 backdrop-blur-xl">
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
                                <span className={`w-1.5 h-1.5 rounded-full ${connected ? 'bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.8)]' : 'bg-amber-500 shadow-[0_0_8px_rgba(245,158,11,0.8)] animate-pulse'}`}></span>
                                <span className="text-[9px] uppercase tracking-wider text-surface-400 font-bold">
                                    {connected ? 'Server Edition • En Línea' : 'Server Edition • Reconectando'}
                                </span>
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
                                onClick={onNavigateLogs}
                                className="flex items-center gap-2 px-3 py-1.5 rounded-lg hover:bg-surface-800 text-surface-400 hover:text-white transition-all duration-200 text-sm font-medium"
                                title="Logs de Auditoría"
                            >
                                <Activity className="w-4 h-4 text-sky-400" />
                                <span className="hidden md:inline">Logs</span>
                            </button>
                            <button
                                onClick={onNavigateSuppressions}
                                className="flex items-center gap-2 px-3 py-1.5 rounded-lg hover:bg-surface-800 text-surface-400 hover:text-white transition-all duration-200 text-sm font-medium"
                                title="Lista de Supresiones y Rebotes"
                            >
                                <ShieldAlert className="w-4 h-4 text-rose-400" />
                                <span className="hidden md:inline">Supresiones</span>
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

                        {/* User & Logout */}
                        <div className="flex items-center gap-3 ml-2 border-l border-surface-800/50 pl-4">
                            <div className="hidden sm:block text-right">
                                <p className="text-surface-300 text-xs font-bold">{user}</p>
                            </div>
                            <button
                                onClick={logout}
                                className="p-2 rounded-lg text-surface-400 hover:text-danger-400 hover:bg-danger-500/10 transition-colors"
                                title="Cerrar sesión"
                            >
                                <LogOut className="w-4 h-4" />
                            </button>
                        </div>
                    </div>
                </header>
            </div>

            {/* Main Content */}
            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-28 pb-6 space-y-6">
                {/* Active Campaign Hub (Persisted in R2 & D1) */}
                <CampaignHub 
                    campaignData={campaignData} 
                    onRefresh={fetchCampaign} 
                    token={token} 
                    showDropzone={showDropzone}
                    setShowDropzone={setShowDropzone}
                />

                {/* Progress Bar (Only during active batch) */}
                {metrics && metrics.total_files > 0 && metrics.processed_count < metrics.total_files && (
                    <ProgressBar metrics={metrics} />
                )}

                {/* Dropzone (shown when no active campaign or requested by user) */}
                {(!campaignData?.active || showDropzone) && (
                    <div className="animate-slide-up">
                        <Dropzone onUploadComplete={handleUploadComplete} />
                    </div>
                )}

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

            {/* Circuit Breaker Alert Modal */}
            {metrics?.bounce_alert_triggered && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-fade-in">
                    <div className="bg-surface-900 border border-danger-500/50 rounded-2xl max-w-lg w-full p-6 shadow-2xl shadow-danger-500/20">
                        <div className="flex items-center justify-center w-16 h-16 rounded-full bg-danger-500/20 mx-auto mb-4">
                            <AlertTriangle className="w-8 h-8 text-danger-500" />
                        </div>
                        <h2 className="text-2xl font-bold text-center text-white mb-2">¡Campaña Pausada Automáticamente!</h2>
                        <p className="text-surface-300 text-center mb-6">
                            El <strong>Circuit Breaker</strong> se ha activado porque la tasa de rebotes alcanzó un nivel crítico.
                        </p>
                        
                        <div className="bg-surface-800/50 rounded-xl p-4 mb-6">
                            <div className="grid grid-cols-2 gap-4">
                                <div className="text-center">
                                    <p className="text-surface-400 text-sm mb-1">Tasa de Rebote</p>
                                    <p className="text-2xl font-bold text-danger-400">{metrics.bounce_rate?.toFixed(2)}%</p>
                                </div>
                                <div className="text-center border-l border-surface-700">
                                    <p className="text-surface-400 text-sm mb-1">Rebotes / Éxitos</p>
                                    <p className="text-2xl font-bold text-white">{metrics.bounce_count} / {metrics.success_count}</p>
                                </div>
                            </div>
                        </div>

                        <div className="bg-danger-500/10 border border-danger-500/20 rounded-lg p-4 mb-6 text-sm text-danger-200">
                            <strong>Advertencia:</strong> Continuar con esta tasa de rebotes puede dañar severamente la reputación del dominio (lista negra en Gmail/Hotmail). Te recomendamos verificar los correos antes de reanudar.
                        </div>

                        <div className="flex gap-3">
                            <button
                                onClick={async () => {
                                    // Resume campaign logic
                                    await fetch(`/api/campaigns/${metrics.job_id}/resume`, {
                                        method: 'POST',
                                        headers: { Authorization: `Bearer ${token}` }
                                    });
                                }}
                                className="flex-1 px-4 py-3 bg-surface-700 hover:bg-surface-600 text-white rounded-xl font-medium transition-colors"
                            >
                                Reanudar (Asumir Riesgo)
                            </button>
                            <button
                                onClick={async () => {
                                    // Stop/Cancel campaign logic
                                    await fetch(`/api/campaigns/${metrics.job_id}/cancel`, {
                                        method: 'POST',
                                        headers: { Authorization: `Bearer ${token}` }
                                    });
                                }}
                                className="flex-1 px-4 py-3 bg-danger-600 hover:bg-danger-500 text-white rounded-xl font-medium transition-colors"
                            >
                                Detener Campaña
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}

export default Dashboard
