package zima

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type Cache struct {
	Set         Set
	ChildrenRec []Set
	Parents     []Set
}

func (c Cache) StoreWithTx(ctx context.Context, tx pgx.Tx) error {
	query := `
		insert into caches(set_type, set_id, set_relation, children_rec, parents)
		values($1, $2, $3, $4, $5)
		on conflict (set_type, set_id, set_relation)
		do update
		set (children_rec, parents) = ($4, $5)
	`

	if _, err := pg.Exec(ctx, query, c.Set.Type, c.Set.ID, c.Set.Relation, c.ChildrenRec, c.Parents); err != nil {
		return err
	}
	return nil
}
