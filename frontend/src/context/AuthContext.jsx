import React, { createContext, useContext, useState, useEffect } from 'react'

const AuthContext = createContext(null)

const API_BASE = ''

export function AuthProvider({ children }) {
    const [token, setToken] = useState(localStorage.getItem('hardsend_token'))
    const [user, setUser] = useState(localStorage.getItem('hardsend_user'))
    const [loading, setLoading] = useState(true)

    useEffect(() => {
        // Validate existing token on mount
        if (token) {
            validateToken(token).then(valid => {
                if (!valid) {
                    logout()
                }
                setLoading(false)
            })
        } else {
            setLoading(false)
        }
    }, [])

    const validateToken = async (t) => {
        try {
            const res = await fetch(`${API_BASE}/api/validate`, {
                headers: { Authorization: `Bearer ${t}` },
            })
            return res.ok
        } catch {
            return false
        }
    }

    const login = async (username, password) => {
        const res = await fetch(`${API_BASE}/api/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password }),
        })

        if (!res.ok) {
            const data = await res.json().catch(() => ({}))
            throw new Error(data.error || 'Credenciales inválidas')
        }

        const data = await res.json()
        setToken(data.token)
        setUser(data.user)
        localStorage.setItem('hardsend_token', data.token)
        localStorage.setItem('hardsend_user', data.user)
        return data
    }

    const logout = () => {
        setToken(null)
        setUser(null)
        localStorage.removeItem('hardsend_token')
        localStorage.removeItem('hardsend_user')
    }

    const value = {
        token,
        user,
        isAuthenticated: !!token,
        loading,
        login,
        logout,
    }

    return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
    const context = useContext(AuthContext)
    if (!context) {
        throw new Error('useAuth must be used within an AuthProvider')
    }
    return context
}
