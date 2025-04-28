package zeroconf

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/miekg/dns"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	// Number of Multicast responses sent for a query message (default: 1 < x < 9)
	multicastRepetitions = 2
)

// Register a service by given arguments. This call will take the system's hostname
// and lookup IP by that hostname.
func Register(
	instance, service, domain string,
	port int,
	text []string,
	ifaces []net.Interface,
	logger golog.Logger,
) (*Server, error) {
	return register(instance, service, domain, port, text, ifaces, false, logger)
}

// RegisterDynamic registers a service by the given arguments. This call will take the system's hostname
// and look up IPs as requests come in on a particular interface.
func RegisterDynamic(
	instance, service, domain string,
	port int,
	text []string,
	ifaces []net.Interface,
	logger golog.Logger,
) (*Server, error) {
	return register(instance, service, domain, port, text, ifaces, true, logger)
}

func register(
	instance, service, domain string,
	port int,
	text []string,
	ifaces []net.Interface,
	dynamic bool,
	logger golog.Logger,
) (*Server, error) {
	entry := NewServiceEntry(instance, service, domain)
	entry.Port = port
	entry.Text = text

	if entry.Instance == "" {
		return nil, errors.New("missing service instance name")
	}
	if entry.Service == "" {
		return nil, errors.New("missing service name")
	}
	if entry.Domain == "" {
		entry.Domain = "local."
	}
	if entry.Port == 0 {
		return nil, errors.New("missing port")
	}

	var err error
	if entry.HostName == "" {
		entry.HostName, err = os.Hostname()
		if err != nil {
			return nil, errors.New("could not determine host")
		}
	}

	if !strings.HasSuffix(trimDot(entry.HostName), entry.Domain) {
		entry.HostName = fmt.Sprintf("%s.%s.", trimDot(entry.HostName), trimDot(entry.Domain))
	}

	if len(ifaces) == 0 {
		ifaces = listMulticastInterfaces()
	}

	if !dynamic {
		for _, iface := range ifaces {
			v4, v6 := addrsForInterface(&iface)
			entry.AddrIPv4 = append(entry.AddrIPv4, v4...)
			entry.AddrIPv6 = append(entry.AddrIPv6, v6...)
		}

		if entry.AddrIPv4 == nil && entry.AddrIPv6 == nil {
			return nil, errors.New("could not determine host IP addresses")
		}
	}

	return newServerForService(entry, ifaces, logger)
}

// RegisterProxy registers a service proxy. This call will skip the hostname/IP lookup and
// will use the provided values.
func RegisterProxy(
	instance, service, domain string,
	port int,
	host string,
	ips []string,
	text []string,
	ifaces []net.Interface,
	logger golog.Logger,
) (*Server, error) {
	entry := NewServiceEntry(instance, service, domain)
	entry.Port = port
	entry.Text = text
	entry.HostName = host

	if entry.Instance == "" {
		return nil, errors.New("missing service instance name")
	}
	if entry.Service == "" {
		return nil, errors.New("missing service name")
	}
	if entry.HostName == "" {
		return nil, errors.New("missing host name")
	}
	if entry.Domain == "" {
		entry.Domain = "local"
	}
	if entry.Port == 0 {
		return nil, errors.New("missing port")
	}

	if !strings.HasSuffix(trimDot(entry.HostName), entry.Domain) {
		entry.HostName = fmt.Sprintf("%s.%s.", trimDot(entry.HostName), trimDot(entry.Domain))
	}

	for _, ip := range ips {
		ipAddr := net.ParseIP(ip)
		if ipAddr == nil {
			return nil, fmt.Errorf("failed to parse given IP: %v", ip)
		} else if ipv4 := ipAddr.To4(); ipv4 != nil {
			entry.AddrIPv4 = append(entry.AddrIPv4, ipAddr)
		} else if ipv6 := ipAddr.To16(); ipv6 != nil {
			entry.AddrIPv6 = append(entry.AddrIPv6, ipAddr)
		} else {
			return nil, fmt.Errorf("the IP is neither IPv4 nor IPv6: %#v", ipAddr)
		}
	}

	if len(ifaces) == 0 {
		ifaces = listMulticastInterfaces()
	}

	return newServerForService(entry, ifaces, logger)
}

