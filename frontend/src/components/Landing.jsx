import React, { useState, useEffect } from 'react';
import { useAuth } from '../context/AuthContext';
import {
    AlertCircle,
    ShieldCheck,
    Cpu,
    Activity,
    Terminal,
    Lock,
    Layers,
    Radio,
    Server,
    CheckCircle2,
} from 'lucide-react';
import { GoogleLogin } from '@react-oauth/google';
import Footer from './Footer';

const TELEMETRY_FRAMES = [
    {
        ts: '09:00:01.104',
        layer: 'CAPA_01',
        tag: 'D1_SUPPRESS',
        msg: 'Loaded 8,247 verified + 772 suppressed addresses from reputation store',
        color: 'text-emerald-400',
    },
    {
        ts: '09:00:01.340',
        layer: 'CAPA_02',
        tag: 'MX_RESOLVER',
        msg: 'DNS MX pre-warm complete (TTL 5s) // Auto-sanitized 14 domain typos',
        color: 'text-cyan-400',
    },
    {
        ts: '09:00:01.812',
        layer: 'CAPA_03',
        tag: 'WAVE_ROUTER',
        msg: 'Wave 01 [VERIFIED_REPUTATION] prioritized -> 15 req/s token bucket active',
        color: 'text-sky-300',
    },
    {
        ts: '09:00:02.190',
        layer: 'CAPA_04',
        tag: 'GUARD_MON',
        msg: 'Circuit Breaker ARMED // Rolling bounce rate: 0.12% (threshold < 1.00%)',
        color: 'text-amber-400',
    },
    {
        ts: '09:00:02.605',
        layer: 'CF_EDGE',
        tag: 'DISPATCH_OK',
        msg: 'POST /email/sending/send -> 200 Accepted [idempotency_key: B0002-00443428]',
        color: 'text-emerald-400',
    },
];

const PIPELINE_LAYERS = [
    {
        code: 'CAPA 01',
        name: 'D1 & SQLite Suppression Store',
        spec: 'BLOQUEO PRE-VUELO',
        desc: 'Filtra rebotes históricos (hard bounces) y clientes de cobro por ventanilla antes de encolar el PDF.',
        metric: '0% Re-envío a Rebotados',
    },
    {
        code: 'CAPA 02',
        name: 'Sanitizer + MX DNS Cache',
        spec: 'VALIDACIÓN DE RED',
        desc: 'Corrige errores tipográficos comunes en dominios, verifica registros MX activos y valida peso <= 3.4 MB.',
        metric: 'Lookup DNS Concurrente',
    },
    {
        code: 'CAPA 03',
        name: 'Wave Priority & Warm-up Scheduler',
        spec: 'ORDENAMIENTO POR OLEADAS',
        desc: 'Despacha primero la Oleada 1 (cuentas verificadas) para blindar la reputación antes de probar cuentas nuevas.',
        metric: 'Warm-up Automático 09:00',
    },
    {
        code: 'CAPA 04',
        name: 'Real-Time Bounce Circuit Breaker',
        spec: 'CORTE DE EMERGENCIA',
        desc: 'Monitorea webhooks en ventana móvil y pausa el pool de workers automáticamente si el rebote supera el 1%.',
        metric: 'Umbral Estricto < 1.0%',
    },
];

function Crosshair({ className = '' }) {
    return (
        <span
            aria-hidden="true"
            className={`pointer-events-none select-none font-mono text-xs text-cyan-400/50 leading-none ${className}`}
        >
            +
        </span>
    );
}

