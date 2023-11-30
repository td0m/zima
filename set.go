package zima

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Set struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Relation string `json:"relation"`
}

func (s Set) AddDirectChild(ctx context.Context, child ...Set) error {
	all, err := s.Children(ctx)
	if err != nil {
		return err
	}
	all = append(all, child...)
	return s.SetChildren(ctx, all)
}

func (s Set) RemoveChild(ctx context.Context, child Set) error {
	all, err := s.Children(ctx)
	if err != nil {
		return err
	}
	filtered := []Set{}
	for _, s := range all {
		if s.Equals(child) {
			continue
		}
		filtered = append(filtered, s)
	}
	return s.SetChildren(ctx, filtered)
}

func (s Set) RemoveParent(ctx context.Context, parent Set) error {
	all, err := s.Parents(ctx)
	if err != nil {
		return err
	}
	filtered := []Set{}
	for _, s := range all {
		if s.Equals(parent) {
			continue
		}
		filtered = append(filtered, s)
	}
	return s.SetParents(ctx, filtered)
}

func (s Set) ComputeSupersets(ctx context.Context) ([]Set, error) {
	direct, err := s.Parents(ctx)
	if err != nil {
		return nil, err
	}

	all := make([]Set, len(direct))
	copy(all, direct)
	for _, parent := range direct {
		supersets, err := parent.ComputeSupersets(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, supersets...)
	}

	return all, nil
}

func (s Set) ComputeSubsets(ctx context.Context) ([]Set, error) {
	direct, err := s.Children(ctx)
	if err != nil {
		return nil, err
	}

	all := []Set{}
	for _, child := range direct {
		subsets, err := child.ComputeSubsets(ctx)
		if err != nil {
			return nil, err
		}
		all = append(all, child)
		if len(subsets) > 0 {
			all = append(all, subsets...)
		}
	}

	return all, nil
}

func (s Set) AddSubsets(ctx context.Context, added []Set) error {
	existing, err := s.Subsets(ctx)
	if err != nil {
		return err
	}

	return s.SetSubsets(ctx, append(existing, added...))
}

func (s Set) CacheSubsets(ctx context.Context) error {
	subsets, err := s.ComputeSubsets(ctx)
	if err != nil {
		return err
	}

	return s.SetSubsets(ctx, subsets)
}

func (s Set) AddDirectParent(ctx context.Context, parent Set) error {
	all, err := s.Parents(ctx)
	if err != nil {
		return err
	}
	all = append(all, parent)
	return s.SetParents(ctx, all)
}

func (s Set) SetChildren(ctx context.Context, children []Set) error {
	query := `
		insert into caches(set_type, set_id, set_relation, children)
		values($1, $2, $3, $4)
		on conflict (set_type, set_id, set_relation)
		do update
		set children = $4
	`

	if _, err := pg.Exec(ctx, query, s.Type, s.ID, s.Relation, children); err != nil {
		return err
	}
	return nil
}

func (s Set) SetParents(ctx context.Context, parents []Set) error {
	query := `
		insert into caches(set_type, set_id, set_relation, parents)
		values($1, $2, $3, $4)
		on conflict (set_type, set_id, set_relation)
		do update
		set parents = $4
	`

	if _, err := pg.Exec(ctx, query, s.Type, s.ID, s.Relation, parents); err != nil {
		return err
	}
	return nil
}

func (s Set) SetSubsets(ctx context.Context, subsets []Set) error {
	query := `
		insert into caches(set_type, set_id, set_relation, subsets)
		values($1, $2, $3, $4)
		on conflict (set_type, set_id, set_relation)
		do update
		set subsets = $4
	`

	if _, err := pg.Exec(ctx, query, s.Type, s.ID, s.Relation, subsets); err != nil {
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

func (s Set) Subsets(ctx context.Context) ([]Set, error) {
	query := `
		select subsets
		from caches
		where (set_type, set_id, set_relation) = ($1, $2, $3)
	`

	subsets := []Set{}
	if err := pg.QueryRow(ctx, query, s.Type, s.ID, s.Relation).Scan(&subsets); err != nil {
		if err == pgx.ErrNoRows {
			return []Set{}, nil
		}
		return nil, err
	}

	return subsets, nil
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
