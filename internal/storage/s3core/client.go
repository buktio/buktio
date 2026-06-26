// Package s3core is the backend-agnostic S3 data plane shared by every
// S3-compatible provider: the object browser, presigned URLs, CORS, lifecycle,
// SSE-C, and (for non-admin backends) plain bucket create/list/delete. The Garage
// adapter pairs it with the Admin API; the generic-S3 adapter (R2/AWS S3/B2/
// SeaweedFS/Ceph) uses it alone with operator-supplied credentials.
//
// Every method is keyed by the S3-addressable bucket NAME (the caller resolves the
// name from its own bucket id). SSE-C keys flow in via the request context.
package s3core

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/buktio/buktio/internal/storage"
)

// IsS3ErrorCode reports whether err is an S3 API error with the given code (e.g.
// "NoSuchCORSConfiguration", "NoSuchBucket").
func IsS3ErrorCode(err error, code string) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == code
}

// ssecFields derives the SSE-C request fields from the context key (base64 of a
// 32-byte AES-256 key). Returns nils when no (or invalid) key is present.
func ssecFields(ctx context.Context) (alg, key, md5b64 *string) {
	k := storage.SSECFrom(ctx)
	if k == nil {
		return nil, nil, nil
	}
	raw, err := base64.StdEncoding.DecodeString(k.KeyB64)
	if err != nil || len(raw) != 32 {
		return nil, nil, nil
	}
	sum := md5.Sum(raw)
	return aws.String("AES256"), aws.String(k.KeyB64), aws.String(base64.StdEncoding.EncodeToString(sum[:]))
}

// Client is an S3 (SigV4, path-style) data-plane client.
type Client struct {
	region  string
	client  *s3.Client
	presign *s3.PresignClient
}

// New builds an S3 data-plane client. publicEndpoint is the externally-reachable
// host used to sign presigned URLs (so browsers can reach them directly); when
// empty it falls back to endpoint.
func New(endpoint, publicEndpoint, region, accessKeyID, secretAccessKey string) *Client {
	creds := credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")
	awsCfg := aws.Config{Region: region, Credentials: creds}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = true
	})

	pe := publicEndpoint
	if pe == "" {
		pe = endpoint
	}
	presignBase := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if pe != "" {
			o.BaseEndpoint = aws.String(pe)
		}
		o.UsePathStyle = true
	})

	return &Client{
		region:  region,
		client:  client,
		presign: s3.NewPresignClient(presignBase),
	}
}

// Raw exposes the underlying AWS S3 client for backend-specific calls.
func (c *Client) Raw() *s3.Client { return c.client }

// --- bucket lifecycle (plain S3; used by non-admin backends) ---

// CreateBucket creates an S3 bucket by name.
func (c *Client) CreateBucket(ctx context.Context, name string) error {
	_, err := c.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(name)})
	if err != nil && (IsS3ErrorCode(err, "BucketAlreadyOwnedByYou") || IsS3ErrorCode(err, "BucketAlreadyExists")) {
		return nil // idempotent
	}
	return err
}

// DeleteBucket deletes an (empty) S3 bucket by name.
func (c *Client) DeleteBucket(ctx context.Context, name string) error {
	_, err := c.client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(name)})
	return err
}

// ListBuckets lists bucket names visible to the credentials.
func (c *Client) ListBuckets(ctx context.Context) ([]string, error) {
	out, err := c.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		names = append(names, aws.ToString(b.Name))
	}
	return names, nil
}

// UsageByScan computes bytes+object count via a full ListObjectsV2 scan — for
// backends with no native per-bucket usage call (generic S3).
func (c *Client) UsageByScan(ctx context.Context, bucket string) (*storage.BucketUsage, error) {
	var bytes, objects int64
	p := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, o := range page.Contents {
			bytes += aws.ToInt64(o.Size)
			objects++
		}
	}
	return &storage.BucketUsage{ObjectCount: objects, BytesUsed: bytes, CapturedAt: time.Now().UTC()}, nil
}

