package utils

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

func IoBind(dst io.ReadWriter, src io.ReadWriter, fn func(isSrcErr bool, err error), cfn func(count int, isPositive bool), bytesPreSec float64) {
	var one = &sync.Once{}
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Printf("IoBind crashed , err : %s , \ntrace:%s", e, string(debug.Stack()))
			}
		}()
		var err error
		var isSrcErr bool
		if bytesPreSec > 0 {
			newreader := NewReader(src)
			newreader.SetRateLimit(bytesPreSec)
			_, isSrcErr, err = ioCopy(dst, newreader, func(c int) {
				cfn(c, false)
			})

		} else {
			_, isSrcErr, err = ioCopy(dst, src, func(c int) {
				cfn(c, false)
			})
		}
		if err != nil {
			one.Do(func() {
				fn(isSrcErr, err)
			})
		}
	}()
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Printf("IoBind crashed , err : %s , \ntrace:%s", e, string(debug.Stack()))
			}
		}()
		var err error
		var isSrcErr bool
		if bytesPreSec > 0 {
			newReader := NewReader(dst)
			newReader.SetRateLimit(bytesPreSec)
			_, isSrcErr, err = ioCopy(src, newReader, func(c int) {
				cfn(c, true)
			})
		} else {
			_, isSrcErr, err = ioCopy(src, dst, func(c int) {
				cfn(c, true)
			})
		}
		if err != nil {
			one.Do(func() {
				fn(isSrcErr, err)
			})
		}
	}()
}
func ioCopy(dst io.Writer, src io.Reader, fn ...func(count int)) (written int64, isSrcErr bool, err error) {
	buf := make([]byte, 32*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
				if len(fn) == 1 {
					fn[0](nw)
				}
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			err = er
			isSrcErr = true
			break
		}
	}
	return written, isSrcErr, err
}
func TlsConnectHost(host string, timeout int, certBytes, keyBytes []byte) (conn tls.Conn, err error) {
	h := strings.Split(host, ":")
	port, _ := strconv.Atoi(h[1])
	return TlsConnect(h[0], port, timeout, certBytes, keyBytes)
}

func TlsConnect(host string, port, timeout int, certBytes, keyBytes []byte) (conn tls.Conn, err error) {
	conf, err := getRequestTlsConfig(certBytes, keyBytes)
	if err != nil {
		return
	}
	_conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Duration(timeout)*time.Millisecond)
	if err != nil {
		return
	}
	return *tls.Client(_conn, conf), err
}
func getRequestTlsConfig(certBytes, keyBytes []byte) (conf *tls.Config, err error) {
	var cert tls.Certificate
	cert, err = tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return
	}
	serverCertPool := x509.NewCertPool()
	ok := serverCertPool.AppendCertsFromPEM(certBytes)
	if !ok {
		err = errors.New("failed to parse root certificate")
	}
	conf = &tls.Config{
		RootCAs:            serverCertPool,
		Certificates:       []tls.Certificate{cert},
		ServerName:         "proxy",
		InsecureSkipVerify: false,
	}
	return
}

func ConnectHost(hostAndPort string, timeout int) (conn net.Conn, err error) {
	conn, err = net.DialTimeout("tcp", hostAndPort, time.Duration(timeout)*time.Millisecond)
	return
}

func ConnectHostWithLAddr(hostAndPort string, laddr string, timeout int) (conn net.Conn, err error) {
	dialer := newDialer(laddr, time.Duration(timeout)*time.Millisecond)
	return dialer.Dial("tcp", hostAndPort)
}

func newDialer(laddr string, timeout time.Duration) *net.Dialer {
	localAddr, _ := net.ResolveTCPAddr("tcp", laddr)
	return &net.Dialer{
		Timeout:   timeout,
		LocalAddr: localAddr,
	}
}

func ListenTls(ip string, port int, certBytes, keyBytes []byte) (ln *net.Listener, err error) {
	var cert tls.Certificate
	cert, err = tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return
	}
	clientCertPool := x509.NewCertPool()
	ok := clientCertPool.AppendCertsFromPEM(certBytes)
	if !ok {
		err = errors.New("failed to parse root certificate")
	}
	config := &tls.Config{
		ClientCAs:    clientCertPool,
		ServerName:   "proxy",
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	_ln, err := tls.Listen("tcp", fmt.Sprintf("%s:%d", ip, port), config)
	if err == nil {
		ln = &_ln
	}
	return
}
func PathExists(_path string) bool {
	_, err := os.Stat(_path)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}
func HTTPGet(URL string, timeout int) (err error) {
	tr := &http.Transport{}
	var resp *http.Response
	var client *http.Client
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		tr.CloseIdleConnections()
	}()
	client = &http.Client{Timeout: time.Millisecond * time.Duration(timeout), Transport: tr}
	resp, err = client.Get(URL)
	if err != nil {
		return
	}
	return
}

