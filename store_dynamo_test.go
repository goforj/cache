package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type dynStub struct {
	items   map[string]map[string]types.AttributeValue
	exists  bool
	putErr  error
	scanErr error
	getErr  error
}

func newDynStub() *dynStub { return &dynStub{items: map[string]map[string]types.AttributeValue{}} }

func (d *dynStub) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if d.getErr != nil {
		return nil, d.getErr
	}
	key := in.Key["k"].(*types.AttributeValueMemberS).Value
	item, ok := d.items[key]
	if !ok {
		return &dynamodb.GetItemOutput{}, nil
	}
	return &dynamodb.GetItemOutput{Item: item}, nil
}

func (d *dynStub) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	if d.putErr != nil {
		return nil, d.putErr
	}
	key := in.Item["k"].(*types.AttributeValueMemberS).Value
	if in.ConditionExpression != nil {
		if _, exists := d.items[key]; exists {
			return nil, &types.ConditionalCheckFailedException{}
		}
	}
	d.items[key] = in.Item
	return &dynamodb.PutItemOutput{}, nil
}

func (d *dynStub) DeleteItem(_ context.Context, in *dynamodb.DeleteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	key := in.Key["k"].(*types.AttributeValueMemberS).Value
	delete(d.items, key)
	return &dynamodb.DeleteItemOutput{}, nil
}

func (d *dynStub) BatchWriteItem(_ context.Context, in *dynamodb.BatchWriteItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	for _, writes := range in.RequestItems {
		for _, wr := range writes {
			if dr := wr.DeleteRequest; dr != nil {
				key := dr.Key["k"].(*types.AttributeValueMemberS).Value
				delete(d.items, key)
			}
		}
	}
	return &dynamodb.BatchWriteItemOutput{}, nil
}

func (d *dynStub) Scan(_ context.Context, in *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	if d.scanErr != nil {
		return nil, d.scanErr
	}
	var items []map[string]types.AttributeValue
	for k := range d.items {
		items = append(items, map[string]types.AttributeValue{
			"k": &types.AttributeValueMemberS{Value: k},
		})
	}
	return &dynamodb.ScanOutput{Items: items}, nil
}

func (d *dynStub) CreateTable(context.Context, *dynamodb.CreateTableInput, ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error) {
	return &dynamodb.CreateTableOutput{}, nil
}

func (d *dynStub) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if d.exists {
		return &dynamodb.DescribeTableOutput{}, nil
	}
	return nil, &types.ResourceNotFoundException{}
}

func TestDynamoStoreBasicOperations(t *testing.T) {
	stub := newDynStub()
	store, err := newDynamoStore(context.Background(), StoreConfig{
		DynamoClient: stub,
		DynamoTable:  "tbl",
		Prefix:       "p",
		DefaultTTL:   time.Minute,
	})
	if err != nil {
		t.Fatalf("store create failed: %v", err)
	}

	ctx := context.Background()
	if err := store.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	body, ok, err := store.Get(ctx, "k")
	if err != nil || !ok || string(body) != "v" {
		t.Fatalf("get failed: ok=%v err=%v val=%s", ok, err, string(body))
	}

	if created, err := store.Add(ctx, "k", []byte("v2"), time.Minute); err != nil || created {
		t.Fatalf("add should fail existing: created=%v err=%v", created, err)
	}

	if val, err := store.Increment(ctx, "n", 2, time.Minute); err != nil || val != 2 {
		t.Fatalf("increment failed: %v val=%d", err, val)
	}

	if err := store.Delete(ctx, "k"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := store.Decrement(ctx, "n", 1, time.Minute); err != nil {
		t.Fatalf("decrement failed: %v", err)
	}
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
}

func TestDynamoEnsureTableCreatesWhenMissing(t *testing.T) {
	stub := newDynStub()
	if err := ensureDynamoTable(context.Background(), stub, "tbl"); err != nil {
		t.Fatalf("ensure table failed: %v", err)
	}
}

func TestDynamoEnsureTableExistsPath(t *testing.T) {
	stub := newDynStub()
	stub.exists = true
	if err := ensureDynamoTable(context.Background(), stub, "tbl"); err != nil {
		t.Fatalf("ensure table exists path failed: %v", err)
	}
}

func TestNewDynamoStoreDefaultsTTL(t *testing.T) {
	stub := newDynStub()
	store, err := newDynamoStore(context.Background(), StoreConfig{
		DynamoClient: stub,
		DynamoTable:  "tbl",
		Prefix:       "p",
		DefaultTTL:   0,
	})
	if err != nil {
		t.Fatalf("expected store: %v", err)
	}
	ds := store.(*dynamoStore)
	if ds.defaultTTL != defaultCacheTTL {
		t.Fatalf("expected default ttl fallback, got %v", ds.defaultTTL)
	}
	if ds.cacheKey("k") != "p:k" {
		t.Fatalf("unexpected cache key")
	}
}

func TestNewDynamoClientBuilds(t *testing.T) {
	client, err := newDynamoClient(context.Background(), StoreConfig{
		DynamoRegion:   "us-east-1",
		DynamoEndpoint: "http://localhost:8000",
	})
	if err != nil {
		t.Fatalf("expected client build: %v", err)
	}
	if client == nil {
		t.Fatalf("client nil")
	}
}

func TestDynamoGetExpiredRemoves(t *testing.T) {
	stub := newDynStub()
	store, err := newDynamoStore(context.Background(), StoreConfig{
		DynamoClient: stub,
		DynamoTable:  "tbl",
		Prefix:       "p",
		DefaultTTL:   time.Minute,
	})
	if err != nil {
		t.Fatalf("store create failed: %v", err)
	}
	expired := map[string]types.AttributeValue{
		"k":  &types.AttributeValueMemberS{Value: "p:gone"},
		"v":  &types.AttributeValueMemberB{Value: []byte("x")},
		"ea": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(-time.Hour).UnixMilli())},
	}
	stub.items["p:gone"] = expired
	if _, ok, err := store.Get(context.Background(), "gone"); err != nil || ok {
		t.Fatalf("expected expired miss")
	}
	if _, exists := stub.items["p:gone"]; exists {
		t.Fatalf("expected expired item removed")
	}
}

