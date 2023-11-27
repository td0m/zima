package zima

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Set struct {
	Type     string
	ID       string
	Relation string
}

func (s Set) CacheChildren(ctx context.Context, children []Set) error {
	query := `
		insert into caches(set_type, set_id, set_relation, children, parents)
		values($1, $2, $3, $4, '[]')
		on conflict (set_type, set_id, set_relation)
		do update
		set children = $4
	`

	if _, err := pg.Exec(ctx, query, s.Type, s.ID, s.Relation, children); err != nil {
		return err
	}
	return nil
}


func (s Set) CacheParents(ctx context.Context, parents []Set) error {
	query := `
		insert into caches(set_type, set_id, set_relation, children, parents)
		values($1, $2, $3, '[]', $4)
		on conflict (set_type, set_id, set_relation)
		do update
		set parents = $4
	`

	if _, err := pg.Exec(ctx, query, s.Type, s.ID, s.Relation, parents); err != nil {
		return err
	}
	return nil
}

func (s Set) Children(ctx context.Context) ([]Set, error) {
	query := `
		select children
		from caches
		where (set_type, set_id, set_relation) = ($1, $2, $3)
	`

	children := []Set{}
	if err := pg.QueryRow(ctx, query, s.Type, s.ID, s.Relation).Scan(&children); err != nil {
		if err == pgx.ErrNoRows {
			return []Set{}, nil
		}
		return nil, err
	}

	return children, nil
}

func (s Set) Parents(ctx context.Context) ([]Set, error) {
	query := `
		select parents
		from caches
		where (set_type, set_id, set_relation) = ($1, $2, $3)
	`

	parents := []Set{}
	if err := pg.QueryRow(ctx, query, s.Type, s.ID, s.Relation).Scan(&parents); err != nil {
		if err == pgx.ErrNoRows {
			return []Set{}, nil
		}
		return nil, err
	}

	return parents, nil
}

func (s Set) Equals(s2 Set) bool {
	return s.Type == s2.Type && s.ID == s2.ID && s.Relation == s2.Relation
}

func (s Set) IsSingleton() bool {
	return s.Relation == ""
}

func NewSet(typ, id, relation string) (Set, error) {
	return Set{typ, id, relation}, nil
}
