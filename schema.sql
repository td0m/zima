create table changes(
  id text primary key,
  type text not null,
  payload jsonb not null,
  processed bool not null default false,
  created_at timestamptz not null default now()
);

create index changes_processed_idx on changes(processed);

create table caches(
  set_type text not null,
  set_id text not null,
  set_relation text not null,

  parents jsonb not null default '[]',
  children jsonb not null default '[]',
  subsets jsonb not null default '[]',

  primary key (set_type, set_id, set_relation)
);

