//go:build integration

package repositories

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Everything of the deleted user goes, in one transaction, and nothing of anyone else.
func TestEraseUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	repo := NewEraseRepository(store)
	const gone, kept = "00000000-0000-4000-8000-0000000000e1", "00000000-0000-4000-8000-0000000000e2"
	const count = `SELECT (SELECT count(*) FROM notifications WHERE user_id = $1) + (SELECT count(*) FROM alert_preferences WHERE user_id = $1)`
	left := func(id string) int {
		var n int
		require.NoError(t, store.Conn(ctx).QueryRow(ctx, count, id).Scan(&n))
		return n
	}
	for _, id := range []string{gone, kept} {
		for _, q := range []string{
			`INSERT INTO notifications (user_id, type, title) VALUES ($1, 'system', 't')`,
			`INSERT INTO alert_preferences (user_id, kind, email, in_app) VALUES ($1, 'tips', true, false)`,
		} {
			_, err := store.Conn(ctx).Exec(ctx, q, id)
			require.NoError(t, err, q)
		}
	}
	require.Equal(t, 2, left(gone))

	rows, err := repo.EraseUser(ctx, gone)
	require.NoError(t, err)
	assert.Positive(t, rows)
	assert.Zero(t, left(gone))
	assert.Equal(t, 2, left(kept), "another student keeps everything")

	rows, err = repo.EraseUser(ctx, gone)
	require.NoError(t, err, "erasing again is not an error")
	assert.Zero(t, rows)
}
