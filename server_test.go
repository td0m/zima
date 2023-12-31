package zima

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func processAll(s *Server, ctx context.Context) error {
	for {
		if err := s.ProcessOne(ctx); err != nil {
			if err == pgx.ErrNoRows {
				return nil
			}
			return fmt.Errorf("processing change failed: %w", err)
		}
	}
}

func add(s *Server, ctx context.Context, t Tuple) error {
	err := s.Add(ctx, t)
	if err := processAll(s, ctx); err != nil {
		panic(err)
	}
	return err
}

func remove(s *Server, ctx context.Context, t Tuple) error {
	err := s.Remove(ctx, t)
	if err := processAll(s, ctx); err != nil {
		panic(err)
	}
	return err
}

func TestDirect(t *testing.T) {
	ctx := context.Background()

	a := Tuple{set("team", "admins", "member"), set("user", "alice", "")}

	t.Run("FailureIfEmpty", func(t *testing.T) {
		s := NewServer(conn())

		res, err := s.Check(ctx, a)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("Success", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))

		res, err := s.Check(ctx, a)
		require.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("FalseAfterRemoval", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))
		require.NoError(t, remove(s, ctx, a))

		res, err := s.Check(ctx, a)
		require.NoError(t, err)
		assert.False(t, res)
	})
}

func TestGroup(t *testing.T) {
	ctx := context.Background()

	a := Tuple{set("group", "admins", "member"), set("user", "alice", "")}
	b := Tuple{set("post", "a", "owner"), set("group", "admins", "member")}
	c := Tuple{set("post", "a", "owner"), set("user", "alice", "")}

	t.Run("FailsIfNoTuples", func(t *testing.T) {
		s := NewServer(conn())

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FailsIfOnlyA", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FailsIfOnlyB", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, b))

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("SuccessIfBothTuples", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))
		require.NoError(t, add(s, ctx, b))

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.True(t, res)
	})
}

func TestLabelling(t *testing.T) {
	ctx := context.Background()

	public := Tuple{set("post", "a", "is"), set("", "public", "")}

	s := NewServer(conn())

	require.NoError(t, add(s, ctx, public))

	res, err := s.Check(ctx, public)
	require.NoError(t, err)
	assert.True(t, res)
}

func TestListChildren(t *testing.T) {
	ctx := context.Background()

	admins := set("team", "admins", "member")
	a := Tuple{admins, set("user", "alice", "")}
	b := Tuple{admins, set("user", "bob", "")}
	c := Tuple{set("team", "nonadmins", "member"), set("user", "alice", "")}

	s := NewServer(conn())

	require.NoError(t, add(s, ctx, a))
	require.NoError(t, add(s, ctx, b))
	require.NoError(t, add(s, ctx, c))

	t.Run("Empty", func(t *testing.T) {
		res, err := s.ListChildren(ctx, ListChildrenRequest{Type: "some", ID: "random"})
		require.NoError(t, err)
		assert.Equal(t, 0, len(res.Items))
	})

	t.Run("Admins", func(t *testing.T) {
		res, err := s.ListChildren(ctx, ListChildrenRequest{Type: admins.Type, ID: admins.ID, Relation: "member"})
		require.NoError(t, err)
		assert.Equal(t, 2, len(res.Items))
		assert.Equal(t, a.Child, res.Items[0])
		assert.Equal(t, b.Child, res.Items[1])
	})
}

func TestListParents(t *testing.T) {
	ctx := context.Background()

	alice := set("user", "alice", "")
	admins := set("team", "admins", "member")

	a := Tuple{admins, alice}
	b := Tuple{admins, set("user", "bob", "")}
	c := Tuple{set("team", "nonadmins", "member"), alice}

	s := NewServer(conn())

	require.NoError(t, add(s, ctx, a))
	require.NoError(t, add(s, ctx, b))
	require.NoError(t, add(s, ctx, c))

	t.Run("Empty", func(t *testing.T) {
		res, err := s.ListParents(ctx, ListParentsRequest{Type: "some", ID: "random"})
		require.NoError(t, err)
		assert.Equal(t, 0, len(res.Items))
	})

	t.Run("Alice", func(t *testing.T) {
		res, err := s.ListParents(ctx, ListParentsRequest{Type: alice.Type, ID: alice.ID})
		require.NoError(t, err)
		assert.Equal(t, 2, len(res.Items))
		assert.Equal(t, a.Parent, res.Items[0])
		assert.Equal(t, c.Parent, res.Items[1])
	})
}

// TODO: Validation? No cycles?

func TestNestedGroup(t *testing.T) {
	ctx := context.Background()

	a := Tuple{set("group", "admins", "member"), set("user", "alice", "")}
	b := Tuple{set("post", "a", "owner"), set("group", "admins", "member")}
	c := Tuple{set("post", "a", "owner"), set("user", "alice", "")}

	t.Run("FailsIfNoTuples", func(t *testing.T) {
		s := NewServer(conn())

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FailsIfOnlyA", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FailsIfOnlyB", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, b))

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("SuccessIfBothTuples", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))
		require.NoError(t, add(s, ctx, b))

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("FalseAfterRemoval", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))
		require.NoError(t, add(s, ctx, b))

		require.NoError(t, remove(s, ctx, a))
		require.NoError(t, remove(s, ctx, b))

		res, err := s.Check(ctx, a)
		require.NoError(t, err)
		assert.False(t, res)
	})
}

