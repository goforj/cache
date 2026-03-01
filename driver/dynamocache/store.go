package dynamocache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/goforj/cache/cachecore"
)

const (
	defaultTTL    = 5 * time.Minute
	defaultPrefix = "app"
	defaultRegion = "us-east-1"
	defaultTable  = "cache_entries"
)

// Config configures a DynamoDB-backed cache store.
type Config struct {
	cachecore.BaseConfig
	Client   DynamoAPI
	Endpoint string
	Region   string
	Table    string
}

// DynamoAPI captures the subset of DynamoDB client methods used by the store.
type DynamoAPI interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error)
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

type dynamoStore struct {
	client     DynamoAPI
	table      string
	prefix     string
	defaultTTL time.Duration
}

const (
	dynamoEnsureTableMaxAttempts = 20
	dynamoEnsureTableRetryDelay  = 150 * time.Millisecond
)

// New builds a DynamoDB-backed cachecore.Store.
//
// Defaults:
// - Region: "us-east-1" when empty
// - Table: "cache_entries" when empty
// - DefaultTTL: 5*time.Minute when zero
// - Prefix: "app" when empty
// - Client: auto-created when nil (uses Region and optional Endpoint)
// - Endpoint: empty by default (normal AWS endpoint resolution)
//
// Example: custom dynamo table via explicit driver config
//
//	ctx := context.Background()
//	store, err := dynamocache.New(ctx, dynamocache.Config{
//		BaseConfig: cachecore.BaseConfig{
//			DefaultTTL: 5 * time.Minute,
//			Prefix:     "app",
//		},
//		Region: "us-east-1",
//		Table:  "cache_entries",
//	})
//	if err != nil {
//		panic(err)
//	}
//	fmt.Println(store.Driver()) // dynamo
func New(ctx context.Context, cfg Config) (cachecore.Store, error) {
	if cfg.Region == "" {
		cfg.Region = defaultRegion
	}
	if cfg.Table == "" {
		cfg.Table = defaultTable
	}
	if cfg.Prefix == "" {
		cfg.Prefix = defaultPrefix
	}
	if cfg.Client == nil {
		client, err := newDynamoClient(ctx, cfg)
		if err != nil {
			return nil, err
		}
		cfg.Client = client
	}
	if err := ensureDynamoTable(ctx, cfg.Client, cfg.Table); err != nil {
		return nil, err
	}
	ttl := cfg.DefaultTTL
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &dynamoStore{
		client:     cfg.Client,
		table:      cfg.Table,
		prefix:     cfg.Prefix,
		defaultTTL: ttl,
	}, nil
}

func newDynamoClient(ctx context.Context, cfg Config) (*dynamodb.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy", "")),
	)
	if err != nil {
		return nil, err
	}
	if cfg.Endpoint != "" {
		resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: cfg.Endpoint, HostnameImmutable: true}, nil
		})
		if _, err := resolver.ResolveEndpoint("dynamodb", cfg.Region); err != nil {
			return nil, err
		}
		awsCfg.EndpointResolverWithOptions = resolver
	}
	return dynamodb.NewFromConfig(awsCfg), nil
}

func (s *dynamoStore) Driver() cachecore.Driver { return cachecore.DriverDynamo }

func (s *dynamoStore) Ready(ctx context.Context) error {
	if s.client == nil {
		return errors.New("dynamodb cache client unavailable")
	}
	_, err := s.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(s.table)})
	return err
}

func (s *dynamoStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            map[string]types.AttributeValue{"k": &types.AttributeValueMemberS{Value: s.cacheKey(key)}},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, false, err
	}
	if out.Item == nil {
		return nil, false, nil
	}
	if expired(out.Item) {
		_, _ = s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(s.table),
			Key:       map[string]types.AttributeValue{"k": &types.AttributeValueMemberS{Value: s.cacheKey(key)}},
		})
		return nil, false, nil
	}
	v, ok := out.Item["v"].(*types.AttributeValueMemberB)
	if !ok {
		return nil, false, errors.New("dynamodb item missing binary value")
	}
	return cloneBytes(v.Value), true, nil
}

func (s *dynamoStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	exp := time.Now().Add(ttl).UnixMilli()
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			"k":  &types.AttributeValueMemberS{Value: s.cacheKey(key)},
			"v":  &types.AttributeValueMemberB{Value: cloneBytes(value)},
			"ea": &types.AttributeValueMemberN{Value: strconv.FormatInt(exp, 10)},
		},
	})
	return err
}