func newServerForService(entry *ServiceEntry, ifaces []net.Interface, logger golog.Logger) (*Server, error) {
	s, err := newServer(ifaces, logger)
	if err != nil {
		return nil, err
	}

	s.service = entry
	s.startReceivers()
	s.shutdownEnd.Add(1)
	s.startupWait.Add(1)
	managedGo(s.probe, s.shutdownEnd.Done)
	s.startupWait.Wait()

	return s, nil
}

const (
	qClassCacheFlush uint16 = 1 << 15
)

// Server structure encapsulates both IPv4/IPv6 UDP connections
type Server struct {
	service              *ServiceEntry
	ipv4conn             *ipv4.PacketConn
	ipv4Ifaces           []net.Interface
	ipv6conn             *ipv6.PacketConn
	ipv6Ifaces           []net.Interface
	selectedIfaceIndexes map[int]struct{}
	logger               golog.Logger
	inboundBufferSize    int

	shutdownCtx       context.Context
	shutdownCtxCancel func()
	shutdownLock      sync.Mutex
	shutdownEnd       sync.WaitGroup
	isShutdown        bool
	ttl               uint32
	startupWait       sync.WaitGroup
}

// Constructs server structure
func newServer(ifaces []net.Interface, logger golog.Logger) (*Server, error) {
	ipv4conn, ipv4Ifaces, err4 := joinUdp4Multicast(ifaces)
	if err4 != nil {
		logger.Debugw("no suitable IPv4 interface", "error", err4.Error())
	}
	ipv6conn, ipv6Ifaces, err6 := joinUdp6Multicast(ifaces)
	if err6 != nil {
		logger.Debugw("no suitable IPv6 interface", "error", err6.Error())
	}
	if err4 != nil && err6 != nil {
		// No supported interface left.
		return nil, errors.New("no supported interface")
	}
	inboundBufferSize := 0
	selectedIfaceIndexes := map[int]struct{}{}
	for _, ifc := range ipv4Ifaces {
		selectedIfaceIndexes[ifc.Index] = struct{}{}
		if ifc.MTU > inboundBufferSize {
			inboundBufferSize = ifc.MTU
		}
	}
	for _, ifc := range ipv6Ifaces {
		selectedIfaceIndexes[ifc.Index] = struct{}{}
		if ifc.MTU > inboundBufferSize {
			inboundBufferSize = ifc.MTU
		}
	}
	if inboundBufferSize == 0 {
		return nil, errNoPositiveMTUFound
	}

	shutdownCtx, shutdownCtxCancel := context.WithCancel(context.Background())
	s := &Server{
		ipv4conn:             ipv4conn,
		ipv4Ifaces:           ipv4Ifaces,
		ipv6conn:             ipv6conn,
		ipv6Ifaces:           ipv6Ifaces,
		selectedIfaceIndexes: selectedIfaceIndexes,
		ttl:                  3200,
		shutdownCtx:          shutdownCtx,
		shutdownCtxCancel:    shutdownCtxCancel,
		logger:               logger,

		// https://www.rfc-editor.org/rfc/rfc6762.html#section-17
		// Multicast DNS messages carried by UDP may be up to the IP MTU of the
		// physical interface, less the space required for the IP header (20
		// bytes for IPv4; 40 bytes for IPv6) and the UDP header (8 bytes).
		inboundBufferSize: inboundBufferSize - 20 - 8,
	}

	return s, nil
}

// startReceivers starts both IPv4/6 receiver loops and and waits for the shutdown signal from exit channel
func (s *Server) startReceivers() {
	if s.ipv4conn != nil {
		s.startupWait.Add(1)
		s.shutdownEnd.Add(1)
		var nextInstance bool
		managedGo(func() {
			defer func() {
				nextInstance = true
			}()
			s.recv4(s.ipv4conn, !nextInstance)
		}, s.shutdownEnd.Done)
	}
	if s.ipv6conn != nil {
		s.shutdownEnd.Add(1)
		s.startupWait.Add(1)
		var nextInstance bool
		managedGo(func() {
			defer func() {
				nextInstance = true
			}()
			s.recv6(s.ipv6conn, !nextInstance)
		}, s.shutdownEnd.Done)
	}
}

