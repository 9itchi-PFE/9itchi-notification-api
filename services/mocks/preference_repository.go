package mocks

import (
	"context"

	"9itchi-notification-api/models/db"
	"9itchi-notification-api/repositories"
)

// PreferenceRepository is an in-memory services/repositories.PreferenceRepository. Err makes every call fail.
type PreferenceRepository struct {
	Rows map[string]db.AlertPreference // by kind, for one user
	Err  error
}

func (m *PreferenceRepository) ListByUser(_ context.Context, _ string) ([]db.AlertPreference, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	out := make([]db.AlertPreference, 0, len(m.Rows))
	for _, r := range m.Rows {
		out = append(out, r)
	}
	return out, nil
}

func (m *PreferenceRepository) Get(_ context.Context, _, kind string) (*db.AlertPreference, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	r, ok := m.Rows[kind]
	if !ok {
		return nil, repositories.ErrNotFound
	}
	return &r, nil
}

func (m *PreferenceRepository) Upsert(_ context.Context, _ string, prefs []db.AlertPreference) error {
	if m.Err != nil {
		return m.Err
	}
	if m.Rows == nil {
		m.Rows = map[string]db.AlertPreference{}
	}
	for _, p := range prefs {
		m.Rows[p.Kind] = p
	}
	return nil
}
