package service

import (
	"context"
	"fmt"
)

// SnippetsDTO holds ready-to-paste client configuration for a bucket + key.
type SnippetsDTO struct {
	Endpoint    string            `json:"endpoint"`
	Region      string            `json:"region"`
	Bucket      string            `json:"bucket"`
	AccessKeyID string            `json:"access_key_id"`
	Addressing  string            `json:"addressing"`
	Signature   string            `json:"signature"`
	Snippets    map[string]string `json:"snippets"`
}

const secretPlaceholder = "<YOUR_SECRET_ACCESS_KEY>"

// Snippets builds connection snippets for the given bucket and access key. The
// secret is always rendered as a placeholder (buktio never re-fetches it).
func (s *Services) Snippets(ctx context.Context, bucketID, accessKeyID string) (*SnippetsDTO, error) {
	bucket := "my-bucket"
	if bucketID != "" {
		b, err := s.Store.GetBucket(ctx, bucketID)
		if err != nil {
			return nil, mapRepoErr(err)
		}
		bucket = b.GarageGlobalAlias
	}
	if accessKeyID == "" {
		accessKeyID = "<YOUR_ACCESS_KEY_ID>"
	}

	endpoint := s.S3PublicEndpoint
	if endpoint == "" {
		endpoint = "https://your-host/s3"
	}
	region := s.S3Region
	if region == "" {
		region = "garage"
	}

	out := &SnippetsDTO{
		Endpoint: endpoint, Region: region, Bucket: bucket, AccessKeyID: accessKeyID,
		Addressing: "path-style", Signature: "s3v4",
		Snippets: map[string]string{
			"aws_cli": fmt.Sprintf(
				"aws configure set aws_access_key_id %s\n"+
					"aws configure set aws_secret_access_key %s\n"+
					"aws configure set region %s\n"+
					"aws --endpoint-url %s s3 ls s3://%s/",
				accessKeyID, secretPlaceholder, region, endpoint, bucket),
			"rclone": fmt.Sprintf(
				"[buktio]\ntype = s3\nprovider = Other\nenv_auth = false\n"+
					"access_key_id = %s\nsecret_access_key = %s\nregion = %s\n"+
					"endpoint = %s\nforce_path_style = true\n# usage: rclone ls buktio:%s",
				accessKeyID, secretPlaceholder, region, endpoint, bucket),
			"boto3": fmt.Sprintf(
				"import boto3\ns3 = boto3.client(\n    's3',\n    endpoint_url='%s',\n"+
					"    aws_access_key_id='%s',\n    aws_secret_access_key='%s',\n"+
					"    region_name='%s',\n    config=boto3.session.Config(s3={'addressing_style': 'path'}, signature_version='s3v4'),\n)\n"+
					"s3.upload_file('local.txt', '%s', 'local.txt')",
				endpoint, accessKeyID, secretPlaceholder, region, bucket),
			"node_sdk": fmt.Sprintf(
				"import { S3Client, PutObjectCommand } from '@aws-sdk/client-s3';\n"+
					"const s3 = new S3Client({\n  endpoint: '%s',\n  region: '%s',\n  forcePathStyle: true,\n"+
					"  credentials: { accessKeyId: '%s', secretAccessKey: '%s' },\n});\n"+
					"await s3.send(new PutObjectCommand({ Bucket: '%s', Key: 'hello.txt', Body: 'hi' }));",
				endpoint, region, accessKeyID, secretPlaceholder, bucket),
			"restic": fmt.Sprintf(
				"export AWS_ACCESS_KEY_ID=%s\nexport AWS_SECRET_ACCESS_KEY=%s\nexport AWS_DEFAULT_REGION=%s\n"+
					"restic -r s3:%s/%s init",
				accessKeyID, secretPlaceholder, region, endpoint, bucket),
		},
	}
	return out, nil
}
