package account

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/internal/users/abc", r.URL.Path)
		assert.Equal(t, "k", r.Header.Get("x-api-key"))
		_, _ = w.Write([]byte(`{"id":"abc","email":"a@b.c"}`))
	}))
	defer srv.Close()

	u, err := New(srv.URL+"/", "k").GetUser(context.Background(), "abc")
	require.NoError(t, err)
	assert.Equal(t, "a@b.c", u.Email)
}

func TestGetUserNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	u, err := New(srv.URL, "k").GetUser(context.Background(), "abc")
	assert.Nil(t, u)
	assert.ErrorContains(t, err, "404")
}
