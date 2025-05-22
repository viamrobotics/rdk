package zeroconf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/edaniels/golog"
	"github.com/miekg/dns"
	"go.uber.org/multierr"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// IPType specifies the IP traffic the client listens for.
// This does not guarantee that only mDNS entries of this sepcific
// type passes. E.g. typical mDNS packets distributed via IPv4, often contain
// both DNS A and AAAA entries.
type IPType uint8

// Options for IPType.
const (
	IPv4        = 0x01
	IPv6        = 0x02
	IPv4AndIPv6 = (IPv4 | IPv6) //< Default option.
)

type clientOpts struct {
	listenOn   IPType
	acceptOnly IPType
	ifaces     []net.Interface
}

// ClientOption fills the option struct to configure intefaces, etc.
type ClientOption func(*clientOpts)

// SelectIPTraffic selects the type of IP packets (IPv4, IPv6, or both) this
// instance listens for.
// This does not guarantee that only mDNS entries of this sepcific
// type passes. E.g. typical mDNS packets distributed via IPv4, may contain
// both DNS A and AAAA entries.
func SelectIPTraffic(t IPType) ClientOption {
	return func(o *clientOpts) {
		o.listenOn = t
	}
}

// SelectIPRecordType selects the type of IPs entries should only contain.
func SelectIPRecordType(t IPType) ClientOption {
	return func(o *clientOpts) {
		o.acceptOnly = t
	}
}

// SelectIfaces selects the interfaces to query for mDNS records
func SelectIfaces(ifaces []net.Interface) ClientOption {
	return func(o *clientOpts) {
		o.ifaces = ifaces
	}
}

// Resolver acts as entry point for service lookups and to browse the DNS-SD.
type Resolver struct {
	c *client

	shutdownCtx       context.Context
	shutdownCtxCancel func()
	shutdownLock      sync.Mutex
	shutdownEnd       *sync.WaitGroup
	isShutdown        bool
}

// NewResolver creates a new resolver and joins the UDP multicast groups to
// listen for mDNS messages.
func NewResolver(logger golog.Logger, options ...ClientOption) (*Resolver, error) {
	// Apply default configuration and load supplied options.
	var conf = clientOpts{
		listenOn:   IPv4AndIPv6,
		acceptOnly: IPv4AndIPv6,
	}
	for _, o := range options {
		if o != nil {
			o(&conf)
		}
	}

	shutdownCtx, shutdownCtxCancel := context.WithCancel(context.Background())
	var shutdownEnd sync.WaitGroup
	c, err := newClient(shutdownCtx, &shutdownEnd, conf, logger)
	if err != nil {
		shutdownCtxCancel()
		return nil, err
	}
	return &Resolver{
		c:                 c,
		shutdownCtx:       shutdownCtx,
		shutdownCtxCancel: shutdownCtxCancel,
		shutdownEnd:       &shutdownEnd,
	}, nil
}

// Browse for all services of a given type in a given domain.
func (r *Resolver) Browse(ctx context.Context, service, domain string, entries chan<- *ServiceEntry) error {
	params := defaultParams("", service, "local")
	if domain != "" {
		params.Domain = domain
	}
	params.Entries = entries
	params.isBrowsing = true

	return r.startClient(ctx, params)
}

func (r *Resolver) startClient(ctx context.Context, params *lookupParams) error {
	ctx, cancel := context.WithCancel(ctx)
	r.c.mainloop(ctx, params)
	err := r.c.query(params)
	if err != nil {
		// cancel mainloop
		cancel()
		return err
	}
	// If previous probe was ok, it should be fine now. In case of an error later on,
	// the entries' queue is closed.
	r.shutdownEnd.Add(1)
	managedGo(func() {
		if err := r.c.periodicQuery(ctx, params); err != nil {
			cancel()
		}
	}, r.shutdownEnd.Done)

	return nil
}

// Lookup a specific service by its name and type in a given domain.
func (r *Resolver) Lookup(ctx context.Context, instance, service, domain string, entries chan<- *ServiceEntry) error {
	params := defaultParams(instance, service, domain)
	params.Entries = entries
	ctx, cancel := context.WithCancel(ctx)
	panicCapturingGo(func() { r.c.mainloop(ctx, params) })
	err := r.c.query(params)
	if err != nil {
		// cancel mainloop
		cancel()
		return err
	}
	// If previous probe was ok, it should be fine now. In case of an error later on,
	// the entries' queue is closed.
	panicCapturingGo(func() {
		if err := r.c.periodicQuery(ctx, params); err != nil {
			cancel()
		}
	})

	return nil
}

