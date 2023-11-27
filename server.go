package zima

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/exp/slog"
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

	if intersects(append(children, t.Parent), parents) {
		return true, nil
	}

	return false, nil
}

func intersects(as, bs []Set) bool {
	for _, a := range as {
		for _, b := range bs {
			if a.Equals(b) {
				return true
			}
		}
	}
	return false
}

func (s *Server) Add(ctx context.Context, t Tuple) error {
	return s.tupleChange(ctx, "ADD_TUPLE", t)
}

func (s *Server) Remove(ctx context.Context, t Tuple) error {
	return s.tupleChange(ctx, "REMOVE_TUPLE", t)
}

func (s *Server) tupleChange(ctx context.Context, ev string, t Tuple) error {
	change := Change{
		Type: ev,
		Payload: t,
	}

	if err := change.Create(ctx); err != nil {
		return fmt.Errorf("change creation failed: %w", err)
	}
	return nil
}

func (s *Server) processChange(ctx context.Context, c Change) error {
	t := c.Payload
	var toUpdateChildren []Set
	var toUpdateParents []Set

	refreshUpdates := func(ctx context.Context) error {
		parents, err := s.tuples.ListParentsRec(ctx, t.Child)
		if err != nil {
			return fmt.Errorf("failed to list parents rec: %w", err)
		}

		children, err := s.tuples.ListChildrenRec(ctx, t.Child)
		if err != nil {
			return fmt.Errorf("failed to list children: %w", err)
		}

		toUpdateParents = append([]Set{t.Child}, children...)
		toUpdateChildren = append([]Set{t.Parent}, parents...)

		return nil
	}

	if c.Type == "ADD_TUPLE" {
		if err := s.tuples.Add(ctx, c.Payload); err != nil {
			return fmt.Errorf("failed to add tuple: %w", err)
		}
		if err := refreshUpdates(ctx); err != nil {
			return fmt.Errorf("failed to get updates: %w", err)
		}
	} else {
		if err := refreshUpdates(ctx); err != nil {
			return fmt.Errorf("failed to get updates: %w", err)
		}
		if err := s.tuples.Remove(ctx, c.Payload); err != nil {
			return fmt.Errorf("failed to remove tuple: %w", err)
		}
	}

	for _, set := range toUpdateChildren {
		children, err := s.tuples.ListConnectingTo(ctx, set)
		if err != nil {
			return fmt.Errorf("ListChildrenRec failed: %w", err)
		}

		if err := set.CacheChildren(ctx, children); err != nil {
			return err
		}
	}

	for _, set := range toUpdateParents {
		parents, err := s.tuples.ListParentsRec(ctx, set)
		if err != nil {
			return fmt.Errorf("ListConnectingTo failed: %w", err)
		}
		if err := set.CacheParents(ctx, parents); err != nil {
			return fmt.Errorf("cacheParents failed: %w", err)
		}
	}

	return nil
}

func (s *Server) processOne(ctx context.Context) error {
	timeout := time.Second * 5
	stalePeriod := time.Hour

	// This timeout should be higher than the "timeout", otherwise the tx.Commit will fail
	ctx, cancel := context.WithTimeout(ctx, timeout+time.Second*2)
	defer cancel()

	tx, err := pg.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin a tx: %w", err)
	}

	var c Change
	err = tx.QueryRow(ctx, `
		update changes
		set processed=true
		where id in
		(
		  select id
		  from changes
			where not processed
		  order by created_at
		  for update skip locked
		  limit 1
		)
		returning id, type, payload, created_at
	`).Scan(&c.ID, &c.Type, &c.Payload, &c.CreatedAt)

	// No rows = no tasks
	if err == pgx.ErrNoRows {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("tx failed to commit: %w", err)
		}
		return pgx.ErrNoRows
	}

	// Failed to execute query, probably a bad query/schema
	if err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return fmt.Errorf("failed to rollback: %w", err)
		}
		return fmt.Errorf("failed to query/scan: %w", err)
	}

	// Process task
	if err := s.processChange(ctx, c); err != nil {
		if err := tx.Rollback(ctx); err != nil {
			return fmt.Errorf("failed to rollback: %w", err)
		}

		time.Sleep(timeout)

		// Tasks older than stalePeriod get logged
		if time.Since(c.CreatedAt) > stalePeriod {
			// TODO: probably log this somewhere else
			slog.Info("stale change", "change", c)
		}

		return fmt.Errorf("failed to process change %s: %w", c.Type, err)
	}

	// No errors, so task can be deleted
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("tx failed to commit: %w", err)
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
	parents, err := s.tuples.ListConnectingTo(ctx, Set{Type: request.Type, ID: request.ID, Relation: request.Relation})
	if err != nil {
		return nil, fmt.Errorf("db failed: %w", err)
	}

	return &Sets{Items: parents}, nil
}

func NewServer(conn *pgxpool.Pool) *Server {
	pg = conn
	return &Server{conn, NewTupleStore(conn)}
}
