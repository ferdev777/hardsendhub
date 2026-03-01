import React, { useState } from 'react'
import { useAuth } from '../context/AuthContext'
import { Mail, Lock, AlertCircle, Zap } from 'lucide-react'

function Login() {
    const { login } = useAuth()
    const [username, setUsername] = useState('')
    const [password, setPassword] = useState('')
    const [error, setError] = useState('')
    const [loading, setLoading] = useState(false)

    const handleSubmit = async (e) => {
        e.preventDefault()
        setError('')
        setLoading(true)

        try {
            await login(username, password)
        } catch (err) {
            setError(err.message || 'Error al iniciar sesión')
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="login-container">
            <div className="animated-bg" />

            <div className="login-card">
                {/* Logo & Branding */}
                <div className="text-center mb-8 animate-fade-in">
                    <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl mb-6 relative"
                        style={{ background: 'var(--gradient-primary)', boxShadow: '0 8px 32px rgba(66, 99, 235, 0.4)' }}>
                        <Zap className="w-10 h-10 text-white" strokeWidth={2.5} />
                        <div className="absolute inset-0 rounded-2xl animate-glow" />
                    </div>
                    <h1 className="text-3xl font-bold text-white mb-2">
                        Hard<span className="text-hardsend-400">send</span>
                    </h1>
                    <p className="text-surface-400 text-sm font-medium tracking-wide">
                        by Devrow 2026
                    </p>
                </div>

                {/* Login Card */}
                <div className="glass-card p-8 animate-slide-up">
                    <h2 className="text-xl font-semibold text-white mb-1">Iniciar Sesión</h2>
                    <p className="text-surface-400 text-sm mb-6">
                        Accede al panel de control de envío de facturas
                    </p>

                    {error && (
                        <div className="flex items-center gap-3 p-3 mb-6 rounded-lg bg-danger/10 border border-danger/30 animate-slide-down">
                            <AlertCircle className="w-5 h-5 text-danger-light flex-shrink-0" />
                            <span className="text-danger-light text-sm">{error}</span>
                        </div>
                    )}

                    <form onSubmit={handleSubmit} className="space-y-5">
                        <div>
                            <label htmlFor="username" className="block text-sm font-medium text-surface-300 mb-2">
                                Usuario
                            </label>
                            <div className="relative">
                                <Mail className="absolute left-3.5 top-1/2 -translate-y-1/2 w-5 h-5 text-surface-500" />
                                <input
                                    id="username"
                                    type="text"
                                    value={username}
                                    onChange={(e) => setUsername(e.target.value)}
                                    className="input-field pl-12"
                                    placeholder="Ingrese su usuario"
                                    required
                                    autoComplete="username"
                                />
                            </div>
                        </div>

                        <div>
                            <label htmlFor="password" className="block text-sm font-medium text-surface-300 mb-2">
                                Contraseña
                            </label>
                            <div className="relative">
                                <Lock className="absolute left-3.5 top-1/2 -translate-y-1/2 w-5 h-5 text-surface-500" />
                                <input
                                    id="password"
                                    type="password"
                                    value={password}
                                    onChange={(e) => setPassword(e.target.value)}
                                    className="input-field pl-12"
                                    placeholder="Ingrese su contraseña"
                                    required
                                    autoComplete="current-password"
                                />
                            </div>
                        </div>

                        <button
                            type="submit"
                            disabled={loading}
                            className="btn-primary w-full flex items-center justify-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            {loading ? (
                                <>
                                    <div className="w-5 h-5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                                    Verificando...
                                </>
                            ) : (
                                'Acceder al Panel'
                            )}
                        </button>
                    </form>
                </div>

                {/* Footer */}
                <p className="text-center text-surface-600 text-xs mt-6 animate-fade-in">
                    © 2026 Fernando Hirschfeld & Devrow. All rights reserved. Closed Source.
                </p>
            </div>
        </div>
    )
}

export default Login
