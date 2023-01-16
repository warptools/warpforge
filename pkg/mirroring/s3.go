package mirroring

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/warptools/warpforge/wfapi"
)

type S3Publisher struct {
	client *s3.Client
	Bucket BucketSpec
}

type BucketSpec struct {
	Name     string
	Endpoint string
	Region   string
}

func wareIdToKey(wareId wfapi.WareID) string {
	return filepath.Join(wareId.Hash[0:3], wareId.Hash[3:6], wareId.Hash)
}

func NewS3Publisher(bucket BucketSpec) (S3Publisher, error) {
	config, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(bucket.Region),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               bucket.Endpoint,
					HostnameImmutable: true,
					SigningRegion:     bucket.Region,
				}, nil
			})),
		config.WithRegion(bucket.Region),
	)

	if err != nil {
		panic(err)
	}

	client := s3.NewFromConfig(config)

	// make sure we can access the specified bucket
	_, err = client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(bucket.Name),
	})
	if err != nil {
		return S3Publisher{}, fmt.Errorf("could not access bucket %q: %s", bucket, err)
	}

	return S3Publisher{
		client: client,
		Bucket: bucket,
	}, nil

}

func (pub *S3Publisher) hasWare(wareId wfapi.WareID) (bool, error) {
	// TODO: this is a bad, bad implementation since it has to do an HTTP request for every ware
	// we should list wares once then check the list instead
	// but for the purposes of getting an end-to-end test going, meh.
	_, err := pub.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(pub.Bucket.Name),
		Key:    aws.String(wareIdToKey(wareId)),
	})

	if err != nil {
		var responseError *awshttp.ResponseError
		if errors.As(err, &responseError) && responseError.ResponseError.HTTPStatusCode() == http.StatusNotFound {
			return false, nil
		} else {
			return false, err
		}
	} else {
		return true, nil
	}
}

func (pub *S3Publisher) putWare(wareId wfapi.WareID, localPath string) error {
	key := wareIdToKey(wareId)
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}

	uploader := manager.NewUploader(pub.client)

	_, err = uploader.Upload(context.TODO(), &s3.PutObjectInput{
		Bucket: &pub.Bucket.Name,
		Key:    &key,
		Body:   file,
	})

	return err
}

func publishToS3(warehouseAddr wfapi.WarehouseAddr, wareId wfapi.WareID, filePath string) error {
	tmp := strings.Replace(string(warehouseAddr), "ca+s3://", "", 1)
	region := strings.Split(tmp, ".")[1]
	endpoint := "https://" + strings.Split(tmp, "/")[0]
	bucket := strings.Split(tmp, "/")[1]

	fmt.Println("publish to S3. endpoint =", endpoint, "region =", region, "bucket =", bucket)

	b := BucketSpec{
		Endpoint: endpoint,
		Region:   region,
		Name:     bucket,
	}
	pub, err := NewS3Publisher(b)
	if err != nil {
		return err
	}

	exists, err := pub.hasWare(wareId)
	if err != nil {
		return err
	}
	if exists {
		fmt.Println("bucket has ware")
	} else {
		fmt.Println("bucket does NOT have ware")
		err := pub.putWare(wareId, filePath)
		if err != nil {
			return err
		}
	}

	return nil
}
