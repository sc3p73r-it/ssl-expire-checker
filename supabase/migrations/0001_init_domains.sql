-- SSL Expiry Checker — initial schema (also applied via Supabase MCP during setup)
create extension if not exists "pgcrypto";

create table public.domains (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null references auth.users(id) on delete cascade,
  url text not null,
  issuer text,
  expiry_date timestamptz,
  status text not null default 'pending'
    check (status in ('pending','valid','expiring','expired','error')),
  last_checked timestamptz,
  last_error text,
  created_at timestamptz not null default now(),
  unique (user_id, url)
);

create index domains_user_id_idx on public.domains(user_id);
create index domains_expiry_idx  on public.domains(expiry_date);

alter table public.domains enable row level security;

create policy "domains_select_own" on public.domains
  for select using ((select auth.uid()) = user_id);
create policy "domains_insert_own" on public.domains
  for insert with check ((select auth.uid()) = user_id);
create policy "domains_update_own" on public.domains
  for update using ((select auth.uid()) = user_id) with check ((select auth.uid()) = user_id);
create policy "domains_delete_own" on public.domains
  for delete using ((select auth.uid()) = user_id);
