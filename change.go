package zima

import (
	"context"

	"github.com/rs/xid"
)

type Change struct {
	Type    string
	Payload Tuple
}

func (c Change) Create(ctx context.Context) error {
	query := `
		INSERT INTO changes(id, type, payload)
		VALUES($1, $2, $3)
	`

	_, err := pg.Exec(ctx, query, xid.New().String(), c.Type, c.Payload)
	return err
}