func (s *dynamoStore) Add(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = s.defaultTTL
	}
	nowMs := time.Now().UnixMilli()
	exp := time.UnixMilli(nowMs).Add(ttl).UnixMilli()
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			"k":  &types.AttributeValueMemberS{Value: s.cacheKey(key)},
			"v":  &types.AttributeValueMemberB{Value: cloneBytes(value)},
			"ea": &types.AttributeValueMemberN{Value: strconv.FormatInt(exp, 10)},
		},
		ConditionExpression: aws.String("attribute_not_exists(k) OR ea < :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now": &types.AttributeValueMemberN{Value: strconv.FormatInt(nowMs, 10)},
		},
	})
	if err != nil {
		var cce *types.ConditionalCheckFailedException
		if errors.As(err, &cce) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *dynamoStore) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	body, ok, err := s.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	current := int64(0)
	if ok {
		n, err := strconv.ParseInt(string(body), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cache key %q does not contain a numeric value", key)
		}
		current = n
	}
	next := current + delta
	if err := s.Set(ctx, key, []byte(strconv.FormatInt(next, 10)), ttl); err != nil {
		return 0, err
	}
	return next, nil
}

func (s *dynamoStore) Decrement(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	return s.Increment(ctx, key, -delta, ttl)
}

func (s *dynamoStore) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(s.table),
		Key:       map[string]types.AttributeValue{"k": &types.AttributeValueMemberS{Value: s.cacheKey(key)}},
	})
	return err
}

func (s *dynamoStore) DeleteMany(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	const maxBatch = 25
	for i := 0; i < len(keys); i += maxBatch {
		end := i + maxBatch
		if end > len(keys) {
			end = len(keys)
		}
		writes := make([]types.WriteRequest, 0, end-i)
		for _, k := range keys[i:end] {
			writes = append(writes, types.WriteRequest{
				DeleteRequest: &types.DeleteRequest{
					Key: map[string]types.AttributeValue{"k": &types.AttributeValueMemberS{Value: s.cacheKey(k)}},
				},
			})
		}
		if _, err := s.client.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{s.table: writes},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *dynamoStore) Flush(ctx context.Context) error {
	var lastEvaluatedKey map[string]types.AttributeValue
	for {
		out, err := s.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:            aws.String(s.table),
			ProjectionExpression: aws.String("k"),
			ExclusiveStartKey:    lastEvaluatedKey,
		})
		if err != nil {
			return err
		}
		if len(out.Items) > 0 {
			var keys []string
			for _, item := range out.Items {
				if kv, ok := item["k"].(*types.AttributeValueMemberS); ok {
					key := kv.Value
					if s.prefix != "" && strings.HasPrefix(key, s.prefix+":") {
						key = strings.TrimPrefix(key, s.prefix+":")
					}
					keys = append(keys, key)
				}
			}
			if err := s.DeleteMany(ctx, keys...); err != nil {
				return err
			}
		}
		if out.LastEvaluatedKey == nil || len(out.LastEvaluatedKey) == 0 {
			return nil
		}
		lastEvaluatedKey = out.LastEvaluatedKey
	}
}

func (s *dynamoStore) cacheKey(key string) string {
	if s.prefix == "" {
		return key
	}
	return s.prefix + ":" + key
}

func expired(item map[string]types.AttributeValue) bool {
	av, ok := item["ea"].(*types.AttributeValueMemberN)
	if !ok {
		return false
	}
	exp, err := strconv.ParseInt(av.Value, 10, 64)
	if err != nil {
		return false
	}
	return time.Now().UnixMilli() > exp
}

func ensureDynamoTable(ctx context.Context, client DynamoAPI, table string) error {
	var lastErr error
	for attempt := 1; attempt <= dynamoEnsureTableMaxAttempts; attempt++ {
		_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(table)})
		if err == nil {
			return nil
		}

		var rnfe *types.ResourceNotFoundException
		if errors.As(err, &rnfe) {
			_, createErr := client.CreateTable(ctx, &dynamodb.CreateTableInput{
				TableName: aws.String(table),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("k"), KeyType: types.KeyTypeHash},
				},
				AttributeDefinitions: []types.AttributeDefinition{
					{AttributeName: aws.String("k"), AttributeType: types.ScalarAttributeTypeS},
				},
				BillingMode: types.BillingModePayPerRequest,
			})
			if createErr == nil {
				return nil
			}
			var inUse *types.ResourceInUseException
			if errors.As(createErr, &inUse) {
				return nil
			}
			if !isDynamoStartupRetryable(createErr) {
				return createErr
			}
			lastErr = createErr
		} else {
			if !isDynamoStartupRetryable(err) {
				return err
			}
			lastErr = err
		}

		if attempt == dynamoEnsureTableMaxAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(dynamoEnsureTableRetryDelay):
		}
	}
	if lastErr == nil {
		lastErr = errors.New("dynamo table ensure failed")
	}
	return fmt.Errorf("ensure dynamo table %q: %w", table, lastErr)
}

func isDynamoStartupRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "request send failed") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "eof")
}

func cloneBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	out := make([]byte, len(value))
	copy(out, value)
	return out
}
