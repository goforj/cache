package cache

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type dynStub struct {
	items map[string]map[string]types.AttributeValue
}

func newDynStub() *dynStub { return &dynStub{items: map[string]map[string]types.AttributeValue{}} }

func (d *dynStub) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	key := in.Key["k"].(*types.AttributeValueMemberS).Value
	item, ok := d.items[key]
	if !ok {
		return &dynamodb.GetItemOutput{}, nil
	}
	return &dynamodb.GetItemOutput{Item: item}, nil
}

func (d *dynStub) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
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
	if err := store.Flush(ctx); err != nil {
		t.Fatalf("flush failed: %v", err)
	}
}
