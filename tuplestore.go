package zima

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type TupleStore struct {
	conn querier
}

func (t *TupleStore) WithTx(tx pgx.Tx) *TupleStore {
	return &TupleStore{conn: tx}
}

func (t *TupleStore) ListSubsets(ctx context.Context, s Set) ([]Set, error) {
	subs, err := t.ListChildren(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("failed to list children: %w", err)
	}

	out := []Set{}
	for _, s := range subs {
		if s.Relation == "" {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func (t *TupleStore) ListChildren(ctx context.Context, s Set) ([]Set, error) {
	query := `
		select child_type, child_id, child_relation
		from tuples
		where
			(parent_type, parent_id, parent_relation) = ($1, $2, $3)
	`

	rows, err := t.conn.Query(ctx, query, s.Type, s.ID, s.Relation)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}

	children := []Set{}
	for rows.Next() {
		sub := Set{}
		if err := rows.Scan(&sub.Type, &sub.ID, &sub.Relation); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}
		children = append(children, sub)
	}

	return children, nil
}

func (t *TupleStore) ListConnectingFrom(ctx context.Context, typ, id string) ([]Connection, error) {
	query := `
		select parent_relation, child_type, child_id, child_relation
		from tuples
		where
			(parent_type, parent_id) = ($1, $2)
	`

	rows, err := t.conn.Query(ctx, query, typ, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}

	children := []Connection{}
	for rows.Next() {
		r := Connection{}
		if err := rows.Scan(&r.Relation, &r.Set.Type, &r.Set.ID, &r.Set.Relation); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}
		children = append(children, r)
	}

	return children, nil
}

func (t *TupleStore) ListConnectingTo(ctx context.Context, s Set) ([]Set, error) {
	query := `
		select parent_type, parent_id, parent_relation
		from tuples
		where
			(child_type, child_id, child_relation) = ($1, $2, $3)
	`

	rows, err := t.conn.Query(ctx, query, s.Type, s.ID, s.Relation)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}

	children := []Set{}
	for rows.Next() {
		child := Set{}
		if err := rows.Scan(&child.Type, &child.ID, &child.Relation); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}
		children = append(children, child)
	}

	return children, nil
}

func (t *TupleStore) Add(ctx context.Context, tuple Tuple) error {
	query := `
		insert into tuples(parent_type, parent_id, parent_relation, child_type, child_id, child_relation)
		values($1, $2, $3, $4, $5, $6)
		on conflict do nothing
	`

	p, c := tuple.Parent, tuple.Child
	if _, err := t.conn.Exec(ctx, query, p.Type, p.ID, p.Relation, c.Type, c.ID, c.Relation); err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}

	return nil
}

func (t *TupleStore) Exists(ctx context.Context, tuple Tuple) (bool, error) {
	query := `
		select exists(
			select 1
			from tuples
			where
				(parent_type, parent_id, parent_relation, child_type, child_id, child_relation) = ($1, $2, $3, $4, $5, $6)
		)
	`

	p, c := tuple.Parent, tuple.Child
	row := t.conn.QueryRow(ctx, query, p.Type, p.ID, p.Relation, c.Type, c.ID, c.Relation)

	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("query failed: %w", err)
	}

	return exists, nil
}

func (t *TupleStore) Remove(ctx context.Context, tuple Tuple) error {
	query := `
		delete from tuples
		where
			(parent_type, parent_id, parent_relation, child_type, child_id, child_relation) = ($1, $2, $3, $4, $5, $6)
	`

	p, c := tuple.Parent, tuple.Child
	if _, err := t.conn.Exec(ctx, query, p.Type, p.ID, p.Relation, c.Type, c.ID, c.Relation); err != nil {
		return fmt.Errorf("exec failed: %w", err)
	}

	return nil
}

func (t *TupleStore) ListChildrenRec(ctx context.Context, parent Set) ([]Set, error) {
	query := `
		with recursive connections as (
			select
				child_type, child_id, child_relation
			from tuples
			where (parent_type, parent_id, parent_relation) = ($1, $2, $3)

			union

			select next.child_type, next.child_id, next.child_relation
			from tuples next
			inner join
				connections prev on (prev.child_type, prev.child_id) = (next.parent_type, next.parent_id)
			where (next.child_type, next.child_id) != ($1, $2)
		) select child_type, child_id, child_relation from connections
	`

	rows, err := t.conn.Query(ctx, query, parent.Type, parent.ID, parent.Relation)
	if err != nil {
		return nil, fmt.Errorf("exec failed: %w", err)
	}

	var children []Set
	for rows.Next() {
		var child Set
		if err := rows.Scan(&child.Type, &child.ID, &child.Relation); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}
		children = append(children, child)
	}

	return children, nil
}


func (t *TupleStore) ListParentsRec(ctx context.Context, child Set) ([]Set, error) {
	query := `
		with recursive connections as (
			select
				parent_type, parent_id, parent_relation
			from tuples
			where (child_type, child_id, child_relation) = ($1, $2, $3)

			union

			select next.parent_type, next.parent_id, next.parent_relation
			from tuples next
			inner join
				connections prev on (prev.parent_type, prev.parent_id) = (next.child_type, next.child_id)
			where (next.parent_type, next.parent_id) != ($1, $2)
		) select parent_type, parent_id, parent_relation from connections
	`

	rows, err := t.conn.Query(ctx, query, child.Type, child.ID, child.Relation)
	if err != nil {
		return nil, fmt.Errorf("exec failed: %w", err)
	}

	parents := []Set{}
	for rows.Next() {
		var parent Set
		if err := rows.Scan(&parent.Type, &parent.ID, &parent.Relation); err != nil {
			return nil, fmt.Errorf("failed to scan: %w", err)
		}
		parents = append(parents, parent)
	}

	return parents, nil
}

func NewTupleStore(c *pgxpool.Pool) *TupleStore {
	return &TupleStore{c}
}