// Shutdown closes all udp connections and unregisters the service
func (s *Server) Shutdown() {
	s.shutdown()
}

// SetText updates and announces the TXT records
func (s *Server) SetText(text []string) {
	s.service.Text = text
	s.announceText()
}

// TTL sets the TTL for DNS replies
func (s *Server) TTL(ttl uint32) {
	s.ttl = ttl
}

// Shutdown server will close currently open connections & channel
func (s *Server) shutdown() error {
	s.shutdownLock.Lock()
	defer s.shutdownLock.Unlock()
	if s.isShutdown {
		return errors.New("server is already shutdown")
	}

	err := s.unregister()

	s.shutdownCtxCancel()

	if s.ipv4conn != nil {
		s.ipv4conn.Close()
	}
	if s.ipv6conn != nil {
		s.ipv6conn.Close()
	}

	// Wait for connection and routines to be closed
	s.shutdownEnd.Wait()
	s.isShutdown = true

	return err
}

// recv is a long running routine to receive packets from an interface
func (s *Server) recv4(c *ipv4.PacketConn, firstInstance bool) {
	if c == nil {
		return
	}
	firstRecv := firstInstance
	buf := make([]byte, 65536)
	for {
		select {
		case <-s.shutdownCtx.Done():
			return
		default:
			if firstRecv {
				s.startupWait.Done()
				firstRecv = false
			}
			var ifIndex int
			n, cm, from, err := c.ReadFrom(buf)
			if err != nil {
				continue
			}
			if cm != nil {
				ifIndex = cm.IfIndex
			}
			_ = s.parsePacket(buf[:n], ifIndex, from)
		}
	}
}

// recv is a long running routine to receive packets from an interface
func (s *Server) recv6(c *ipv6.PacketConn, firstInstance bool) {
	if c == nil {
		return
	}
	firstRecv := firstInstance
	buf := make([]byte, 65536)
	for {
		select {
		case <-s.shutdownCtx.Done():
			return
		default:
			if firstRecv {
				s.startupWait.Done()
				firstRecv = false
			}
			var ifIndex int
			n, cm, from, err := c.ReadFrom(buf)
			if err != nil {
				continue
			}
			if cm != nil {
				ifIndex = cm.IfIndex
			}
			_ = s.parsePacket(buf[:n], ifIndex, from)
		}
	}
}

// parsePacket is used to parse an incoming packet
func (s *Server) parsePacket(packet []byte, ifIndex int, from net.Addr) error {
	var msg dns.Msg
	if err := msg.Unpack(packet); err != nil {
		// log.Printf("[ERR] zeroconf: Failed to unpack packet: %v", err)
		return err
	}
	return s.handleQuery(&msg, ifIndex, from)
}

// handleQuery is used to handle an incoming query
func (s *Server) handleQuery(query *dns.Msg, ifIndex int, from net.Addr) error {
	// Ignore questions with authoritative section for now
	if len(query.Ns) > 0 {
		return nil
	}

	// Handle each question
	var err error
	for _, q := range query.Question {
		resp := dns.Msg{}
		resp.SetReply(query)
		resp.Compress = true
		resp.RecursionDesired = false
		resp.Authoritative = true
		resp.Question = nil // RFC6762 section 6 "responses MUST NOT contain any questions"
		resp.Answer = []dns.RR{}
		resp.Extra = []dns.RR{}
		if err = s.handleQuestion(q, &resp, query, ifIndex); err != nil {
			// log.Printf("[ERR] zeroconf: failed to handle question %v: %v", q, err)
			continue
		}
		// Check if there is an answer
		if len(resp.Answer) == 0 {
			continue
		}

		if isUnicastQuestion(q) {
			// Send unicast
			if e := s.unicastResponse(&resp, ifIndex, from); e != nil {
				err = e
			}
		} else {
			// Send mulicast
			if e := s.multicastResponse(&resp, ifIndex); e != nil {
				err = e
			}
		}
	}

	return err
}

