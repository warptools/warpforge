package mirroring

import (
	"context"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/warptools/warpforge/wfapi"

	"github.com/serum-errors/go-serum"
)

type S3Pusher struct {
	ctx          context.Context
	client       *s3.Client
	cfg          wfapi.S3PushConfig
	existingKeys map[string]bool
}

func wareIdToKey(wareId wfapi.WareID) string {
	return filepath.Join(wareId.Hash[0:3], wareId.Hash[3:6], wareId.Hash)
}

func newS3Pusher(ctx context.Context, cfg wfapi.S3PushConfig) (S3Pusher, error) {
	config, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               cfg.Endpoint,
					HostnameImmutable: true, // prevent the S3 client from mangling the user-provided endpoint
					SigningRegion:     cfg.Region,
				}, nil
			})),
		config.WithRegion(cfg.Region),
	)

	if err != nil {
		return S3Pusher{}, serum.Errorf(wfapi.ECodeIo, "could not connect to S3 endpoint %q with region %q: %s", cfg.Endpoint, cfg.Region, err)
	}

	client := s3.NewFromConfig(config)

	// make sure we can access the specified bucket
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return S3Pusher{}, serum.Errorf(wfapi.ECodeIo, "could not access bucket %q: %s", cfg.Bucket, err)
	}

	// list all the objects currently in the bucket
	result, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return S3Pusher{}, serum.Errorf(wfapi.ECodeIo, "could not list contents of bucket %q: %s", cfg.Bucket, err)
	}

	// store the list of existing keys so we can ignore writes for existing WareIDs
	existingKeys := make(map[string]bool)
	for _, object := range result.Contents {
		existingKeys[*object.Key] = true
	}

	return S3Pusher{
		client:       client,
		cfg:          cfg,
		existingKeys: existingKeys,
	}, nil

}

func (p *S3Pusher) hasWare(wareId wfapi.WareID) (bool, error) {
	key := wareIdToKey(wareId)
	if _, exists := p.existingKeys[key]; exists {
		// key already exsits in bucket
		return true, nil
	} else {
		return false, nil
	}
}

func (p *S3Pusher) pushWare(wareId wfapi.WareID, localPath string) error {
	key := wareIdToKey(wareId)
	file, err := os.Open(localPath)
	if err != nil {
		return serum.Errorf(wfapi.ECodeIo, "failed to open %q: %s", localPath, err)
	}

	uploader := manager.NewUploader(p.client)

	_, err = uploader.Upload(p.ctx, &s3.PutObjectInput{
		Bucket: &p.cfg.Bucket,
		Key:    &key,
		Body:   file,
	})

	if err != nil {
		return serum.Errorf(wfapi.ECodeIo, "failed to write to S3 bucket %q: %s", p.cfg.Bucket, err)
	}

	return nil
}
