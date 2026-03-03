import React, { useState } from 'react'
import { AuthProvider, useAuth } from './context/AuthContext'
import Login from './components/Login'
import Dashboard from './components/Dashboard'
import History from './components/History'
import MissingEmails from './components/MissingEmails'

function AppContent() {
    const { isAuthenticated, loading } = useAuth()
    const [currentPage, setCurrentPage] = useState('dashboard')

    if (loading) {
        return (
            <div className="min-h-screen flex items-center justify-center">
                <div className="animated-bg" />
                <div className="flex flex-col items-center gap-4">
                    <div className="w-12 h-12 border-4 border-hardsend-500/30 border-t-hardsend-500 rounded-full animate-spin" />
                    <p className="text-surface-400 font-medium">Cargando...</p>
                </div>
            </div>
        )
    }

    if (!isAuthenticated) {
        return <Login />
    }

    if (currentPage === 'history') {
        return <History onBack={() => setCurrentPage('dashboard')} />
    }

    if (currentPage === 'missing-emails') {
        return <MissingEmails onBack={() => setCurrentPage('dashboard')} />
    }

    return (
        <Dashboard
            onNavigateHistory={() => setCurrentPage('history')}
            onNavigateMissingEmails={() => setCurrentPage('missing-emails')}
        />
    )
}

function App() {
    return (
        <AuthProvider>
            <AppContent />
        </AuthProvider>
    )
}

export default App
