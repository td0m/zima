package zima

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidType        = errors.New("invalid subject or object type")
	ErrInvalidSystemGroup = errors.New("invalid system group")
)

type Server struct {
	conn   *pgxpool.Pool
	tuples *TupleStore
}

type CheckResponse struct {
	Success bool
	Cache   bool
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

	// Direct connection check
	success, err := srv.tuples.Exists(ctx, t)
	if err != nil {
		return false, fmt.Errorf("failed to read tuples in db: %w", err)
	}
	if success {
		return true, nil
	}

	// Groups
	subjects, err := srv.tuples.ListSubsets(ctx, t.Parent)
	if err != nil {
		return false, fmt.Errorf("failed to list subjects: %w", err)
	}
	for _, subject := range subjects {
		// TODO: parallel
		res, err := srv.Check(ctx, Tuple{Parent: Set{ID: subject.ID, Type: subject.Type, Relation: subject.Relation}, Child: t.Child})
		if err != nil {
			return false, fmt.Errorf("failed to check tupleset: %w", err)
		}
		if res {
			return true, nil
		}
	}

	return false, nil
}

func (s *Server) Write(ctx context.Context, add []Tuple, remove []Tuple) error {
	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}

	for _, t := range add {
		if err := s.tuples.WithTx(tx).Add(ctx, t); err != nil {
			if err := tx.Rollback(ctx); err != nil {
				return fmt.Errorf("failed to rollback '%s': %w", t, err)
			}
			return fmt.Errorf("failed to add tuple '%s': %w", t, err)
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

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	return nil
}

func NewServer(conn *pgxpool.Pool) *Server {
	return &Server{conn, NewTupleStore(conn)}
}
