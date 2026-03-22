import React, { useState, useEffect, useCallback, useMemo } from 'react'
import { useAuth } from '../context/AuthContext'
import FolderPicker from './FolderPicker'
import {
    FolderOpen,
    FileText,
    Search,
    Play,
    RefreshCw,
    XCircle,
    CheckCircle2,
    AlertTriangle,
    Ban,
    SkipForward,
    Loader2,
    ChevronLeft,
    Calendar,
    Settings,
    Gauge,
    Eye,
    EyeOff,
    HardDrive,
} from 'lucide-react'

function CampaignPlan({ onBack, onStarted }) {
    const { token } = useAuth()

    // Form state
    const [folderPath, setFolderPath] = useState('')
    const [txtPath, setTxtPath] = useState('')
    const [dueDate, setDueDate] = useState('')
    const [dailyLimit, setDailyLimit] = useState('')
    const [emailSubject, setEmailSubject] = useState('FACTURA MENSUAL VIDEO DIGITAL S.R.L')
    const [emailBody, setEmailBody] = useState('Se adjunta la factura mensual correspondiente al servicio de Cable e Internet de VIDEO DIGITAL S.R.L.')
    const [apologyText, setApologyText] = useState('')
    const [forceResend, setForceResend] = useState(false)

    // Campaign state
    const [analyzing, setAnalyzing] = useState(false)
    const [starting, setStarting] = useState(false)
    const [rescanning, setRescanning] = useState(false)
    const [campaign, setCampaign] = useState(null)
    const [invoices, setInvoices] = useState([])
    const [error, setError] = useState(null)
    const [statusFilter, setStatusFilter] = useState('')
    const [showInvoices, setShowInvoices] = useState(false)
    const [searchQuery, setSearchQuery] = useState('')

    // Folder picker state
    const [showFolderPicker, setShowFolderPicker] = useState(false)
    const [showTxtPicker, setShowTxtPicker] = useState(false)

    // Check for active campaign on mount
    useEffect(() => {
        checkActiveCampaign()
    }, [])

    const checkActiveCampaign = async () => {
        try {
            const res = await fetch('/api/campaigns/active', {
                headers: { Authorization: `Bearer ${token}` },
            })
            if (res.status === 204) return
            if (res.ok) {
                const data = await res.json()
                setCampaign(data.campaign)
                setInvoices(data.invoices || [])
                if (data.campaign.folder_path) setFolderPath(data.campaign.folder_path)
                if (data.campaign.txt_path) setTxtPath(data.campaign.txt_path)
                if (data.campaign.due_date) setDueDate(data.campaign.due_date)
            }
        } catch (err) {
            console.error('[CampaignPlan] Error checking active campaign:', err)
        }
    }

    const handleAnalyze = async () => {
        if (!folderPath || !txtPath) return
        setAnalyzing(true)
        setError(null)

        try {
            const res = await fetch('/api/campaigns/analyze', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${token}`,
                },
                body: JSON.stringify({
                    folder_path: folderPath,
                    txt_path: txtPath,
                    due_date: dueDate,
                    force_resend: forceResend,
                    template: {
                        subject: emailSubject,
                        body_text: emailBody,
                        apology_text: apologyText,
                    },
                }),
            })

            const data = await res.json()
            if (!res.ok) {
                setError(data.error || 'Error al analizar')
                return
            }

            // Fetch full campaign details
            const detailRes = await fetch(`/api/campaigns/${data.campaign_id}`, {
                headers: { Authorization: `Bearer ${token}` },
            })
            if (detailRes.ok) {
                const detail = await detailRes.json()
                setCampaign(detail.campaign)
                setInvoices(detail.invoices || [])
            }
        } catch (err) {
            setError('Error de conexión al servidor')
        } finally {
            setAnalyzing(false)
        }
    }

    const handleRescan = async () => {
        if (!campaign) return
        setRescanning(true)
        setError(null)

        try {
            const res = await fetch(`/api/campaigns/${campaign.id}/rescan`, {
                method: 'POST',
                headers: { 
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${token}` 
                },
                body: JSON.stringify({ force_resend: forceResend })
            })
            const data = await res.json()
            if (!res.ok) {
                setError(data.error || 'Error al re-escanear')
                return
            }

            // Refresh campaign details
            const detailRes = await fetch(`/api/campaigns/${campaign.id}`, {
                headers: { Authorization: `Bearer ${token}` },
            })
            if (detailRes.ok) {
                const detail = await detailRes.json()
                setCampaign(detail.campaign)
                setInvoices(detail.invoices || [])
            }
        } catch (err) {
            setError('Error de conexión al servidor')
        } finally {
            setRescanning(false)
        }
    }

    const handleStart = async () => {
        if (!campaign || !dailyLimit) return
        setStarting(true)
        setError(null)

        try {
            const res = await fetch(`/api/campaigns/${campaign.id}/start`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    Authorization: `Bearer ${token}`,
                },
            })
            const data = await res.json()
            if (!res.ok) {
                setError(data.error || 'Error al iniciar envío')
                setStarting(false)
                return
            }

            if (onStarted) onStarted(data)
        } catch (err) {
            setError('Error de conexión al servidor')
            setStarting(false)
        }
    }

    const handleCancel = async () => {
        if (!campaign) return
        if (!window.confirm('¿Estás seguro que querés cancelar esta campaña?')) return

        try {
            const res = await fetch(`/api/campaigns/${campaign.id}/cancel`, {
                method: 'POST',
                headers: { Authorization: `Bearer ${token}` },
            })
            if (res.ok) {
                setCampaign(null)
                setInvoices([])
            }
        } catch (err) {
            setError('Error al cancelar')
        }
    }

    const filteredInvoices = useMemo(() => {
        return invoices.filter(inv => {
            const matchesStatus = !statusFilter || inv.status === statusFilter
            const matchesSearch = !searchQuery ||
                inv.invoice_number.toLowerCase().includes(searchQuery.toLowerCase()) ||
                inv.client_name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                inv.email.toLowerCase().includes(searchQuery.toLowerCase())
            return matchesStatus && matchesSearch
        })
    }, [invoices, statusFilter, searchQuery])

    const getStatusBadge = (status) => {
        switch (status) {
            case 'QUEUED':
                return <span className="badge badge-processing">Pendiente</span>
            case 'NO_EMAIL':
                return <span className="badge badge-warning">Sin Email</span>

            case 'SKIPPED':
                return <span className="badge badge-success">Omitida</span>
            case 'SENT':
                return <span className="badge badge-processing">Enviada</span>
            case 'CANCELLED':
                return <span className="badge badge-error">Cancelada</span>
            default:
                return <span className="badge">{status}</span>
        }
    }

    const isReady = campaign && campaign.status === 'READY'
    const canStart = isReady && dailyLimit

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
                                className="p-2 rounded-lg hover:bg-surface-800/50 transition-colors"
                                id="campaign-back-button"
                            >
                                <ChevronLeft className="w-5 h-5 text-surface-400" />
                            </button>
                            <div className="p-2 rounded-lg bg-emerald-500/20">
                                <HardDrive className="w-5 h-5 text-emerald-400" />
                            </div>
                            <div>
                                <h1 className="text-lg font-bold text-white leading-tight">
                                    Carpeta <span className="text-emerald-400">Local</span>
                                </h1>
                                <p className="text-[10px] text-surface-500 font-medium -mt-0.5">
                                    Escaneo directo de PDFs
                                </p>
                            </div>
                        </div>
                        {campaign && (
                            <span className={`badge ${campaign.status === 'READY' ? 'badge-success' : campaign.status === 'SENDING' ? 'badge-processing' : 'badge-warning'}`}>
                                {campaign.status}
                            </span>
                        )}
                    </div>
                </div>
            </header>

            <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6 space-y-6">
                {/* Error Alert */}
                {error && (
                    <div className="glass-card p-4 border-danger/30 bg-danger/10 animate-slide-down">
                        <div className="flex items-center gap-3">
                            <XCircle className="w-5 h-5 text-danger-light flex-shrink-0" />
                            <p className="text-danger-light text-sm">{error}</p>
                            <button onClick={() => setError(null)} className="ml-auto p-1 hover:bg-danger/20 rounded-lg">
                                <XCircle className="w-4 h-4 text-surface-500" />
                            </button>
                        </div>
                    </div>
                )}

                {/* Configuration Form */}
                {!campaign && (
                    <div className="glass-card p-6 animate-slide-up">
                        <div className="flex items-center gap-3 mb-6">
                            <div className="p-2 rounded-lg bg-emerald-500/20">
                                <FolderOpen className="w-5 h-5 text-emerald-400" />
                            </div>
                            <div>
                                <h3 className="text-white font-semibold text-sm">Configurar Análisis</h3>
                                <p className="text-surface-500 text-xs">Ingresá las rutas locales del facturador</p>
                            </div>
                        </div>

                        <div className="space-y-4">
                            {/* Folder Path */}
                            <div>
                                <label className="text-surface-300 text-xs font-medium block mb-1.5">
                                    📂 Carpeta de Facturas (PDFs)
                                </label>
                                <div className="flex gap-2">
                                    <input
                                        type="text"
                                        value={folderPath}
                                        onChange={(e) => setFolderPath(e.target.value)}
                                        placeholder="Ej: C:\Facturas\Marzo2026"
                                        className="flex-1 px-3 py-2 rounded-lg bg-surface-900/50 border border-surface-600/50
                                            text-surface-200 text-sm placeholder:text-surface-600 font-mono
                                            focus:outline-none focus:border-emerald-500/50 focus:ring-1 focus:ring-emerald-500/20
                                            transition-all duration-200"
                                        id="folder-path-input"
                                    />
                                    <button
                                        onClick={() => setShowFolderPicker(true)}
                                        className="flex items-center gap-2 px-4 py-2 rounded-lg
                                            bg-emerald-500/15 text-emerald-400 hover:bg-emerald-500/25 hover:text-emerald-300
                                            border border-emerald-500/20 hover:border-emerald-500/40
                                            transition-all duration-200 text-sm font-medium whitespace-nowrap"
                                        id="browse-folder-button"
                                    >
                                        <FolderOpen className="w-4 h-4" />
                                        Explorar
                                    </button>
                                </div>
                            </div>

                            {/* TXT Path */}
                            <div>
                                <label className="text-surface-300 text-xs font-medium block mb-1.5">
                                    📄 Archivo TXT (Base de datos de clientes)
                                </label>
                                <div className="flex gap-2">
                                    <input
                                        type="text"
                                        value={txtPath}
                                        onChange={(e) => setTxtPath(e.target.value)}
                                        placeholder="Ej: C:\Datos\clientes.txt"
                                        className="flex-1 px-3 py-2 rounded-lg bg-surface-900/50 border border-surface-600/50
                                            text-surface-200 text-sm placeholder:text-surface-600 font-mono
                                            focus:outline-none focus:border-emerald-500/50 focus:ring-1 focus:ring-emerald-500/20
                                            transition-all duration-200"
                                        id="txt-path-input"
                                    />
                                    <button
                                        onClick={() => setShowTxtPicker(true)}
                                        className="flex items-center gap-2 px-4 py-2 rounded-lg
                                            bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 hover:text-blue-300
                                            border border-blue-500/20 hover:border-blue-500/40
                                            transition-all duration-200 text-sm font-medium whitespace-nowrap"
                                        id="browse-txt-button"
                                    >
                                        <FileText className="w-4 h-4" />
                                        Explorar
                                    </button>
                                </div>
                            </div>

                            {/* Due Date */}
                            <div className="p-3 rounded-lg bg-surface-800/40 border border-surface-700/30">
                                <div className="flex items-center gap-3">
                                    <div className="p-2 rounded-lg bg-amber-500/10">
                                        <Calendar className="w-4 h-4 text-amber-400" />
                                    </div>
                                    <div className="flex-1">
                                        <label className="text-surface-300 text-xs font-medium block mb-1">
                                            Fecha de Vencimiento <span className="text-surface-600">(opcional)</span>
                                        </label>
                                        <input
                                            type="date"
                                            value={dueDate}
                                            onChange={(e) => setDueDate(e.target.value)}
                                            className="w-full px-3 py-1.5 rounded-lg bg-surface-900/50 border border-surface-600/50
                                                text-surface-200 text-sm
                                                focus:outline-none focus:border-hardsend-500/50 focus:ring-1 focus:ring-hardsend-500/20
                                                transition-all duration-200"
                                            id="campaign-due-date"
                                        />
                                    </div>
                                </div>
                            </div>

                            {/* Email Template */}
                            <div className="p-3 rounded-lg bg-surface-800/40 border border-surface-700/30">
                                <div className="flex items-center gap-3 mb-3">
                                    <div className="p-2 rounded-lg bg-blue-500/10">
                                        <Settings className="w-4 h-4 text-blue-400" />
                                    </div>
                                    <span className="text-surface-300 text-xs font-medium">Configuración del Email</span>
                                </div>
                                <div className="space-y-3 ml-11">
                                    <div>
                                        <label className="text-surface-400 text-xs block mb-1">Asunto</label>
                                        <input
                                            type="text"
                                            value={emailSubject}
                                            onChange={(e) => setEmailSubject(e.target.value)}
                                            className="w-full px-3 py-1.5 rounded-lg bg-surface-900/50 border border-surface-600/50
                                                text-surface-200 text-sm
                                                focus:outline-none focus:border-hardsend-500/50 focus:ring-1 focus:ring-hardsend-500/20
                                                transition-all duration-200"
                                        />
                                    </div>
                                    <div>
                                        <label className="text-surface-400 text-xs block mb-1">Texto del cuerpo</label>
                                        <textarea
                                            value={emailBody}
                                            onChange={(e) => setEmailBody(e.target.value)}
                                            rows={3}
                                            className="w-full px-3 py-1.5 rounded-lg bg-surface-900/50 border border-surface-600/50
                                                text-surface-200 text-sm resize-none
                                                focus:outline-none focus:border-hardsend-500/50 focus:ring-1 focus:ring-hardsend-500/20
                                                transition-all duration-200"
                                        />
                                    </div>
                                    <div>
                                        <label className="text-surface-400 text-xs block mb-1">Texto de Disculpas (Opcional)</label>
                                        <textarea
                                            value={apologyText}
                                            onChange={(e) => setApologyText(e.target.value)}
                                            rows={2}
                                            placeholder="Aviso importante (se mostrará en amarillo resaltado)..."
                                            className="w-full px-3 py-1.5 rounded-lg bg-surface-900/50 border border-surface-600/50
                                                text-surface-200 text-sm resize-none
                                                focus:outline-none focus:border-amber-500/50 focus:ring-1 focus:ring-amber-500/20
                                                transition-all duration-200"
                                        />
                                    </div>
                                </div>
                            </div>

                            {/* Force Resend Toggle */}
                            <div className="flex items-center gap-3 p-3 rounded-lg bg-surface-800/40 border border-surface-700/30">
                                <label className="relative inline-flex items-center cursor-pointer">
                                    <input
                                        type="checkbox"
                                        checked={forceResend}
                                        onChange={(e) => setForceResend(e.target.checked)}
                                        className="sr-only peer"
                                    />
                                    <div className="w-9 h-5 bg-surface-700 peer-focus:outline-none rounded-full peer 
                                        peer-checked:after:translate-x-full peer-checked:after:border-white 
                                        after:content-[''] after:absolute after:top-[2px] after:left-[2px] 
                                        after:bg-white after:border-surface-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all
                                        peer-checked:bg-danger-light"></div>
                                </label>
                                <div>
                                    <span className="text-surface-300 text-xs font-medium block">
                                        Forzar reenvío (Ignorar control de envíos previos)
                                    </span>
                                    <span className="text-surface-500 text-[10px]">
                                        Útil si hubo un error masivo y necesitás reenviar facturas este mismo mes.
                                    </span>
                                </div>
                            </div>

                            {/* Analyze Button */}
                            <button
                                onClick={handleAnalyze}
                                disabled={analyzing || !folderPath || !txtPath}
                                className={`w-full flex items-center justify-center gap-2 mt-2 py-3 rounded-xl font-semibold text-sm
                                    transition-all duration-200 ${(!folderPath || !txtPath)
                                        ? 'bg-surface-800 text-surface-500 cursor-not-allowed'
                                        : 'bg-gradient-to-r from-emerald-600 to-emerald-500 text-white hover:from-emerald-500 hover:to-emerald-400 shadow-lg shadow-emerald-500/20'
                                    }`}
                                id="analyze-button"
                            >
                                {analyzing ? (
                                    <>
                                        <Loader2 className="w-5 h-5 animate-spin" />
                                        Analizando...
                                    </>
                                ) : (
                                    <>
                                        <Search className="w-5 h-5" />
                                        Analizar Carpeta
                                    </>
                                )}
                            </button>
                        </div>
                    </div>
                )}

                {/* Campaign Plan Summary */}
                {campaign && (
                    <>
                        {/* Summary Cards */}
                        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 animate-slide-up">
                            <SummaryCard
                                icon={FileText}
                                label="Total PDFs"
                                value={campaign.total_invoices}
                                color="text-white"
                                bg="bg-surface-800/50"
                            />
                            <SummaryCard
                                icon={CheckCircle2}
                                label="Válidas"
                                value={campaign.valid_count}
                                color="text-emerald-400"
                                bg="bg-emerald-500/10"
                            />
                            <SummaryCard
                                icon={AlertTriangle}
                                label="Sin Email"
                                value={campaign.no_email_count}
                                color="text-amber-400"
                                bg="bg-amber-500/10"
                            />
                            <SummaryCard
                                icon={SkipForward}
                                label="Omitidas"
                                value={(campaign.skipped_count || 0)}
                                color="text-surface-400"
                                bg="bg-surface-800/50"
                            />
                        </div>

                        {/* Daily Limit (required before starting) */}
                        {isReady && (
                            <div className="glass-card p-4 animate-slide-up" style={{ animationDelay: '100ms' }}>
                                <div className="flex items-center gap-3">
                                    <div className="p-2 rounded-lg bg-hardsend-500/10">
                                        <Gauge className="w-4 h-4 text-hardsend-400" />
                                    </div>
                                    <div className="flex-1">
                                        <label className="text-surface-300 text-xs font-medium block mb-1">
                                            Límite diario de envío *
                                        </label>
                                        <input
                                            type="number"
                                            min={1}
                                            max={50000}
                                            value={dailyLimit}
                                            onChange={(e) => setDailyLimit(e.target.value === '' ? '' : parseInt(e.target.value) || '')}
                                            placeholder="Ej: 500, 1000, 1500..."
                                            className="w-full px-3 py-1.5 rounded-lg bg-surface-900/50 border border-surface-600/50
                                                text-surface-200 text-sm placeholder:text-surface-600
                                                focus:outline-none focus:border-hardsend-500/50 focus:ring-1 focus:ring-hardsend-500/20
                                                transition-all duration-200"
                                            id="campaign-daily-limit"
                                        />
                                    </div>
                                </div>
                            </div>
                        )}

                        {/* Action Buttons */}
                        <div className="flex flex-wrap gap-3 animate-slide-up" style={{ animationDelay: '150ms' }}>
                            {isReady && (
                                <>
                                    <button
                                        onClick={handleStart}
                                        disabled={starting || !canStart}
                                        className={`flex-1 flex items-center justify-center gap-2 py-3 rounded-xl font-semibold text-sm
                                            transition-all duration-200 ${!canStart
                                                ? 'bg-surface-800 text-surface-500 cursor-not-allowed'
                                                : 'bg-gradient-to-r from-hardsend-600 to-hardsend-500 text-white hover:from-hardsend-500 hover:to-hardsend-400 shadow-lg shadow-hardsend-500/20'
                                            }`}
                                        id="start-campaign-button"
                                    >
                                        {starting ? (
                                            <><Loader2 className="w-5 h-5 animate-spin" /> Iniciando...</>
                                        ) : (
                                            <><Play className="w-5 h-5" /> Iniciar Envío ({campaign.valid_count})</>
                                        )}
                                    </button>
                                    {campaign.folder_path && (
                                        <button
                                            onClick={handleRescan}
                                            disabled={rescanning}
                                            className="flex items-center gap-2 px-4 py-3 rounded-xl font-semibold text-sm
                                                bg-surface-800/50 text-surface-300 hover:bg-surface-700/50 hover:text-white
                                                border border-surface-700/30 transition-all duration-200"
                                            id="rescan-button"
                                        >
                                            <RefreshCw className={`w-4 h-4 ${rescanning ? 'animate-spin' : ''}`} />
                                            Re-escanear
                                        </button>
                                    )}
                                    <button
                                        onClick={handleCancel}
                                        className="flex items-center gap-2 px-4 py-3 rounded-xl font-semibold text-sm
                                            bg-danger/10 text-danger-light hover:bg-danger/20
                                            border border-danger/20 transition-all duration-200"
                                        id="cancel-campaign-button"
                                    >
                                        <XCircle className="w-4 h-4" />
                                        Cancelar
                                    </button>
                                </>
                            )}
                        </div>

                        {/* Invoice Preview Table */}
                        <div className="glass-card p-6 animate-slide-up" style={{ animationDelay: '200ms' }}>
                            <div className="flex items-center justify-between mb-4">
                                <div className="flex items-center gap-3">
                                    <div className="p-2 rounded-lg bg-blue-500/20">
                                        <FileText className="w-5 h-5 text-blue-400" />
                                    </div>
                                    <div>
                                        <h3 className="text-white font-semibold text-sm">Detalle de Facturas</h3>
                                        <p className="text-surface-500 text-xs">{invoices.length} facturas analizadas</p>
                                    </div>
                                </div>
                                <button
                                    onClick={() => setShowInvoices(!showInvoices)}
                                    className="flex items-center gap-2 px-3 py-1.5 rounded-lg text-sm
                                        bg-surface-800/50 text-surface-400 hover:text-white hover:bg-surface-700/50
                                        transition-all duration-200"
                                >
                                    {showInvoices ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                                    {showInvoices ? 'Ocultar' : 'Ver'}
                                </button>
                            </div>

                            {showInvoices && (
                                <>
                                    {/* Filters */}
                                    <div className="flex flex-wrap gap-3 mb-4">
                                        <div className="flex-1 min-w-[200px]">
                                            <input
                                                type="text"
                                                value={searchQuery}
                                                onChange={(e) => setSearchQuery(e.target.value)}
                                                placeholder="Buscar por factura, cliente o email..."
                                                className="w-full px-3 py-2 rounded-lg bg-surface-900/50 border border-surface-600/50
                                                    text-surface-200 text-sm placeholder:text-surface-600
                                                    focus:outline-none focus:border-hardsend-500/50 focus:ring-1 focus:ring-hardsend-500/20
                                                    transition-all duration-200"
                                            />
                                        </div>
                                        <select
                                            value={statusFilter}
                                            onChange={(e) => setStatusFilter(e.target.value)}
                                            className="px-3 py-2 rounded-lg bg-surface-900/50 border border-surface-600/50
                                                text-surface-200 text-sm
                                                focus:outline-none focus:border-hardsend-500/50
                                                transition-all duration-200"
                                        >
                                            <option value="">Todos</option>
                                            <option value="QUEUED">Pendientes</option>
                                            <option value="NO_EMAIL">Sin Email</option>
                                            <option value="SKIPPED">Omitidas</option>

                                        </select>
                                    </div>

                                    {/* Table */}
                                    <div className="overflow-x-auto max-h-[400px] overflow-y-auto custom-scrollbar">
                                        <table className="w-full text-sm">
                                            <thead className="sticky top-0 bg-surface-900/95 backdrop-blur-sm">
                                                <tr className="text-surface-400 text-xs uppercase tracking-wider">
                                                    <th className="text-left p-3">Factura</th>
                                                    <th className="text-left p-3">Cliente</th>
                                                    <th className="text-left p-3">Email</th>
                                                    <th className="text-left p-3">Estado</th>
                                                </tr>
                                            </thead>
                                            <tbody className="divide-y divide-surface-800/50">
                                                {filteredInvoices.slice(0, 200).map((inv) => (
                                                    <tr key={inv.id} className="hover:bg-surface-800/30 transition-colors">
                                                        <td className="p-3 text-white font-mono text-xs">{inv.invoice_number}</td>
                                                        <td className="p-3 text-surface-300">{inv.client_name || '—'}</td>
                                                        <td className="p-3 text-surface-400 font-mono text-xs">{inv.email || '—'}</td>
                                                        <td className="p-3">{getStatusBadge(inv.status)}</td>
                                                    </tr>
                                                ))}
                                            </tbody>
                                        </table>
                                        {filteredInvoices.length > 200 && (
                                            <p className="text-center text-surface-500 text-xs py-3">
                                                Mostrando 200 de {filteredInvoices.length} facturas
                                            </p>
                                        )}
                                        {filteredInvoices.length === 0 && (
                                            <div className="py-8 text-center">
                                                <p className="text-surface-500 text-sm">No se encontraron facturas con estos filtros</p>
                                            </div>
                                        )}
                                    </div>
                                </>
                            )}
                        </div>
                    </>
                )}
            </main>

            {/* Folder Picker Modals */}
            <FolderPicker
                isOpen={showFolderPicker}
                onClose={() => setShowFolderPicker(false)}
                onSelect={(path) => setFolderPath(path)}
                mode="folder"
                title="Seleccionar Carpeta de Facturas"
            />
            <FolderPicker
                isOpen={showTxtPicker}
                onClose={() => setShowTxtPicker(false)}
                onSelect={(path) => setTxtPath(path)}
                mode="file"
                fileExtension=".txt"
                title="Seleccionar Archivo TXT"
            />
        </div>
    )
}

function SummaryCard({ icon: Icon, label, value, color, bg }) {
    return (
        <div className={`glass-card p-4 ${bg} border-surface-700/30`}>
            <div className="flex items-center gap-3 mb-2">
                <Icon className={`w-5 h-5 ${color}`} />
                <span className="text-surface-400 text-xs font-medium">{label}</span>
            </div>
            <p className={`text-2xl font-bold ${color} tabular-nums`}>
                {(value || 0).toLocaleString()}
            </p>
        </div>
    )
}

export default CampaignPlan
