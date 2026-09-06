package membership

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/redis/go-redis/v9"
)

// NewEngine builds the optional membership runtime. A disabled runtime keeps
// public catalog reads available after migration while refusing all writes.
func NewEngine(db *sql.DB, cfg *config.Config) (*Engine, error) {
	if db == nil || cfg == nil {
		return nil, fmt.Errorf("membership: missing database or configuration")
	}
	c := cfg.Membership
	e := &Engine{DB: db, Enabled: c.Enabled, PaymentsEnabled: c.Enabled && c.PaymentsEnabled}
	if !c.Enabled {
		return e, nil
	}

	keys, err := ParseKeyring(c.KeyringJSON, c.ActiveKeyID, c.IdentityKey)
	if err != nil || keys == nil {
		return nil, fmt.Errorf("membership: invalid keyring configuration")
	}
	redisOptions, err := redis.ParseURL(strings.TrimSpace(c.CredentialRedisURL))
	if err != nil {
		return nil, fmt.Errorf("membership: invalid credential redis URL")
	}
	credentialRedis := redis.NewClient(redisOptions)
	vault := &RedisVault{Client: credentialRedis, Keys: keys}
	if err = vault.Ready(context.Background()); err != nil {
		_ = credentialRedis.Close()
		return nil, fmt.Errorf("membership: credential redis must have persistence disabled")
	}

	browser, err := NewHTTPBrowser(c.BrowserURL, c.BrowserToken)
	if err != nil {
		_ = credentialRedis.Close()
		return nil, fmt.Errorf("membership: invalid browser service configuration")
	}
	audit, err := newAuditSink(c)
	if err != nil {
		_ = credentialRedis.Close()
		return nil, err
	}

	e.Keys = keys
	e.Vault = vault
	e.Browser = browser
	e.Audit = audit
	e.credentialRedis = credentialRedis
	return e, nil
}

func newAuditSink(c config.MembershipConfig) (AuditSink, error) {
	if strings.TrimSpace(c.AuditBucket) == "" || strings.TrimSpace(c.AuditRegion) == "" {
		return nil, fmt.Errorf("membership: audit bucket and region are required")
	}
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(c.AuditRegion)}
	if c.AuditAccessKeyID != "" || c.AuditSecretAccessKey != "" {
		if c.AuditAccessKeyID == "" || c.AuditSecretAccessKey == "" {
			return nil, fmt.Errorf("membership: incomplete audit credentials")
		}
		options = append(options, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AuditAccessKeyID, c.AuditSecretAccessKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), options...)
	if err != nil {
		return nil, fmt.Errorf("membership: load audit storage configuration: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = c.AuditUsePathStyle
		if endpoint := strings.TrimSpace(c.AuditEndpoint); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	return &S3AuditSink{Client: client, Bucket: c.AuditBucket}, nil
}

func (e *Engine) Close() error {
	e.Stop()
	if e.credentialRedis != nil {
		return e.credentialRedis.Close()
	}
	return nil
}
