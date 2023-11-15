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

	cfg := Config{
		Types: map[string]Type{
			"user": {},
			"team": {
				Relations: []string{"member"},
			},
		},
	}

	a := Tuple{set("team", "admins", "member"), set("user", "alice", "")}

	t.Run("FailureIfEmpty", func(t *testing.T) {
		s := NewServer(cfg, conn())

		res, err := s.Check(ctx, a)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("Success", func(t *testing.T) {
		s := NewServer(cfg, conn())

		err := s.Write(ctx, []Tuple{a}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, a)
		require.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("Caches", func(t *testing.T) {
		s := NewServer(cfg, conn())

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

func TestPermission(t *testing.T) {
	ctx := context.Background()

	cfg := Config{
		Types: map[string]Type{
			"user": {},
			"post": {
				Relations: []string{"creator", "reader"},
				Permissions: map[string][]string{
					"can_read": {"creator", "reader"},
				},
			},
		},
	}

	creator := Tuple{set("post", "a", "creator"), set("user", "alice", "")}
	reader := Tuple{set("post", "a", "reader"), set("user", "alice", "")}
	canRead := Tuple{set("post", "a", "can_read"), set("user", "alice", "")}

	t.Run("FailureIfEmpty", func(t *testing.T) {
		s := NewServer(cfg, conn())

		res, err := s.Check(ctx, canRead)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("SuccessIfCreator", func(t *testing.T) {
		s := NewServer(cfg, conn())

		err := s.Write(ctx, []Tuple{creator}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, canRead)
		require.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("SuccessIfReader", func(t *testing.T) {
		s := NewServer(cfg, conn())

		err := s.Write(ctx, []Tuple{reader}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, canRead)
		require.NoError(t, err)
		assert.True(t, res)
	})

	t.Run("SuccessIfBoth", func(t *testing.T) {
		s := NewServer(cfg, conn())

		err := s.Write(ctx, []Tuple{reader, creator}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, canRead)
		require.NoError(t, err)
		assert.True(t, res)
	})
}

func TestLabelling(t *testing.T) {
	ctx := context.Background()

	cfg := Config{
		Types: map[string]Type{
			"user": {},
			"post": {
				Relations: []string{"is"},
			},
		},
	}

	public := Tuple{set("post", "a", "is"), set("", "public", "")}

	s := NewServer(cfg, conn())

	err := s.Write(ctx, []Tuple{public}, nil)
	require.NoError(t, err)

	res, err := s.Check(ctx, public)
	require.NoError(t, err)
	assert.True(t, res)
}

func TestGroup(t *testing.T) {
	ctx := context.Background()

	cfg := Config{
		Types: map[string]Type{
			"user": {},
			"group": {
				Relations: []string{"member"},
			},
			"post": {
				Relations: []string{"owner"},
			},
		},
	}

	a := Tuple{set("group", "admins", "member"), set("user", "alice", "")}
	b := Tuple{set("post", "a", "owner"), set("group", "admins", "member")}
	c := Tuple{set("post", "a", "owner"), set("user", "alice", "")}

	t.Run("FailsIfNoTuples", func(t *testing.T) {
		s := NewServer(cfg, conn())

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FailsIfOnlyA", func(t *testing.T) {
		s := NewServer(cfg, conn())

		err := s.Write(ctx, []Tuple{a}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FailsIfOnlyB", func(t *testing.T) {
		s := NewServer(cfg, conn())

		err := s.Write(ctx, []Tuple{b}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("SuccessIfBothTuples", func(t *testing.T) {
		s := NewServer(cfg, conn())

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

	cfg := Config{
		Types: map[string]Type{
			"user": {},
			"group": {
				Relations: []string{"member"},
			},
			"post": {
				Relations: []string{"owner"},
			},
		},
	}

	a := Tuple{set("group", "admins", "member"), set("user", "alice", "")}
	b := Tuple{set("post", "a", "owner"), set("group", "admins", "member")}
	c := Tuple{set("post", "a", "owner"), set("user", "alice", "")}

	t.Run("FailsIfNoTuples", func(t *testing.T) {
		s := NewServer(cfg, conn())

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FailsIfOnlyA", func(t *testing.T) {
		s := NewServer(cfg, conn())

		err := s.Write(ctx, []Tuple{a}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("FailsIfOnlyB", func(t *testing.T) {
		s := NewServer(cfg, conn())

		err := s.Write(ctx, []Tuple{b}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.False(t, res)
	})

	t.Run("SuccessIfBothTuples", func(t *testing.T) {
		s := NewServer(cfg, conn())

		err := s.Write(ctx, []Tuple{a, b}, nil)
		require.NoError(t, err)

		res, err := s.Check(ctx, c)
		require.NoError(t, err)
		assert.True(t, res)
	})
}

func TestSystemUsers(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		Types: map[string]Type{
			"user":  {},
			"group": {},
		},
	}

	t.Run("InvalidGroup", func(t *testing.T) {
		s := NewServer(cfg, conn())

		_, err := s.Check(ctx, Tuple{set("system", "users", "god"), set("user", "alice", "")})
		assert.ErrorIs(t, err, ErrInvalidSystemGroup)
	})

	t.Run("*", func(t *testing.T) {
		s := NewServer(cfg, conn())

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
		s := NewServer(cfg, conn())

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

// func TestLazy(t *testing.T) {
// 	ctx := context.Background()
// 	cfg := Config{
// 		Types: map[string]Type{
// 			"user":  {},
// 			"group": {},
// 		},
// 	}
//
// 	t.Run("False Tuple", func(t *testing.T) {
// 		s := NewServer(cfg, conn())
// 		eval := LazyLib(s)
//
// 		expr := C(O("group", "admins"), "member", O("user", "alice"))
// 		success, err := eval(ctx, expr)
// 		require.NoError(t, err)
// 		require.Equal(t, false, success)
// 	})
//
// 	t.Run("True Tuple", func(t *testing.T) {
// 		s := NewServer(cfg, conn())
// 		eval := LazyLib(s)
//
// 		a := C(O("group", "admins"), "member", O("user", "alice"))
//
// 		err := s.Write(ctx, []Tuple{a}, nil)
// 		require.NoError(t, err)
//
// 		expr := a
// 		success, err := eval(ctx, expr)
// 		require.NoError(t, err)
// 		require.Equal(t, true, success)
// 	})
//
// 	t.Run("And(false, true)", func(t *testing.T) {
// 		s := NewServer(cfg, conn())
// 		eval := LazyLib(s)
//
// 		a := C(O("group", "admins"), "owner", O("user", "alice"))
// 		b := C(O("group", "admins"), "member", O("user", "alice"))
//
// 		err := s.Write(ctx, []Tuple{b}, nil)
// 		require.NoError(t, err)
//
// 		expr := And(a, b)
// 		success, err := eval(ctx, expr)
// 		require.NoError(t, err)
// 		require.Equal(t, false, success)
// 	})
//
// 	t.Run("And(true, true)", func(t *testing.T) {
// 		s := NewServer(cfg, conn())
// 		eval := LazyLib(s)
//
// 		a := C(O("group", "admins"), "owner", O("user", "alice"))
// 		b := C(O("group", "admins"), "member", O("user", "alice"))
//
// 		err := s.Write(ctx, []Tuple{a, b}, nil)
// 		require.NoError(t, err)
//
// 		expr := And(a, b)
// 		success, err := eval(ctx, expr)
// 		require.NoError(t, err)
// 		require.Equal(t, true, success)
// 	})
// }
