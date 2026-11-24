//go:build integration

package repositories

import (
	"context"
	"testing"
	"time"

	"9itchi-notification-api/models/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	userB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

func TestNotificationCreateAndList(t *testing.T) {
	repo := NewNotificationRepository(newTestStore(t))
	ctx := context.Background()

	first, err := repo.Create(ctx, &db.Notification{UserID: userA, Type: "system", Title: "one", Data: map[string]any{"k": "v"}})
	require.NoError(t, err)
	assert.Equal(t, "v", first.Data["k"])
	assert.False(t, first.Read)
	time.Sleep(5 * time.Millisecond)
	second, err := repo.Create(ctx, &db.Notification{UserID: userA, Type: "email_opened", Title: "two"})
	require.NoError(t, err)

	items, err := repo.List(ctx, userA, false, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, second.ID, items[0].ID, "newest first")

	page2, err := repo.List(ctx, userA, false, 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)
	assert.Equal(t, first.ID, page2[0].ID)

	_, err = repo.MarkRead(ctx, userA, first.ID)
	require.NoError(t, err)
	unread, err := repo.List(ctx, userA, true, 10, 0)
	require.NoError(t, err)
	require.Len(t, unread, 1)
	assert.Equal(t, second.ID, unread[0].ID)

	total, err := repo.Count(ctx, userA, false)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	unreadCount, err := repo.Count(ctx, userA, true)
	require.NoError(t, err)
	assert.Equal(t, 1, unreadCount)
}

func TestNotificationRejectsUnknownType(t *testing.T) {
	repo := NewNotificationRepository(newTestStore(t))
	_, err := repo.Create(context.Background(), &db.Notification{UserID: userA, Type: "bogus", Title: "x"})
	assert.Error(t, err)
}

func TestNotificationOwnershipIsolation(t *testing.T) {
	repo := NewNotificationRepository(newTestStore(t))
	ctx := context.Background()

	n, err := repo.Create(ctx, &db.Notification{UserID: userA, Type: "system", Title: "private"})
	require.NoError(t, err)

	_, err = repo.MarkRead(ctx, userB, n.ID)
	assert.ErrorIs(t, err, ErrNotFound)

	updated, err := repo.MarkAllRead(ctx, userB)
	require.NoError(t, err)
	assert.EqualValues(t, 0, updated)

	items, err := repo.List(ctx, userB, false, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, items)

	still, err := repo.Count(ctx, userA, true)
	require.NoError(t, err)
	assert.Equal(t, 1, still, "other user's calls must not mark A's rows read")
}

func TestNotificationMarkAllRead(t *testing.T) {
	repo := NewNotificationRepository(newTestStore(t))
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := repo.Create(ctx, &db.Notification{UserID: userA, Type: "system", Title: "t"})
		require.NoError(t, err)
	}
	_, err := repo.Create(ctx, &db.Notification{UserID: userB, Type: "system", Title: "t"})
	require.NoError(t, err)

	updated, err := repo.MarkAllRead(ctx, userA)
	require.NoError(t, err)
	assert.EqualValues(t, 3, updated)

	again, err := repo.MarkAllRead(ctx, userA)
	require.NoError(t, err)
	assert.EqualValues(t, 0, again)

	bUnread, err := repo.Count(ctx, userB, true)
	require.NoError(t, err)
	assert.Equal(t, 1, bUnread)
}

func TestNotificationEventIDIsUnique(t *testing.T) {
	repo := NewNotificationRepository(newTestStore(t))
	ctx := context.Background()
	id := "evt-1"

	first, err := repo.Create(ctx, &db.Notification{UserID: userA, Type: "email_opened", Title: "one", EventID: &id})
	require.NoError(t, err)
	require.NotNil(t, first.EventID)
	assert.Equal(t, id, *first.EventID)

	_, err = repo.Create(ctx, &db.Notification{UserID: userA, Type: "email_opened", Title: "again", EventID: &id})
	assert.ErrorIs(t, err, ErrConflict, "the same event makes one notification")

	// Notifications sent through the API have no event id and are never taken for each other.
	_, err = repo.Create(ctx, &db.Notification{UserID: userA, Type: "system", Title: "a"})
	require.NoError(t, err)
	_, err = repo.Create(ctx, &db.Notification{UserID: userA, Type: "system", Title: "b"})
	require.NoError(t, err)
}
