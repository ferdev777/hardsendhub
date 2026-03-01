import React from 'react'
import {
    BarChart3,
    CheckCircle2,
    XCircle,
    AlertTriangle,
    Activity,
    TrendingUp,
    Wifi,
    WifiOff,
    Shield,
} from 'lucide-react'

function StatsRow({ metrics, connected }) {
    const totalProcessed = metrics?.processed_count || 0
    const successCount = metrics?.success_count || 0
    const errorValidation = metrics?.error_validation_count || 0
    const errorNetwork = metrics?.error_network_count || 0
    const totalErrors = errorValidation + errorNetwork
    const successRate = metrics?.success_rate || 0
    const activeWorkers = metrics?.active_workers || 0
    const circuitState = metrics?.circuit_breaker_state || 'CLOSED'

    const stats = [
        {
            label: 'Total Procesados',
            value: totalProcessed.toLocaleString(),
            icon: BarChart3,
            colorClass: 'stat-card-blue',
            iconColor: 'text-hardsend-400',
            subtext: `de ${(metrics?.total_files || 0).toLocaleString()} archivos`,
        },
        {
            label: 'Tasa de Éxito',
            value: `${successRate.toFixed(1)}%`,
            icon: TrendingUp,
            colorClass: 'stat-card-green',
            iconColor: 'text-success',
            subtext: `${successCount.toLocaleString()} exitosos`,
        },
        {
            label: 'Fallidos',
            value: totalErrors.toLocaleString(),
            icon: XCircle,
            colorClass: 'stat-card-red',
            iconColor: 'text-danger-light',
            subtext: `${errorValidation} validación / ${errorNetwork} red`,
        },
        {
            label: 'Workers Activos',
            value: activeWorkers.toString(),
            icon: Activity,
            colorClass: 'stat-card-yellow',
            iconColor: 'text-warning-light',
            subtext: circuitState === 'OPEN' ? '⚠️ Circuit Breaker ABIERTO' : 'Circuit Breaker: OK',
        },
    ]

    return (
        <div className="space-y-4">
            {/* Connection Status */}
            <div className="flex items-center gap-2 text-sm">
                {connected ? (
                    <>
                        <div className="pulse-dot pulse-dot-green" />
                        <Wifi className="w-4 h-4 text-success" />
                        <span className="text-success font-medium">Conectado en tiempo real</span>
                    </>
                ) : (
                    <>
                        <div className="pulse-dot pulse-dot-red" />
                        <WifiOff className="w-4 h-4 text-danger-light" />
                        <span className="text-danger-light font-medium">Desconectado - Reconectando...</span>
                    </>
                )}

                {circuitState === 'OPEN' && (
                    <div className="ml-auto flex items-center gap-2 px-3 py-1 rounded-lg bg-danger/10 border border-danger/30">
                        <Shield className="w-4 h-4 text-danger-light" />
                        <span className="text-danger-light text-xs font-medium">Circuit Breaker ABIERTO - Pausado 5 min</span>
                    </div>
                )}
            </div>

            {/* Stats Grid */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                {stats.map((stat, index) => {
                    const Icon = stat.icon
                    return (
                        <div
                            key={stat.label}
                            className={`stat-card ${stat.colorClass} animate-slide-up`}
                            style={{ animationDelay: `${index * 100}ms` }}
                        >
                            <div className="flex items-start justify-between mb-3">
                                <div>
                                    <p className="text-surface-400 text-xs font-medium uppercase tracking-wider mb-1">
                                        {stat.label}
                                    </p>
                                    <p className="text-3xl font-bold text-white tabular-nums">
                                        {stat.value}
                                    </p>
                                </div>
                                <div className={`p-2.5 rounded-xl bg-surface-800/50 ${stat.iconColor}`}>
                                    <Icon className="w-5 h-5" />
                                </div>
                            </div>
                            <p className="text-surface-500 text-xs">{stat.subtext}</p>
                        </div>
                    )
                })}
            </div>
        </div>
    )
}

export default StatsRow
