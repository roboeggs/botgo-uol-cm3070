-- SR Levels — database schema
-- PostgreSQL 17+
--
-- Apply with:  psql -d <database> -f docs/schema.sql
--
-- Note: this file documents the schema used by the bot. The
-- `detector_state` table is populated by an external ML pipeline
-- running separately.

-- ============================================================
--  Helper: updated_at trigger function
-- ============================================================
create or replace function update_updated_at_column()
returns trigger as $$
begin
    new.updated_at = now();
    return new;
end;
$$ language plpgsql;

-- ============================================================
--  users
-- ============================================================
create table public.users (
  id bigint not null,
  username text null,
  first_name text null,
  last_name text null,
  language_code text null default 'en'::text,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  is_blocked boolean null default false,
  metadata jsonb null,
  constraint users_pkey primary key (id)
);

create trigger update_users_updated_at
  before update on users
  for each row
  execute function update_updated_at_column();

-- ============================================================
--  user_favorites
-- ============================================================
create table public.user_favorites (
  user_id bigint not null,
  ticker text not null,
  added_at timestamp with time zone not null default now(),
  constraint user_favorites_pkey primary key (user_id, ticker),
  constraint user_favorites_user_id_fkey
    foreign key (user_id) references users (id) on delete cascade,
  constraint user_favorites_ticker_format
    check (ticker ~ '^[A-Z0-9]{1,12}USDT$'::text)
);

create index if not exists user_favorites_user_idx
  on public.user_favorites using btree (user_id);

-- ============================================================
--  detector_state
--  Populated by the external ML pipeline. One row per Binance pair.
-- ============================================================
create table public.detector_state (
  ticker text not null,
  state_blob jsonb not null default '{}'::jsonb,
  has_active_levels boolean not null default false,
  saved_at timestamp with time zone not null default now(),
  constraint detector_state_pkey primary key (ticker)
);

create index if not exists detector_state_active_idx
  on public.detector_state (ticker)
  where has_active_levels = true;

-- ============================================================
--  processing_log
--  Tracks the last processing time per ticker.
-- ============================================================
create table public.processing_log (
  ticker text not null,
  last_processed_time bigint not null,
  first_loaded_time bigint null,
  params_json jsonb null,
  created_at timestamp with time zone null default now(),
  updated_at timestamp with time zone null default now(),
  constraint processing_log_pkey primary key (ticker)
);

create trigger trg_processing_log_updated_at
  before update on processing_log
  for each row
  execute function update_updated_at_column();