package check

import (
	"fmt"
	"net"
	"time"
)

func (r *Runner) CheckTCP() (redirectStatus, httpsStatus Status) {
	redirectStatus = OK
	httpsStatus = OK

	r.Verbosef("\n\033[1m== TCP Ports ==\033[0m\n")

	ports := []int{80, 443}
	for _, port := range ports {
		addr := net.JoinHostPort(r.Domain, fmt.Sprintf("%d", port))
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)

		if err != nil {
			if port == 80 {
				redirectStatus = FAIL
				r.Fail(fmt.Sprintf("TCP %s:%d unreachable", r.Domain, port))
			} else {
				httpsStatus = FAIL
				r.Fail(fmt.Sprintf("TCP %s:%d unreachable", r.Domain, port))
			}
		} else {
			conn.Close()
			r.Verbosef("\033[32mPASS\033[0m  TCP %s:%d reachable\n", r.Domain, port)
		}
	}

	return redirectStatus, httpsStatus
}
