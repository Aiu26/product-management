package config

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

type AWSConfig struct {
	AccessKey string
	SecretKey string
	Region string
	BucketName string
}

type Config struct {
	Host string
	SDN string
	Redis string
	RabbitMQHost string
	RabbitMQQueue string
	AWS AWSConfig
}

func LoadConfig(withoutHost bool, withS3 bool) *Config {
	host := os.Getenv("HOST")
	if !withoutHost && len(host) < 1 {
		logrus.Fatal("HOST is not declared")
	}

	databaseName := os.Getenv("DATABASE_NAME")
	databaseUser := os.Getenv("DATABASE_USER")
	databasePassword := os.Getenv("DATABASE_PASSWORD")
	databaseHost := os.Getenv("DATABASE_HOST")
	databasePort := os.Getenv("DATABASE_PORT")
	timezone := os.Getenv("TZ")
	if len(databaseName) < 1 || len(databaseUser) < 1 || len(databasePassword) < 1 || len(databaseHost) < 1 || len(databasePort) < 1 || len(timezone) < 1 {
		logrus.Fatal("Please provide database configuration")
	}
	
	sdn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=%s", databaseHost, databaseUser, databasePassword, databaseName, databasePort, timezone)

	redisHost := os.Getenv("REDIS_HOST")
	if len(redisHost) < 1 {
		logrus.Fatal("REDIS_HOST is not declared")
	}

	rabbitMqHost := os.Getenv("RABBITMQ_HOST")
	if len(rabbitMqHost) < 1 {
		logrus.Fatal("RABBITMQ_HOST is not declared")
	}

	rabbitMqQueue := os.Getenv("RABBITMQ_QUEUE")
	if len(rabbitMqQueue) < 1 {
		logrus.Fatal("RABBITMQ_QUEUE is not declared")
	}

	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	awsRegion := os.Getenv("AWS_BUCKET_REGION")
	s3BucketName := os.Getenv("S3_BUCKET_NAME")
	if withS3 && (len(awsAccessKey) < 1 || len(awsSecretKey) < 1 || len(awsRegion) < 1 || len(s3BucketName) < 1) {
		logrus.Fatal("Please provide AWS S3 configuration")
	}

	return &Config{
		Host: host,
		SDN: sdn,
		Redis: redisHost,
		RabbitMQHost: rabbitMqHost,
		RabbitMQQueue: rabbitMqQueue,
		AWS: AWSConfig{
			AccessKey: awsAccessKey,
			SecretKey: awsSecretKey,
			Region: awsRegion,
			BucketName: s3BucketName,
		},
	}
}