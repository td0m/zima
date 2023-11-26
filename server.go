package zima

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidSystemGroup = errors.New("invalid system group")
)

type Server struct {
	conn   *pgxpool.Pool
	tuples *TupleStore
}

type ListChildrenRequest struct {
	Type string
	ID   string
}

type ListParentsRequest struct {
	Type     string
	ID       string
	Relation string
}

type Sets struct {
	Items []Set
}

type Connections struct {
	Items []Connection
}

func (srv *Server) Check(ctx context.Context, t Tuple) (bool, error) {
	// System groups
	if t.Parent.Type == "system" && t.Parent.ID == "users" {
		switch t.Parent.Relation {
		case "authenticated":
			return t.Child.Type == "user" && len(t.Child.ID) > 0, nil
		case "*":
			return t.Child.Type == "user", nil
		default:
			return false, ErrInvalidSystemGroup
		}
	}

	children, err := t.Parent.Children(ctx)
	if err != nil {
		return false, fmt.Errorf("getting parent cache failed: %w", err)
	}

	parents, err := t.Child.Parents(ctx)
	if err != nil {
		return false, fmt.Errorf("getting child cache failed: %w", err)
	}

	fmt.Println("check", t)

	childrenWithSelf := append(children, t.Parent)
	if intersects(childrenWithSelf, parents) {
		return true, nil
	}

	return false, nil
}

func intersects(as, bs []Set) bool {
	fmt.Println("intersects(children, parents)?", as, bs)

	for _, a := range as {
		for _, b := range bs {
			if a.Equals(b) {
				return true
			}
		}
	}
	return false
}

func (s *Server) Write(ctx context.Context, add []Tuple, remove []Tuple) error {
	// TODO: ensure one write at a time.

	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}

	var toUpdateParents []Set
	var toUpdateChildren []Set

	if len(remove) > 0 {
		// copy pasted
		for _, t := range remove {
			parents, err := s.tuples.ListParentsRec(ctx, t.Child)
			if err != nil {
				return fmt.Errorf("failed to list parents rec: %w", err)
			}

			toUpdateChildren = append(toUpdateChildren, t.Parent)
			toUpdateParents = append(toUpdateParents, t.Child)

			toUpdateChildren = append(toUpdateChildren, parents...)

			children, err := s.tuples.ListChildrenRec(ctx, t.Child)
			if err != nil {
				return fmt.Errorf("failed to list children: %w", err)
			}
			toUpdateParents = append(toUpdateParents, children...)
		}
	}

	for _, t := range add {
		if t.Parent.Relation == "" {
			return fmt.Errorf("no relation")
		}
		if err := s.tuples.WithTx(tx).Add(ctx, t); err != nil {
			if err := tx.Rollback(ctx); err != nil {
				return fmt.Errorf("failed to rollback '%s': %w", t, err)
			}
			return fmt.Errorf("failed to add tuple '%s': %w", t, err)
		}
	}

	if len(add) > 0 {
		// copy pasted
		for _, t := range add {
			parents, err := s.tuples.ListParentsRec(ctx, t.Child)
			if err != nil {
				return fmt.Errorf("failed to list parents rec: %w", err)
			}

			toUpdateChildren = append(toUpdateChildren, t.Parent)
			toUpdateParents = append(toUpdateParents, t.Child)

			toUpdateChildren = append(toUpdateChildren, parents...)

			children, err := s.tuples.ListChildrenRec(ctx, t.Child)
			if err != nil {
				return fmt.Errorf("failed to list children: %w", err)
			}
			toUpdateParents = append(toUpdateParents, children...)
		}
	}

	for _, t := range remove {
		if err := s.tuples.WithTx(tx).Remove(ctx, t); err != nil {
			if err := tx.Rollback(ctx); err != nil {
				return fmt.Errorf("failed to rollback '%s': %w", t, err)
			}
			return fmt.Errorf("failed to remove tuple '%s': %w", t, err)
		}
	}

	for _, set := range toUpdateChildren {
		children, err := s.tuples.WithTx(tx).ListConnectingTo(ctx, set)
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("ListChildrenRec failed: %w", err)
		}

		if err := set.CacheChildren(ctx, tx, children); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
	}

	for _, set := range toUpdateParents {
		parents, err := s.tuples.WithTx(tx).ListParentsRec(ctx, set)
		if err != nil {
			return fmt.Errorf("ListConnectingTo failed: %w", err)
		}
		if err := set.CacheParents(ctx, tx, parents); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("cacheParents failed: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	return nil
}

func (s *Server) ListChildren(ctx context.Context, request ListChildrenRequest) (*Connections, error) {
	children, err := s.tuples.ListConnectingFrom(ctx, request.Type, request.ID)
	if err != nil {
		return nil, fmt.Errorf("db failed: %w", err)
	}

	return &Connections{Items: children}, nil
}

func (s *Server) ListParents(ctx context.Context, request ListParentsRequest) (*Sets, error) {
	parents, err := s.tuples.ListConnectingTo(ctx, Set{request.Type, request.ID, request.Relation})
	if err != nil {
		return nil, fmt.Errorf("db failed: %w", err)
	}

	return &Sets{Items: parents}, nil
}

func NewServer(conn *pgxpool.Pool) *Server {
	pg = conn
	return &Server{conn, NewTupleStore(conn)}
}