// RFC6762 7.1. Known-Answer Suppression
func isKnownAnswer(resp *dns.Msg, query *dns.Msg) bool {
	if len(resp.Answer) == 0 || len(query.Answer) == 0 {
		return false
	}

	if resp.Answer[0].Header().Rrtype != dns.TypePTR {
		return false
	}
	answer := resp.Answer[0].(*dns.PTR)

	for _, known := range query.Answer {
		hdr := known.Header()
		if hdr.Rrtype != answer.Hdr.Rrtype {
			continue
		}
		ptr := known.(*dns.PTR)
		if ptr.Ptr == answer.Ptr && hdr.Ttl >= answer.Hdr.Ttl/2 {
			// log.Printf("skipping known answer: %v", ptr)
			return true
		}
	}

	return false
}

// handleQuestion is used to handle an incoming question
func (s *Server) handleQuestion(q dns.Question, resp *dns.Msg, query *dns.Msg, ifIndex int) error {
	if s.service == nil {
		return nil
	}

	switch q.Name {
	case s.service.ServiceTypeName():
		s.serviceTypeName(resp, s.ttl)
		if isKnownAnswer(resp, query) {
			resp.Answer = nil
		}

	case s.service.ServiceName(), s.service.NonServiceHostName():
		s.composeBrowsingAnswers(resp, ifIndex)
		if isKnownAnswer(resp, query) {
			resp.Answer = nil
		}

	case s.service.ServiceInstanceName():
		s.composeLookupAnswers(resp, s.ttl, ifIndex, false)
	default:
		// handle matching subtype query
		for _, subtype := range s.service.Subtypes {
			subtype = fmt.Sprintf("%s._sub.%s", subtype, s.service.ServiceName())
			if q.Name == subtype {
				s.composeBrowsingAnswers(resp, ifIndex)
				if isKnownAnswer(resp, query) {
					resp.Answer = nil
				}
				break
			}
		}
	}

	return nil
}

func (s *Server) composeBrowsingAnswers(resp *dns.Msg, ifIndex int) {
	ptr := &dns.PTR{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceName(),
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    s.ttl,
		},
		Ptr: s.service.ServiceInstanceName(),
	}
	resp.Answer = append(resp.Answer, ptr)

	txt := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    s.ttl,
		},
		Txt: s.service.Text,
	}
	srv := &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    s.ttl,
		},
		Priority: 0,
		Weight:   0,
		Port:     uint16(s.service.Port),
		Target:   s.service.HostName,
	}
	resp.Extra = append(resp.Extra, srv, txt)

	resp.Extra = s.appendAddrs(resp.Extra, s.ttl, ifIndex, false)
}

func (s *Server) composeLookupAnswers(resp *dns.Msg, ttl uint32, ifIndex int, flushCache bool) {
	// From RFC6762
	//    The most significant bit of the rrclass for a record in the Answer
	//    Section of a response message is the Multicast DNS cache-flush bit
	//    and is discussed in more detail below in Section 10.2, "Announcements
	//    to Flush Outdated Cache Entries".
	ptr := &dns.PTR{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceName(),
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ptr: s.service.ServiceInstanceName(),
	}
	srv := &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET | qClassCacheFlush,
			Ttl:    ttl,
		},
		Priority: 0,
		Weight:   0,
		Port:     uint16(s.service.Port),
		Target:   s.service.HostName,
	}
	txt := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET | qClassCacheFlush,
			Ttl:    ttl,
		},
		Txt: s.service.Text,
	}
	dnssd := &dns.PTR{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceTypeName(),
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ptr: s.service.ServiceName(),
	}
	resp.Answer = append(resp.Answer, srv, txt, ptr, dnssd)

	for _, subtype := range s.service.Subtypes {
		resp.Answer = append(resp.Answer,
			&dns.PTR{
				Hdr: dns.RR_Header{
					Name:   subtype,
					Rrtype: dns.TypePTR,
					Class:  dns.ClassINET,
					Ttl:    ttl,
				},
				Ptr: s.service.ServiceInstanceName(),
			})
	}

	resp.Answer = s.appendAddrs(resp.Answer, ttl, ifIndex, flushCache)
}

func (s *Server) serviceTypeName(resp *dns.Msg, ttl uint32) {
	// From RFC6762
	// 9.  Service Type Enumeration
	//
	//    For this purpose, a special meta-query is defined.  A DNS query for
	//    PTR records with the name "_services._dns-sd._udp.<Domain>" yields a
	//    set of PTR records, where the rdata of each PTR record is the two-
	//    label <Service> name, plus the same domain, e.g.,
	//    "_http._tcp.<Domain>".
	dnssd := &dns.PTR{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceTypeName(),
			Rrtype: dns.TypePTR,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ptr: s.service.ServiceName(),
	}
	resp.Answer = append(resp.Answer, dnssd)
}

