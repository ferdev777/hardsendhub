import React, { useState } from 'react'
import { AuthProvider, useAuth } from './context/AuthContext'
import Landing from './components/Landing'
import Dashboard from './components/Dashboard'
import History from './components/History'
import MissingEmails from './components/MissingEmails'
import LogsViewer from './components/LogsViewer'
import SuppressionsView from './components/SuppressionsView'

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
        return <Landing />
    }

    if (currentPage === 'history') {
        return <History onBack={() => setCurrentPage('dashboard')} />
    }

    if (currentPage === 'missing-emails') {
        return <MissingEmails onBack={() => setCurrentPage('dashboard')} />
    }

    if (currentPage === 'logs') {
        return <LogsViewer onBack={() => setCurrentPage('dashboard')} />
    }

    if (currentPage === 'suppressions') {
        return <SuppressionsView onBack={() => setCurrentPage('dashboard')} />
    }

    return (
        <Dashboard
            onNavigateHistory={() => setCurrentPage('history')}
            onNavigateMissingEmails={() => setCurrentPage('missing-emails')}
            onNavigateLogs={() => setCurrentPage('logs')}
            onNavigateSuppressions={() => setCurrentPage('suppressions')}
        />
    )
}

import { GoogleOAuthProvider } from '@react-oauth/google'

function App() {
    return (
        <GoogleOAuthProvider clientId={import.meta.env.VITE_GOOGLE_CLIENT_ID || "1234567890-placeholder.apps.googleusercontent.com"}>
            <AuthProvider>
                <AppContent />
            </AuthProvider>
        </GoogleOAuthProvider>
    )
}

export default App

