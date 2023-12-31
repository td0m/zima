package zima

import (
	"context"
	"time"

	"github.com/rs/xid"
)

type Change struct {
	ID        string
	Type      string
	Payload   Tuple
	CreatedAt time.Time
}

func (c Change) Create(ctx context.Context) error {
	query := `
		INSERT INTO changes(id, type, payload)
		VALUES($1, $2, $3)
	`

	_, err := pg.Exec(ctx, query, xid.New().String(), c.Type, c.Payload)
	return err
}
