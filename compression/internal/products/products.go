package products

import (
	"context"
	"strconv"
	"time"

	"github.com/aiu26/product-management/common/types"
	"github.com/aiu26/product-management/compression/internal/compress"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func GetProductImages(db *gorm.DB, id string) ([]types.Image, error) {
	var images []types.Image

    if err := db.Where("product_id = ?", id).Find(&images).Error; err != nil {
        logrus.Errorf("Failed to fetch product images for product ID %s: %s", id, err.Error())
        return nil, err
    }

    return images, nil
}

func StoreCompressedImages(db *gorm.DB, rdb *redis.Client, id string, compressedImages []compress.Compressed) error {
    productId, err := strconv.ParseInt(id, 10, 64)
    if err != nil {
        logrus.Errorf("Failed to parse product ID %s to int64: %s", id, err.Error())
        return err
    }

    err = db.Transaction(func(tx *gorm.DB) error {
        for _, compressedImage := range compressedImages {
            compressedImage := types.CompressedImage{
                Url:       compressedImage.Url,
                ImageId:  compressedImage.ImageId,
                ProductId: productId,
            }

            if err := tx.Create(&compressedImage).Error; err != nil {
                logrus.Errorf("Failed to store compressed image for product ID %s: %s", id, err.Error())
                return err
            }
        }
        return nil
    })
    if err != nil {
        logrus.Errorf("Transaction failed while storing compressed images for product ID %d: %s", productId, err.Error())
        return err
    }

    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
    if err := rdb.Del(ctx, id).Err(); err != nil {
        logrus.Errorf("Failed to delete product from cache with product ID %d: %s", productId, err.Error())
        return err
    }

    logrus.Infof("Compressed images stored successfully for product ID %d", productId)
    return nil
}
