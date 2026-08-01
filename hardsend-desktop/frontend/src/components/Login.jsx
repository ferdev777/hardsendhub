import React, { useState } from 'react'
import { useAuth } from '../context/AuthContext'
import { Mail, Lock, AlertCircle, Zap } from 'lucide-react'
import Footer from './Footer'

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
                    <div className="inline-flex items-center justify-center w-24 h-24 rounded-3xl mb-6 relative overflow-hidden bg-surface-950"
                        style={{ boxShadow: '0 8px 32px rgba(66, 99, 235, 0.4)' }}>
                        <img src="/favicon.png" alt="Hardsend" className="w-full h-full object-cover" />
                        <div className="absolute inset-0 rounded-3xl animate-glow shadow-[inset_0_0_20px_rgba(255,255,255,0.1)] border border-white/10" />
                    </div>
                    <h1 className="text-3xl font-black text-white mb-2 tracking-tight">
                        Hard<span className="text-transparent bg-clip-text bg-gradient-to-r from-indigo-400 to-cyan-400">send</span>
                    </h1>
                    <p className="text-surface-400 text-xs font-bold tracking-widest uppercase">
                        by Devrow
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
                <div className="mt-6 animate-fade-in">
                    <Footer />
                </div>
            </div>
        </div>
    )
}

export default Login
