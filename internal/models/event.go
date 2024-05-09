package models

import "gorm.io/gorm"

type (
	Event struct {
		gorm.Model
		ID          string `gorm:"primary_key"`
		PhoneNumber string `json:"phone_number"`
		EventType   string
	}
)