// --- object plane ---

func (c *Client) ListObjects(ctx context.Context, bucket string, in storage.ListObjectsInput) (*storage.ListObjectsResult, error) {
	input := &s3.ListObjectsV2Input{Bucket: aws.String(bucket)}
	if in.Prefix != "" {
		input.Prefix = aws.String(in.Prefix)
	}
	if in.Delimiter != "" {
		input.Delimiter = aws.String(in.Delimiter)
	}
	if in.ContinuationToken != "" {
		input.ContinuationToken = aws.String(in.ContinuationToken)
	}
	if in.MaxKeys > 0 {
		input.MaxKeys = aws.Int32(in.MaxKeys)
	}

	out, err := c.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, err
	}

	res := &storage.ListObjectsResult{
		NextContinuationToken: aws.ToString(out.NextContinuationToken),
		IsTruncated:           aws.ToBool(out.IsTruncated),
	}
	for _, o := range out.Contents {
		res.Objects = append(res.Objects, storage.Object{
			Key:          aws.ToString(o.Key),
			Size:         aws.ToInt64(o.Size),
			LastModified: aws.ToTime(o.LastModified),
			ETag:         strings.Trim(aws.ToString(o.ETag), `"`),
		})
	}
	for _, p := range out.CommonPrefixes {
		res.CommonPrefixes = append(res.CommonPrefixes, aws.ToString(p.Prefix))
	}
	return res, nil
}

func (c *Client) HeadObject(ctx context.Context, bucket, key string) (*storage.Object, error) {
	out, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return &storage.Object{
		Key:          key,
		Size:         aws.ToInt64(out.ContentLength),
		LastModified: aws.ToTime(out.LastModified),
		ETag:         strings.Trim(aws.ToString(out.ETag), `"`),
	}, nil
}

func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func (c *Client) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	ids := make([]s3types.ObjectIdentifier, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, s3types.ObjectIdentifier{Key: aws.String(k)})
	}
	_, err := c.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &s3types.Delete{Objects: ids, Quiet: aws.Bool(true)},
	})
	return err
}

func (c *Client) CopyObject(ctx context.Context, bucket, srcKey, dstKey string) error {
	_, err := c.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(bucket + "/" + srcKey),
	})
	return err
}

// --- object versioning ---

func (c *Client) GetBucketVersioning(ctx context.Context, bucket string) (bool, error) {
	out, err := c.client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
	if err != nil {
		return false, err
	}
	return out.Status == s3types.BucketVersioningStatusEnabled, nil
}

func (c *Client) SetBucketVersioning(ctx context.Context, bucket string, enabled bool) error {
	status := s3types.BucketVersioningStatusSuspended
	if enabled {
		status = s3types.BucketVersioningStatusEnabled
	}
	_, err := c.client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket:                  aws.String(bucket),
		VersioningConfiguration: &s3types.VersioningConfiguration{Status: status},
	})
	return err
}

func (c *Client) ListObjectVersions(ctx context.Context, bucket, prefix string) ([]storage.ObjectVersion, error) {
	in := &s3.ListObjectVersionsInput{Bucket: aws.String(bucket)}
	if prefix != "" {
		in.Prefix = aws.String(prefix)
	}
	out, err := c.client.ListObjectVersions(ctx, in)
	if err != nil {
		return nil, err
	}
	versions := make([]storage.ObjectVersion, 0, len(out.Versions)+len(out.DeleteMarkers))
	for _, v := range out.Versions {
		versions = append(versions, storage.ObjectVersion{
			Key:          aws.ToString(v.Key),
			VersionID:    aws.ToString(v.VersionId),
			IsLatest:     aws.ToBool(v.IsLatest),
			Size:         aws.ToInt64(v.Size),
			LastModified: aws.ToTime(v.LastModified),
			ETag:         strings.Trim(aws.ToString(v.ETag), `"`),
		})
	}
	for _, d := range out.DeleteMarkers {
		versions = append(versions, storage.ObjectVersion{
			Key:            aws.ToString(d.Key),
			VersionID:      aws.ToString(d.VersionId),
			IsLatest:       aws.ToBool(d.IsLatest),
			IsDeleteMarker: true,
			LastModified:   aws.ToTime(d.LastModified),
		})
	}
	return versions, nil
}

