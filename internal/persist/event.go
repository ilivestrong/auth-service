package persist

import (
	"errors"

	"github.com/google/uuid"
	"github.com/ilivestrong/auth-service/internal/models"
	"gorm.io/gorm"
)

var (
	ErrCreateEventFailed = errors.New("failed to create event")
	ErrListProfileFailed = errors.New("failed to get event list")
)

type (
	EventRepo interface {
		Create(profile *models.Profile, eventType string) (string, error)
		List(phoneNumber string) (*[]models.Event, error)
	}
	eventRepository struct {
		db *gorm.DB
	}
)

func (pr *eventRepository) Create(profile *models.Profile, eventType string) (string, error) {
	newEvent := models.Event{
		ID:          uuid.New().String(),
		PhoneNumber: profile.PhoneNumber,
		EventType:   eventType,
	}
	result := pr.db.Create(&newEvent)

	if result.Error != nil || result.RowsAffected == 0 {
		return "", ErrCreateEventFailed
	}
	return newEvent.ID, nil
}

func (pr *eventRepository) List(phoneNumber string) (*[]models.Event, error) {
	var events []models.Event
	result := pr.db.Where("phone_number = ?", phoneNumber).Find(&events)

	if result.Error != nil {
		return nil, ErrGetProfileFailed
	}
	return &events, nil
}

func NewEventRepository(db *gorm.DB) EventRepo {
	return &eventRepository{db}
}
