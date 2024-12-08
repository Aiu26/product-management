package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aiu26/product-management/common/config"
	"github.com/aiu26/product-management/common/types"
	"github.com/aiu26/product-management/products/internal/products"
	"github.com/aiu26/product-management/products/internal/utils/request"
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
	conf := config.LoadConfig(false, false)

	// Database setup
	db, err := gorm.Open(postgres.Open(conf.SDN), &gorm.Config{})
	if err != nil {
		logrus.Fatalf("Failed to connect to database: %s", err.Error())
	}
	logrus.Info("Connected to database")

	// Migrate schema
	db.AutoMigrate(&types.Product{})
	db.AutoMigrate(&types.User{})
	db.AutoMigrate(&types.Image{})
	db.AutoMigrate(&types.CompressedImage{})

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

	_, err = channel.QueueDeclare(conf.RabbitMQQueue, true, false, false, false, nil)
	if err != nil {
		logrus.Fatalf("Failed to declare a queue: %s", err.Error())
	}

	logrus.Info("Connected to RabbitMQ")

	// Router setup
	router := http.NewServeMux()
	router.HandleFunc("POST /products", request.Timer(products.NewProduct(db, channel, conf)))
	router.HandleFunc("GET /products", request.Timer(products.GetProducts(db)))
	router.HandleFunc("GET /products/{id}", request.Timer(products.GetProduct(db, rdb)))

	// Server setup
	server := http.Server {
		Addr: conf.Host,
		Handler: router,
	}

	logrus.Infof("Starting server on %s", conf.Host)

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func () {
		server.ListenAndServe()
	}()

	<- done

	// Server shutdown
	logrus.Info("Shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.Error("Failed to shutdown server", err.Error())
	}

	logrus.Info("Server shutdown succesfully")
}
