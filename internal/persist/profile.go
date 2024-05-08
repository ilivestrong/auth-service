package persist

import (
	"errors"

	"github.com/google/uuid"
	"github.com/ilivestrong/auth-service/internal/models"
	"gorm.io/gorm"
)

var (
	ErrCreateProfileFailed = errors.New("failed to create profile")
	ErrUpdateProfileFailed = errors.New("failed to update profile")
	ErrGetProfileFailed    = errors.New("failed to get profile")
	ErrProfileNotFound     = errors.New("profile not found")
)

type (
	ProfileRepo interface {
		Create(phoneNumber string, name string) (string, error)
		Get(phoneNumber string) (*models.Profile, error)
		UpdateOTP(phone_number string, otp string) error
		SetOTPVerified(phone_number string) error
	}
	profileRepository struct {
		db *gorm.DB
	}
)

func (pr *profileRepository) Create(phone_number string, name string) (string, error) {
	newProfile := &models.Profile{
		ID:          uuid.New().String(),
		Name:        name,
		PhoneNumber: phone_number,
		IsVerified:  false,
	}
	result := pr.db.Create(newProfile)

	if result.Error != nil || result.RowsAffected == 0 {
		return "", ErrCreateProfileFailed
	}
	return newProfile.ID, nil
}

func (pr *profileRepository) Get(phone_number string) (*models.Profile, error) {
	var profile models.Profile
	result := pr.db.Where("phone_number = ?", phone_number).First(&profile)

	if result.Error != nil {
		return nil, ErrGetProfileFailed
	}
	return &profile, nil
}

func (pr *profileRepository) UpdateOTP(phone_number string, otp string) error {
	profile, err := pr.Get(phone_number)
	if err != nil {
		return err
	}

	profile.Otp = otp
	result := pr.db.Save(profile)

	if result.Error != nil || result.RowsAffected == 0 {
		return ErrUpdateProfileFailed
	}
	return nil
}

func (pr *profileRepository) SetOTPVerified(phone_number string) error {
	profile, err := pr.Get(phone_number)
	if err != nil {
		return err
	}

	profile.IsVerified = true
	result := pr.db.Save(profile)

	if result.Error != nil || result.RowsAffected == 0 {
		return ErrUpdateProfileFailed
	}
	return nil
}

func NewProfileRepository(db *gorm.DB) ProfileRepo {
	return &profileRepository{db}
}
