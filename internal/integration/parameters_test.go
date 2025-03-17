package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-faster/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/ogen-go/ogen/internal/integration/test_parameters"
)

type testParameters struct{}

var _ api.Handler = (*testParameters)(nil)

func (s *testParameters) OptionalParameters(ctx context.Context, params api.OptionalParametersParams) (*api.OptionalQueryParametersResponse, error) {
	return nil, nil
}

func (s *testParameters) OptionalArrayParameter(ctx context.Context, params api.OptionalArrayParameterParams) (string, error) {
	return "", nil
}

func (s *testParameters) ObjectQueryParameter(ctx context.Context, params api.ObjectQueryParameterParams) (*api.ObjectQueryParameterOK, error) {
	if param, ok := params.FormObject.Get(); ok {
		return &api.ObjectQueryParameterOK{
			Style: "form",
			Value: param,
		}, nil
	}
	if param, ok := params.DeepObject.Get(); ok {
		return &api.ObjectQueryParameterOK{
			Style: "deepObject",
			Value: param,
		}, nil
	}
	return &api.ObjectQueryParameterOK{}, errors.New("invalid input")
}

func (s *testParameters) ObjectCookieParameter(ctx context.Context, params api.ObjectCookieParameterParams) (*api.OneLevelObject, error) {
	return &params.Value, nil
}

func (s *testParameters) ContentParameters(ctx context.Context, params api.ContentParametersParams) (*api.ContentParameters, error) {
	return &api.ContentParameters{
		Query:  params.Query,
		Path:   params.Path,
		Header: params.XHeader,
		Cookie: params.Cookie,
	}, nil
}

func (s *testParameters) PathParameter(ctx context.Context, params api.PathParameterParams) (*api.Value, error) {
	return &api.Value{
		Value: params.Value,
	}, nil
}

func (s *testParameters) HeaderParameter(ctx context.Context, params api.HeaderParameterParams) (*api.Value, error) {
	return &api.Value{
		Value: params.XValue,
	}, nil
}

func (s *testParameters) CookieParameter(ctx context.Context, params api.CookieParameterParams) (*api.Value, error) {
	return &api.Value{
		Value: params.Value,
	}, nil
}

func (s *testParameters) ComplicatedParameterNameGet(ctx context.Context, params api.ComplicatedParameterNameGetParams) error {
	panic("implement me")
}

func (s *testParameters) SameName(ctx context.Context, params api.SameNameParams) error {
	panic("implement me")
}

func (s *testParameters) SimilarNames(ctx context.Context, params api.SimilarNamesParams) error {
	panic("implement me")
}

func TestParameters(t *testing.T) {
	ctx := context.Background()

	h, err := api.NewServer(&testParameters{})
	require.NoError(t, err)

	s := httptest.NewServer(h)
	defer s.Close()

	client, err := api.NewClient(s.URL, api.WithClient(s.Client()))
	require.NoError(t, err)

	oneLevel := api.OneLevelObject{
		Min:    1,
		Max:    5,
		Filter: "abc",
	}

	t.Run("ObjectQueryParameter", func(t *testing.T) {
		t.Run("formStyle", func(t *testing.T) {
			resp, err := client.ObjectQueryParameter(ctx, api.ObjectQueryParameterParams{
				FormObject: api.NewOptOneLevelObject(oneLevel),
			})
			require.NoError(t, err)
			require.Equal(t, resp.Style, "form")
			require.Equal(t, oneLevel, resp.Value)
		})
		t.Run("deepObjectStyle", func(t *testing.T) {
			resp, err := client.ObjectQueryParameter(ctx, api.ObjectQueryParameterParams{
				DeepObject: api.NewOptOneLevelObject(oneLevel),
			})
			require.NoError(t, err)
			require.Equal(t, resp.Style, "deepObject")
			require.Equal(t, oneLevel, resp.Value)
		})
	})
	t.Run("ObjectCookieParameter", func(t *testing.T) {
		resp, err := client.ObjectCookieParameter(ctx, api.ObjectCookieParameterParams{Value: oneLevel})
		require.NoError(t, err)
		require.Equal(t, oneLevel, *resp)
	})

	const plainParam = "`\"';,./<>?[]{}\\|~!@#$%^&*()_+-="
	for i, param := range []string{
		"%",
		"/",
		"&",
		"/%",
		plainParam,
	} {
		param := param
		t.Run(fmt.Sprintf("Test%d", i+1), func(t *testing.T) {
			t.Run("PathParameter", func(t *testing.T) {
				h, err := client.PathParameter(ctx, api.PathParameterParams{Value: param})
				require.NoError(t, err)
				assert.Equal(t, param, h.Value)
			})
			t.Run("HeaderParameter", func(t *testing.T) {
				h, err := client.HeaderParameter(ctx, api.HeaderParameterParams{XValue: param})
				require.NoError(t, err)
				assert.Equal(t, param, h.Value)
			})
			t.Run("CookieParameter", func(t *testing.T) {
				h, err := client.CookieParameter(ctx, api.CookieParameterParams{Value: param})
				require.NoError(t, err)
				assert.Equal(t, param, h.Value)
			})
		})
	}

	t.Run("ContentParameters", func(t *testing.T) {
		user := api.User{
			ID:       1,
			Username: "admin",
			Role:     api.UserRoleAdmin,
			Friends: []api.User{
				{
					ID:       2,
					Username: "alice",
					Role:     api.UserRoleUser,
				},
				{
					ID:       3,
					Username: plainParam,
					Role:     api.UserRoleBot,
				},
			},
		}
		resp, err := client.ContentParameters(ctx, api.ContentParametersParams{
			Query:   user,
			Path:    user,
			XHeader: user,
			Cookie:  user,
		})
		require.NoError(t, err)
		assert.Equal(t, user, resp.Query)
		assert.Equal(t, user, resp.Path)
		assert.Equal(t, user, resp.Header)
		assert.Equal(t, user, resp.Cookie)
	})
}

func TestOptionalArrayParameter(t *testing.T) {
	ctx := context.Background()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ensure that client does not send these query parameters and headers.
		if q := r.URL.Query(); q.Has("query") {
			http.Error(w, "must have not query", http.StatusBadRequest)
			return
		}
		if h := r.Header; len(h.Values("header")) > 0 {
			http.Error(w, "must have not header", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = io.WriteString(w, `"ok"`)
	}))
	defer s.Close()

	client, err := api.NewClient(s.URL, api.WithClient(s.Client()))
	require.NoError(t, err)

	resp, err := client.OptionalArrayParameter(ctx, api.OptionalArrayParameterParams{})
	require.NoError(t, err)
	require.Equal(t, "ok", resp)
}
