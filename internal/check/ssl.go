package check

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"net"
	"time"
)

const sslExpiryWarningDays = 30

func (r *Runner) CheckSSL() *SSLResult {
	result := &SSLResult{Status: OK}

	r.Verbosef("\n\033[1m== SSL Certificate ==\033[0m\n")

	dialer := &tls.Dialer{
		Config: &tls.Config{
			ServerName: r.Domain,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := net.JoinHostPort(r.Domain, "443")
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("SSL %s — certificate chain or hostname verification failed", r.Domain))
		return result
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("SSL %s — connection is not TLS", r.Domain))
		return result
	}

	state := tlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		result.Status = FAIL
		r.Fail(fmt.Sprintf("SSL %s — no peer certificate", r.Domain))
		return result
	}

	cert := state.PeerCertificates[0]
	result.Subject = cert.Subject.CommonName
	result.Issuer = cert.Issuer.CommonName

	if result.Issuer == "" {
		if len(cert.Issuer.Organization) > 0 {
			result.Issuer = cert.Issuer.Organization[0]
		}
	}

	result.ExpiresAt = cert.NotAfter.UTC().Format(time.RFC3339)

	daysRemaining := int(math.Ceil(time.Until(cert.NotAfter).Hours() / 24))
	result.DaysRemaining = &daysRemaining

	warningSeconds := sslExpiryWarningDays * 24 * 60 * 60
	timeUntilExpiry := time.Until(cert.NotAfter)

	if timeUntilExpiry.Seconds() < float64(warningSeconds) {
		result.Status = WARN
		r.Warn(fmt.Sprintf("SSL %s — certificate expires in %d days", r.Domain, daysRemaining))
	} else {
		r.Verbosef("\033[32mPASS\033[0m  SSL %s\n", r.Domain)
	}

	r.Verbosef("      → subject: %s\n", cert.Subject.CommonName)
	r.Verbosef("      → issuer: %s\n", result.Issuer)
	r.Verbosef("      → expires: %s\n", result.ExpiresAt)

	_ = x509.Certificate{}

	return result
}
