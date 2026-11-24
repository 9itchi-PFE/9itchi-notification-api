package repositories

import (
	"context"

	"9itchi-notification-api/models/db"

	"github.com/jackc/pgx/v5"
)

// PreferenceRepository is the only layer that touches PostgreSQL for alert preferences.
type PreferenceRepository interface {
	// ListByUser returns the kinds the user chose for; a kind without a row keeps its default.
	ListByUser(ctx context.Context, userID string) ([]db.AlertPreference, error)
	// Get returns the choice of userID for kind (ErrNotFound when they never chose).
	Get(ctx context.Context, userID, kind string) (*db.AlertPreference, error)
	// Upsert stores the given choices in one statement; the other kinds are left as they are.
	Upsert(ctx context.Context, userID string, prefs []db.AlertPreference) error
}

type preferenceRepository struct {
	store *Store
}

func NewPreferenceRepository(store *Store) PreferenceRepository {
	return &preferenceRepository{store: store}
}

const preferenceColumns = `user_id, kind, email, in_app, updated_at`

func (r *preferenceRepository) ListByUser(ctx context.Context, userID string) ([]db.AlertPreference, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+preferenceColumns+` FROM alert_preferences WHERE user_id = $1`, userID)
	if err != nil {
		return nil, MapError(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.AlertPreference])
	return items, MapError(err)
}

func (r *preferenceRepository) Get(ctx context.Context, userID, kind string) (*db.AlertPreference, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `SELECT `+preferenceColumns+` FROM alert_preferences WHERE user_id = $1 AND kind = $2`, userID, kind)
	if err != nil {
		return nil, MapError(err)
	}
	out, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.AlertPreference])
	return out, MapError(err)
}

func (r *preferenceRepository) Upsert(ctx context.Context, userID string, prefs []db.AlertPreference) error {
	if len(prefs) == 0 {
		return nil
	}
	kinds := make([]string, len(prefs))
	emails := make([]bool, len(prefs))
	inApps := make([]bool, len(prefs))
	for i, p := range prefs {
		kinds[i], emails[i], inApps[i] = p.Kind, p.Email, p.InApp
	}
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx, `
		INSERT INTO alert_preferences (user_id, kind, email, in_app)
		SELECT $1, k, e, i FROM unnest($2::text[], $3::boolean[], $4::boolean[]) AS t(k, e, i)
		ON CONFLICT (user_id, kind) DO UPDATE SET email = EXCLUDED.email, in_app = EXCLUDED.in_app, updated_at = now()`,
		userID, kinds, emails, inApps)
	return MapError(err)
}