// Shutdown closes the client and waits for all goroutines to finish.
func (r *Resolver) Shutdown() {
	r.shutdownLock.Lock()
	defer r.shutdownLock.Unlock()
	if r.isShutdown {
		return
	}

	r.shutdownCtxCancel()

	r.c.shutdown()

	// Wait for goroutines to finish.
	r.shutdownEnd.Wait()
	r.isShutdown = true
}

// defaultParams returns a default set of QueryParams.
func defaultParams(instance, service, domain string) *lookupParams {
	return newLookupParams(instance, service, domain, false, make(chan *ServiceEntry))
}

// Client structure encapsulates both IPv4/IPv6 UDP connections.
type client struct {
	shutdownCtx       context.Context
	shutdownEnd       *sync.WaitGroup
	ipv4conn          *ipv4.PacketConn
	ipv4Ifaces        []net.Interface
	ipv6conn          *ipv6.PacketConn
	ipv6Ifaces        []net.Interface
	acceptOnly        IPType
	logger            golog.Logger
	inboundBufferSize int
}

var errNoPositiveMTUFound = errors.New("no positive MTU found")

// Client structure constructor
func newClient(
	shutdownCtx context.Context,
	shutdownEnd *sync.WaitGroup,
	opts clientOpts,
	logger golog.Logger,
) (*client, error) {
	ifaces := opts.ifaces
	if len(ifaces) == 0 {
		ifaces = listMulticastInterfaces()
	}
	// IPv4 interfaces
	var ipv4conn *ipv4.PacketConn
	var ipv4Ifaces []net.Interface
	var ipv4Err, ipv6Err error
	if (opts.listenOn & IPv4) > 0 {
		ipv4conn, ipv4Ifaces, ipv4Err = joinUdp4Multicast(ifaces)
		if opts.listenOn == IPv4 && ipv4Err != nil {
			return nil, ipv4Err
		}
	}
	// IPv6 interfaces
	var ipv6conn *ipv6.PacketConn
	var ipv6Ifaces []net.Interface
	if (opts.listenOn & IPv6) > 0 {
		ipv6conn, ipv6Ifaces, ipv6Err = joinUdp6Multicast(ifaces)
		if opts.listenOn == IPv6 && ipv6Err != nil {
			return nil, ipv6Err
		}
	}

	if opts.listenOn == IPv4AndIPv6 && ipv4conn == nil && ipv6conn == nil {
		return nil, fmt.Errorf("failed to listen on a socket; error: %w", multierr.Combine(ipv4Err, ipv6Err))
	}

	inboundBufferSize := 0
	for _, ifc := range ipv4Ifaces {
		if ifc.MTU > inboundBufferSize {
			inboundBufferSize = ifc.MTU
		}
	}
	for _, ifc := range ipv6Ifaces {
		if ifc.MTU > inboundBufferSize {
			inboundBufferSize = ifc.MTU
		}
	}
	if inboundBufferSize == 0 {
		return nil, errNoPositiveMTUFound
	}

	return &client{
		shutdownCtx: shutdownCtx,
		shutdownEnd: shutdownEnd,
		ipv4conn:    ipv4conn,
		ipv4Ifaces:  ipv4Ifaces,
		ipv6conn:    ipv6conn,
		ipv6Ifaces:  ipv6Ifaces,
		acceptOnly:  opts.acceptOnly,
		logger:      logger,

		// https://www.rfc-editor.org/rfc/rfc6762.html#section-17
		// Multicast DNS messages carried by UDP may be up to the IP MTU of the
		// physical interface, less the space required for the IP header (20
		// bytes for IPv4; 40 bytes for IPv6) and the UDP header (8 bytes).
		inboundBufferSize: inboundBufferSize - 20 - 8,
	}, nil
}

