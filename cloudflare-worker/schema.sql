-- ============================================================================
-- HARDSEND PRODUCTION SCHEMA (Cloudflare D1 SQLite)
-- Month-over-month Auditing, Queueing, Delivery Logs & Suppression Store
-- ============================================================================

-- 1. Monthly Campaigns Table
CREATE TABLE IF NOT EXISTS campaigns (
    id TEXT PRIMARY KEY,
    period TEXT NOT NULL,                     -- e.g. "2026-11"
    name TEXT NOT NULL,                       -- e.g. "Facturación Noviembre 2026"
    archive_filename TEXT,                    -- e.g. "PDF.rar"
    total_invoices INTEGER DEFAULT 0,
    queued_count INTEGER DEFAULT 0,
    accepted_count INTEGER DEFAULT 0,
    delivered_count INTEGER DEFAULT 0,
    bounced_count INTEGER DEFAULT 0,
    daily_limit INTEGER DEFAULT 200,
    status TEXT DEFAULT 'ACTIVE',             -- ACTIVE, PAUSED, COMPLETED
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_campaigns_period ON campaigns(period);

-- 2. Invoices Table (One entry per PDF in the monthly cycle)
CREATE TABLE IF NOT EXISTS invoices (
    id TEXT PRIMARY KEY,
    campaign_id TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    invoice_number TEXT NOT NULL,             -- e.g. "0002-00443428"
    client_code TEXT,                         -- e.g. "443428"
    client_name TEXT,                         -- e.g. "GARCIA JUAN CARLOS"
    recipient_email TEXT NOT NULL,
    due_date TEXT,                            -- e.g. "10/11/2026"
    pdf_r2_key TEXT,                          -- e.g. "invoices/2026-11/0002-00443428.pdf"
    pdf_size INTEGER,
    priority_wave INTEGER DEFAULT 1,          -- 1: Verified reputation, 2: Standard, 3: New
    status TEXT DEFAULT 'QUEUED',             -- QUEUED, SENDING, ACCEPTED, DELIVERED, BOUNCED_SOFT, BOUNCED_HARD, SUPPRESSED, FAILED
    message_id TEXT,                          -- Cloudflare Message-ID header
    attempts INTEGER DEFAULT 0,
    sent_at DATETIME,
    delivered_at DATETIME,
    bounced_at DATETIME,
    error_reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(campaign_id, invoice_number)
);

CREATE INDEX IF NOT EXISTS idx_invoices_campaign_status ON invoices(campaign_id, status);
CREATE INDEX IF NOT EXISTS idx_invoices_recipient_email ON invoices(recipient_email);
CREATE INDEX IF NOT EXISTS idx_invoices_message_id ON invoices(message_id);

-- 3. Granular Delivery Events Log (Directly fed by Cloudflare Event Subscriptions)
CREATE TABLE IF NOT EXISTS delivery_events (
    id TEXT PRIMARY KEY,
    invoice_id TEXT REFERENCES invoices(id) ON DELETE SET NULL,
    campaign_id TEXT REFERENCES campaigns(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,                 -- accepted, delivered, deferred, bounced, complained, rejected, failed
    event_id TEXT,                            -- Cloudflare eventId
    message_id TEXT,                          -- Cloudflare messageId
    recipient TEXT NOT NULL,
    provider TEXT,                            -- gmail, hotmail, external_smtp, cloudflare
    smtp_code TEXT,                           -- e.g. "250", "550", "451"
    smtp_response TEXT,                       -- e.g. "250 2.0.0 OK"
    bounce_type TEXT,                         -- hard, soft
    bounce_reason TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_delivery_events_invoice ON delivery_events(invoice_id);
CREATE INDEX IF NOT EXISTS idx_delivery_events_created ON delivery_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_delivery_events_type ON delivery_events(event_type);

-- 4. Permanent Suppressions Store (Anti-Bounce Persistent Memory)
CREATE TABLE IF NOT EXISTS suppressions (
    email TEXT PRIMARY KEY,
    reason TEXT NOT NULL,                     -- hard_bounce, soft_bounce_exhausted, complaint, manual
    source TEXT DEFAULT 'cloudflare',         -- cloudflare, system, operator
    original_invoice TEXT,
    expires_at DATETIME,                      -- NULL for permanent, timestamp for soft
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_suppressions_expires ON suppressions(expires_at);

-- 5. Daily Usage & Rate Quota Tracker
CREATE TABLE IF NOT EXISTS daily_usage (
    date TEXT PRIMARY KEY,                    -- YYYY-MM-DD
    sent_count INTEGER DEFAULT 0,
    quota_limit INTEGER DEFAULT 200,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
