package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/aiu26/product-management/common/config"
	"github.com/aiu26/product-management/compression/internal/compress"
	"github.com/aiu26/product-management/compression/internal/products"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Logger setup
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp : true,
	})

	// Config setup
	conf := config.LoadConfig(true, true)

	// Database setup
	db, err := gorm.Open(postgres.Open(conf.SDN), &gorm.Config{})
	if err != nil {
		logrus.Fatalf("Failed to connect to database: %s", err.Error())
	}
	logrus.Info("Connected to database")

	// Redis setup
	rdb := redis.NewClient(&redis.Options{
		Addr: conf.Redis,
		Password: "",
		DB: 0,
	})
	logrus.Info("Connected to redis")

	// RabbitMQ setup
	rabbitConn, err := amqp.Dial(conf.RabbitMQHost)
	if err != nil {
		logrus.Fatalf("Failed to connect to RabbitMQ: %s", err.Error())
	}
	defer rabbitConn.Close()

	channel, err := rabbitConn.Channel()
	if err != nil {
		logrus.Fatalf("Failed to open a channel: %s", err.Error())
	}
	defer channel.Close()

	// S3 setup
	cfg, err := awsConfig.LoadDefaultConfig(
		context.TODO(),
		awsConfig.WithRegion(conf.AWS.Region),
		awsConfig.WithCredentialsProvider(aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(conf.AWS.AccessKey, conf.AWS.SecretKey, ""))),
	)
	if err != nil {
		logrus.Fatalf("Failed to setup S3: %s", err.Error())
	}

	s3Client := s3.NewFromConfig(cfg)

	// Listen for messages
	messages, err := channel.Consume(conf.RabbitMQQueue, "", true, false, false, false, nil)
	if err != nil {
		logrus.Fatalf("Failed to consume messages: %s", err.Error())
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	logrus.Info("Waiting for messages")
	for {
		select {
		case message := <-messages:
			id := string(message.Body)
			logrus.Infof("Received product: %s", message.Body)
			
			images, _ := products.GetProductImages(db, id)
			logrus.Infof("Compressing images: %v", images)

			compressed, _ := compress.CompressImages(images, s3Client, conf.AWS.BucketName)
			logrus.Infof("Compressed images: %v", compressed)
			
			products.StoreCompressedImages(db, rdb, id, compressed)
		case <-done:
			logrus.Info("Shutting down")
			os.Exit(0)
		}
	}
}