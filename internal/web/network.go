package web

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/atillalab/site-health/internal/domain"
)

var lookupHostIPs = net.DefaultResolver.LookupIPAddr

func blockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsLinkLocalMulticast()
}

func validateWebDomain(raw string) (string, error) {
	host, err := domain.ValidateWebTarget(raw)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(host, ".localhost") {
		return "", errors.New("private or local targets are not allowed in web mode")
	}
	if ip := net.ParseIP(host); ip != nil && blockedIP(ip) {
		return "", errors.New("private or local targets are not allowed in web mode")
	}
	return host, nil
}

func publicWebDomain(raw string) (string, error) {
	host, err := validateWebDomain(raw)
	if err != nil {
		return "", err
	}
	if ip := net.ParseIP(host); ip != nil {
		return host, nil
	}
	addrs, err := lookupHostIPs(context.Background(), host)
	if err != nil {
		return "", err
	}
	if len(addrs) == 0 {
		return "", errors.New("target did not resolve")
	}
	for _, addr := range addrs {
		if blockedIP(addr.IP) {
			return "", errors.New("target resolves to a private or local address")
		}
	}
	return host, nil
}