func (c *Client) DeleteObjectVersion(ctx context.Context, bucket, key, versionID string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket:    aws.String(bucket),
		Key:       aws.String(key),
		VersionId: aws.String(versionID),
	})
	return err
}

// RestoreObjectVersion copies an old version onto the current key, making it the
// newest version (the prior current version is preserved in history).
func (c *Client) RestoreObjectVersion(ctx context.Context, bucket, key, versionID string) error {
	_, err := c.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		CopySource: aws.String(bucket + "/" + key + "?versionId=" + versionID),
	})
	return err
}

func (c *Client) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	if alg, key, md5b64 := ssecFields(ctx); alg != nil {
		input.SSECustomerAlgorithm, input.SSECustomerKey, input.SSECustomerKeyMD5 = alg, key, md5b64
	}
	_, err := c.client.PutObject(ctx, input)
	return err
}

func (c *Client) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, *storage.Object, error) {
	in := &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if alg, key, md5b64 := ssecFields(ctx); alg != nil {
		in.SSECustomerAlgorithm, in.SSECustomerKey, in.SSECustomerKeyMD5 = alg, key, md5b64
	}
	out, err := c.client.GetObject(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	obj := &storage.Object{
		Key:          key,
		Size:         aws.ToInt64(out.ContentLength),
		LastModified: aws.ToTime(out.LastModified),
		ETag:         strings.Trim(aws.ToString(out.ETag), `"`),
	}
	return out.Body, obj, nil
}

// --- presign ---

func (c *Client) PresignURL(ctx context.Context, in storage.PresignInput) (string, error) {
	expires := in.Expires
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	switch strings.ToUpper(in.Method) {
	case "PUT":
		req, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(in.BucketID),
			Key:         aws.String(in.Key),
			ContentType: optString(in.ContentType),
		}, s3.WithPresignExpires(expires))
		if err != nil {
			return "", err
		}
		return req.URL, nil
	default: // GET
		req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(in.BucketID),
			Key:    aws.String(in.Key),
		}, s3.WithPresignExpires(expires))
		if err != nil {
			return "", err
		}
		return req.URL, nil
	}
}

// --- CORS ---

func (c *Client) SetCORS(ctx context.Context, bucket string, rules []storage.CORSRule) error {
	_, err := c.client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket:            aws.String(bucket),
		CORSConfiguration: &s3types.CORSConfiguration{CORSRules: CORSRulesToAWS(rules)},
	})
	return err
}

func (c *Client) GetCORS(ctx context.Context, bucket string) ([]storage.CORSRule, error) {
	out, err := c.client.GetBucketCors(ctx, &s3.GetBucketCorsInput{Bucket: aws.String(bucket)})
	if err != nil {
		if IsS3ErrorCode(err, "NoSuchCORSConfiguration") {
			return nil, nil // not configured == empty
		}
		return nil, err
	}
	return CORSRulesFromAWS(out.CORSRules), nil
}

func (c *Client) DeleteCORS(ctx context.Context, bucket string) error {
	_, err := c.client.DeleteBucketCors(ctx, &s3.DeleteBucketCorsInput{Bucket: aws.String(bucket)})
	return err
}

// --- lifecycle (Expiration + AbortIncompleteMultipartUpload) ---

