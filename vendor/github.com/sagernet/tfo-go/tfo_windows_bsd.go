//go:build darwin || freebsd || windows

package tfo

import (
	"context"
	"net"
	"os"
	"syscall"
	"time"
	_ "unsafe"
)

const (
	defaultTCPKeepAlive  = 15 * time.Second
	defaultFallbackDelay = 300 * time.Millisecond
)

// Boolean to int.
func boolint(b bool) int {
	if b {
		return 1
	}
	return 0
}

// A sockaddr represents a TCP, UDP, IP or Unix network endpoint
// address that can be converted into a syscall.Sockaddr.
//
// Copied from src/net/sockaddr_posix.go
type sockaddr interface {
	net.Addr

	// family returns the platform-dependent address family
	// identifier.
	family() int

	// isWildcard reports whether the address is a wildcard
	// address.
	isWildcard() bool

	// sockaddr returns the address converted into a syscall
	// sockaddr type that implements syscall.Sockaddr
	// interface. It returns a nil interface when the address is
	// nil.
	sockaddr(family int) (syscall.Sockaddr, error)

	// toLocal maps the zero address to a local system address (127.0.0.1 or ::1)
	toLocal(net string) sockaddr
}

type tcpSockaddr net.TCPAddr

func (a *tcpSockaddr) Network() string {
	return "tcp"
}

func (a *tcpSockaddr) String() string {
	return (*net.TCPAddr)(a).String()
}

// Copied from src/net/tcpsock_posix.go
func (a *tcpSockaddr) family() int {
	if a == nil || len(a.IP) <= net.IPv4len {
		return syscall.AF_INET
	}
	if a.IP.To4() != nil {
		return syscall.AF_INET
	}
	return syscall.AF_INET6
}

// Copied from src/net/tcpsock_posix.go
func (a *tcpSockaddr) isWildcard() bool {
	if a == nil || a.IP == nil {
		return true
	}
	return a.IP.IsUnspecified()
}

//go:linkname ipToSockaddr net.ipToSockaddr
func ipToSockaddr(family int, ip net.IP, port int, zone string) (syscall.Sockaddr, error)

// Copied from src/net/tcpsock_posix.go
func (a *tcpSockaddr) sockaddr(family int) (syscall.Sockaddr, error) {
	if a == nil {
		return nil, nil
	}
	return ipToSockaddr(family, a.IP, a.Port, a.Zone)
}

//go:linkname loopbackIP net.loopbackIP
func loopbackIP(net string) net.IP

// Modified from src/net/tcpsock_posix.go
func (a *tcpSockaddr) toLocal(net string) sockaddr {
	la := *a
	la.IP = loopbackIP(net)
	return &la
}

//go:linkname favoriteAddrFamily net.favoriteAddrFamily
func favoriteAddrFamily(network string, laddr, raddr sockaddr, mode string) (family int, ipv6only bool)

func (d *Dialer) dialTFOContext(ctx context.Context, network, address string, b []byte) (net.Conn, error) {
	if ctx == nil {
		panic("nil context")
	}
	deadline := d.deadline(ctx, time.Now())
	if !deadline.IsZero() {
		if d, ok := ctx.Deadline(); !ok || deadline.Before(d) {
			subCtx, cancel := context.WithDeadline(ctx, deadline)
			defer cancel()
			ctx = subCtx
		}
	}

	var laddr *net.TCPAddr
	if d.LocalAddr != nil {
		la, ok := d.LocalAddr.(*net.TCPAddr)
		if !ok {
			return nil, &net.OpError{
				Op:     "dial",
				Net:    network,
				Source: nil,
				Addr:   nil,
				Err: &net.AddrError{
					Err:  "mismatched local address type",
					Addr: d.LocalAddr.String(),
				},
			}
		}
		laddr = la
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: err}
	}
	portNum, err := d.Resolver.LookupPort(ctx, network, port)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: err}
	}
	ipaddrs, err := d.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: err}
	}

	var addrs []net.TCPAddr

	for _, ipaddr := range ipaddrs {
		if laddr != nil && !laddr.IP.IsUnspecified() && !matchAddrFamily(laddr.IP, ipaddr.IP) {
			continue
		}
		addrs = append(addrs, net.TCPAddr{
			IP:   ipaddr.IP,
			Port: portNum,
			Zone: ipaddr.Zone,
		})
	}

	var primaries, fallbacks []net.TCPAddr
	if d.FallbackDelay >= 0 && network == "tcp" {
		primaries, fallbacks = partition(addrs, func(a net.TCPAddr) bool {
			return a.IP.To4() != nil
		})
	} else {
		primaries = addrs
	}

	var c *net.TCPConn
	if len(fallbacks) > 0 {
		c, err = d.dialParallel(ctx, network, laddr, primaries, fallbacks, b)
	} else {
		c, err = d.dialSerial(ctx, network, laddr, primaries, b)
	}
	if err != nil {
		return nil, err
	}

	if d.KeepAlive >= 0 {
		c.SetKeepAlive(true)
		ka := d.KeepAlive
		if d.KeepAlive == 0 {
			ka = defaultTCPKeepAlive
		}
		c.SetKeepAlivePeriod(ka)
	}
	return c, nil
}

