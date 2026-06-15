package aws

import (
	"context"
	"os"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

const defaultRegion = "us-east-1"

type Config struct {
	Profile            string
	Region             string
	LocalStackEndpoint string
}

type Client struct {
	cfg awssdk.Config

	Lambda         *lambda.Client
	APIGatewayV2   *apigatewayv2.Client
	CloudWatchLogs *cloudwatchlogs.Client
	CloudFront     *cloudfront.Client
}

type LambdaFunction struct {
	Name         string
	Runtime      string
	LastModified string
	MemoryMB     int32
}

type API struct {
	Name        string
	Protocol    string
	Routes      []Route
	Description string
}

type Route struct {
	Method         string
	Path           string
	LambdaFunction string
}

type LogGroup struct {
	Name          string
	StoredBytes   int64
	RetentionDays int32
}

type Distribution struct {
	ID         string
	Status     string
	DomainName string
}

func ConfigFromEnv() Config {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = defaultRegion
	}

	endpoint := os.Getenv("LOCALSTACK_ENDPOINT")
	if endpoint == "" && os.Getenv("LAZYINFRA_LOCALSTACK") == "1" {
		endpoint = "http://localhost:4566"
	}

	return Config{
		Profile:            os.Getenv("AWS_PROFILE"),
		Region:             region,
		LocalStackEndpoint: endpoint,
	}
}

func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	options := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}
	if cfg.Profile != "" {
		options = append(options, config.WithSharedConfigProfile(cfg.Profile))
	}
	if cfg.LocalStackEndpoint != "" {
		options = append(options, config.WithBaseEndpoint(cfg.LocalStackEndpoint))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
	}

	return &Client{
		cfg:            awsCfg,
		Lambda:         lambda.NewFromConfig(awsCfg),
		APIGatewayV2:   apigatewayv2.NewFromConfig(awsCfg),
		CloudWatchLogs: cloudwatchlogs.NewFromConfig(awsCfg),
		CloudFront:     cloudfront.NewFromConfig(awsCfg),
	}, nil
}

func (c *Client) ListLambdaFunctions(ctx context.Context) ([]LambdaFunction, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(180 * time.Millisecond):
	}

	return []LambdaFunction{
		{Name: "users-create", Runtime: "nodejs20.x", LastModified: "2026-06-12T10:41:00Z", MemoryMB: 256},
		{Name: "orders-process", Runtime: "go1.x", LastModified: "2026-06-13T19:15:32Z", MemoryMB: 512},
		{Name: "billing-webhook", Runtime: "python3.12", LastModified: "2026-06-14T08:05:11Z", MemoryMB: 128},
	}, nil
}

func (c *Client) ListAPIs(ctx context.Context) ([]API, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(220 * time.Millisecond):
	}

	return []API{
		{
			Name:     "backend-dev-http",
			Protocol: "HTTP",
			Routes: []Route{
				{Method: "GET", Path: "/users", LambdaFunction: "users-list"},
				{Method: "POST", Path: "/users", LambdaFunction: "users-create"},
				{Method: "POST", Path: "/orders", LambdaFunction: "orders-process"},
			},
		},
		{
			Name:     "legacy-admin-rest",
			Protocol: "REST",
			Routes: []Route{
				{Method: "GET", Path: "/admin/reports", LambdaFunction: "reports-generate"},
				{Method: "DELETE", Path: "/admin/cache", LambdaFunction: "cache-purge"},
			},
		},
	}, nil
}

func (c *Client) ListLogGroups(ctx context.Context) ([]LogGroup, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(160 * time.Millisecond):
	}

	return []LogGroup{
		{Name: "/aws/lambda/users-create", StoredBytes: 834912, RetentionDays: 14},
		{Name: "/aws/lambda/orders-process", StoredBytes: 2238912, RetentionDays: 30},
		{Name: "/aws/apigateway/backend-dev-http", StoredBytes: 549222, RetentionDays: 7},
	}, nil
}

func (c *Client) TailLogSample(ctx context.Context) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(350 * time.Millisecond):
	}

	return []string{
		"2026-06-14T13:01:22Z INFO request started route=/orders",
		"2026-06-14T13:01:23Z INFO lambda invoked function=orders-process",
		"2026-06-14T13:01:23Z ERROR validation failed: missing customerId",
		"2026-06-14T13:01:24Z Exception: retry scheduled for message id=evt_123",
	}, nil
}

func (c *Client) ListDistributions(ctx context.Context) ([]Distribution, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(200 * time.Millisecond):
	}

	return []Distribution{
		{ID: "E1DEV12345", Status: "Deployed", DomainName: "d111111abcdef8.cloudfront.net"},
		{ID: "E2STAGE6789", Status: "InProgress", DomainName: "d222222abcdef8.cloudfront.net"},
	}, nil
}
