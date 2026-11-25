import React, { useState } from 'react'
import {
    Rocket,
    CheckCircle2,
    Clock,
    AlertCircle,
    Layers,
    Send,
    Database,
    HardDrive,
    ChevronDown,
    ChevronUp,
    RefreshCw,
    ShieldCheck,
} from 'lucide-react'

export default function CampaignHub({ campaignData, onRefresh, token, showDropzone, setShowDropzone }) {
    const [dispatching, setDispatching] = useState(false)
    const [dispatchMsg, setDispatchMsg] = useState(null)
    const [dispatchError, setDispatchError] = useState(null)

    if (!campaignData || !campaignData.active) {
        return null
    }

    const { campaign, today } = campaignData
    const total = campaign?.total_invoices || 0
    const queued = campaign?.queued_count || 0
    const accepted = campaign?.accepted_count || 0
    const delivered = campaign?.delivered_count || 0
    const bounced = campaign?.bounced_count || 0
    const sentToday = today?.sent_count || 0
    const dailyLimit = today?.daily_limit || 200
    const remainingToday = today?.remaining_quota || 0

    const progressPct = total > 0 ? ((accepted / total) * 100).toFixed(1) : 0
    const todayPct = dailyLimit > 0 ? Math.min(100, (sentToday / dailyLimit) * 100).toFixed(0) : 0

    const handleDispatch = async () => {
        if (remainingToday <= 0) return
        setDispatching(true)
        setDispatchMsg(null)
        setDispatchError(null)

        try {
            const res = await fetch('/api/campaigns/dispatch-daily', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({ limit: remainingToday }),
            })

            const data = await res.json()
            if (res.ok && data.success) {
                setDispatchMsg(data.message || `Lote de ${data.dispatched} facturas iniciado en segundo plano.`)
                if (onRefresh) onRefresh()
            } else {
                setDispatchError(data.error || 'Error al iniciar el lote diario.')
            }
        } catch (err) {
            setDispatchError('Error de red al contactar al servidor.')
        } finally {
            setDispatching(false)
        }
    }

    return (
        <div className="glass-card p-6 border border-indigo-500/30 bg-gradient-to-br from-surface-900/90 via-surface-900/60 to-indigo-950/20 shadow-[0_8px_32px_rgba(0,0,0,0.5)] animate-fade-in space-y-6">
            {/* Header */}
            <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 border-b border-surface-800/80 pb-4">
                <div className="flex items-center gap-3">
                    <div className="p-2.5 rounded-xl bg-indigo-500/20 border border-indigo-500/30 text-indigo-400 shadow-[0_0_15px_rgba(99,102,241,0.3)]">
                        <Layers className="w-6 h-6" />
                    </div>
                    <div>
                        <div className="flex items-center gap-2">
                            <h2 className="text-white font-bold text-lg tracking-tight">
                                {campaign.name || `Facturación ${campaign.period}`}
                            </h2>
                            <span className="badge badge-success text-[10px] tracking-wider uppercase">
                                Persistido en R2 & D1
                            </span>
                        </div>
                        <p className="text-surface-400 text-xs mt-0.5">
                            Período: <span className="text-indigo-300 font-mono font-medium">{campaign.period}</span> • Archivo origen: <span className="text-surface-300 font-mono">{campaign.archive_filename || 'PDF.rar'}</span>
                        </p>
                    </div>
                </div>

                <div className="flex items-center gap-2 w-full sm:w-auto justify-end">
                    <button
                        onClick={onRefresh}
                        className="p-2 rounded-lg bg-surface-800/60 hover:bg-surface-700 text-surface-400 hover:text-white transition-colors"
                        title="Actualizar estado de campaña"
                    >
                        <RefreshCw className="w-4 h-4" />
                    </button>
                    <button
                        onClick={() => setShowDropzone(!showDropzone)}
                        className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface-800/60 hover:bg-surface-700 text-surface-300 hover:text-white text-xs font-medium transition-colors"
                    >
                        <HardDrive className="w-3.5 h-3.5 text-cyan-400" />
                        <span>{showDropzone ? 'Ocultar Carga de Archivos' : 'Subir Nueva Campaña'}</span>
                        {showDropzone ? <ChevronUp className="w-3.5 h-3.5" /> : <ChevronDown className="w-3.5 h-3.5" />}
                    </button>
                </div>
            </div>

            {/* Ingestion & Monthly Progress Metrics */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                <div className="p-3.5 rounded-xl bg-surface-800/40 border border-surface-700/40">
                    <span className="text-surface-400 text-xs block mb-1">Total en Almacén</span>
                    <div className="flex items-baseline gap-1.5">
                        <span className="text-2xl font-black text-white font-mono">{total.toLocaleString()}</span>
                        <span className="text-[11px] text-surface-500">PDFs en R2</span>
                    </div>
                </div>

                <div className="p-3.5 rounded-xl bg-surface-800/40 border border-surface-700/40">
                    <span className="text-surface-400 text-xs block mb-1">Restantes en Cola</span>
                    <div className="flex items-baseline gap-1.5">
                        <span className="text-2xl font-black text-amber-400 font-mono">{queued.toLocaleString()}</span>
                        <span className="text-[11px] text-surface-500">pendientes</span>
                    </div>
                </div>

                <div className="p-3.5 rounded-xl bg-surface-800/40 border border-surface-700/40">
                    <span className="text-surface-400 text-xs block mb-1">Aceptados por CF</span>
                    <div className="flex items-baseline gap-1.5">
                        <span className="text-2xl font-black text-emerald-400 font-mono">{accepted.toLocaleString()}</span>
                        <span className="text-[11px] text-emerald-500/80 font-medium">({progressPct}%)</span>
                    </div>
                </div>

                <div className="p-3.5 rounded-xl bg-surface-800/40 border border-surface-700/40">
                    <span className="text-surface-400 text-xs block mb-1">Entregados / Rebotes</span>
                    <div className="flex items-baseline gap-1.5">
                        <span className="text-2xl font-black text-sky-400 font-mono">{delivered.toLocaleString()}</span>
                        <span className="text-[11px] text-rose-400 font-mono font-medium">/ {bounced} reb.</span>
                    </div>
                </div>
            </div>

            {/* Daily Quota & Dispatch Controls */}
            <div className="p-4 rounded-xl bg-surface-950/60 border border-surface-800/80 flex flex-col md:flex-row items-stretch md:items-center justify-between gap-5">
                <div className="space-y-2 flex-1">
                    <div className="flex items-center justify-between text-xs">
                        <span className="text-surface-300 font-semibold flex items-center gap-1.5">
                            <Clock className="w-3.5 h-3.5 text-indigo-400" />
                            Cuota Diaria de Envíos (Cloudflare Warmup)
                        </span>
                        <span className="font-mono text-surface-400">
                            <strong className="text-white">{sentToday}</strong> / {dailyLimit} diarios ({todayPct}%)
                        </span>
                    </div>

                    <div className="w-full bg-surface-800/80 rounded-full h-2.5 overflow-hidden p-0.5 border border-surface-700/30">
                        <div
                            className={`h-full rounded-full transition-all duration-500 ${
                                remainingToday > 0
                                    ? 'bg-gradient-to-r from-indigo-500 to-emerald-400 shadow-[0_0_10px_rgba(16,185,129,0.5)]'
                                    : 'bg-gradient-to-r from-amber-500 to-rose-500'
                            }`}
                            style={{ width: `${todayPct}%` }}
                        />
                    </div>

                    <p className="text-[11px] text-surface-400">
                        {remainingToday > 0 ? (
                            <span>
                                Quedan <strong className="text-emerald-400 font-mono">{remainingToday}</strong> envíos disponibles hoy antes del límite de seguridad.
                            </span>
                        ) : (
                            <span className="text-amber-400 flex items-center gap-1">
                                <AlertCircle className="w-3 h-3 flex-shrink-0" />
                                Cupo de hoy alcanzado. El próximo lote se habilitará automáticamente tras el restablecimiento a las 00:00 UTC.
                            </span>
                        )}
                    </p>
                </div>

                <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
                    {remainingToday > 0 && queued > 0 ? (
                        <button
                            onClick={handleDispatch}
                            disabled={dispatching}
                            className="px-5 py-3 rounded-xl bg-gradient-to-r from-indigo-500 to-emerald-500 hover:from-indigo-600 hover:to-emerald-600 text-white font-bold text-sm shadow-[0_0_20px_rgba(99,102,241,0.4)] flex items-center justify-center gap-2 transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            {dispatching ? (
                                <>
                                    <RefreshCw className="w-4 h-4 animate-spin" />
                                    <span>Iniciando lote...</span>
                                </>
                            ) : (
                                <>
                                    <Rocket className="w-4 h-4" />
                                    <span>Despachar Lote de Hoy ({remainingToday} Facturas)</span>
                                </>
                            )}
                        </button>
                    ) : queued === 0 ? (
                        <div className="px-4 py-2.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 text-xs font-semibold flex items-center gap-2">
                            <CheckCircle2 className="w-4 h-4" />
                            <span>Campaña Mensual Completada</span>
                        </div>
                    ) : (
                        <div className="px-4 py-2.5 rounded-xl bg-surface-800/80 border border-surface-700/50 text-surface-400 text-xs font-medium flex items-center gap-2">
                            <Clock className="w-4 h-4 text-amber-400" />
                            <span>Esperando Reset Diario</span>
                        </div>
                    )}
                </div>
            </div>

            {/* Feedback Notifications */}
            {dispatchMsg && (
                <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs flex items-center gap-2">
                    <CheckCircle2 className="w-4 h-4 flex-shrink-0 text-emerald-400" />
                    <span>{dispatchMsg}</span>
                </div>
            )}
            {dispatchError && (
                <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center gap-2">
                    <AlertCircle className="w-4 h-4 flex-shrink-0 text-rose-400" />
                    <span>{dispatchError}</span>
                </div>
            )}
        </div>
    )
}

