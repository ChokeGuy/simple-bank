package user

import (
	"context"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

const (
	grpcUserAgentHeader = "grpcgateway-user-agent"
	userAgentHeader     = "user-agent"
	xForwardedForHeader = "x-forwarded-for"
)

type Metadata struct {
	UserClient string
	ClientIP   string
}

func (u *UserHandler) extractMetadata(ctx context.Context) *Metadata {
	_metadata := &Metadata{}

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if userAgent := md.Get(grpcUserAgentHeader); len(userAgent) > 0 {
			_metadata.UserClient = userAgent[0]
		}

		if userAgent := md.Get(userAgentHeader); len(userAgent) > 0 {
			_metadata.UserClient = userAgent[0]
		}

		if clientIP := md.Get(xForwardedForHeader); len(clientIP) > 0 {
			_metadata.ClientIP = clientIP[0]
		}
	}

	if p, ok := peer.FromContext(ctx); ok {
		_metadata.ClientIP = p.Addr.String()
	}

	return _metadata
}
