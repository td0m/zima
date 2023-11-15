create table tuples(
  parent_type  text not null,
  parent_id    text not null,
  parent_relation text not null,

  child_type  text not null,
  child_id    text not null,
  child_relation text not null, -- empty = user

  primary key(parent_type, parent_id, parent_relation, child_type, child_id, child_relation)
);

