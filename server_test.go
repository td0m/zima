package zima

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func try[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}

func set(t, i, r string) Set {
	return try(NewSet(t, i, r))
}

func cleanup(ctx context.Context, conn *pgxpool.Pool) {
	query := `
		delete from tuples;
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

		err := s.Write(ctx, []Tuple{a}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, a)
		require.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("Caches", func(t *testing.T) {
		s := NewServer(conn())

		err := s.Write(ctx, []Tuple{a}, nil)
		require.NoError(t, err)

		t.Run("NoCacheFirst", func(t *testing.T) {
			res, err := s.Check(ctx, a)
			require.NoError(t, err)
			assert.True(t, res)
		})

		t.Run("FailureAfterRevoke", func(t *testing.T) {
			err := s.Write(ctx, nil, []Tuple{a})
			require.NoError(t, err)

			res, err := s.Check(ctx, a)
			require.NoError(t, err)
			assert.False(t, res)
		})
	})
}

func TestLabelling(t *testing.T) {
	ctx := context.Background()

	public := Tuple{set("post", "a", "is"), set("", "public", "")}

	s := NewServer(conn())

	err := s.Write(ctx, []Tuple{public}, nil)
	require.NoError(t, err)

	res, err := s.Check(ctx, public)
	require.NoError(t, err)
	assert.True(t, res)
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

		err := s.Write(ctx, []Tuple{a}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FailsIfOnlyB", func(t *testing.T) {
		s := NewServer(conn())

		err := s.Write(ctx, []Tuple{b}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("SuccessIfBothTuples", func(t *testing.T) {
		s := NewServer(conn())

		err := s.Write(ctx, []Tuple{a, b}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.True(t, res)
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

		err := s.Write(ctx, []Tuple{a}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FailsIfOnlyB", func(t *testing.T) {
		s := NewServer(conn())

		err := s.Write(ctx, []Tuple{b}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("SuccessIfBothTuples", func(t *testing.T) {
		s := NewServer(conn())

		err := s.Write(ctx, []Tuple{a, b}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.True(t, res)
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

			err := s.Write(ctx, []Tuple{tuple}, nil)
			require.NoError(t, err)

			res, err := s.Check(ctx, tuple)
			require.NoError(t, err)
			assert.True(t, res)
		})

		t.Run("FailureOnNotUser", func(t *testing.T) {
			tuple := Tuple{set("system", "users", "*"), set("f", "", "")}

			err := s.Write(ctx, []Tuple{tuple}, nil)
			require.NoError(t, err)

			res, err := s.Check(ctx, tuple)
			require.NoError(t, err)
			assert.False(t, res)
		})
	})

	t.Run("Authenticated", func(t *testing.T) {
		s := NewServer(conn())

		t.Run("Success", func(t *testing.T) {
			tuple := Tuple{set("system", "users", "authenticated"), set("user", "alice", "")}

			err := s.Write(ctx, []Tuple{tuple}, nil)
			require.NoError(t, err)

			res, err := s.Check(ctx, tuple)
			require.NoError(t, err)
			assert.True(t, res)
		})

		t.Run("FailureOnEmpty", func(t *testing.T) {
			tuple := Tuple{set("system", "users", "authenticated"), set("user", "", "")}
			tuple.Child.ID = ""

			err := s.Write(ctx, []Tuple{tuple}, nil)
			require.NoError(t, err)

			res, err := s.Check(ctx, tuple)
			require.NoError(t, err)
			assert.False(t, res)
		})

		t.Run("FailureOnNotUser", func(t *testing.T) {
			tuple := Tuple{set("system", "users", "authenticated"), set("foo", "", "")}

			err := s.Write(ctx, []Tuple{tuple}, nil)
			require.NoError(t, err)

			res, err := s.Check(ctx, tuple)
			require.NoError(t, err)
			assert.False(t, res)
		})
	})
}
