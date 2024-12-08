package compress

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/aiu26/product-management/common/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"
)

type Compressed struct {
	Url string
	ImageId int64
}

func getFileNameFromURL(url string) string {
	segments := strings.Split(url, "/")
	return segments[len(segments)-1]
}

func compressJPEG(input io.Reader, quality int) (*bytes.Buffer, error) {
	img, _, err := image.Decode(input)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	return buf, nil
}


func CompressImages(images []types.Image, s3Client *s3.Client, bucketName string) ([]Compressed, error) {
	var wg sync.WaitGroup
	resultCh := make(chan Compressed)
	errorCh := make(chan error)
	doneCh := make(chan struct{})

	processImage := func(image types.Image) {
		defer wg.Done()

		resp, err := http.Get(image.Url)
		if err != nil {
			logrus.Errorf("failed to fetch image from URL %s: %s", image.Url, err.Error())
			errorCh <- err
			return
		}
		defer resp.Body.Close()

		compressedData, err := compressJPEG(resp.Body, 75)
		if err != nil {
			logrus.Errorf("failed to compress image from URL %s: %s", image.Url, err.Error())
			errorCh <- err
			return
		}
		logrus.Infof("Compressed image %s", image.Url)

		key := fmt.Sprintf("compressed_images/%d_%s", image.Id, getFileNameFromURL(image.Url))
		_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket:      aws.String(bucketName),
			Key:         aws.String(key),
			Body:        bytes.NewReader(compressedData.Bytes()),
			ContentType: aws.String("image/jpeg"),
		})
		if err != nil {
			logrus.Errorf("failed to upload image %s to S3: %s", image.Url, err.Error())
			errorCh <- err
			return
		}

		s3Url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucketName, key)
		logrus.Infof("Uploaded compressed image %s to S3", s3Url)

		resultCh <- Compressed{
			Url:     s3Url,
			ImageId: image.Id,
		}
	}

	// Start processing images in parallel
	for _, image := range images {
		wg.Add(1)
		go processImage(image)
	}

	go func() {
		wg.Wait()
		close(resultCh)
		close(errorCh)
		close(doneCh)
	}()

	var uploadedUrls []Compressed
	var finalErr error = nil
	for {
		select {
		case res, ok := <-resultCh:
			if ok {
				uploadedUrls = append(uploadedUrls, res)
			}
		case err, ok := <-errorCh:
			if ok && finalErr == nil {
				finalErr = err
			}
		case <-doneCh:
			return uploadedUrls, finalErr
		}
	}
}