func TestDynamoGetNonBinaryValue(t *testing.T) {
	stub := newDynStub()
	stub.items["p:weird"] = map[string]types.AttributeValue{
		"k":  &types.AttributeValueMemberS{Value: "p:weird"},
		"v":  &types.AttributeValueMemberS{Value: "not-binary"},
		"ea": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(time.Hour).UnixMilli())},
	}
	store, err := newDynamoStore(context.Background(), StoreConfig{
		DynamoClient: stub,
		DynamoTable:  "tbl",
		Prefix:       "p",
		DefaultTTL:   time.Minute,
	})
	if err != nil {
		t.Fatalf("store create failed: %v", err)
	}
	if _, _, err := store.Get(context.Background(), "weird"); err == nil {
		t.Fatalf("expected type error")
	}
}

func TestDynamoDeleteManyEmpty(t *testing.T) {
	store := &dynamoStore{client: newDynStub(), table: "tbl"}
	if err := store.DeleteMany(context.Background()); err != nil {
		t.Fatalf("delete many empty should be nil: %v", err)
	}
}

func TestDynamoCacheKeyEmptyPrefix(t *testing.T) {
	ds := &dynamoStore{prefix: ""}
	if ds.cacheKey("k") != "k" {
		t.Fatalf("expected raw key")
	}
}

func TestDynamoFlushRemovesPrefixedKeys(t *testing.T) {
	stub := newDynStub()
	stub.items["p:a"] = map[string]types.AttributeValue{
		"k": &types.AttributeValueMemberS{Value: "p:a"},
	}
	stub.items["p:b"] = map[string]types.AttributeValue{
		"k": &types.AttributeValueMemberS{Value: "p:b"},
	}
	store := &dynamoStore{
		client: stub,
		table:  "tbl",
		prefix: "p",
	}
	if err := store.Flush(context.Background()); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
	if len(stub.items) != 0 {
		t.Fatalf("expected items cleared, got %d", len(stub.items))
	}
}

func TestDynamoAddErrorPath(t *testing.T) {
	stub := newDynStub()
	stub.putErr = errors.New("put boom")
	store := &dynamoStore{
		client:     stub,
		table:      "tbl",
		prefix:     "p",
		defaultTTL: time.Second,
	}
	if _, err := store.Add(context.Background(), "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected add error")
	}
}

func TestDynamoFlushScanError(t *testing.T) {
	stub := newDynStub()
	stub.scanErr = errors.New("scan boom")
	store := &dynamoStore{
		client: stub,
		table:  "tbl",
	}
	if err := store.Flush(context.Background()); err == nil {
		t.Fatalf("expected scan error")
	}
}

func TestDynamoSetAndAddDefaultTTL(t *testing.T) {
	stub := newDynStub()
	store := &dynamoStore{
		client:     stub,
		table:      "tbl",
		prefix:     "",
		defaultTTL: time.Second,
	}
	if err := store.Set(context.Background(), "k", []byte("v"), 0); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if _, err := store.Add(context.Background(), "k", []byte("v2"), 0); err != nil {
		t.Fatalf("add failed: %v", err)
	}
}

func TestDynamoIncrementNegativeUsesDecrement(t *testing.T) {
	stub := newDynStub()
	store := &dynamoStore{
		client:     stub,
		table:      "tbl",
		prefix:     "",
		defaultTTL: time.Second,
	}
	if _, err := store.Increment(context.Background(), "n", -1, time.Second); err != nil {
		t.Fatalf("increment negative failed: %v", err)
	}
}

func TestDynamoIncrementNonNumeric(t *testing.T) {
	stub := newDynStub()
	stub.items["p:num"] = map[string]types.AttributeValue{
		"k":  &types.AttributeValueMemberS{Value: "p:num"},
		"v":  &types.AttributeValueMemberB{Value: []byte("NaN")},
		"ea": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(time.Hour).UnixMilli())},
	}
	store := &dynamoStore{
		client:     stub,
		table:      "tbl",
		prefix:     "p",
		defaultTTL: time.Second,
	}
	if _, err := store.Increment(context.Background(), "num", 1, time.Second); err == nil {
		t.Fatalf("expected non-numeric error")
	}
}

func TestDynamoSetErrorPath(t *testing.T) {
	stub := newDynStub()
	stub.putErr = errors.New("put fail")
	store := &dynamoStore{
		client:     stub,
		table:      "tbl",
		prefix:     "",
		defaultTTL: time.Second,
	}
	if err := store.Set(context.Background(), "k", []byte("v"), time.Second); err == nil {
		t.Fatalf("expected set error")
	}
}

func TestDynamoIncrementGetError(t *testing.T) {
	stub := newDynStub()
	stub.getErr = errors.New("get fail")
	store := &dynamoStore{
		client:     stub,
		table:      "tbl",
		prefix:     "",
		defaultTTL: time.Second,
	}
	if _, err := store.Increment(context.Background(), "k", 1, time.Second); err == nil {
		t.Fatalf("expected get error")
	}
}
