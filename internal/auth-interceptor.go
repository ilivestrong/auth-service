package internal

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
)

const (
	tokenHeader   = "authorization"
	RpcGetProfile = "GetProfile"
	RpcLogout     = "Logout"

	PhoneNumberHeader = "x-phone-number"
)

var (
	ErrInvalidToken = errors.New("invalid token provided")
	ErrTokenMissing = errors.New("no token provided")

	securedAPIs = map[string]struct{}{RpcGetProfile: {}, RpcLogout: {}}
)

func NewTokenInterceptor(auth SessionAuthenticator, cache Cache) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			urlBits := strings.Split(req.Spec().Procedure, "/")
			rpcinvoked := urlBits[len(urlBits)-1]

			if _, ok := securedAPIs[rpcinvoked]; !ok {
				return next(ctx, req)
			}

			authHeaders := req.Header().Get(tokenHeader)
			headerSlice := strings.Split(authHeaders, " ")
			if len(headerSlice) < 2 {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					ErrInvalidToken,
				)
			}

			tokenString := headerSlice[1]
			if tokenString == "" {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					ErrTokenMissing,
				)
			}

			phoneNumber, err := auth.ParseToken(tokenString)
			if err != nil {
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					err,
				)
			}

			if rpcinvoked == RpcLogout {
				if exists := cache.Get(phoneNumber); exists {
					req.Header().Set(PhoneNumberHeader, phoneNumber)
				}
			} else {
				req.Header().Set(PhoneNumberHeader, phoneNumber)
			}

			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
