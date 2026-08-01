import React from 'react'
import { Timer, Zap } from 'lucide-react'

function ProgressBar({ metrics }) {
    const total = metrics?.total_files || 0
    const processed = metrics?.processed_count || 0
    const percentage = total > 0 ? (processed / total) * 100 : 0
    const isComplete = total > 0 && processed >= total
    const jobId = metrics?.job_id || ''

    return (
        <div className="glass-card p-6 animate-slide-up" style={{ animationDelay: '200ms' }}>
            <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-3">
                    <div className={`p-2 rounded-lg ${isComplete ? 'bg-success/20' : 'bg-hardsend-500/20'}`}>
                        {isComplete ? (
                            <Zap className="w-5 h-5 text-success" />
                        ) : (
                            <Timer className="w-5 h-5 text-hardsend-400" />
                        )}
                    </div>
                    <div>
                        <h3 className="text-white font-semibold text-sm">
                            {isComplete ? 'Procesamiento Completado' : 'Progreso del Lote Actual'}
                        </h3>
                        {jobId && (
                            <p className="text-surface-500 text-xs font-mono mt-0.5">
                                Job: {jobId.substring(0, 8)}...
                            </p>
                        )}
                    </div>
                </div>
                <div className="text-right">
                    <p className="text-white font-bold text-lg tabular-nums">
                        {processed.toLocaleString()} / {total.toLocaleString()}
                    </p>
                    <p className="text-surface-400 text-xs">archivos procesados</p>
                </div>
            </div>

            {/* Progress Bar */}
            <div className="relative">
                <div className="w-full h-4 bg-surface-800 rounded-full overflow-hidden">
                    <div
                        className="progress-bar-fill h-full rounded-full"
                        style={{ width: `${Math.min(percentage, 100)}%` }}
                    />
                </div>
                <div className="flex items-center justify-between mt-2">
                    <span className="text-surface-500 text-xs">0%</span>
                    <span className={`text-sm font-bold tabular-nums ${isComplete ? 'text-success' : 'text-hardsend-300'}`}>
                        {percentage.toFixed(1)}%
                    </span>
                    <span className="text-surface-500 text-xs">100%</span>
                </div>
            </div>

            {/* Animated processing indicator */}
            {!isComplete && total > 0 && (
                <div className="flex items-center gap-2 mt-3 text-xs text-surface-400">
                    <div className="flex gap-1">
                        <div className="w-1.5 h-1.5 rounded-full bg-hardsend-400 animate-bounce" style={{ animationDelay: '0ms' }} />
                        <div className="w-1.5 h-1.5 rounded-full bg-hardsend-400 animate-bounce" style={{ animationDelay: '150ms' }} />
                        <div className="w-1.5 h-1.5 rounded-full bg-hardsend-400 animate-bounce" style={{ animationDelay: '300ms' }} />
                    </div>
                    <span>Procesando facturas...</span>
                </div>
            )}
        </div>
    )
}

export default ProgressBar