// Start listeners and waits for the shutdown signal from exit channel
func (c *client) mainloop(ctx context.Context, params *lookupParams) {
	// start listening for responses
	msgCh := make(chan *dns.Msg, 32)
	if c.ipv4conn != nil {
		c.shutdownEnd.Add(1)
		managedGo(func() { c.recv(ctx, c.ipv4conn, msgCh) }, c.shutdownEnd.Done)
	}
	if c.ipv6conn != nil {
		c.shutdownEnd.Add(1)
		managedGo(func() { c.recv(ctx, c.ipv6conn, msgCh) }, c.shutdownEnd.Done)
	}

	c.shutdownEnd.Add(1)
	managedGo(func() {
		// Iterate through channels from listeners goroutines
		var entries, sentEntries map[string]*ServiceEntry
		sentEntries = make(map[string]*ServiceEntry)
		for {
			select {
			case <-c.shutdownCtx.Done():
				// Context expired. Notify subscriber that we are done here.
				params.done()
				return
			case <-ctx.Done():
				// Context expired. Notify subscriber that we are done here.
				params.done()
				return
			case msg := <-msgCh:
				entries = make(map[string]*ServiceEntry)
				sections := append(msg.Answer, msg.Ns...)
				sections = append(sections, msg.Extra...)

				for _, answer := range sections {
					switch rr := answer.(type) {
					case *dns.PTR:
						if params.ServiceName() != rr.Hdr.Name {
							continue
						}
						if params.ServiceInstanceName() != "" && params.ServiceInstanceName() != rr.Ptr {
							continue
						}
						if _, ok := entries[rr.Ptr]; !ok {
							entries[rr.Ptr] = NewServiceEntry(
								trimDot(strings.Replace(rr.Ptr, rr.Hdr.Name, "", -1)),
								params.Service,
								params.Domain)
						}
						entries[rr.Ptr].TTL = rr.Hdr.Ttl
					case *dns.SRV:
						if params.ServiceInstanceName() != "" && params.ServiceInstanceName() != rr.Hdr.Name {
							continue
						} else if !strings.HasSuffix(rr.Hdr.Name, params.ServiceName()) {
							continue
						}
						if _, ok := entries[rr.Hdr.Name]; !ok {
							entries[rr.Hdr.Name] = NewServiceEntry(
								trimDot(strings.Replace(rr.Hdr.Name, params.ServiceName(), "", 1)),
								params.Service,
								params.Domain)
						}
						entries[rr.Hdr.Name].HostName = rr.Target
						entries[rr.Hdr.Name].Port = int(rr.Port)
						entries[rr.Hdr.Name].TTL = rr.Hdr.Ttl
					case *dns.TXT:
						if params.ServiceInstanceName() != "" && params.ServiceInstanceName() != rr.Hdr.Name {
							continue
						} else if !strings.HasSuffix(rr.Hdr.Name, params.ServiceName()) {
							continue
						}
						if _, ok := entries[rr.Hdr.Name]; !ok {
							entries[rr.Hdr.Name] = NewServiceEntry(
								trimDot(strings.Replace(rr.Hdr.Name, params.ServiceName(), "", 1)),
								params.Service,
								params.Domain)
						}
						entries[rr.Hdr.Name].Text = rr.Txt
						entries[rr.Hdr.Name].TTL = rr.Hdr.Ttl
					}
				}
				// Associate IPs in a second round as other fields should be filled by now.
				for _, answer := range sections {
					switch rr := answer.(type) {
					case *dns.A:
						if (c.acceptOnly & IPv4) > 0 {
							for k, e := range entries {
								if e.HostName == rr.Hdr.Name {
									entries[k].AddrIPv4 = append(entries[k].AddrIPv4, rr.A)
								}
							}
						}
					case *dns.AAAA:
						if (c.acceptOnly & IPv6) > 0 {
							for k, e := range entries {
								if e.HostName == rr.Hdr.Name {
									entries[k].AddrIPv6 = append(entries[k].AddrIPv6, rr.AAAA)
								}
							}
						}
					}
				}
			}

			if len(entries) > 0 {
				for k, e := range entries {
					if e.TTL == 0 {
						delete(entries, k)
						delete(sentEntries, k)
						continue
					}
					if _, ok := sentEntries[k]; ok {
						continue
					}

					// If this is an DNS-SD query do not throw PTR away.
					// It is expected to have only PTR for enumeration
					if params.ServiceRecord.ServiceTypeName() != params.ServiceRecord.ServiceName() {
						// Require at least one resolved IP address for ServiceEntry
						// TODO: wait some more time as chances are high both will arrive.
						if len(e.AddrIPv4) == 0 && len(e.AddrIPv6) == 0 {
							continue
						}
					}
					// Submit entry to subscriber and cache it.
					// This is also a point to possibly stop probing actively for a
					// service entry.
					params.Entries <- e
					sentEntries[k] = e
					if !params.isBrowsing {
						params.disableProbing()
					}
				}
			}
		}
	}, c.shutdownEnd.Done)
}

// Shutdown client will close currently open connections and channel implicitly.
func (c *client) shutdown() {
	if c.ipv4conn != nil {
		c.ipv4conn.Close()
	}
	if c.ipv6conn != nil {
		c.ipv6conn.Close()
	}
}