function Landing() {
    const { loginWithGoogle } = useAuth();
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);
    const [activeLogIndex, setActiveLogIndex] = useState(0);
    const [clock, setClock] = useState(() => new Date());

    useEffect(() => {
        const timer = setInterval(() => {
            setClock(new Date());
            setActiveLogIndex((prev) => (prev + 1) % TELEMETRY_FRAMES.length);
        }, 2200);
        return () => clearInterval(timer);
    }, []);

    const handleGoogleSuccess = async (credentialResponse) => {
        setError('');
        setLoading(true);
        try {
            await loginWithGoogle(credentialResponse.credential);
        } catch (err) {
            setError(err.message || 'Error al iniciar sesión con Google');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen blueprint-surface text-slate-100 font-sans selection:bg-cyan-500/30 overflow-x-hidden relative flex flex-col">
            {/* Ambient radial vignette to focus the CAD schematic */}
            <div className="fixed inset-0 pointer-events-none z-0 bg-[radial-gradient(circle_at_50%_20%,rgba(6,182,212,0.10),transparent_65%)]" />

            {/* Top Technical Telemetry Ribbon */}
            <div className="relative z-20 border-b border-cyan-500/25 bg-[#020813]/90 font-mono text-[11px] text-cyan-300/80">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 h-9 flex items-center justify-between gap-4 overflow-x-auto whitespace-nowrap">
                    <div className="flex items-center gap-4">
                        <span className="inline-flex items-center gap-1.5 text-emerald-400 font-semibold">
                            <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
                            SYS_ONLINE // SERVER_EDITION
                        </span>
                        <span className="text-cyan-500/40">|</span>
                        <span>SCHEMATIC: HS-CORE-V2.6</span>
                        <span className="hidden md:inline text-cyan-500/40">|</span>
                        <span className="hidden md:inline">TRANSPORT: CLOUDFLARE_EDGE + MAILCHANNELS</span>
                    </div>
                    <div className="flex items-center gap-4 text-cyan-400/70">
                        <span className="hidden sm:inline">RATE_LIMIT: 15 REQ/S</span>
                        <span className="text-cyan-500/40">|</span>
                        <span>
                            SYS_TIME:{' '}
                            {clock.toLocaleTimeString('es-AR', {
                                hour: '2-digit',
                                minute: '2-digit',
                                second: '2-digit',
                            })}
                        </span>
                    </div>
                </div>
            </div>

            {/* Navigation Header */}
            <nav className="relative z-10 border-b border-cyan-500/20 bg-[#040d1a]/80 backdrop-blur-md">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 h-20 flex items-center justify-between">
                    <div className="flex items-center gap-3.5">
                        <div className="w-11 h-11 rounded-lg overflow-hidden relative border border-cyan-400/40 bg-[#07192f] shadow-[0_0_20px_rgba(6,182,212,0.25)]">
                            <img src="/favicon.png" alt="Hardsend Logo" className="w-full h-full object-cover" />
                        </div>
                        <div>
                            <div className="flex items-center gap-2">
                                <span className="text-xl font-black tracking-tight text-white">
                                    HARD<span className="text-cyan-400">SEND</span>
                                </span>
                                <span className="px-1.5 py-0.5 text-[10px] font-mono font-semibold bg-cyan-500/15 text-cyan-300 border border-cyan-400/30 rounded">
                                    REV_4.0
                                </span>
                            </div>
                            <p className="text-[11px] font-mono text-cyan-300/60 tracking-widest uppercase">
                                High-Deliverability Invoice Dispatch // Devrow
                            </p>
                        </div>
                    </div>

                    <div className="hidden md:flex items-center gap-6 font-mono text-xs">
                        <a
                            href="#pipeline"
                            className="text-cyan-200/70 hover:text-cyan-300 transition-colors flex items-center gap-1.5"
                        >
                            <span className="text-cyan-500">[01]</span> MOTOR_ANTI_BOUNCE
                        </a>
                        <a
                            href="#specs"
                            className="text-cyan-200/70 hover:text-cyan-300 transition-colors flex items-center gap-1.5"
                        >
                            <span className="text-cyan-500">[02]</span> ARQUITECTURA_EDGE
                        </a>
                    </div>
                </div>
            </nav>

            {/* Main Schematic Workspace */}
            <main className="relative z-10 flex-1">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 pt-12 pb-20">
                    {/* Hero Grid: Left Blueprint Spec + Terminal / Right Operator Login */}
                    <div className="grid lg:grid-cols-12 gap-8 items-start">
                        {/* Left 7 Columns: Blueprint Schematic & Terminal Brutalism */}
                        <div className="lg:col-span-7 space-y-6">
                            {/* Blueprint Header Frame */}
                            <div className="blueprint-panel p-6 sm:p-8 rounded-xl relative">
                                <Crosshair className="absolute top-2 left-2.5" />
                                <Crosshair className="absolute top-2 right-2.5" />
                                <Crosshair className="absolute bottom-2 left-2.5" />
                                <Crosshair className="absolute bottom-2 right-2.5" />

                                {/* Technical Dimension Callout */}
                                <div className="flex flex-wrap items-center justify-between gap-2 pb-4 mb-6 border-b border-dashed border-cyan-500/30 font-mono text-xs text-cyan-300/80">
                                    <span className="inline-flex items-center gap-2">
                                        <Cpu className="w-3.5 h-3.5 text-cyan-400" />
                                        SPEC // DESPACHO MASIVO DE FACTURACIÓN AFIP/ARCA
                                    </span>
                                    <span className="text-cyan-400/60">DIM: 4-STAGE_PIPELINE</span>
                                </div>

                                <h1 className="text-3xl sm:text-5xl font-black tracking-tight text-white leading-[1.1] mb-5">
                                    Ingeniería de entregabilidad{' '}
                                    <span className="text-transparent bg-clip-text bg-gradient-to-r from-cyan-300 via-sky-400 to-emerald-400">
                                        con rebote menor al 1%.
                                    </span>
                                </h1>

                                <p className="text-slate-300 text-base sm:text-lg leading-relaxed mb-6">
                                    Plataforma de despacho transaccional impulsada por{' '}
                                    <span className="text-cyan-300 font-medium">Cloudflare Workers</span> y motor
                                    local en Go. Analiza carpetas completas de PDFs, sanea dominios, valida registros
                                    MX y ordena envíos por oleadas de reputación antes de emitir un solo correo.
                                </p>

                                {/* Technical Spec Metrics Strip */}
                                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-2 font-mono">
                                    <div className="p-3 rounded-lg bg-[#030b17]/90 border border-cyan-500/25">
                                        <div className="text-[10px] text-cyan-400/70 uppercase">CAPACIDAD_LOTE</div>
                                        <div className="text-lg font-bold text-white mt-0.5">11,800+ PDF</div>
                                        <div className="text-[10px] text-emerald-400">Cruce TXT en seg.</div>
                                    </div>
                                    <div className="p-3 rounded-lg bg-[#030b17]/90 border border-cyan-500/25">
                                        <div className="text-[10px] text-cyan-400/70 uppercase">WARM-UP_MODE</div>
                                        <div className="text-lg font-bold text-white mt-0.5">Progresivo</div>
                                        <div className="text-[10px] text-cyan-300">Cuota diaria activa</div>
                                    </div>
                                    <div className="p-3 rounded-lg bg-[#030b17]/90 border border-cyan-500/25">
                                        <div className="text-[10px] text-cyan-400/70 uppercase">OLEADA_01</div>
                                        <div className="text-lg font-bold text-emerald-400 mt-0.5">Verificados</div>
                                        <div className="text-[10px] text-slate-400">Prioridad de salida</div>
                                    </div>
                                    <div className="p-3 rounded-lg bg-[#030b17]/90 border border-cyan-500/25">
                                        <div className="text-[10px] text-cyan-400/70 uppercase">CIRCUIT_BREAKER</div>
                                        <div className="text-lg font-bold text-amber-400 mt-0.5">&lt; 1.00%</div>
                                        <div className="text-[10px] text-slate-400">Auto-pausa activa</div>
                                    </div>
                                </div>
                            </div>

                            {/* Archetype #4: Terminal Brutalism Live Console */}
                            <div className="terminal-brutal-box rounded-xl overflow-hidden relative">
                                <div className="crt-scanlines absolute inset-0 pointer-events-none z-10" />

                                {/* ASCII / Brutalist Header Bar */}
                                <div className="px-4 py-2.5 bg-[#08131c] border-b border-emerald-500/40 flex items-center justify-between text-xs text-emerald-400">
                                    <div className="flex items-center gap-2 font-bold">
                                        <Terminal className="w-4 h-4" />
                                        <span>┌── HARDSEND // PRE-FLIGHT &amp; EDGE TELEMETRY ──┐</span>
                                    </div>
                                    <span className="text-[11px] text-emerald-300/70 hidden sm:inline">
                                        TTY: /dev/hardsend0 [READ_ONLY]
                                    </span>
                                </div>

                                {/* Terminal Stream Body */}
                                <div className="p-4 sm:p-5 space-y-2 text-xs leading-relaxed relative z-20">
                                    <div className="text-slate-500 pb-1 border-b border-emerald-500/15">
                                        $ hardsend-server --mode=warmup --anti-bounce=4-layers --provider=cloudflare
                                    </div>

                                    {TELEMETRY_FRAMES.map((frame, idx) => {
                                        const isCurrent = idx === activeLogIndex;
                                        return (
                                            <div
                                                key={frame.layer}
                                                className={`flex flex-wrap items-baseline gap-x-2 py-0.5 transition-opacity duration-300 ${
                                                    isCurrent ? 'opacity-100' : 'opacity-75'
                                                }`}
                                            >
                                                <span className="text-slate-500">[{frame.ts}]</span>
                                                <span className="px-1.5 py-0.2 bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-[10px] rounded">
                                                    {frame.layer}
                                                </span>
                                                <span className="text-cyan-400 font-semibold">{frame.tag}:</span>
                                                <span className={frame.color}>{frame.msg}</span>
                                            </div>
                                        );
                                    })}

                                    <div className="pt-2 flex items-center gap-2 text-emerald-400">
                                        <span>root@hardsend-edge:~#</span>
                                        <span className="text-emerald-300">awaiting_operator_session</span>
                                        <span className="inline-block w-2 h-4 bg-emerald-400 animate-cursor-blink" />
                                    </div>
                                </div>
                            </div>
                        </div>

                        {/* Right 5 Columns: Operator Security Access & Pre-Flight Status */}
                        <div className="lg:col-span-5 space-y-6">
                            <div className="blueprint-panel p-6 sm:p-8 rounded-xl relative border-cyan-400/40">
                                <Crosshair className="absolute top-2 left-2.5" />
                                <Crosshair className="absolute top-2 right-2.5" />
                                <Crosshair className="absolute bottom-2 left-2.5" />
                                <Crosshair className="absolute bottom-2 right-2.5" />

                                {/* Security Clearance Header */}
                                <div className="flex items-center justify-between pb-4 mb-6 border-b border-dashed border-cyan-500/30 font-mono text-xs">
                                    <span className="inline-flex items-center gap-2 text-cyan-300 font-semibold">
                                        <Lock className="w-3.5 h-3.5 text-cyan-400" />
                                        MOD_AUTH // ACCESO DE OPERADOR
                                    </span>
                                    <span className="px-2 py-0.5 rounded bg-emerald-500/15 text-emerald-300 border border-emerald-500/30 text-[10px]">
                                        OAUTH_2.0
                                    </span>
                                </div>

                                <div className="mb-6">
                                    <h2 className="text-2xl font-bold text-white mb-2">
                                        Consola de Control Hardsend
                                    </h2>
                                    <p className="text-sm text-slate-300 leading-relaxed">
                                        Iniciá sesión con tu cuenta corporativa autorizada en la whitelist para acceder
                                        al analizador de carpetas locales, semáforo pre-vuelo y monitor WebSocket.
                                    </p>
                                </div>

                                {error && (
                                    <div className="flex items-start gap-2.5 p-3.5 mb-6 rounded-lg bg-red-500/10 border border-red-500/40 text-left font-mono">
                                        <AlertCircle className="w-4 h-4 text-red-400 flex-shrink-0 mt-0.5" />
                                        <span className="text-red-300 text-xs leading-relaxed">{error}</span>
                                    </div>
                                )}

                                {/* Google OAuth Container */}
                                <div className="p-5 rounded-xl bg-[#020914]/90 border border-cyan-500/30 flex flex-col items-center justify-center gap-4">
                                    <div className="text-[11px] font-mono text-cyan-300/70 uppercase tracking-wider">
                                        {loading ? 'VERIFICANDO CREDENCIAL...' : 'AUTENTICACIÓN FEDERADA REQUERIDA'}
                                    </div>

                                    <div className="flex justify-center min-h-[44px] items-center">
                                        <GoogleLogin
                                            onSuccess={handleGoogleSuccess}
                                            onError={() => setError('Error de conexión con Google OAuth')}
                                            useOneTap
                                            theme="filled_black"
                                            shape="pill"
                                            size="large"
                                        />
                                    </div>
                                </div>

                                {/* Pre-Flight Readiness Checklist */}
                                <div className="mt-6 pt-5 border-t border-dashed border-cyan-500/25 space-y-2.5 font-mono text-xs">
                                    <div className="text-[11px] text-cyan-400/70 uppercase mb-2">
                                        VERIFICACIÓN DE SUBSISTEMAS:
                                    </div>
                                    <div className="flex items-center justify-between text-slate-300">
                                        <span className="inline-flex items-center gap-2">
                                            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                                            Base de Reputación SQLite WAL
                                        </span>
                                        <span className="text-emerald-400">READY</span>
                                    </div>
                                    <div className="flex items-center justify-between text-slate-300">
                                        <span className="inline-flex items-center gap-2">
                                            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                                            Escáner Local de PDFs y Cruce TXT
                                        </span>
                                        <span className="text-emerald-400">READY</span>
                                    </div>
                                    <div className="flex items-center justify-between text-slate-300">
                                        <span className="inline-flex items-center gap-2">
                                            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                                            Scheduler de Calentamiento (Warm-up)
                                        </span>
                                        <span className="text-cyan-300">ARMED</span>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Section 01: 4-Layer Anti-Bounce Schematic Diagram */}
                    <section id="pipeline" className="mt-16 pt-12 border-t border-dashed border-cyan-500/25">
                        <div className="flex flex-col md:flex-row md:items-end justify-between mb-8 gap-4">
                            <div>
                                <div className="font-mono text-xs text-cyan-400 uppercase tracking-widest mb-1">
                                    [01] // ESQUEMA TÉCNICO DE PROTECCIÓN DE DOMINIO
                                </div>
                                <h2 className="text-2xl sm:text-3xl font-black text-white">
                                    Motor Anti-Bounce de 4 Capas
                                </h2>
                            </div>
                            <p className="font-mono text-xs text-cyan-300/70 max-w-md">
                                Cada archivo PDF atraviesa cuatro validaciones determinísticas antes de consumir cuota
                                diaria en los servidores de correo.
                            </p>
                        </div>

                        <div className="grid md:grid-cols-2 lg:grid-cols-4 gap-5">
                            {PIPELINE_LAYERS.map((layer) => (
                                <div
                                    key={layer.code}
                                    className="blueprint-panel blueprint-panel-hover p-5 rounded-xl relative flex flex-col justify-between"
                                >
                                    <Crosshair className="absolute top-1.5 right-2" />
                                    <div>
                                        <div className="flex items-center justify-between font-mono text-xs mb-3 pb-2 border-b border-cyan-500/20">
                                            <span className="font-bold text-cyan-300">{layer.code}</span>
                                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-cyan-500/10 text-cyan-300 border border-cyan-500/30">
                                                {layer.spec}
                                            </span>
                                        </div>
                                        <h3 className="text-base font-bold text-white mb-2">{layer.name}</h3>
                                        <p className="text-xs text-slate-300 leading-relaxed mb-4">{layer.desc}</p>
                                    </div>
                                    <div className="pt-3 border-t border-dashed border-cyan-500/20 font-mono text-[11px] text-emerald-400 flex items-center justify-between">
                                        <span>STATUS:</span>
                                        <span>{layer.metric}</span>
                                    </div>
                                </div>
                            ))}
                        </div>
                    </section>

                    {/* Section 02: Infrastructure Specifications */}
                    <section id="specs" className="mt-14 pt-12 border-t border-dashed border-cyan-500/25">
                        <div className="grid md:grid-cols-3 gap-6">
                            <SpecCard
                                code="SPEC_A1"
                                icon={<Server className="w-5 h-5 text-cyan-400" />}
                                title="Escaneo Directo en Disco"
                                subtitle="MODO CARPETA LOCAL"
                                description="Procesa miles de facturas directamente desde el sistema de archivos local sin límites de subida HTTP, cruzando el padrón TXT en memoria en milisegundos."
                            />
                            <SpecCard
                                code="SPEC_A2"
                                icon={<Layers className="w-5 h-5 text-emerald-400" />}
                                title="Semáforo Pre-Vuelo (Dry-Run)"
                                subtitle="AUDITORÍA ANTES DE ENVIAR"
                                description="Clasifica la campaña en Verificados (Verde), Nuevos (Azul), Corregibles (Amarillo), Suprimidos (Gris) y Errores PDF (Rojo) antes de disparar la cola."
                            />
                            <SpecCard
                                code="SPEC_A3"
                                icon={<Activity className="w-5 h-5 text-amber-400" />}
                                title="Telemetría WebSocket en Vivo"
                                subtitle="MONITOREO AL SEGUNDO"
                                description="Seguimiento en tiempo real de workers activos, cuota diaria de calentamiento, aperturas, entregas confirmadas y estado del Circuit Breaker."
                            />
                        </div>
                    </section>
                </div>
            </main>

            <div className="relative z-10 border-t border-cyan-500/20 bg-[#020813]/90">
                <Footer />
            </div>
        </div>
    );
}

function SpecCard({ code, icon, title, subtitle, description }) {
    return (
        <div className="blueprint-panel blueprint-panel-hover p-6 rounded-xl relative">
            <div className="flex items-center justify-between font-mono text-xs text-cyan-400/70 mb-4">
                <span>// {code}</span>
                <span>{subtitle}</span>
            </div>
            <div className="w-10 h-10 rounded-lg bg-cyan-500/10 border border-cyan-400/30 flex items-center justify-center mb-4">
                {icon}
            </div>
            <h3 className="text-lg font-bold text-white mb-2">{title}</h3>
            <p className="text-slate-300 text-xs leading-relaxed">{description}</p>
        </div>
    );
}

export default Landing;
