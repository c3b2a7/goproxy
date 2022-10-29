package services

import (
	"fmt"
	"github.com/c3b2a7/goproxy/utils"
	"io"
	"log"
	"net"
	"runtime/debug"
	"strconv"
	"time"
)

type HTTP struct {
	cfg        HTTPArgs
	outPool    utils.OutPool
	checker    utils.Checker
	basicAuth  utils.BasicAuth
	ipResolver utils.IPResolver
	mapping    utils.Mapping
}

func NewHTTP() Service {
	return &HTTP{
		cfg:       HTTPArgs{},
		outPool:   utils.OutPool{},
		checker:   utils.Checker{},
		basicAuth: utils.BasicAuth{},
	}
}
func (s *HTTP) InitService() {
	s.InitBasicAuth()
	if *s.cfg.Parent != "" {
		s.checker = utils.NewChecker(*s.cfg.HTTPTimeout, int64(*s.cfg.Interval), *s.cfg.Blocked, *s.cfg.Direct)
	}
}

func (s *HTTP) InitMapping() {
	s.mapping = utils.NewInMemoryMapping()
	s.ipResolver, _ = utils.NewFallBackIPResolver(*s.cfg.IPResolver...)
	logMapping := func(k, v string) {
		s.mapping.Put(k, v)
		log.Printf("detect mapping: %s -> %s", v, k)
	}
	if *s.cfg.AutoMapping {
		utils.ResolveMapping(s.ipResolver, logMapping)
		if interval := *s.cfg.CheckMappingInterval; interval > 0 {
			utils.StartMonitor(s.ipResolver, s.mapping, time.Duration(interval)*time.Second)
		}
	}
	if *s.cfg.MappingFile != "" {
		utils.UnmarshalMapping(*s.cfg.MappingFile, logMapping)
	}
}

func (s *HTTP) StopService() {
	if s.outPool.Pool != nil {
		s.outPool.Pool.ReleaseAll()
	}
}
func (s *HTTP) Start(args interface{}) (err error) {
	s.cfg = args.(HTTPArgs)
	if *s.cfg.Parent != "" {
		log.Printf("use %s parent %s", *s.cfg.ParentType, *s.cfg.Parent)
		s.InitOutConnPool()
	}
	if *s.cfg.AutoMapping || *s.cfg.MappingFile != "" {
		s.InitMapping()
	}

	s.InitService()

	host, port, _ := net.SplitHostPort(*s.cfg.Local)
	p, _ := strconv.Atoi(port)
	sc := utils.NewServerChannel(host, p)
	if *s.cfg.LocalType == TYPE_TCP {
		err = sc.ListenTCP(s.callback)
	} else {
		err = sc.ListenTls(s.cfg.CertBytes, s.cfg.KeyBytes, s.callback)
	}
	if err != nil {
		return
	}
	log.Printf("%s http(s) proxy on %s", *s.cfg.LocalType, (*sc.Listener).Addr())
	return
}

