package aws

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
)

const (
	ssoClientName = "lazyinfra"
	ssoClientType = "public"
	ssoGrantType  = "urn:ietf:params:oauth:grant-type:device_code"
	ssoPollInterval = 2 * time.Second
	ssoPollTimeout  = 5 * time.Minute
)

type SSOConfig struct {
	StartURL string
	Region   string
}

type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
}

type SSOAccount struct {
	AccountID   string
	AccountName string
}

type SSORole struct {
	RoleName string
}

func DeviceAuthInfo(ctx context.Context, cfg SSOConfig) (
	deviceCode, userCode, verificationURI, verificationURIComplete *string,
	clientSecret, clientID *string, interval int64, err error,
) {
	ssoCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, 0, fmt.Errorf("load sso config: %w", err)
	}

	oidcClient := ssooidc.NewFromConfig(ssoCfg)

	registerOutput, err := oidcClient.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String(ssoClientName),
		ClientType: aws.String(ssoClientType),
	})
	if err != nil {
		return nil, nil, nil, nil, nil, nil, 0, fmt.Errorf("register oidc client: %w", err)
	}

	deviceOutput, err := oidcClient.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     registerOutput.ClientId,
		ClientSecret: registerOutput.ClientSecret,
		StartUrl:     aws.String(cfg.StartURL),
	})
	if err != nil {
		return nil, nil, nil, nil, nil, nil, 0, fmt.Errorf("start device auth: %w", err)
	}

	return deviceOutput.DeviceCode,
		deviceOutput.UserCode,
		deviceOutput.VerificationUri,
		deviceOutput.VerificationUriComplete,
		registerOutput.ClientSecret,
		registerOutput.ClientId,
		int64(deviceOutput.Interval),
		nil
}

func PollToken(ctx context.Context, cfg SSOConfig, clientID, clientSecret, deviceCode *string) (*string, error) {
	ssoCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("load sso config: %w", err)
	}

	oidcClient := ssooidc.NewFromConfig(ssoCfg)

	tokenCtx, cancel := context.WithTimeout(ctx, ssoPollTimeout)
	defer cancel()

	for {
		createOutput, err := oidcClient.CreateToken(tokenCtx, &ssooidc.CreateTokenInput{
			ClientId:     clientID,
			ClientSecret: clientSecret,
			GrantType:    aws.String(ssoGrantType),
			DeviceCode:   deviceCode,
		})
		if err != nil {
			var pending *types.AuthorizationPendingException
			var slowDown *types.SlowDownException
			if errors.As(err, &pending) || errors.As(err, &slowDown) {
				select {
				case <-tokenCtx.Done():
					return nil, tokenCtx.Err()
				case <-time.After(ssoPollInterval):
					continue
				}
			}
			return nil, fmt.Errorf("create token: %w", err)
		}
		return createOutput.AccessToken, nil
	}
}

func ListAccounts(ctx context.Context, region string, accessToken *string) ([]SSOAccount, error) {
	ssoCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load sso config: %w", err)
	}
	ssoClient := sso.NewFromConfig(ssoCfg)

	var accounts []SSOAccount
	var nextToken *string
	for {
		output, err := ssoClient.ListAccounts(ctx, &sso.ListAccountsInput{
			AccessToken: accessToken,
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list accounts: %w", err)
		}
		for _, a := range output.AccountList {
			accounts = append(accounts, SSOAccount{
				AccountID:   aws.ToString(a.AccountId),
				AccountName: aws.ToString(a.AccountName),
			})
		}
		nextToken = output.NextToken
		if nextToken == nil || aws.ToString(nextToken) == "" {
			break
		}
	}
	return accounts, nil
}

func ListAccountRoles(ctx context.Context, region string, accessToken *string, accountID string) ([]SSORole, error) {
	ssoCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load sso config: %w", err)
	}
	ssoClient := sso.NewFromConfig(ssoCfg)

	var roles []SSORole
	var nextToken *string
	for {
		output, err := ssoClient.ListAccountRoles(ctx, &sso.ListAccountRolesInput{
			AccessToken: accessToken,
			AccountId:   aws.String(accountID),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list account roles: %w", err)
		}
		for _, r := range output.RoleList {
			roles = append(roles, SSORole{
				RoleName: aws.ToString(r.RoleName),
			})
		}
		nextToken = output.NextToken
		if nextToken == nil || aws.ToString(nextToken) == "" {
			break
		}
	}
	return roles, nil
}

func GetRoleCredentials(ctx context.Context, region string, accessToken *string, accountID, roleName string) (*AWSCredentials, error) {
	ssoCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load sso config: %w", err)
	}
	ssoClient := sso.NewFromConfig(ssoCfg)

	output, err := ssoClient.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: accessToken,
		AccountId:   aws.String(accountID),
		RoleName:    aws.String(roleName),
	})
	if err != nil {
		return nil, fmt.Errorf("get role credentials: %w", err)
	}

	creds := output.RoleCredentials
	return &AWSCredentials{
		AccessKeyID:     aws.ToString(creds.AccessKeyId),
		SecretAccessKey: aws.ToString(creds.SecretAccessKey),
		SessionToken:    aws.ToString(creds.SessionToken),
		Expiration:      time.UnixMilli(creds.Expiration),
	}, nil
}

func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start()
}
