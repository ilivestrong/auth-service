package internal

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	Jwt_Signing_Secret_Key = "@@secret@@key@@"
)

type (
	SessionAuthenticator interface {
		GenerateToken(phoneNumber string) (string, error)
		ParseToken(token string) (string, error)
	}
	authenticator struct {
		tokenTimeoutInMins int
	}
)

func (auth *authenticator) GenerateToken(phoneNumber string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["phone_number"] = phoneNumber
	claims["exp"] = time.Now().Add(time.Minute * time.Duration(auth.tokenTimeoutInMins)).Unix()

	tokenString, err := token.SignedString([]byte(Jwt_Signing_Secret_Key))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

func (auth *authenticator) ParseToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(Jwt_Signing_Secret_Key), nil
	})
	if err != nil {
		log.Println(err)
		return "", errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		fmt.Println(err)
		return "", err
	}

	if !token.Valid {
		return "", errors.New("invalid token")
	}

	return claims["phone_number"].(string), nil
}

func NewAuthenticator(timeoutInMins int) SessionAuthenticator {
	return &authenticator{timeoutInMins}
}