func CloseConn(conn *net.Conn) {
	if conn != nil && *conn != nil {
		(*conn).SetDeadline(time.Now().Add(time.Millisecond))
		(*conn).Close()
	}
}
func Keygen() (err error) {
	cmd := exec.Command("sh", "-c", "openssl genrsa -out proxy.key 2048")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("err:%s", err)
		return
	}
	fmt.Println(string(out))
	cmd = exec.Command("sh", "-c", `openssl req -new -key proxy.key -x509 -days 3650 -out proxy.crt -subj /C=CN/ST=BJ/O="Localhost Ltd"/CN=proxy`)
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Printf("err:%s", err)
		return
	}
	fmt.Println(string(out))
	return
}
func GetAllInterfaceAddr() ([]net.IP, error) {

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	addresses := []net.IP{}
	for _, iface := range ifaces {

		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		// if iface.Flags&net.FlagLoopback != 0 {
		// 	continue // loopback interface
		// }
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			// if ip == nil || ip.IsLoopback() {
			// 	continue
			// }
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			addresses = append(addresses, ip)
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no address Found, net.InterfaceAddrs: %v", addresses)
	}
	//only need first
	return addresses, nil
}
func UDPPacket(srcAddr string, packet []byte) []byte {
	addrBytes := []byte(srcAddr)
	addrLength := uint16(len(addrBytes))
	bodyLength := uint16(len(packet))
	pkg := new(bytes.Buffer)
	binary.Write(pkg, binary.LittleEndian, addrLength)
	binary.Write(pkg, binary.LittleEndian, addrBytes)
	binary.Write(pkg, binary.LittleEndian, bodyLength)
	binary.Write(pkg, binary.LittleEndian, packet)
	return pkg.Bytes()
}
func ReadUDPPacket(conn *net.Conn) (srcAddr string, packet []byte, err error) {
	reader := bufio.NewReader(*conn)
	var addrLength uint16
	var bodyLength uint16
	err = binary.Read(reader, binary.LittleEndian, &addrLength)
	if err != nil {
		return
	}
	_srcAddr := make([]byte, addrLength)
	n, err := reader.Read(_srcAddr)
	if err != nil {
		return
	}
	if n != int(addrLength) {
		return
	}
	srcAddr = string(_srcAddr)

	err = binary.Read(reader, binary.LittleEndian, &bodyLength)
	if err != nil {
		return
	}
	packet = make([]byte, bodyLength)
	n, err = reader.Read(packet)
	if err != nil {
		return
	}
	if n != int(bodyLength) {
		return
	}
	return
}

func ResolveMapping(consume func(k, v string)) error {
	addrs, err := GetAllInterfaceAddr()
	if err != nil {
		return err
	}
	for _, addr := range addrs {
		if !(addr.IsGlobalUnicast() && addr.IsPrivate()) {
			continue
		}
		laddr := addr.String()
		go func() {
			if ip, err := requestIpInfo(laddr); err == nil {
				consume(ip, laddr)
			} else {
				log.Printf("detect mapping failed, laddr: %s err: %s", laddr, err)
			}
		}()
	}
	return nil
}

func UnmarshalMapping(path string, consume func(k, v string)) (err error) {
	if _, err = os.Stat(path); os.IsNotExist(err) {
		return
	}
	var data []byte
	data, err = os.ReadFile(path)
	if err != nil {
		return
	}
	m := make(map[string]string)
	err = json.Unmarshal(data, &m)
	if err != nil {
		return
	}
	for k, v := range m {
		consume(k, v)
	}
	return
}

func StartMonitor(mapping Mapping, checkInterval int) {
	go func() {
		for {
			time.Sleep(time.Duration(checkInterval) * time.Second)
			addrs, _ := GetAllInterfaceAddr()
			m := make(map[string]string)
			mapping.Consume(func(k, v string) {
				m[v] = k
			})
			for _, addr := range addrs {
				if !(addr.IsGlobalUnicast() && addr.IsPrivate()) {
					continue
				}
				laddr := addr.String()
				if newVal, err := requestIpInfo(laddr); err == nil {
					oldVal, ok := m[laddr]
					if newVal != "" && (!ok || oldVal != newVal) {
						log.Printf("detect new mapping: %s -> %s", laddr, newVal)
						mapping.Put(newVal, laddr)
					}
				}
			}
		}
	}()
}

func requestIpInfo(laddr string) (string, error) {
	dialer := newDialer(laddr+":0", time.Duration(3000)*time.Millisecond)
	transport, _ := http.DefaultTransport.(*http.Transport)
	transport.DialContext = dialer.DialContext
	transport.DisableKeepAlives = true
	request, err := http.NewRequest("GET", "https://api.ip.sb/jsonip", nil)
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/105.0.0.0 Safari/537.36")
	if err != nil {
		return "", err
	}
	response, err := transport.RoundTrip(request)
	if err != nil {
		return "", err
	}
	if response.StatusCode != 200 {
		return "", fmt.Errorf("failed to request ip resovler, response status: %s", response.Status)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	var ipInfo struct {
		Ip string `json:"ip"`
	}
	json.Unmarshal(data, &ipInfo)
	return ipInfo.Ip, nil
}
