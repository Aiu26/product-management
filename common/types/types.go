package types

type User struct {
	Id    int64  `json:"user_id" gorm:"primaryKey,autoIncrement,not null"`
	Email string `json:"email" validate:"required,email" gorm:"not null"`
}

type Product struct {
	Id               int64             `json:"product_id" gorm:"primaryKey,autoIncrement,not null"`
	Name             string            `json:"product_name" validate:"required" gorm:"not null"`
	Description      string            `json:"product_description" validate:"required" gorm:"not null"`
	Price            float32           `json:"product_price" validate:"required,gt=0" gorm:"not null"`
	UserId           int64             `json:"user_id" gorm:"not null"`
	User             User              `json:"-" gorm:"foreignkey:UserId;references:Id;constraint:OnDelete:CASCADE;not null"`
	Images           []Image           `json:"images" gorm:"foreignKey:ProductId;references:Id;constraint:OnDelete:CASCADE"`
	CompressedImages []CompressedImage `json:"compressed_images" gorm:"foreignKey:ProductId;references:Id;constraint:OnDelete:CASCADE"`
}

type Image struct {
	Id        int64  `json:"id" gorm:"primaryKey,autoIncrement,not null"`
	Url       string `json:"url" validate:"required,url" gorm:"not null"`
	ProductId int64  `json:"-" gorm:"not null"`
}

type CompressedImage struct {
	Id        int64  `json:"id" gorm:"primaryKey,autoIncrement,not null"`
	Url       string `json:"url" validate:"required,url" gorm:"not null"`
	ProductId int64  `json:"-" gorm:"not null"`
	ImageId   int64  `json:"-" gorm:"not null"`
	Image     Image  `json:"-" gorm:"foreignKey:ImageId;references:Id;constraint:OnDelete:CASCADE"`
}