// dialParallel races two copies of dialSerial, giving the first a
// head start. It returns the first established connection and
// closes the others. Otherwise it returns an error from the first
// primary address.
func (d *Dialer) dialParallel(ctx context.Context, network string, laddr *net.TCPAddr, primaries, fallbacks []net.TCPAddr, b []byte) (*net.TCPConn, error) {
	if len(fallbacks) == 0 {
		return d.dialSerial(ctx, network, laddr, primaries, b)
	}

	returned := make(chan struct{})
	defer close(returned)

	type dialResult struct {
		*net.TCPConn
		error
		primary bool
		done    bool
	}
	results := make(chan dialResult) // unbuffered

	startRacer := func(ctx context.Context, primary bool) {
		ras := primaries
		if !primary {
			ras = fallbacks
		}
		c, err := d.dialSerial(ctx, network, laddr, ras, b)
		select {
		case results <- dialResult{TCPConn: c, error: err, primary: primary, done: true}:
		case <-returned:
			if c != nil {
				c.Close()
			}
		}
	}

	var primary, fallback dialResult

	// Start the main racer.
	primaryCtx, primaryCancel := context.WithCancel(ctx)
	defer primaryCancel()
	go startRacer(primaryCtx, true)

	// Start the timer for the fallback racer.
	fallbackDelay := d.FallbackDelay
	if fallbackDelay == 0 {
		fallbackDelay = defaultFallbackDelay
	}
	fallbackTimer := time.NewTimer(fallbackDelay)
	defer fallbackTimer.Stop()

	for {
		select {
		case <-fallbackTimer.C:
			fallbackCtx, fallbackCancel := context.WithCancel(ctx)
			defer fallbackCancel()
			go startRacer(fallbackCtx, false)

		case res := <-results:
			if res.error == nil {
				return res.TCPConn, nil
			}
			if res.primary {
				primary = res
			} else {
				fallback = res
			}
			if primary.done && fallback.done {
				return nil, primary.error
			}
			if res.primary && fallbackTimer.Stop() {
				// If we were able to stop the timer, that means it
				// was running (hadn't yet started the fallback), but
				// we just got an error on the primary path, so start
				// the fallback immediately (in 0 nanoseconds).
				fallbackTimer.Reset(0)
			}
		}
	}
}

// dialSerial connects to a list of addresses in sequence, returning
// either the first successful connection, or the first error.
func (d *Dialer) dialSerial(ctx context.Context, network string, laddr *net.TCPAddr, ras []net.TCPAddr, b []byte) (*net.TCPConn, error) {
	var firstErr error // The error from the first address is most relevant.

	for i, ra := range ras {
		select {
		case <-ctx.Done():
			return nil, &net.OpError{Op: "dial", Net: network, Source: d.LocalAddr, Addr: &ra, Err: ctx.Err()}
		default:
		}

		var ddl time.Time
		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			partialDeadline, err := partialDeadline(time.Now(), deadline, len(ras)-i)
			if err != nil {
				// Ran out of time.
				if firstErr == nil {
					firstErr = &net.OpError{Op: "dial", Net: network, Source: d.LocalAddr, Addr: &ra, Err: err}
				}
				break
			}
			if partialDeadline.Before(deadline) {
				ddl = partialDeadline
			}
		}

		c, err := dialTFO(ctx, network, laddr, &ra, b, d.ctrlCtxFn())
		if err == nil {
			err = c.SetDeadline(ddl)
			return c, err
		}
		if firstErr == nil {
			firstErr = err
		}
	}

	if firstErr == nil {
		firstErr = &net.OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: errMissingAddress}
	}
	return nil, firstErr
}

func matchAddrFamily(x, y net.IP) bool {
	return x.To4() != nil && y.To4() != nil || x.To16() != nil && x.To4() == nil && y.To16() != nil && y.To4() == nil
}

// partition divides an address list into two categories, using a
// strategy function to assign a boolean label to each address.
// The first address, and any with a matching label, are returned as
// primaries, while addresses with the opposite label are returned
// as fallbacks. For non-empty inputs, primaries is guaranteed to be
// non-empty.
func partition(addrs []net.TCPAddr, strategy func(net.TCPAddr) bool) (primaries, fallbacks []net.TCPAddr) {
	var primaryLabel bool
	for i, addr := range addrs {
		label := strategy(addr)
		if i == 0 || label == primaryLabel {
			primaryLabel = label
			primaries = append(primaries, addr)
		} else {
			fallbacks = append(fallbacks, addr)
		}
	}
	return
}

func minNonzeroTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() || a.Before(b) {
		return a
	}
	return b
}

// deadline returns the earliest of:
//   - now+Timeout
//   - d.Deadline
//   - the context's deadline
//
// Or zero, if none of Timeout, Deadline, or context's deadline is set.
func (d *Dialer) deadline(ctx context.Context, now time.Time) (earliest time.Time) {
	if d.Timeout != 0 { // including negative, for historical reasons
		earliest = now.Add(d.Timeout)
	}
	if d, ok := ctx.Deadline(); ok {
		earliest = minNonzeroTime(earliest, d)
	}
	return minNonzeroTime(earliest, d.Deadline)
}

// partialDeadline returns the deadline to use for a single address,
// when multiple addresses are pending.
func partialDeadline(now, deadline time.Time, addrsRemaining int) (time.Time, error) {
	if deadline.IsZero() {
		return deadline, nil
	}
	timeRemaining := deadline.Sub(now)
	if timeRemaining <= 0 {
		return time.Time{}, os.ErrDeadlineExceeded
	}
	// Tentatively allocate equal time to each remaining address.
	timeout := timeRemaining / time.Duration(addrsRemaining)
	// If the time per address is too short, steal from the end of the list.
	const saneMinimum = 2 * time.Second
	if timeout < saneMinimum {
		if timeRemaining < saneMinimum {
			timeout = timeRemaining
		} else {
			timeout = saneMinimum
		}
	}
	return now.Add(timeout), nil
}
