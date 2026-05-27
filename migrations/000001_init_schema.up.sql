-- Включаем расширение для генерации UUID
create extension if not exists "uuid-ossp"

-- =========================================================================
-- 1. ДОМЕН МЕРОПРИЯТИЙ (Каталог)
-- =========================================================================

create type event_status as enum ('draft', 'active', 'cancelled', 'completed');

create table events (
	id uuid primary key default uuid_generate_v4(),
	title varchar(255) not null,
	description text,
	metadata jsonb default '{}'::jsonb, --Для гибких настроек организаторов
	status event_status not null default 'draft',
	start_at timestamp with time zone not null,
	created_at timestamp with time zone not null default now(),
	updated_at timestamp with time zone not null default now()
);

CREATE TABLE zones (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    price NUMERIC(12, 2) NOT NULL CHECK (price >= 0), -- Деньги ТОЛЬКО в numeric
    capacity INT NOT NULL CHECK (capacity > 0)
);

CREATE TABLE seats (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    zone_id UUID NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
    row INT NOT NULL CHECK (row > 0),
    number INT NOT NULL CHECK (number > 0),
    UNIQUE (zone_id, row, number) -- Защита от дублей мест в одном секторе
);

-- =========================================================================
-- 2. ДОМЕН ИНВЕНТАРЯ И БРОНИРОВАНИЯ
-- =========================================================================

CREATE TYPE ticket_status AS ENUM ('available', 'held', 'sold', 'blocked');

CREATE TABLE tickets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    zone_id UUID NOT NULL REFERENCES zones(id),
    seat_id UUID REFERENCES seats(id), -- Nullable, если это танцпол без мест
    status ticket_status NOT NULL DEFAULT 'available',
    version INT NOT NULL DEFAULT 1, -- Для Optimistic Locking (оптимистичные блокировки)
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE ticket_holds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticket_id UUID NOT NULL UNIQUE REFERENCES tickets(id) ON DELETE CASCADE, -- Строго 1 холд на 1 билет
    user_id UUID NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- =========================================================================
-- 3. ДОМЕН ЗАКАЗОВ
-- =========================================================================

CREATE TYPE order_status AS ENUM ('created', 'paying', 'paid', 'failed', 'cancelled');

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL,
    status order_status NOT NULL DEFAULT 'created',
    total_amount NUMERIC(12, 2) NOT NULL CHECK (total_amount >= 0),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    ticket_id UUID NOT NULL REFERENCES tickets(id),
    price NUMERIC(12, 2) NOT NULL, -- Фиксируем цену на момент покупки
    UNIQUE (order_id, ticket_id)
);