func (s *HTTP) Clean() {
	s.StopService()
}
func (s *HTTP) callback(inConn net.Conn) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("http(s) conn handler crashed with err : %s \nstack: %s", err, string(debug.Stack()))
		}
	}()
	req, err := utils.NewHTTPRequest(&inConn, 4096, s.IsBasicAuth(), &s.basicAuth)
	if err != nil {
		if err != io.EOF {
			log.Printf("decoder error, form %s, ERR:%s", inConn.RemoteAddr(), err)
		}
		utils.CloseConn(&inConn)
		return
	}
	address := req.Host

	useProxy := true
	if *s.cfg.Parent == "" {
		useProxy = false
	} else if *s.cfg.Always {
		useProxy = true
	} else {
		if req.IsHTTPS() {
			s.checker.Add(address, true, req.Method, "", nil)
		} else {
			s.checker.Add(address, false, req.Method, req.URL, req.HeadBuf)
		}
		useProxy, _, _ = s.checker.IsBlocked(req.Host)
	}
	err = s.OutToTCP(useProxy, address, &inConn, &req)
	if err != nil {
		if *s.cfg.Parent == "" {
			log.Printf("connect to %s fail, err: %s", address, err)
		} else {
			log.Printf("connect to %s parent %s fail, err: %s", *s.cfg.ParentType, *s.cfg.Parent, err)
		}
		utils.CloseConn(&inConn)
	}
}
func (s *HTTP) OutToTCP(useProxy bool, address string, inConn *net.Conn, req *utils.HTTPRequest) (err error) {
	inAddr := (*inConn).RemoteAddr().String()
	inLocalAddr := (*inConn).LocalAddr().String()
	if s.IsDeadLoop(inLocalAddr, req.Host) {
		utils.CloseConn(inConn)
		err = fmt.Errorf("dead loop detected , %s", req.Host)
		return
	}
	var outConn net.Conn
	var _outConn interface{}
	if useProxy {
		_outConn, err = s.outPool.Pool.Get()
		if err == nil {
			outConn = _outConn.(net.Conn)
		}
	} else {
		var laddr string
		if *s.cfg.MagicHeader != "" {
			if outbound, _ := req.GetHeader(*s.cfg.MagicHeader); outbound != "" {
				req.DelHeader(*s.cfg.MagicHeader)
				if laddr = s.mapping.Get(outbound); laddr == "" {
					return fmt.Errorf("no mapping for outbound: %s", outbound)
				}
			} else {
				return fmt.Errorf("not found %s in request headers", *s.cfg.MagicHeader)
			}
		}
		if laddr != "" {
			timeout := time.Duration(*s.cfg.Timeout) * time.Millisecond
			outConn, err = utils.ConnectHostWithLAddr(address, laddr+":0", timeout)
		} else {
			outConn, err = utils.ConnectHost(address, *s.cfg.Timeout)
		}
	}
	if err != nil {
		return
	}

	outAddr := outConn.RemoteAddr().String()
	outLocalAddr := outConn.LocalAddr().String()

	if req.IsHTTPS() && !useProxy {
		req.HTTPSReply()
	} else {
		req.RemoveHopByHopHeaders()
		outConn.Write(req.HeadBuf)
	}
	utils.IoBind(*inConn, outConn, func(isSrcErr bool, err error) {
		log.Printf("conn %s - %s - %s - %s released [%s]", inAddr, inLocalAddr, outLocalAddr, outAddr, req.Host)
		utils.CloseConn(inConn)
		utils.CloseConn(&outConn)
	}, func(n int, d bool) {}, 0)
	log.Printf("conn %s - %s - %s - %s connected [%s]", inAddr, inLocalAddr, outLocalAddr, outAddr, req.Host)
	return
}
func (s *HTTP) OutToUDP(inConn *net.Conn) (err error) {
	return
}
func (s *HTTP) InitOutConnPool() {
	if *s.cfg.ParentType == TYPE_TLS || *s.cfg.ParentType == TYPE_TCP {
		//dur int, isTLS bool, certBytes, keyBytes []byte,
		//parent string, timeout int, InitialCap int, MaxCap int
		s.outPool = utils.NewOutPool(
			*s.cfg.CheckParentInterval,
			*s.cfg.ParentType == TYPE_TLS,
			s.cfg.CertBytes, s.cfg.KeyBytes,
			*s.cfg.Parent,
			*s.cfg.Timeout,
			*s.cfg.PoolSize,
			*s.cfg.PoolSize*2,
		)
	}
}
func (s *HTTP) InitBasicAuth() (err error) {
	s.basicAuth = utils.NewBasicAuth()
	if *s.cfg.AuthFile != "" {
		var n = 0
		n, err = s.basicAuth.AddFromFile(*s.cfg.AuthFile)
		if err != nil {
			err = fmt.Errorf("auth-file ERR:%s", err)
			return
		}
		log.Printf("auth data added from file %d , total:%d", n, s.basicAuth.Total())
	}
	if len(*s.cfg.Auth) > 0 {
		n := s.basicAuth.Add(*s.cfg.Auth)
		log.Printf("auth data added %d, total:%d", n, s.basicAuth.Total())
	}
	return
}
func (s *HTTP) IsBasicAuth() bool {
	return *s.cfg.AuthFile != "" || len(*s.cfg.Auth) > 0
}
func (s *HTTP) IsDeadLoop(inLocalAddr string, host string) bool {
	inIP, inPort, err := net.SplitHostPort(inLocalAddr)
	if err != nil {
		return false
	}
	outDomain, outPort, err := net.SplitHostPort(host)
	if err != nil {
		return false
	}
	if inPort == outPort {
		var outIPs []net.IP
		outIPs, err = net.LookupIP(outDomain)
		if err == nil {
			for _, ip := range outIPs {
				if ip.String() == inIP || s.mapping.Get(ip.String()) != "" {
					return true
				}
			}
		}
		interfaceIPs, err := utils.GetAllInterfaceAddr()
		if err == nil {
			for _, localIP := range interfaceIPs {
				for _, outIP := range outIPs {
					if localIP.Equal(outIP) {
						return true
					}
				}
			}
		}
	}
	return false
}
