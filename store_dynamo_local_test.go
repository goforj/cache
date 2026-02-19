//go:build integration

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func TestDynamoStoreLocalIntegration(t *testing.T) {
	endpoint := integrationAddr("dynamodb")
	if endpoint == "" {
		t.Skip("dynamodb integration not started")
	}
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy", "")),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: endpoint, HostnameImmutable: true}, nil
		})),
	)
	if err != nil {
		t.Fatalf("aws cfg: %v", err)
	}
	client := dynamodb.NewFromConfig(awsCfg)
	store := NewStore(context.Background(), StoreConfig{
		Driver:       DriverDynamo,
		DynamoClient: client,
		DynamoTable:  "cache_entries",
		Prefix:       "itest",
		DefaultTTL:   time.Second,
	})

	runStoreContractSuite(t, store)
}
