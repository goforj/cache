package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

type failingDynamo struct{}

func (f failingDynamo) GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return nil, errors.New("boom")
}
func (f failingDynamo) PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	return nil, errors.New("boom")
}
func (f failingDynamo) DeleteItem(context.Context, *dynamodb.DeleteItemInput, ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	return nil, errors.New("boom")
}
func (f failingDynamo) BatchWriteItem(context.Context, *dynamodb.BatchWriteItemInput, ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error) {
	return nil, errors.New("boom")
}
func (f failingDynamo) Scan(context.Context, *dynamodb.ScanInput, ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	return nil, errors.New("boom")
}
func (f failingDynamo) CreateTable(context.Context, *dynamodb.CreateTableInput, ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error) {
	return nil, errors.New("boom")
}
func (f failingDynamo) DescribeTable(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	return nil, errors.New("boom")
}

func TestNewStoreDynamoErrorReturnsErrorStore(t *testing.T) {
	store := NewStore(context.Background(), StoreConfig{
		Driver:       DriverDynamo,
		DynamoClient: failingDynamo{},
		DynamoTable:  "tbl",
	})
	if store.Driver() != DriverDynamo {
		t.Fatalf("expected dynamo driver")
	}
	if _, _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatalf("expected propagated error")
	}
}

func TestNewStoreSQLMissingConfigReturnsErrorStore(t *testing.T) {
	store := NewStore(context.Background(), StoreConfig{
		Driver: DriverSQL,
	})
	if store.Driver() != DriverSQL {
		t.Fatalf("expected sql driver")
	}
	if _, _, err := store.Get(context.Background(), "k"); err == nil {
		t.Fatalf("expected error")
	}
}
