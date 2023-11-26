create table changes(
  id text primary key,
  type text not null,
  payload jsonb not null,
  processed bool not null default false,
  created_at timestamptz not null default now()
);

create index changes_processed_idx on changes(processed);

create table tuples(
  parent_type  text not null,
  parent_id    text not null,
  parent_relation text not null,

  child_type  text not null,
  child_id    text not null,
  child_relation text not null,

  primary key(parent_type, parent_id, parent_relation, child_type, child_id, child_relation)
);

-- cache!
-- drop table caches;
create table caches(
  set_type text not null,
  set_id text not null,
  set_relation text not null,

  parents jsonb not null,
  children_rec jsonb not null,

  primary key (set_type, set_id, set_relation)
);

