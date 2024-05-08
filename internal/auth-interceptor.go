package internal

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
)

const tokenHeader = "authorization"

var (
	ErrInvalidToken = errors.New("invalid token provided")
	ErrTokenMissing = errors.New("no token provided")

	securedAPIs = map[string]struct{}{"GetProfile": {}}
)

func NewTokenInterceptor(auth SessionAuthenticator) connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			urlBits := strings.Split(req.Spec().Procedure, "/")
			if _, ok := securedAPIs[urlBits[len(urlBits)-1]]; !ok {
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

			req.Header().Set("x-phone-number", phoneNumber)
			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}