func (c *Client) SetLifecycle(ctx context.Context, bucket string, rules []storage.LifecycleRule) error {
	awsRules := make([]s3types.LifecycleRule, 0, len(rules))
	for i, r := range rules {
		status := s3types.ExpirationStatusEnabled
		if !r.Enabled {
			status = s3types.ExpirationStatusDisabled
		}
		ar := s3types.LifecycleRule{
			ID:     aws.String(ruleID(r.ID, i)),
			Status: status,
			Filter: &s3types.LifecycleRuleFilter{Prefix: aws.String(r.Prefix)},
		}
		if r.ExpireDays > 0 {
			ar.Expiration = &s3types.LifecycleExpiration{Days: aws.Int32(int32(r.ExpireDays))}
		}
		if r.AbortIncompleteMPUDays > 0 {
			ar.AbortIncompleteMultipartUpload = &s3types.AbortIncompleteMultipartUpload{
				DaysAfterInitiation: aws.Int32(int32(r.AbortIncompleteMPUDays)),
			}
		}
		awsRules = append(awsRules, ar)
	}
	_, err := c.client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket:                 aws.String(bucket),
		LifecycleConfiguration: &s3types.BucketLifecycleConfiguration{Rules: awsRules},
	})
	return err
}

func (c *Client) GetLifecycle(ctx context.Context, bucket string) ([]storage.LifecycleRule, error) {
	out, err := c.client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(bucket)})
	if err != nil {
		if IsS3ErrorCode(err, "NoSuchLifecycleConfiguration") {
			return nil, nil
		}
		return nil, err
	}
	rules := make([]storage.LifecycleRule, 0, len(out.Rules))
	for _, r := range out.Rules {
		lr := storage.LifecycleRule{
			ID:      aws.ToString(r.ID),
			Enabled: r.Status == s3types.ExpirationStatusEnabled,
		}
		if r.Filter != nil {
			lr.Prefix = aws.ToString(r.Filter.Prefix)
		}
		if r.Expiration != nil {
			lr.ExpireDays = int(aws.ToInt32(r.Expiration.Days))
		}
		if r.AbortIncompleteMultipartUpload != nil {
			lr.AbortIncompleteMPUDays = int(aws.ToInt32(r.AbortIncompleteMultipartUpload.DaysAfterInitiation))
		}
		rules = append(rules, lr)
	}
	return rules, nil
}

func (c *Client) DeleteLifecycle(ctx context.Context, bucket string) error {
	_, err := c.client.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{Bucket: aws.String(bucket)})
	return err
}

func ruleID(id string, i int) string {
	if id != "" {
		return id
	}
	return "rule-" + strconv.Itoa(i)
}

// --- pure CORS mapping (unit-tested) ---

// CORSRulesToAWS maps buktio CORS rules to the AWS SDK type.
func CORSRulesToAWS(rules []storage.CORSRule) []s3types.CORSRule {
	out := make([]s3types.CORSRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, s3types.CORSRule{
			AllowedOrigins: r.AllowedOrigins,
			AllowedMethods: r.AllowedMethods,
			AllowedHeaders: r.AllowedHeaders,
			ExposeHeaders:  r.ExposeHeaders,
			MaxAgeSeconds:  aws.Int32(int32(r.MaxAgeSeconds)),
		})
	}
	return out
}

// CORSRulesFromAWS maps AWS SDK CORS rules back to buktio rules.
func CORSRulesFromAWS(rules []s3types.CORSRule) []storage.CORSRule {
	out := make([]storage.CORSRule, 0, len(rules))
	for _, r := range rules {
		out = append(out, storage.CORSRule{
			AllowedOrigins: r.AllowedOrigins,
			AllowedMethods: r.AllowedMethods,
			AllowedHeaders: r.AllowedHeaders,
			ExposeHeaders:  r.ExposeHeaders,
			MaxAgeSeconds:  int(aws.ToInt32(r.MaxAgeSeconds)),
		})
	}
	return out
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return aws.String(s)
}