// Perform probing & announcement
// TODO: implement a proper probing & conflict resolution
func (s *Server) probe() {
	q := new(dns.Msg)
	q.SetQuestion(s.service.ServiceInstanceName(), dns.TypePTR)
	q.RecursionDesired = false

	srv := &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    s.ttl,
		},
		Priority: 0,
		Weight:   0,
		Port:     uint16(s.service.Port),
		Target:   s.service.HostName,
	}
	txt := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    s.ttl,
		},
		Txt: s.service.Text,
	}
	q.Ns = []dns.RR{srv, txt}

	randomizer := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < multicastRepetitions; i++ {
		if err := s.multicastResponse(q, 0); err != nil {
			s.logger.Debugw("failed to send probe", "error", err.Error())
		}
		if i == 0 {
			s.startupWait.Done()
		}
		if !selectContextOrWait(s.shutdownCtx, time.Duration(randomizer.Intn(250))*time.Millisecond) {
			return
		}
	}

	// From RFC6762
	//    The Multicast DNS responder MUST send at least two unsolicited
	//    responses, one second apart. To provide increased robustness against
	//    packet loss, a responder MAY send up to eight unsolicited responses,
	//    provided that the interval between unsolicited responses increases by
	//    at least a factor of two with every response sent.
	timeout := 1 * time.Second
	for i := 0; i < multicastRepetitions; i++ {
		for intfIndex := range s.selectedIfaceIndexes {
			resp := new(dns.Msg)
			resp.MsgHdr.Response = true
			// TODO: make response authoritative if we are the publisher
			resp.Compress = true
			resp.Answer = []dns.RR{}
			resp.Extra = []dns.RR{}
			s.composeLookupAnswers(resp, s.ttl, intfIndex, true)
			if err := s.multicastResponse(resp, intfIndex); err != nil {
				s.logger.Debugw("failed to send announcement", "error", err.Error())
			}
		}
		if !selectContextOrWait(s.shutdownCtx, timeout) {
			return
		}
		timeout *= 2
	}
}

// announceText sends a Text announcement with cache flush enabled
func (s *Server) announceText() {
	resp := new(dns.Msg)
	resp.MsgHdr.Response = true

	txt := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   s.service.ServiceInstanceName(),
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET | qClassCacheFlush,
			Ttl:    s.ttl,
		},
		Txt: s.service.Text,
	}

	resp.Answer = []dns.RR{txt}
	s.multicastResponse(resp, 0)
}

func (s *Server) unregister() error {
	resp := new(dns.Msg)
	resp.MsgHdr.Response = true
	resp.Answer = []dns.RR{}
	resp.Extra = []dns.RR{}
	s.composeLookupAnswers(resp, 0, 0, true)
	return s.multicastResponse(resp, 0)
}

func (s *Server) appendAddrs(list []dns.RR, ttl uint32, ifIndex int, flushCache bool) []dns.RR {
	v4 := s.service.AddrIPv4
	v6 := s.service.AddrIPv6
	if len(v4) == 0 && len(v6) == 0 {
		iface, _ := net.InterfaceByIndex(ifIndex)
		if iface != nil {
			a4, a6 := addrsForInterface(iface)
			v4 = append(v4, a4...)
			v6 = append(v6, a6...)
		}
	}
	if iface, _ := net.InterfaceByIndex(ifIndex); iface != nil {
		if (iface.Flags & net.FlagLoopback) > 0 {
			v4 = append(v4, net.IPv4(127, 0, 0, 1))
		}
	}
	if ttl > 0 {
		// RFC6762 Section 10 says A/AAAA records SHOULD
		// use TTL of 120s, to account for network interface
		// and IP address changes.
		ttl = 120
	}
	var cacheFlushBit uint16
	if flushCache {
		cacheFlushBit = qClassCacheFlush
	}
	for _, ipv4 := range v4 {
		a := &dns.A{
			Hdr: dns.RR_Header{
				Name:   s.service.HostName,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET | cacheFlushBit,
				Ttl:    ttl,
			},
			A: ipv4,
		}
		list = append(list, a)
	}
	for _, ipv6 := range v6 {
		aaaa := &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   s.service.HostName,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET | cacheFlushBit,
				Ttl:    ttl,
			},
			AAAA: ipv6,
		}
		list = append(list, aaaa)
	}
	return list
}

