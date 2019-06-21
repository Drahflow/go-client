package ldclient_test

import (
	"testing"

	ld "github.com/drahflow/go-client"
	ldtest "github.com/drahflow/go-client/shared_test"
)

func makeInMemoryStore() (ld.FeatureStore, error) {
	return ld.NewInMemoryFeatureStore(nil), nil
}

func TestInMemoryFeatureStore(t *testing.T) {
	ldtest.RunFeatureStoreTests(t, makeInMemoryStore, nil, false)
}
