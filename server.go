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
	conn       *pgxpool.Pool
	processing chan bool
}

type ListChildrenRequest struct {
	Type     string
	ID       string
	Relation string
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

	children, err := t.Parent.Subsets(ctx)
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
		Type:    ev,
		Payload: t,
	}

	if err := change.Create(ctx); err != nil {
		return fmt.Errorf("change creation failed: %w", err)
	}

	s.processChangesImmediately()

	return nil
}

func (s *Server) processChange(ctx context.Context, c Change) error {
	// TODO: add mutex
	if c.Type == "ADD_TUPLE" {
		return s.processAddTuple(ctx, c.Payload)
	} else {
		return s.processRemoveTuple(ctx, c.Payload)
	}
}

func (s *Server) processChangesImmediately() {
	go func() {
		s.processing <- true
	}()
}

func (s *Server) processAddTuple(ctx context.Context, t Tuple) error {
	start := time.Now()
	a, b := t.Parent, t.Child
	if err := a.AddDirectChild(ctx, b); err != nil {
		return err
	}
	if err := b.AddDirectParent(ctx, a); err != nil {
		return err
	}

	bSubsets, err := b.Subsets(ctx)
	if err != nil {
		return err
	}

	bSubsets = append(bSubsets, b)

	if err := a.AddSubsets(ctx, bSubsets); err != nil {
		return err
	}

	supersetsOfA, err := a.ComputeSupersets(ctx)
	if err != nil {
		return err
	}
	for _, parent := range supersetsOfA {
		if err := parent.AddSubsets(ctx, bSubsets); err != nil {
			return err
		}
	}
	fmt.Println(time.Since(start).Milliseconds())
	return nil
}

func (s *Server) processRemoveTuple(ctx context.Context, t Tuple) error {
	a, b := t.Parent, t.Child
	if err := a.RemoveChild(ctx, b); err != nil {
		return err
	}
	if err := b.RemoveParent(ctx, a); err != nil {
		return err
	}

	if err := a.CacheSubsets(ctx); err != nil {
		return err
	}

	supersetsOfA, err := a.ComputeSupersets(ctx)
	if err != nil {
		return err
	}
	for _, parent := range supersetsOfA {
		if err := parent.CacheSubsets(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) ProcessOne(ctx context.Context) error {
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
		slog.Debug("no tasks, sleeping")

		select {
		case <-s.processing:
		case <-time.After(timeout):
		}

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

func (s *Server) ListChildren(ctx context.Context, request ListChildrenRequest) (*Sets, error) {
	children, err := Set{Type: request.Type, ID: request.ID, Relation: request.Relation}.Children(ctx)
	if err != nil {
		return nil, fmt.Errorf("db failed: %w", err)
	}
	return &Sets{Items: children}, nil
}

func (s *Server) ListParents(ctx context.Context, request ListParentsRequest) (*Sets, error) {
	parents, err := Set{Type: request.Type, ID: request.ID, Relation: request.Relation}.Parents(ctx)
	if err != nil {
		return nil, fmt.Errorf("db failed: %w", err)
	}
	return &Sets{Items: parents}, nil
}

func NewServer(conn *pgxpool.Pool) *Server {
	pg = conn
	return &Server{conn, make(chan bool, 1)}
}