func addrsForInterface(iface *net.Interface) ([]net.IP, []net.IP) {
	var v4, v6, v6local []net.IP
	addrs, _ := iface.Addrs()
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				v4 = append(v4, ipnet.IP)
			} else {
				switch ip := ipnet.IP.To16(); ip != nil {
				case ip.IsGlobalUnicast():
					v6 = append(v6, ipnet.IP)
				case ip.IsLinkLocalUnicast():
					v6local = append(v6local, ipnet.IP)
				}
			}
		}
	}
	if len(v6) == 0 {
		v6 = v6local
	}
	return v4, v6
}

// unicastResponse is used to send a unicast response packet
func (s *Server) unicastResponse(resp *dns.Msg, ifIndex int, from net.Addr) error {
	b := make([]byte, s.inboundBufferSize)
	buf, err := resp.PackBuffer(b)
	if err != nil {
		return err
	}
	addr := from.(*net.UDPAddr)
	if addr.IP.To4() != nil {
		if ifIndex != 0 {
			var wcm ipv4.ControlMessage
			wcm.IfIndex = ifIndex
			_, err = s.ipv4conn.WriteTo(buf, &wcm, addr)
		} else {
			_, err = s.ipv4conn.WriteTo(buf, nil, addr)
		}
		return err
	} else {
		if ifIndex != 0 {
			var wcm ipv6.ControlMessage
			wcm.IfIndex = ifIndex
			_, err = s.ipv6conn.WriteTo(buf, &wcm, addr)
		} else {
			_, err = s.ipv6conn.WriteTo(buf, nil, addr)
		}
		return err
	}
}

// multicastResponse us used to send a multicast response packet
func (s *Server) multicastResponse(msg *dns.Msg, ifIndex int) error {
	b := make([]byte, s.inboundBufferSize)
	buf, err := msg.PackBuffer(b)
	if err != nil {
		return err
	}
	if s.ipv4conn != nil {
		if ifIndex != 0 {
			iface, _ := net.InterfaceByIndex(ifIndex)
			if err := s.ipv4conn.SetMulticastInterface(iface); err != nil {
				s.logger.Debugw("mdns: failed to set multicast interface", "error", err)
			} else {
				s.ipv4conn.WriteTo(buf, nil, ipv4Addr)
			}
		} else {
			for ifcIdx := range s.ipv4Ifaces {
				if err := s.ipv4conn.SetMulticastInterface(&s.ipv4Ifaces[ifcIdx]); err != nil {
					s.logger.Debugw("mdns: failed to set multicast interface", "error", err)
				} else {
					s.ipv4conn.WriteTo(buf, nil, ipv4Addr)
				}
			}
		}
	}

	if s.ipv6conn != nil {
		if ifIndex != 0 {
			iface, _ := net.InterfaceByIndex(ifIndex)
			if err := s.ipv6conn.SetMulticastInterface(iface); err != nil {
				s.logger.Debugw("mdns: failed to set multicast interface", "error", err)
			} else {
				s.ipv6conn.WriteTo(buf, nil, ipv6Addr)
			}
		} else {
			for ifcIdx := range s.ipv6Ifaces {
				if err := s.ipv6conn.SetMulticastInterface(&s.ipv6Ifaces[ifcIdx]); err != nil {
					s.logger.Debugw("mdns: failed to set multicast interface", "error", err)
				} else {
					s.ipv6conn.WriteTo(buf, nil, ipv6Addr)
				}
			}
		}
	}
	return nil
}

func isUnicastQuestion(q dns.Question) bool {
	// From RFC6762
	// 18.12.  Repurposing of Top Bit of qclass in Question Section
	//
	//    In the Question Section of a Multicast DNS query, the top bit of the
	//    qclass field is used to indicate that unicast responses are preferred
	//    for this particular question.  (See Section 5.4.)
	return q.Qclass&qClassCacheFlush != 0
}
