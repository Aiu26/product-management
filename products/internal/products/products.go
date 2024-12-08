package products

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/aiu26/product-management/common/config"
	"github.com/aiu26/product-management/common/types"
	"github.com/aiu26/product-management/products/internal/utils/response"
	"github.com/go-playground/validator"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ProductPayload struct {
	UserId int64 `json:"user_id" validate:"required"`
	ProductName string `json:"product_name" validate:"required"`
	ProductDescription string `json:"product_description" validate:"required"`
	ProductPrice float32 `json:"product_price" validate:"required,gt=0"`
	ProductImages []string `json:"product_images" validate:"required"`
}

func NewProduct(db *gorm.DB, channel *amqp.Channel, conf *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload ProductPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			logrus.Infof("Failed to decode request body: %s", err.Error())
			response.WriteError(w, http.StatusBadRequest, "Invalid request")
			return
		}

		err = validator.New().Struct(payload)
		if err != nil {
			logrus.Infof("Invalid payload: %s", err.Error())
			response.WriteValidationErrors(w, err, payload)
			return
		}

		if err := db.First(&types.User{}, payload.UserId).Error; err != nil {
			logrus.Infof("User not found: %s", err.Error())
			response.WriteError(w, http.StatusBadRequest, "Invalid user_id")
			return
		}
		
		product := types.Product{
			Name: payload.ProductName,
			Description: payload.ProductDescription,
			Price: payload.ProductPrice,
			UserId: payload.UserId,
		}
		
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&product).Error; err != nil {
				logrus.Errorf("Failed to create product: %s", err.Error())
				response.WriteError(w, http.StatusInternalServerError, "Failed to create product")
				return err
			}
			
			for _, image := range payload.ProductImages {
				productImage := types.Image{
					Url: image,
					ProductId: product.Id,
				}
				if err := tx.Create(&productImage).Error; err != nil {
					logrus.Errorf("Failed to create product image: %s", err.Error())
					response.WriteError(w, http.StatusInternalServerError, "Failed to create product")
					return err
				}
			}

			if err := tx.Preload("Images").Find(&product).Error; err != nil {
				logrus.Errorf("Failed to load product images: %s", err.Error())
				response.WriteError(w, http.StatusInternalServerError, "Failed to create product")
				return err
			}

			return nil
		})
		if err != nil {
			return
		}

		message := amqp.Publishing{
			ContentType: "text/plain",
			Body: []byte(strconv.FormatInt(product.Id, 10)),
		}
		err = channel.Publish("", conf.RabbitMQQueue, false, false, message)
		if err != nil {
			logrus.Errorf("Failed to publish product creation message: %s", err.Error())
			response.WriteError(w, http.StatusInternalServerError, "Failed to create product")
			return
		}
		logrus.Infof("Product creation message published")

		logrus.Infof("Product created: %d", product.Id)
		response.WriteJson(w, http.StatusCreated, product)
	}
}

func GetProducts(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userIdStr := r.URL.Query().Get("user_id")
		if userIdStr == "" {
			logrus.Infof("Missing user_id parameter")
			response.WriteError(w, http.StatusBadRequest, "Missing user_id parameter")
			return
		}

		userId, err := strconv.ParseInt(userIdStr, 10, 64)
		if err != nil {
			logrus.Infof("Invalid user_id parameter: %s", err.Error())
			response.WriteError(w, http.StatusBadRequest,  "Invalid user_id parameter")
			return
		}
		
		query := db.Preload("Images").Preload("CompressedImages").Where("user_id = ?", userId)

		minPriceStr := r.URL.Query().Get("min_price")
		if minPriceStr != "" {
			minPrice, err := strconv.ParseFloat(minPriceStr, 64)
			if err != nil {
				logrus.Infof("Invalid min_price parameter: %s", err.Error())
				response.WriteError(w, http.StatusBadRequest, "Invalid min_price parameter")
				return
			}
			query = query.Where("price >= ?", minPrice)
		}
		
		maxPriceStr := r.URL.Query().Get("max_price")
		if maxPriceStr != "" {
			maxPrice, err := strconv.ParseFloat(maxPriceStr, 64)
			if err != nil {
				logrus.Infof("Invalid max_price parameter: %s", err.Error())
				response.WriteError(w, http.StatusBadRequest, "Invalid max_price parameter")
				return
			}
			query = query.Where("price <= ?", maxPrice)
		}

		productName := r.URL.Query().Get("product_name")
		if productName != "" {
			query = query.Where("name ILIKE ?", "%"+productName+"%")
		}
		
		var products []types.Product
		if err := query.Find(&products).Error; err != nil {
			logrus.Errorf("Failed to fetch products: %s", err.Error())
			response.WriteError(w, http.StatusInternalServerError, "Error fetching products")
			return
		}
		
		logrus.Infof("Fetched %d products for user_id %d with filters min_price=%s, max_price=%s", len(products), userId, minPriceStr, maxPriceStr)
		response.WriteJson(w, http.StatusOK, products)
	}
}

func GetProduct(db *gorm.DB, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		productIdStr := r.PathValue("id")

		if productIdStr == ""{
			logrus.Infof("Missing product id")
			response.WriteError(w, http.StatusBadRequest, "Missing product id")
			return
		}

		productId, err := strconv.ParseInt(productIdStr, 10, 64)
		if err != nil {
			logrus.Infof("Invalid product id: %s", err.Error())
			response.WriteError(w, http.StatusBadRequest,  "Invalid product id")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		val, err := rdb.Get(ctx, productIdStr).Result()
		if !errors.Is(err, redis.Nil) {
			if err != nil {
				logrus.Errorf("Failed to fetch product from cache: %s", err.Error())
			} else {
				var product types.Product
				if err := json.Unmarshal([]byte(val), &product); err != nil {
					logrus.Errorf("Failed to unmarshal product from cache: %s", err.Error())
				} else {
					logrus.Infof("Product fetched from cache")
					response.WriteJson(w, http.StatusOK, product)
					return
				}
			}
		} else {
			logrus.Infof("Product not found in cache")
		}
		
		
		var product types.Product
		if err := db.Preload("Images").Preload("CompressedImages").First(&product, productId).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logrus.Infof("Product not found")
				response.WriteError(w, http.StatusNotFound, "Product not found")
				return
			} else {
				logrus.Errorf("Failed to fetch product: %s", err.Error())
				response.WriteError(w, http.StatusInternalServerError, "Error fetching product")
				return
			}
		}
		
		productJson, err := json.Marshal(product)
		if err != nil {
			logrus.Errorf("Failed to marshal product: %s", err.Error())
		} else if err := rdb.Set(ctx, productIdStr, productJson, 0).Err(); err != nil {
			logrus.Errorf("Failed to cache product: %s", err.Error())
		} else {
			logrus.Infof("Product cached")
		}

		logrus.Infof("Product fetched")
		response.WriteJson(w, http.StatusOK, product)
	}
}
