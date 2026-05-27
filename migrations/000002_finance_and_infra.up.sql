-- =========================================================================
-- 4. ДОМЕН ФИНАНСОВ (Double-Entry Journaling)
-- =========================================================================

CREATE TABLE accounts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL UNIQUE, -- У одного юзера один кошелек (в рамках одной валюты)
    currency VARCHAR(3) NOT NULL DEFAULT 'RUB',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TYPE tx_type AS ENUM ('deposit', 'withdraw', 'hold', 'refund');

CREATE TABLE wallet_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    account_id UUID NOT NULL REFERENCES accounts(id),
    type tx_type NOT NULL,
    amount NUMERIC(12, 2) NOT NULL, -- Может быть негативным при списании
    reference_id UUID, -- ID заказа или внешней транзакции для аудита
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- =========================================================================
-- 5. ИНФРАСТРУКТУРА (Transactional Outbox Pattern)
-- =========================================================================

CREATE TYPE outbox_status AS ENUM ('pending', 'processed', 'failed');

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status outbox_status NOT NULL DEFAULT 'pending',
    trace_id VARCHAR(255), -- Прокидываем OpenTelemetry trace_id сквозь БД
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMP WITH TIME ZONE
);

-- =========================================================================
-- 6. ИНДЕКСЫ ДЛЯ ВЫСОКИХ НАГРУЗОК (HIGH LOAD OPTIMIZATION)
-- =========================================================================

-- Частичный индекс: ищем только СВОБОДНЫЕ билеты в конкретной зоне (индекс будет крошечным)
CREATE INDEX idx_tickets_available_zones 
ON tickets (zone_id) 
WHERE status = 'available';

-- Индекс для демона-воркера, который каждую секунду чистит просроченные холды
CREATE INDEX idx_ticket_holds_expiry 
ON ticket_holds (expires_at);

-- Индекс для Outbox воркера, чтобы быстро выбирать неотправленные события в хронологическом порядке
CREATE INDEX idx_outbox_pending_events 
ON outbox_events (created_at) 
WHERE status = 'pending';

-- Индекс по JSONB полю для быстрого поиска по метаданным мероприятий
CREATE INDEX idx_events_metadata_gin 
ON events USING gin (metadata);