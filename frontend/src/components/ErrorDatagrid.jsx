import React, { useState, useEffect } from 'react'
import { useAuth } from '../context/AuthContext'
import {
    AlertTriangle,
    XCircle,
    WifiOff,
    RefreshCw,
    ChevronDown,
    ChevronUp,
    Search,
} from 'lucide-react'

function ErrorDatagrid() {
    const { token } = useAuth()
    const [errors, setErrors] = useState([])
    const [loading, setLoading] = useState(true)
    const [searchTerm, setSearchTerm] = useState('')
    const [sortField, setSortField] = useState('invoice_number')
    const [sortDirection, setSortDirection] = useState('asc')
    const [filterType, setFilterType] = useState('all')

    const fetchErrors = async () => {
        try {
            const res = await fetch('/api/errors', {
                headers: { Authorization: `Bearer ${token}` },
            })
            if (res.ok) {
                const data = await res.json()
                setErrors(data || [])
            }
        } catch (err) {
            console.error('Failed to fetch errors:', err)
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchErrors()
        // Auto-refresh every 5 seconds
        const interval = setInterval(fetchErrors, 5000)
        return () => clearInterval(interval)
    }, [token])

    const handleSort = (field) => {
        if (sortField === field) {
            setSortDirection(d => d === 'asc' ? 'desc' : 'asc')
        } else {
            setSortField(field)
            setSortDirection('asc')
        }
    }

    const SortIcon = ({ field }) => {
        if (sortField !== field) return null
        return sortDirection === 'asc'
            ? <ChevronUp className="w-3.5 h-3.5 inline ml-1" />
            : <ChevronDown className="w-3.5 h-3.5 inline ml-1" />
    }

    // Filter and sort
    let filteredErrors = [...errors]

    if (filterType !== 'all') {
        filteredErrors = filteredErrors.filter(e => e.status === filterType)
    }

    if (searchTerm) {
        const lower = searchTerm.toLowerCase()
        filteredErrors = filteredErrors.filter(e =>
            (e.invoice_number || '').toLowerCase().includes(lower) ||
            (e.recipient_email || '').toLowerCase().includes(lower) ||
            (e.error_reason || '').toLowerCase().includes(lower)
        )
    }

    filteredErrors.sort((a, b) => {
        const aVal = (a[sortField] || '').toString().toLowerCase()
        const bVal = (b[sortField] || '').toString().toLowerCase()
        const result = aVal.localeCompare(bVal)
        return sortDirection === 'asc' ? result : -result
    })

    const validationCount = errors.filter(e => e.status === 'ERROR_VALIDATION').length
    const networkCount = errors.filter(e => e.status === 'ERROR_NETWORK').length

    return (
        <div className="glass-card animate-slide-up" style={{ animationDelay: '400ms' }}>
            {/* Header */}
            <div className="p-6 pb-4">
                <div className="flex items-center justify-between mb-4">
                    <div className="flex items-center gap-3">
                        <div className="p-2 rounded-lg bg-danger/20">
                            <AlertTriangle className="w-5 h-5 text-danger-light" />
                        </div>
                        <div>
                            <h3 className="text-white font-semibold text-sm">Registro de Errores</h3>
                            <p className="text-surface-500 text-xs">
                                {errors.length} error{errors.length !== 1 ? 'es' : ''} encontrado{errors.length !== 1 ? 's' : ''}
                            </p>
                        </div>
                    </div>
                    <button
                        onClick={fetchErrors}
                        className="p-2 rounded-lg hover:bg-surface-800/50 transition-colors text-surface-400 hover:text-white"
                        title="Actualizar"
                    >
                        <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
                    </button>
                </div>

                {/* Filters Row */}
                <div className="flex flex-col sm:flex-row gap-3">
                    {/* Search */}
                    <div className="relative flex-1">
                        <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-surface-500" />
                        <input
                            type="text"
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            placeholder="Buscar por factura, email o error..."
                            className="input-field pl-10 py-2.5 text-sm"
                            id="error-search"
                        />
                    </div>

                    {/* Filter Buttons */}
                    <div className="flex gap-2">
                        <button
                            onClick={() => setFilterType('all')}
                            className={`px-3 py-2 rounded-lg text-xs font-medium transition-colors border ${filterType === 'all'
                                    ? 'bg-surface-700 border-surface-600 text-white'
                                    : 'bg-transparent border-surface-700/50 text-surface-400 hover:text-white hover:border-surface-600'
                                }`}
                        >
                            Todos ({errors.length})
                        </button>
                        <button
                            onClick={() => setFilterType('ERROR_VALIDATION')}
                            className={`px-3 py-2 rounded-lg text-xs font-medium transition-colors border flex items-center gap-1.5 ${filterType === 'ERROR_VALIDATION'
                                    ? 'bg-danger/20 border-danger/40 text-danger-light'
                                    : 'bg-transparent border-surface-700/50 text-surface-400 hover:text-danger-light hover:border-danger/30'
                                }`}
                        >
                            <XCircle className="w-3.5 h-3.5" />
                            Validación ({validationCount})
                        </button>
                        <button
                            onClick={() => setFilterType('ERROR_NETWORK')}
                            className={`px-3 py-2 rounded-lg text-xs font-medium transition-colors border flex items-center gap-1.5 ${filterType === 'ERROR_NETWORK'
                                    ? 'bg-warning/20 border-warning/40 text-warning-light'
                                    : 'bg-transparent border-surface-700/50 text-surface-400 hover:text-warning-light hover:border-warning/30'
                                }`}
                        >
                            <WifiOff className="w-3.5 h-3.5" />
                            Red ({networkCount})
                        </button>
                    </div>
                </div>
            </div>

            {/* Table */}
            <div className="overflow-x-auto max-h-96 overflow-y-auto">
                {filteredErrors.length === 0 ? (
                    <div className="text-center py-12 px-6">
                        {errors.length === 0 ? (
                            <>
                                <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-success/10 flex items-center justify-center">
                                    <AlertTriangle className="w-8 h-8 text-success/40" />
                                </div>
                                <p className="text-surface-400 font-medium">Sin errores</p>
                                <p className="text-surface-600 text-sm mt-1">
                                    Todas las facturas se procesaron correctamente
                                </p>
                            </>
                        ) : (
                            <>
                                <Search className="w-8 h-8 mx-auto mb-3 text-surface-600" />
                                <p className="text-surface-400 text-sm">
                                    No se encontraron resultados para "{searchTerm}"
                                </p>
                            </>
                        )}
                    </div>
                ) : (
                    <table className="error-table">
                        <thead>
                            <tr>
                                <th
                                    className="cursor-pointer hover:text-surface-200 transition-colors"
                                    onClick={() => handleSort('invoice_number')}
                                >
                                    N° Factura <SortIcon field="invoice_number" />
                                </th>
                                <th
                                    className="cursor-pointer hover:text-surface-200 transition-colors"
                                    onClick={() => handleSort('recipient_email')}
                                >
                                    Email <SortIcon field="recipient_email" />
                                </th>
                                <th
                                    className="cursor-pointer hover:text-surface-200 transition-colors"
                                    onClick={() => handleSort('status')}
                                >
                                    Tipo <SortIcon field="status" />
                                </th>
                                <th>Razón del Error</th>
                                <th
                                    className="cursor-pointer hover:text-surface-200 transition-colors"
                                    onClick={() => handleSort('attempts')}
                                >
                                    Intentos <SortIcon field="attempts" />
                                </th>
                            </tr>
                        </thead>
                        <tbody>
                            {filteredErrors.map((err) => (
                                <tr key={err.id}>
                                    <td>
                                        <span className="font-mono text-hardsend-300 text-xs">
                                            {err.invoice_number || '—'}
                                        </span>
                                    </td>
                                    <td>
                                        <span className="text-surface-300 text-xs">
                                            {err.recipient_email || '—'}
                                        </span>
                                    </td>
                                    <td>
                                        {err.status === 'ERROR_VALIDATION' ? (
                                            <span className="badge-danger">
                                                <XCircle className="w-3 h-3 mr-1" />
                                                Validación
                                            </span>
                                        ) : (
                                            <span className="badge-warning">
                                                <WifiOff className="w-3 h-3 mr-1" />
                                                Red
                                            </span>
                                        )}
                                    </td>
                                    <td>
                                        <span className="text-surface-400 text-xs leading-relaxed">
                                            {err.error_reason || '—'}
                                        </span>
                                    </td>
                                    <td>
                                        <span className="text-surface-300 text-xs font-mono">
                                            {err.attempts}
                                        </span>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                )}
            </div>
        </div>
    )
}

export default ErrorDatagrid
