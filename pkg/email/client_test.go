package email

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "key-1", r.Header.Get("x-api-key"))
		switch r.URL.Path {
		case "/internal/emails/e1/context":
			_, _ = w.Write([]byte(`{"email_id":"e1","user_id":"u1","application_id":"a1","job_id":"j1","job_title":"Portail RH","company":"Acme"}`))
		case "/internal/emails/none/context":
			w.WriteHeader(http.StatusNotFound)
		case "/internal/emails/bad/context":
			_, _ = w.Write([]byte(`{`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	c := New(srv.URL+"/", "key-1")

	got, err := c.GetContext(context.Background(), "e1")
	require.NoError(t, err)
	assert.Equal(t, &Context{EmailID: "e1", UserID: "u1", ApplicationID: "a1", JobID: "j1", JobTitle: "Portail RH", Company: "Acme"}, got)

	_, err = c.GetContext(context.Background(), "none")
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = c.GetContext(context.Background(), "bad")
	assert.Error(t, err)
	_, err = c.GetContext(context.Background(), "boom")
	assert.ErrorContains(t, err, "status 500")

	srv.Close()
	_, err = c.GetContext(context.Background(), "e1")
	assert.Error(t, err, "peer unreachable")
}
