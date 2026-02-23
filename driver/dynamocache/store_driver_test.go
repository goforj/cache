package dynamocache

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/goforj/cache/cachecore"
)

func TestDynamoDriverMethods(t *testing.T) {
	ctx := context.Background()
	stub := newDynStub()
	store, err := New(ctx, Config{
		BaseConfig: cachecore.BaseConfig{Prefix: "p", DefaultTTL: time.Second},
		Client:     stub,
		Table:      "tbl",
	})
	if err != nil {
		t.Fatalf("store create failed: %v", err)
	}
	if store.Driver() != cachecore.DriverDynamo {
		t.Fatalf("expected driver dynamodb")
	}
	// Get miss path
	if _, ok, err := store.Get(ctx, "missing"); err != nil || ok {
		t.Fatalf("expected miss; ok=%v err=%v", ok, err)
	}

	// cacheKey and expired helpers
	ds := store.(*dynamoStore)
	if ds.cacheKey("k") != "p:k" {
		t.Fatalf("cacheKey prefix mismatch")
	}
	item := map[string]types.AttributeValue{
		"ea": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(-time.Hour).UnixMilli())},
	}
	if !expired(item) {
		t.Fatalf("expected expired")
	}
}

func TestNewDynamoClientError(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	emptyConfig := createEmptyTempFile(t)
	t.Setenv("AWS_CONFIG_FILE", emptyConfig)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", emptyConfig)
	t.Setenv("AWS_PROFILE", "missing-profile")

	if _, err := newDynamoClient(context.Background(), Config{Region: ""}); err == nil {
		t.Fatalf("expected error when config cannot provide region")
	}
}

func createEmptyTempFile(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "empty-aws-config-*.txt")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestExpiredHelper(t *testing.T) {
	if expired(map[string]types.AttributeValue{}) {
		t.Fatalf("missing attribute should not expire")
	}
	if expired(map[string]types.AttributeValue{"ea": &types.AttributeValueMemberN{Value: "bad"}}) {
		t.Fatalf("parse error should not expire")
	}
}

func TestNewDynamoClientWithoutEndpoint(t *testing.T) {
	client, err := newDynamoClient(context.Background(), Config{Region: "us-east-1"})
	if err != nil {
		t.Fatalf("expected client without endpoint: %v", err)
	}
	if client == nil {
		t.Fatalf("client nil")
	}
}
