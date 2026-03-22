import React, { useState, useEffect, useCallback } from 'react'
import { useAuth } from '../context/AuthContext'
import {
    Folder,
    FolderOpen,
    FileText,
    ChevronRight,
    ChevronLeft,
    HardDrive,
    X,
    Check,
    ArrowUp,
    Loader2,
} from 'lucide-react'

/**
 * FolderPicker — Modal component to browse and select a local folder or file.
 *
 * Props:
 *   - isOpen: boolean
 *   - onClose: () => void
 *   - onSelect: (path: string) => void
 *   - mode: 'folder' | 'file'
 *   - fileExtension: optional, e.g. '.txt' (only for mode='file')
 *   - title: string
 */
function FolderPicker({ isOpen, onClose, onSelect, mode = 'folder', fileExtension, title }) {
    const { token } = useAuth()
    const [currentPath, setCurrentPath] = useState('')
    const [parentPath, setParentPath] = useState('')
    const [entries, setEntries] = useState([])
    const [drives, setDrives] = useState([])
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState(null)
    const [selectedEntry, setSelectedEntry] = useState(null)
    const [showDrives, setShowDrives] = useState(false)

    // Load drives on first open
    useEffect(() => {
        if (isOpen) {
            loadDrives()
            browse('')
        }
    }, [isOpen])

    const loadDrives = async () => {
        try {
            const res = await fetch('/api/filesystem/drives', {
                headers: { Authorization: `Bearer ${token}` },
            })
            if (res.ok) {
                const data = await res.json()
                setDrives(data || [])
            }
        } catch (err) {
            console.error('[FolderPicker] Error loading drives:', err)
        }
    }

    const browse = async (path) => {
        setLoading(true)
        setError(null)
        setSelectedEntry(null)

        try {
            let url = `/api/filesystem/browse?path=${encodeURIComponent(path)}`
            if (mode === 'folder') {
                url += '&type=folder'
            } else if (fileExtension) {
                url += `&ext=${encodeURIComponent(fileExtension)}`
            }

            const res = await fetch(url, {
                headers: { Authorization: `Bearer ${token}` },
            })

            if (!res.ok) {
                const data = await res.json()
                setError(data.error || 'Error al acceder a la carpeta')
                setLoading(false)
                return
            }

            const data = await res.json()
            setCurrentPath(data.current_path)
            setParentPath(data.parent)
            setEntries(data.entries || [])
            setShowDrives(false)
        } catch (err) {
            setError('Error de conexión')
        } finally {
            setLoading(false)
        }
    }

    const handleEntryClick = (entry) => {
        if (entry.is_dir) {
            browse(entry.path)
        } else {
            // File: select it
            setSelectedEntry(entry)
        }
    }

    const handleEntryDoubleClick = (entry) => {
        if (entry.is_dir) {
            if (mode === 'folder') {
                // Double-click on folder in folder mode = select it
                onSelect(entry.path)
                onClose()
            } else {
                browse(entry.path)
            }
        } else {
            // Double-click on file = select it
            onSelect(entry.path)
            onClose()
        }
    }

    const handleConfirm = () => {
        if (mode === 'folder') {
            onSelect(currentPath)
            onClose()
        } else if (selectedEntry) {
            onSelect(selectedEntry.path)
            onClose()
        }
    }

    const handleGoUp = () => {
        if (parentPath) {
            browse(parentPath)
        }
    }

    const formatSize = (bytes) => {
        if (!bytes) return ''
        if (bytes < 1024) return `${bytes} B`
        if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
        return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
    }

    if (!isOpen) return null

    return (
        <div className="fixed inset-0 z-[100] flex items-center justify-center">
            {/* Backdrop */}
            <div
                className="absolute inset-0 bg-black/60 backdrop-blur-sm"
                onClick={onClose}
            />

            {/* Modal */}
            <div className="relative w-full max-w-xl mx-4 glass-card border border-surface-700/50 shadow-2xl animate-slide-up"
                style={{ maxHeight: '80vh' }}>

                {/* Header */}
                <div className="flex items-center justify-between p-4 border-b border-surface-800/50">
                    <div className="flex items-center gap-3">
                        <div className="p-2 rounded-lg bg-emerald-500/20">
                            {mode === 'folder'
                                ? <FolderOpen className="w-5 h-5 text-emerald-400" />
                                : <FileText className="w-5 h-5 text-emerald-400" />
                            }
                        </div>
                        <div>
                            <h3 className="text-white font-semibold text-sm">
                                {title || (mode === 'folder' ? 'Seleccionar Carpeta' : 'Seleccionar Archivo')}
                            </h3>
                            <p className="text-surface-500 text-xs font-mono truncate max-w-[300px]">
                                {currentPath || 'Cargando...'}
                            </p>
                        </div>
                    </div>
                    <button
                        onClick={onClose}
                        className="p-2 rounded-lg hover:bg-surface-800/50 transition-colors"
                    >
                        <X className="w-4 h-4 text-surface-400" />
                    </button>
                </div>

                {/* Navigation bar */}
                <div className="flex items-center gap-2 px-4 py-2 border-b border-surface-800/30">
                    <button
                        onClick={handleGoUp}
                        disabled={!parentPath}
                        className={`p-1.5 rounded-lg transition-colors ${parentPath
                            ? 'hover:bg-surface-800/50 text-surface-300'
                            : 'text-surface-700 cursor-not-allowed'
                            }`}
                        title="Ir arriba"
                    >
                        <ArrowUp className="w-4 h-4" />
                    </button>
                    <button
                        onClick={() => setShowDrives(!showDrives)}
                        className="p-1.5 rounded-lg hover:bg-surface-800/50 text-surface-300 transition-colors"
                        title="Unidades"
                    >
                        <HardDrive className="w-4 h-4" />
                    </button>

                    {/* Breadcrumb */}
                    <div className="flex-1 flex items-center gap-1 overflow-x-auto text-xs custom-scrollbar">
                        {currentPath && currentPath.split(/[/\\]/).filter(Boolean).map((part, idx, arr) => {
                            const pathUpTo = arr.slice(0, idx + 1).join('\\')
                            const fullPath = currentPath.startsWith('/') ? '/' + pathUpTo : pathUpTo + (idx === 0 ? '\\' : '')
                            return (
                                <React.Fragment key={idx}>
                                    {idx > 0 && <ChevronRight className="w-3 h-3 text-surface-600 flex-shrink-0" />}
                                    <button
                                        onClick={() => browse(fullPath)}
                                        className="text-surface-400 hover:text-white transition-colors truncate max-w-[100px] flex-shrink-0"
                                    >
                                        {part}
                                    </button>
                                </React.Fragment>
                            )
                        })}
                    </div>
                </div>

                {/* Drive selector */}
                {showDrives && drives.length > 0 && (
                    <div className="flex items-center gap-2 px-4 py-2 border-b border-surface-800/30 bg-surface-900/30">
                        {drives.map((drive) => (
                            <button
                                key={drive.path}
                                onClick={() => browse(drive.path)}
                                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg
                                    bg-surface-800/50 hover:bg-surface-700/50 text-surface-300 hover:text-white
                                    border border-surface-700/30 transition-all text-xs font-mono"
                            >
                                <HardDrive className="w-3 h-3" />
                                {drive.name}
                            </button>
                        ))}
                    </div>
                )}

                {/* Error */}
                {error && (
                    <div className="mx-4 mt-3 p-3 rounded-lg bg-danger/10 border border-danger/30">
                        <p className="text-danger-light text-xs">{error}</p>
                    </div>
                )}

                {/* File list */}
                <div className="overflow-y-auto custom-scrollbar" style={{ maxHeight: '45vh' }}>
                    {loading ? (
                        <div className="flex items-center justify-center py-12">
                            <Loader2 className="w-6 h-6 text-emerald-400 animate-spin" />
                        </div>
                    ) : entries.length === 0 ? (
                        <div className="py-12 text-center">
                            <Folder className="w-10 h-10 text-surface-700 mx-auto mb-2" />
                            <p className="text-surface-500 text-sm">
                                {mode === 'folder' ? 'No hay subcarpetas' : 'No hay archivos'}
                            </p>
                        </div>
                    ) : (
                        <div className="p-2 space-y-0.5">
                            {entries.map((entry) => {
                                const isSelected = selectedEntry?.path === entry.path
                                return (
                                    <button
                                        key={entry.path}
                                        onClick={() => handleEntryClick(entry)}
                                        onDoubleClick={() => handleEntryDoubleClick(entry)}
                                        className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-left
                                            transition-all duration-150
                                            ${isSelected
                                                ? 'bg-emerald-500/15 border border-emerald-500/30'
                                                : 'hover:bg-surface-800/50 border border-transparent'
                                            }`}
                                    >
                                        {entry.is_dir ? (
                                            <Folder className="w-4 h-4 text-amber-400 flex-shrink-0" />
                                        ) : (
                                            <FileText className="w-4 h-4 text-surface-400 flex-shrink-0" />
                                        )}
                                        <span className={`text-sm truncate flex-1 ${isSelected ? 'text-white font-medium' : 'text-surface-300'}`}>
                                            {entry.name}
                                        </span>
                                        {!entry.is_dir && entry.size > 0 && (
                                            <span className="text-xs text-surface-600 flex-shrink-0">
                                                {formatSize(entry.size)}
                                            </span>
                                        )}
                                        {entry.is_dir && (
                                            <ChevronRight className="w-3.5 h-3.5 text-surface-600 flex-shrink-0" />
                                        )}
                                    </button>
                                )
                            })}
                        </div>
                    )}
                </div>

                {/* Footer */}
                <div className="flex items-center justify-between p-4 border-t border-surface-800/50">
                    <div className="text-xs text-surface-500 truncate max-w-[250px]">
                        {mode === 'folder' ? (
                            <span>Carpeta actual: <span className="text-surface-300 font-mono">{currentPath}</span></span>
                        ) : selectedEntry ? (
                            <span>Archivo: <span className="text-surface-300 font-mono">{selectedEntry.name}</span></span>
                        ) : (
                            <span>Seleccioná un archivo</span>
                        )}
                    </div>
                    <div className="flex items-center gap-2">
                        <button
                            onClick={onClose}
                            className="px-4 py-2 rounded-lg text-sm text-surface-400
                                hover:bg-surface-800/50 hover:text-white transition-all"
                        >
                            Cancelar
                        </button>
                        <button
                            onClick={handleConfirm}
                            disabled={mode === 'file' && !selectedEntry}
                            className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold
                                transition-all duration-200
                                ${(mode === 'file' && !selectedEntry)
                                    ? 'bg-surface-800 text-surface-500 cursor-not-allowed'
                                    : 'bg-emerald-600 text-white hover:bg-emerald-500 shadow-lg shadow-emerald-500/20'
                                }`}
                        >
                            <Check className="w-4 h-4" />
                            Seleccionar
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}

export default FolderPicker
