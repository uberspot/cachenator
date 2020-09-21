package main

import (
	"fmt"

	"github.com/adrianchifor/go-parallel"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

var (
	bucket       string
	s3Endpoint   string
	s3Uploader   s3manager.Uploader
	s3Downloader s3manager.Downloader
)

func initS3() {
	s3Session, err := session.NewSession(&aws.Config{
		Endpoint: aws.String(s3Endpoint),
	})
	if err != nil {
		log.Fatalf("Failed to initialize S3 session: %v", err)
	}

	s3Uploader = *s3manager.NewUploader(s3Session)
	s3Downloader = *s3manager.NewDownloader(s3Session)
}

func s3Upload(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		log.Errorf("Failed to parse multipart form: %v", err)
		c.String(400, "Expecting a multipart form")
		return
	}
	if _, found := form.File["files"]; !found {
		c.String(400, "'files' not found in multipart form")
		return
	}

	files := form.File["files"]

	uploadPool := parallel.SmallJobPool()
	defer uploadPool.Close()

	for _, file := range files {
		file := file
		uploadPool.AddJob(func() {
			key := file.Filename
			body, err := file.Open()
			if err != nil {
				// TODO: Propagate error higher
				log.Errorf("Failed to read '%s' file when trying to upload to S3: %v", key, err)
				return
			}
			defer body.Close()

			_, err = s3Uploader.Upload(&s3manager.UploadInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(key),
				Body:   body,
			})
			if err != nil {
				// TODO: Propagate error higher
				log.Errorf("Failed to upload '%s' to S3: %v", key, err)
			}
		})
	}

	err = uploadPool.Wait()
	if err != nil {
		log.Error(err)
		c.String(500, "Internal error, check server logs")
	}

	c.String(200, fmt.Sprintf("Uploaded %d object(s) to S3", len(files)))
}

func s3Download(key string, buf *aws.WriteAtBuffer) error {
	_, err := s3Downloader.Download(buf, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}