func TestDeeplyNestedGroup(t *testing.T) {
	ctx := context.Background()

	a := Tuple{set("group", "superadmins", "member"), set("group", "duperadmins", "member")}
	b := Tuple{set("post", "a", "owner"), set("group", "admins", "member")}
	c := Tuple{set("group", "admins", "member"), set("group", "superadmins", "member")}
	d := Tuple{set("group", "duperadmins", "member"), set("user", "alice", "")}

	expected := Tuple{set("post", "a", "owner"), set("user", "alice", "")}

	t.Run("True", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))
		require.NoError(t, add(s, ctx, b))
		require.NoError(t, add(s, ctx, c))
		require.NoError(t, add(s, ctx, d))

		res, err := s.Check(ctx, expected)
		require.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("FalseOnOnlyABC", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))
		require.NoError(t, add(s, ctx, b))
		require.NoError(t, add(s, ctx, c))

		res, err := s.Check(ctx, expected)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FalseAfterRemovingC", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))
		require.NoError(t, add(s, ctx, b))
		require.NoError(t, add(s, ctx, c))
		require.NoError(t, add(s, ctx, d))

		require.NoError(t, remove(s, ctx, c))

		res, err := s.Check(ctx, expected)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FalseAfterRemovingD", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))
		require.NoError(t, add(s, ctx, b))
		require.NoError(t, add(s, ctx, c))
		require.NoError(t, add(s, ctx, d))

		require.NoError(t, remove(s, ctx, d))

		res, err := s.Check(ctx, expected)

		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FalseAfterRemovingB", func(t *testing.T) {
		s := NewServer(conn())

		require.NoError(t, add(s, ctx, a))
		require.NoError(t, add(s, ctx, b))
		require.NoError(t, add(s, ctx, c))
		require.NoError(t, add(s, ctx, d))

		require.NoError(t, remove(s, ctx, b))

		res, err := s.Check(ctx, expected)
		require.NoError(t, err)
		assert.False(t, res)
	})
}

func TestSystemUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("InvalidGroup", func(t *testing.T) {
		s := NewServer(conn())

		_, err := s.Check(ctx, Tuple{set("system", "users", "god"), set("user", "alice", "")})
		assert.ErrorIs(t, err, ErrInvalidSystemGroup)
	})

	t.Run("*", func(t *testing.T) {
		s := NewServer(conn())

		t.Run("Success", func(t *testing.T) {
			tuple := Tuple{set("system", "users", "*"), set("user", "alice", "")}

			require.NoError(t, add(s, ctx, tuple))

			res, err := s.Check(ctx, tuple)
			require.NoError(t, err)
			assert.True(t, res)
		})

		t.Run("FailureOnNotUser", func(t *testing.T) {
			tuple := Tuple{set("system", "users", "*"), set("f", "", "")}

			require.NoError(t, add(s, ctx, tuple))

			res, err := s.Check(ctx, tuple)
			require.NoError(t, err)
			assert.False(t, res)
		})
	})

	t.Run("Authenticated", func(t *testing.T) {
		s := NewServer(conn())

		t.Run("Success", func(t *testing.T) {
			tuple := Tuple{set("system", "users", "authenticated"), set("user", "alice", "")}

			require.NoError(t, add(s, ctx, tuple))

			res, err := s.Check(ctx, tuple)
			require.NoError(t, err)
			assert.True(t, res)
		})

		t.Run("FailureOnEmpty", func(t *testing.T) {
			tuple := Tuple{set("system", "users", "authenticated"), set("user", "", "")}
			tuple.Child.ID = ""

			require.NoError(t, add(s, ctx, tuple))

			res, err := s.Check(ctx, tuple)
			require.NoError(t, err)
			assert.False(t, res)
		})

		t.Run("FailureOnNotUser", func(t *testing.T) {
			tuple := Tuple{set("system", "users", "authenticated"), set("foo", "", "")}

			require.NoError(t, add(s, ctx, tuple))

			res, err := s.Check(ctx, tuple)
			require.NoError(t, err)
			assert.False(t, res)
		})
	})
}

func cleanup(ctx context.Context, conn *pgxpool.Pool) {
	query := `
		delete from tuples;
		delete from caches;
		delete from changes;
	`
	if _, err := conn.Exec(ctx, query); err != nil {
		panic(err)
	}
}

func conn() *pgxpool.Pool {
	ctx := context.Background()
	conn, err := pgxpool.New(ctx, "")
	if err != nil {
		panic(err)
	}
	cleanup(ctx, conn)
	return conn
}

func set(t, i, r string) Set {
	return try(NewSet(t, i, r))
}

func try[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