// Data receiving routine reads from connection, unpacks packets into dns.Msg
// structures and sends them to a given msgCh channel
func (c *client) recv(ctx context.Context, l interface{}, msgCh chan *dns.Msg) {
	var readFrom func([]byte) (n int, src net.Addr, err error)

	switch pConn := l.(type) {
	case *ipv6.PacketConn:
		readFrom = func(b []byte) (n int, src net.Addr, err error) {
			n, _, src, err = pConn.ReadFrom(b)
			return
		}
	case *ipv4.PacketConn:
		readFrom = func(b []byte) (n int, src net.Addr, err error) {
			n, _, src, err = pConn.ReadFrom(b)
			return
		}

	default:
		return
	}

	buf := make([]byte, 65536)
	var fatalErr error
	for {
		// Handles the following cases:
		// - ReadFrom aborts with error due to closed UDP connection -> causes ctx cancel
		// - ReadFrom aborts otherwise.
		// TODO: the context check can be removed. Verify!
		if ctx.Err() != nil || c.shutdownCtx.Err() != nil || fatalErr != nil {
			return
		}

		n, _, err := readFrom(buf)
		if err != nil {
			fatalErr = err
			continue
		}
		msg := new(dns.Msg)
		if err := msg.Unpack(buf[:n]); err != nil {
			// log.Printf("[WARN] mdns: Failed to unpack packet: %v", err)
			continue
		}
		select {
		case msgCh <- msg:
			// Submit decoded DNS message and continue.
		case <-ctx.Done():
			// Abort.
			return
		case <-c.shutdownCtx.Done():
			// Abort.
			return
		}
	}
}

// periodicQuery sens multiple probes until a valid response is received by
// the main processing loop or some timeout/cancel fires.
// TODO: move error reporting to shutdown function as periodicQuery is called from
// go routine context.
func (c *client) periodicQuery(ctx context.Context, params *lookupParams) error {
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 4 * time.Second
	bo.MaxInterval = 60 * time.Second
	bo.MaxElapsedTime = 0
	bo.Reset()

	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		// Backoff and cancel logic.
		wait := bo.NextBackOff()
		if wait == backoff.Stop {
			return errors.New("periodicQuery: abort due to timeout")
		}
		if timer == nil {
			timer = time.NewTimer(wait)
		} else {
			timer.Reset(wait)
		}
		select {
		case <-timer.C:
			// Wait for next iteration.
		case <-params.stopProbing:
			// Chan is closed (or happened in the past).
			// Done here. Received a matching mDNS entry.
			return nil
		case <-c.shutdownCtx.Done():
			return c.shutdownCtx.Err()
		case <-ctx.Done():
			return ctx.Err()
		}
		// Do periodic query.
		if err := c.query(params); err != nil {
			return err
		}
	}
}

// Performs the actual query by service name (browse) or service instance name (lookup),
// start response listeners goroutines and loops over the entries channel.
func (c *client) query(params *lookupParams) error {
	var serviceName, serviceInstanceName string
	serviceName = fmt.Sprintf("%s.%s.", trimDot(params.Service), trimDot(params.Domain))

	// send the query
	m := new(dns.Msg)
	if params.Instance != "" { // service instance name lookup
		serviceInstanceName = fmt.Sprintf("%s.%s", params.Instance, serviceName)
		m.Question = []dns.Question{
			{Name: serviceInstanceName, Qtype: dns.TypeSRV, Qclass: dns.ClassINET},
			{Name: serviceInstanceName, Qtype: dns.TypeTXT, Qclass: dns.ClassINET},
		}
	} else if len(params.Subtypes) > 0 { // service subtype browse
		m.SetQuestion(params.Subtypes[0], dns.TypePTR)
	} else { // service name browse
		m.SetQuestion(serviceName, dns.TypePTR)
	}
	m.RecursionDesired = false
	if err := c.sendQuery(m); err != nil {
		return err
	}

	return nil
}

// Pack the dns.Msg and write to available connections (multicast)
func (c *client) sendQuery(msg *dns.Msg) error {
	b := make([]byte, c.inboundBufferSize)
	buf, err := msg.PackBuffer(b)
	if err != nil {
		return err
	}
	if c.ipv4conn != nil {
		for ifcIdx := range c.ipv4Ifaces {
			if err := c.ipv4conn.SetMulticastInterface(&c.ipv4Ifaces[ifcIdx]); err != nil {
				c.logger.Debugw("mdns: failed to set multicast interface", "error", err)
			} else {
				c.ipv4conn.WriteTo(buf, nil, ipv4Addr)
			}
		}
	}
	if c.ipv6conn != nil {
		for ifcIdx := range c.ipv6Ifaces {
			if err := c.ipv6conn.SetMulticastInterface(&c.ipv6Ifaces[ifcIdx]); err != nil {
				c.logger.Debugw("mdns: failed to set multicast interface", "error", err)
			} else {
				c.ipv6conn.WriteTo(buf, nil, ipv6Addr)
			}
		}
	}
	return nil
}
