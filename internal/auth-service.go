package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"time"

	"connectrpc.com/connect"
	"github.com/ilivestrong/auth-service/internal/models"
	"github.com/ilivestrong/auth-service/internal/persist"
	authv1 "github.com/ilivestrong/auth-service/internal/protos/gen/auth/v1"
	mq "github.com/ilivestrong/auth-service/internal/rabbitmq"
)

const (
	SendOTPMessage  = "SendOTP"
	EventTypeLogin  = "PROFILE_LOGIN"
	EventTypeLogout = "PROFILE_LOGOUT"
)

var (
	ErrPhoneNumberAlreadyVerified = errors.New("this phone number is already verified")
	ErrPhoneNumberNotVerified     = errors.New("this phone number is not verified yet")
	ErrIncorrectOtp               = errors.New("otp provided is incorrect")
	ErrProfileNotFound            = errors.New("failed to find profile")
	ErrGenerateTokenFailed        = errors.New("failed to generate session token")
	ErrInvaliPhoneNumber          = errors.New("invalid phone number")
	ErrInvalidSession             = errors.New("token is invalid or user logged out")
)

type (
	authService struct {
		profileRepo   persist.ProfileRepo
		eventRepo     persist.EventRepo
		mqclient      mq.MQClient
		authenticator SessionAuthenticator
		cache         Cache
	}
)

func (auth *authService) SignupWithPhoneNumber(
	ctx context.Context,
	req *connect.Request[authv1.SignupWithPhoneNumberRequest],
) (*connect.Response[authv1.SignupWithPhoneNumberResponse], error) {
	profileID, err := auth.profileRepo.Create(req.Msg.GetPhoneNumber(), req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if valid := validatePhoneNumber(req.Msg.PhoneNumber); !valid {
		return nil, connect.NewError(connect.CodeInvalidArgument, ErrInvaliPhoneNumber)
	}

	c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	auth.mqclient.Publish(c, req.Msg.GetPhoneNumber())

	return connect.NewResponse(&authv1.SignupWithPhoneNumberResponse{
		Id:          profileID,
		PhoneNumber: req.Msg.GetPhoneNumber(),
		IsVerified:  false,
	}), nil
}

func (auth *authService) VerifyPhoneNumber(
	ctx context.Context,
	req *connect.Request[authv1.VerifyPhoneNumberRequest],
) (*connect.Response[authv1.VerifyPhoneNumberResponse], error) {
	profile, err := auth.profileRepo.Get(req.Msg.GetPhoneNumber())
	if err != nil || profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, ErrProfileNotFound)
	}

	if profile.IsVerified {
		return nil, connect.NewError(connect.CodeInvalidArgument, ErrPhoneNumberAlreadyVerified)
	}

	if profile.Otp != req.Msg.Otp {
		return nil, connect.NewError(connect.CodeInvalidArgument, ErrIncorrectOtp)
	}

	if err := auth.profileRepo.SetOTPVerified(req.Msg.GetPhoneNumber()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&authv1.VerifyPhoneNumberResponse{
		Verified: true,
	}), nil
}

func (auth *authService) LoginWithPhoneNumber(
	ctx context.Context,
	req *connect.Request[authv1.LoginWithPhoneNumberRequest],
) (*connect.Response[authv1.LoginWithPhoneNumberResponse], error) {
	profile, err := auth.profileRepo.Get(req.Msg.GetPhoneNumber())
	if err != nil || profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, ErrProfileNotFound)
	}

	if !profile.IsVerified {
		return nil, connect.NewError(connect.CodeInternal, ErrPhoneNumberNotVerified)
	}

	if profile.Otp != req.Msg.Otp {
		return nil, connect.NewError(connect.CodeInvalidArgument, ErrIncorrectOtp)
	}

	token, err := auth.authenticator.GenerateToken(req.Msg.PhoneNumber)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, ErrGenerateTokenFailed)
	}

	log.Println("event repo setting", profile.PhoneNumber)

	if _, err := auth.eventRepo.Create(profile, EventTypeLogin); err != nil {
		log.Println("event repo error", err)
		log.Printf("failed to create event log for phone number:%s, event: %s\n", profile.PhoneNumber, EventTypeLogin)
	}

	auth.cache.Set(req.Msg.PhoneNumber) // logged in users cache

	return connect.NewResponse(&authv1.LoginWithPhoneNumberResponse{
		SessionToken: token,
	}), nil
}

func (auth *authService) GetProfile(
	ctx context.Context,
	req *connect.Request[authv1.GetProfileRequest],
) (*connect.Response[authv1.GetProfileResponse], error) {
	loggedInUserPhoneNumber := req.Header().Get(PhoneNumberHeader)

	if !auth.cache.Get(loggedInUserPhoneNumber) {
		return nil, connect.NewError(connect.CodeUnauthenticated, ErrInvalidSession)
	}

	profile, err := auth.profileRepo.Get(loggedInUserPhoneNumber)
	if err != nil || profile == nil {
		return nil, connect.NewError(connect.CodeNotFound, ErrProfileNotFound)
	}

	return connect.NewResponse(&authv1.GetProfileResponse{
		Id:          profile.ID,
		Name:        profile.Name,
		PhoneNumber: profile.PhoneNumber,
		IsVerified:  profile.IsVerified,
		CreatedAt:   profile.CreatedAt.String(),
	}), nil
}

func (auth *authService) Logout(
	ctx context.Context,
	req *connect.Request[authv1.LogoutRequest],
) (*connect.Response[authv1.LogoutResponse], error) {
	loggedInUserPhoneNumber := req.Header().Get(PhoneNumberHeader)

	if loggedInUserPhoneNumber == "" {
		return connect.NewResponse(&authv1.LogoutResponse{
			Message: "user is already logged out, invalid token",
		}), nil
	}

	auth.cache.Remove(loggedInUserPhoneNumber) // remove user from logged in users cache

	// log the logout event
	auth.eventRepo.Create(&models.Profile{PhoneNumber: loggedInUserPhoneNumber}, EventTypeLogout)

	return connect.NewResponse(&authv1.LogoutResponse{
		Message: fmt.Sprintf("user with phone number: %s logged out successfully.", loggedInUserPhoneNumber),
	}), nil
}

func NewAuthService(
	profileRepo persist.ProfileRepo,
	eventRepo persist.EventRepo,
	publisher mq.MQClient,
	authenticator SessionAuthenticator,
	cache Cache,
) *authService {
	return &authService{profileRepo, eventRepo, publisher, authenticator, cache}
}

func validatePhoneNumber(phoneNumber string) bool {
	pattern := `^\+\d{1,3}\d{10}$`
	regexpattern := regexp.MustCompile(pattern)
	return regexpattern.MatchString(phoneNumber)
}
