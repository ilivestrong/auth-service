package internal

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/ilivestrong/auth-service/internal/persist"
	authv1 "github.com/ilivestrong/auth-service/internal/protos/gen/auth/v1"
	mq "github.com/ilivestrong/auth-service/internal/rabbitmq"
)

const (
	SendOTPMessage = "SendOTP"
)

var (
	ErrPhoneNumberAlreadyVerified = errors.New("this phone number is already verified")
	ErrPhoneNumberNotVerified     = errors.New("this phone number is not verified yet")
	ErrIncorrectOtp               = errors.New("otp provided is incorrect")
	ErrProfileNotFound            = errors.New("failed to find profile")
	ErrGenerateTokenFailed        = errors.New("failed to generate session token")
)

type (
	authService struct {
		profileRepo   persist.ProfileRepo
		mqclient      mq.MQClient
		authenticator SessionAuthenticator
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

	return connect.NewResponse(&authv1.LoginWithPhoneNumberResponse{
		SessionToken: token,
	}), nil
}

func (auth *authService) GetProfile(
	ctx context.Context,
	req *connect.Request[authv1.GetProfileRequest],
) (*connect.Response[authv1.GetProfileResponse], error) {
	profile, err := auth.profileRepo.Get(req.Header().Get("x-phone-number"))
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

func NewAuthService(repository persist.ProfileRepo, publisher mq.MQClient, authenticator SessionAuthenticator) *authService {
	return &authService{repository, publisher, authenticator}
}
