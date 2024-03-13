package api_test

import (
	"context"
	dkvv1 "distributed-kv/gen/dkv/v1"
	"distributed-kv/gen/dkv/v1/dkvv1connect"
	"distributed-kv/internal/api"
	"distributed-kv/mocks/mockstore"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

func TestDkvAPIHandler(t *testing.T) {
	t.Parallel()

	store := mockstore.NewStore(t)
	svc := api.NewDkvAPIHandler(store)
	path, h := dkvv1connect.NewDkvAPIHandler(svc)
	mux := http.NewServeMux()
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := dkvv1connect.NewDkvAPIClient(srv.Client(), srv.URL)

	t.Run("Set", func(t *testing.T) {
		// Arrange
		store.EXPECT().Set("key", "value").Return(nil)

		// Act
		_, err := client.Set(context.Background(), &connect.Request[dkvv1.SetRequest]{
			Msg: &dkvv1.SetRequest{
				Key:   "key",
				Value: "value",
			},
		})

		// Assert
		require.NoError(t, err)
	})

	t.Run("Get", func(t *testing.T) {
		// Arrange
		store.EXPECT().Get("key").Return("value", nil)

		// Act
		res, err := client.Get(context.Background(), &connect.Request[dkvv1.GetRequest]{
			Msg: &dkvv1.GetRequest{
				Key: "key",
			},
		})

		// Assert
		require.NoError(t, err)
		require.Equal(t, "value", res.Msg.Value)
	})

	t.Run("Delete", func(t *testing.T) {
		// Arrange
		store.EXPECT().Delete("key").Return(nil)

		// Act
		_, err := client.Delete(context.Background(), &connect.Request[dkvv1.DeleteRequest]{
			Msg: &dkvv1.DeleteRequest{
				Key: "key",
			},
		})

		// Assert
		require.NoError(t, err)
	})
}
