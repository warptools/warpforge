package mirroring

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/warptools/warpforge/wfapi"
)

type S3Pusher struct {
	client       *s3.Client
	cfg          wfapi.S3PushConfig
	existingKeys map[string]bool
}

func wareIdToKey(wareId wfapi.WareID) string {
	return filepath.Join(wareId.Hash[0:3], wareId.Hash[3:6], wareId.Hash)
}

func NewS3Pusher(cfg wfapi.S3PushConfig) (S3Pusher, error) {
	config, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               cfg.Endpoint,
					HostnameImmutable: true,
					SigningRegion:     cfg.Region,
				}, nil
			})),
		config.WithRegion(cfg.Region),
	)

	if err != nil {
		panic(err)
	}

	client := s3.NewFromConfig(config)

	// make sure we can access the specified bucket
	_, err = client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return S3Pusher{}, fmt.Errorf("could not access bucket %q: %s", cfg.Bucket, err)
	}

	// list all the objects currently in the bucket
	result, err := client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return S3Pusher{}, fmt.Errorf("could not list contents of bucket %q: %s", cfg.Bucket, err)
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
		return err
	}

	uploader := manager.NewUploader(p.client)

	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: &p.cfg.Bucket,
		Key:    &key,
		Body:   file,
	})

	return err
}
