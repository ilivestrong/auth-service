package models

import (
	"gorm.io/gorm"
)

type (
	Profile struct {
		gorm.Model
		ID          string `gorm:"primary_key"`
		Name        string
		PhoneNumber string `json:"phone_number" gorm:"unique"`
		Otp         string
		IsVerified  bool `json:"is_verified"`
	}
)
