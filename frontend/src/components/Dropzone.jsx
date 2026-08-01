import React, { useCallback, useState } from 'react'
import { useDropzone } from 'react-dropzone'
import { useAuth } from '../context/AuthContext'
import {
    Upload,
    FileText,
    FileArchive,
    Database,
    CheckCircle2,
    XCircle,
    Loader2,
    Calendar,
    Settings,
    Gauge,
} from 'lucide-react'

function Dropzone({ onUploadComplete }) {
    const { token } = useAuth()
    const [uploading, setUploading] = useState(false)
    const [uploadResult, setUploadResult] = useState(null)
    const [selectedFiles, setSelectedFiles] = useState([])
    const [txtFile, setTxtFile] = useState(null)
    const [dueDate, setDueDate] = useState('')
    const [dailyLimit, setDailyLimit] = useState('')
    const [emailSubject, setEmailSubject] = useState('FACTURA MENSUAL VIDEO DIGITAL S.R.L')
    const [emailBody, setEmailBody] = useState('Se adjunta la factura mensual correspondiente al servicio de Cable e Internet de VIDEO DIGITAL S.R.L.')
    const [apologyText, setApologyText] = useState('')
    const [forceResend, setForceResend] = useState(false)

    const onDrop = useCallback((acceptedFiles) => {
        const txtFiles = acceptedFiles.filter(f =>
            f.name.toLowerCase().endsWith('.txt')
        )
        const otherFiles = acceptedFiles.filter(f =>
            !f.name.toLowerCase().endsWith('.txt')
        )

        if (txtFiles.length > 0) {
            setTxtFile(txtFiles[0])
        }
        if (otherFiles.length > 0) {
            setSelectedFiles(prev => [...prev, ...otherFiles])
        }
    }, [])

    const { getRootProps, getInputProps, isDragActive } = useDropzone({
        onDrop,
        accept: {
            'application/pdf': ['.pdf'],
            'application/zip': ['.zip'],
            'application/x-zip-compressed': ['.zip'],
            'application/x-rar-compressed': ['.rar'],
            'application/vnd.rar': ['.rar'],
            'text/plain': ['.txt'],
        },
        multiple: true,
        disabled: uploading,
    })

    const removeFile = (index) => {
        setSelectedFiles(prev => prev.filter((_, i) => i !== index))
    }

    const removeTxtFile = () => {
        setTxtFile(null)
    }

    const handleUpload = async () => {
        if (!txtFile && selectedFiles.length === 0) return

        setUploading(true)
        setUploadResult(null)

        try {
            const formData = new FormData()

            if (txtFile) {
                formData.append('txt_file', txtFile)
            }

            selectedFiles.forEach(file => {
                formData.append('files', file)
            })

            // Add due date in DD/MM/YYYY format if set
            if (dueDate) {
                // Parse directly from YYYY-MM-DD to avoid timezone issues
                const [year, month, day] = dueDate.split('-')
                const formatted = `${day}/${month}/${year}`
                formData.append('due_date', formatted)
            }

            // Add daily limit (required)
            formData.append('daily_limit', dailyLimit.toString())

            // Add email template as JSON
            const template = {
                subject: emailSubject,
                body_text: emailBody,
                apology_text: apologyText,
            }
            formData.append('email_template', JSON.stringify(template))
            formData.append('force_resend', forceResend.toString())

            const res = await fetch('/api/upload', {
                method: 'POST',
                headers: {
                    Authorization: `Bearer ${token}`,
                },
                body: formData,
            })

            const data = await res.json()

            if (res.ok) {
                setUploadResult({ success: true, message: data.message, jobId: data.job_id })
                setSelectedFiles([])
                setTxtFile(null)
                if (onUploadComplete) onUploadComplete(data)
            } else {
                setUploadResult({ success: false, message: data.error || 'Error al subir archivos' })
            }
        } catch (err) {
            setUploadResult({ success: false, message: 'Error de conexión al servidor' })
        } finally {
            setUploading(false)
        }
    }

    const getFileIcon = (filename) => {
        const lower = filename.toLowerCase()
        if (lower.endsWith('.zip') || lower.endsWith('.rar')) return FileArchive
        if (lower.endsWith('.txt')) return Database
        return FileText
    }

    const formatFileSize = (bytes) => {
        if (bytes < 1024) return `${bytes} B`
        if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
        return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
    }

    const totalFiles = selectedFiles.length + (txtFile ? 1 : 0)

    return (
        <div className="glass-card p-6 animate-slide-up" style={{ animationDelay: '300ms' }}>
            <div className="flex items-center gap-3 mb-5">
                <div className="p-2 rounded-lg bg-hardsend-500/20">
                    <Upload className="w-5 h-5 text-hardsend-400" />
                </div>
                <div>
                    <h3 className="text-white font-semibold text-sm">Zona de Carga</h3>
                    <p className="text-surface-500 text-xs">
                        Arrastra archivos TXT, ZIP, RAR o PDF
                    </p>
                </div>
            </div>

            {/* Dropzone Area */}
            <div
                {...getRootProps()}
                className={`dropzone ${isDragActive ? 'dropzone-active' : ''} ${uploading ? 'opacity-50 cursor-not-allowed' : ''}`}
            >
                <input {...getInputProps()} id="file-dropzone" />
                <div className="text-center">
                    <Upload className={`w-10 h-10 mx-auto mb-3 ${isDragActive ? 'text-hardsend-400' : 'text-surface-500'}`} />
                    {isDragActive ? (
                        <p className="text-hardsend-300 font-medium">Suelta los archivos aquí...</p>
                    ) : (
                        <>
                            <p className="text-surface-300 font-medium mb-1">
                                Arrastra y suelta archivos aquí
                            </p>
                            <p className="text-surface-500 text-sm">
                                o haz clic para seleccionar
                            </p>
                            <div className="flex items-center justify-center gap-4 mt-4 text-xs text-surface-500">
                                <span className="flex items-center gap-1">
                                    <Database className="w-3.5 h-3.5" /> TXT (Base de datos)
                                </span>
                                <span className="flex items-center gap-1">
                                    <FileArchive className="w-3.5 h-3.5" /> ZIP / RAR
                                </span>
                                <span className="flex items-center gap-1">
                                    <FileText className="w-3.5 h-3.5" /> PDF
                                </span>
                            </div>
                        </>
                    )}
                </div>
            </div>

            {/* Selected Files List */}
            {totalFiles > 0 && (
                <div className="mt-4 space-y-2">
                    <p className="text-surface-400 text-xs font-medium uppercase tracking-wider">
                        {totalFiles} archivo{totalFiles !== 1 ? 's' : ''} seleccionado{totalFiles !== 1 ? 's' : ''}
                    </p>

                    {/* TXT File */}
                    {txtFile && (
                        <div className="flex items-center justify-between p-3 rounded-lg bg-success/5 border border-success/20">
                            <div className="flex items-center gap-3">
                                <Database className="w-4 h-4 text-success" />
                                <div>
                                    <p className="text-white text-sm font-medium">{txtFile.name}</p>
                                    <p className="text-surface-500 text-xs">{formatFileSize(txtFile.size)} — Base de datos de clientes</p>
                                </div>
                            </div>
                            <button
                                onClick={(e) => { e.stopPropagation(); removeTxtFile(); }}
                                className="p-1 rounded-lg hover:bg-danger/20 transition-colors"
                            >
                                <XCircle className="w-4 h-4 text-surface-500 hover:text-danger-light" />
                            </button>
                        </div>
                    )}

                    {/* Other Files */}
                    <div className="max-h-40 overflow-y-auto space-y-1">
                        {selectedFiles.map((file, index) => {
                            const Icon = getFileIcon(file.name)
                            return (
                                <div
                                    key={`${file.name}-${index}`}
                                    className="flex items-center justify-between p-2.5 rounded-lg bg-surface-800/30 hover:bg-surface-800/50 transition-colors"
                                >
                                    <div className="flex items-center gap-3 min-w-0">
                                        <Icon className="w-4 h-4 text-hardsend-400 flex-shrink-0" />
                                        <div className="min-w-0">
                                            <p className="text-surface-200 text-sm truncate">{file.name}</p>
                                            <p className="text-surface-500 text-xs">{formatFileSize(file.size)}</p>
                                        </div>
                                    </div>
                                    <button
                                        onClick={(e) => { e.stopPropagation(); removeFile(index); }}
                                        className="p-1 rounded-lg hover:bg-danger/20 transition-colors flex-shrink-0"
                                    >
                                        <XCircle className="w-4 h-4 text-surface-500 hover:text-danger-light" />
                                    </button>
                                </div>
                            )
                        })}
                    </div>

                    {/* Due Date Picker */}
                    <div className="mt-4 p-3 rounded-lg bg-surface-800/40 border border-surface-700/30">
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
                                    className="input-field text-sm py-2"
                                    id="due-date-input"
                                />
                            </div>
                        </div>
                        {dueDate && (() => {
                            const [y, m, d] = dueDate.split('-')
                            return (
                                <p className="text-xs text-amber-400/80 mt-2 ml-11">
                                    Se informará: {d}/{m}/{y}
                                </p>
                            )
                        })()}
                    </div>

                    {/* Daily Limit & Email Config */}
                    <div className="mt-4 p-3 rounded-lg bg-surface-800/40 border border-surface-700/30">
                        <div className="flex items-center gap-3 mb-3">
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
                                    className="input-field text-sm py-2"
                                    id="daily-limit-input"
                                />
                            </div>
                        </div>
                        {dailyLimit ? (
                            <p className="text-xs text-surface-500 ml-11">
                                El sistema pausará automáticamente al alcanzar {Number(dailyLimit).toLocaleString()} envíos diarios y continuará al día siguiente.
                            </p>
                        ) : (
                            <p className="text-xs text-orange-400 ml-11">
                                ⚠️ Campo obligatorio — Defina cuántos emails enviar por día
                            </p>
                        )}
                    </div>

                    {/* Email Template Config */}
                    <div className="mt-4 p-3 rounded-lg bg-surface-800/40 border border-surface-700/30">
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
                                    className="input-field text-sm py-2"
                                    id="email-subject-input"
                                />
                            </div>

                            <div>
                                <label className="text-surface-400 text-xs block mb-1">Texto del cuerpo</label>
                                <textarea
                                    value={emailBody}
                                    onChange={(e) => setEmailBody(e.target.value)}
                                    rows={3}
                                    className="input-field text-sm py-2 resize-none"
                                    id="email-body-input"
                                />
                            </div>

                            <div>
                                <label className="text-surface-400 text-xs block mb-1">
                                    Texto de disculpa <span className="text-surface-600">(opcional)</span>
                                </label>
                                <textarea
                                    value={apologyText}
                                    onChange={(e) => setApologyText(e.target.value)}
                                    rows={2}
                                    placeholder="Dejar vacío si no aplica"
                                    className="input-field text-sm py-2 resize-none"
                                    id="email-apology-input"
                                />
                            </div>
                        </div>
                    </div>

                    {/* Force Resend Toggle */}
                    <div className="flex items-center gap-3 mt-4 p-3 rounded-lg bg-surface-800/40 border border-surface-700/30">
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
                                after:bg-white after:border-surface-300 after:border after:rounded-full 
                                after:h-4 after:w-4 after:transition-all peer-checked:bg-danger-500">
                            </div>
                        </label>
                        <div className="flex-1">
                            <span className="text-surface-200 text-sm font-medium">Forzar Reenvío</span>
                            <p className="text-surface-500 text-xs mt-0.5">
                                Ignorar el filtro "Factura ya enviada este mes" y enviar de todas formas
                            </p>
                        </div>
                    </div>

                    {/* Upload Button */}
                    <button
                        onClick={handleUpload}
                        disabled={uploading || !dailyLimit}
                        className={`w-full flex items-center justify-center gap-2 mt-3 ${(!dailyLimit) ? 'btn-primary opacity-50 cursor-not-allowed' : 'btn-primary'}`}
                    >
                        {uploading ? (
                            <>
                                <Loader2 className="w-5 h-5 animate-spin" />
                                Subiendo archivos...
                            </>
                        ) : (
                            <>
                                <Upload className="w-5 h-5" />
                                Iniciar Procesamiento
                            </>
                        )}
                    </button>
                </div>
            )}

            {/* Upload Result */}
            {uploadResult && (
                <div className={`mt-4 p-4 rounded-lg border animate-slide-down ${uploadResult.success
                    ? 'bg-success/10 border-success/30'
                    : 'bg-danger/10 border-danger/30'
                    }`}>
                    <div className="flex items-center gap-3">
                        {uploadResult.success ? (
                            <CheckCircle2 className="w-5 h-5 text-success flex-shrink-0" />
                        ) : (
                            <XCircle className="w-5 h-5 text-danger-light flex-shrink-0" />
                        )}
                        <div>
                            <p className={`text-sm font-medium ${uploadResult.success ? 'text-success' : 'text-danger-light'}`}>
                                {uploadResult.message}
                            </p>
                            {uploadResult.jobId && (
                                <p className="text-surface-500 text-xs font-mono mt-1">
                                    Job ID: {uploadResult.jobId}
                                </p>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>
    )
}

export default Dropzone
