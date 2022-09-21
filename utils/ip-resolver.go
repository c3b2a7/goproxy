package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var resolvers = []IPResolver{
	// see https://ip.sb/api
	&CommonIpResolver{
		name:       "ip.sb",
		newRequest: newRequest("GET", "https://api.ip.sb/jsonip", map[string]string{"User-Agent": "Mozilla/5.0"}),
		extractFun: newExtractFun("ip"),
	},
	// see https://www.ipify.org
	&CommonIpResolver{
		name:       "ipify.org",
		newRequest: newRequest("GET", "https://api.ipify.org?format=json", nil),
		extractFun: newExtractFun("ip"),
	},
	// see https://ipinfo.io/developers
	&CommonIpResolver{
		name:       "ipinfo.io",
		newRequest: newRequest("GET", "https://ipinfo.io/json", nil),
		extractFun: newExtractFun("ip"),
	},
	// see https://ip-api.com/docs/api:json
	&CommonIpResolver{
		name:       "ip-api.com",
		newRequest: newRequest("GET", "http://ip-api.com/json", nil),
		extractFun: newExtractFun("query"),
	},
	// see https://www.bigdatacloud.com/docs/api/public-ip-address-api
	&CommonIpResolver{
		name:       "bigdatacloud.net",
		newRequest: newRequest("GET", "https://api.bigdatacloud.net/data/client-ip", nil),
		extractFun: newExtractFun("ipString"),
	},
}

func AvailableIPRResolvers() (ret []string) {
	for _, resolver := range resolvers {
		ret = append(ret, resolver.Name())
	}
	return ret
}

func PickIPResolver(name string) IPResolver {
	for _, resolver := range resolvers {
		if resolver.Name() == name {
			return resolver
		}
	}
	return nil
}

func PickIPResolvers(names ...string) []IPResolver {
	var list []IPResolver
	rmap := make(map[string]bool)
	for _, name := range names {
		if resolver := PickIPResolver(name); resolver != nil {
			if _, ok := rmap[resolver.Name()]; !ok {
				list = append(list, resolver)
				rmap[resolver.Name()] = true
			}
		}
	}
	return list
}

type IPResolver interface {
	Get(ifaceAddr string) (string, error)
	Name() string
}

// CommonIpResolver do a request to an api to resolve iface address
type CommonIpResolver struct {
	name       string
	newRequest func() (*http.Request, error)
	extractFun func(map[string]interface{}) string
}

func (r CommonIpResolver) Get(ifaceAddr string) (string, error) {
	request, err := r.newRequest()
	if err != nil {
		return "", err
	}
	transport := newTransport(ifaceAddr)
	response, err := transport.RoundTrip(request)
	if err != nil {
		return "", fmt.Errorf("failed to %s %s, err: %s", request.Method, request.URL, err)
	}
	if response.StatusCode != 200 {
		return "", fmt.Errorf("%s respond: %s", request.Host, response.Status)
	}
	data, err := io.ReadAll(response.Body)
	defer response.Body.Close()
	if err != nil {
		return "", fmt.Errorf("failed to read %s respond body, err: %s", request.Host, err)
	}
	var raw map[string]interface{}
	if err = json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("failed to unmarshal json: %s, err: %s", data, err)
	}
	return r.extractFun(raw), nil
}

func (r CommonIpResolver) Name() string {
	if r.name == "" {
		return "UnKnown"
	}
	return r.name
}

func (r CommonIpResolver) String() string {
	return r.Name()
}

func newRequest(method, url string, header map[string]string) func() (*http.Request, error) {
	request, err := http.NewRequest(method, url, nil)
	fun := func() (*http.Request, error) {
		return request, err
	}
	if err == nil && header != nil {
		for k, v := range header {
			request.Header.Set(k, v)
		}
	}
	return fun
}

func newExtractFun(k string) func(map[string]interface{}) (v string) {
	return func(raw map[string]interface{}) (v string) {
		val, ok := raw[k]
		if v, ok = val.(string); ok {
			return
		}
		return ""
	}
}

var transportHolder struct {
	transportMap map[string]http.RoundTripper
	sync.Mutex
	sync.Once
}

func newTransport(laddr string) http.RoundTripper {
	transportHolder.Do(func() {
		transportHolder.transportMap = make(map[string]http.RoundTripper)
	})
	transportHolder.Lock()
	defer transportHolder.Unlock()
	if transport, ok := transportHolder.transportMap[laddr]; ok {
		return transport
	}
	var transport *http.Transport
	transport = http.DefaultTransport.(*http.Transport)
	localAddr, err := net.ResolveTCPAddr("tcp", laddr+":0")
	if laddr != "" && err == nil {
		transport = transport.Clone()
		dialer := newDialer(localAddr, time.Duration(3000)*time.Millisecond)
		transport.DialContext = dialer.DialContext
	}
	transportHolder.transportMap[laddr] = transport
	return transport
}

// FallbackIPResolver that resolve iface address using multiple IPResolver(s) until it succeeds
type FallbackIPResolver struct {
	resolvers []IPResolver
}

func (r FallbackIPResolver) Get(ifaceAddr string) (ret string, err error) {
	for _, resolver := range r.resolvers {
		if ret, err = resolver.Get(ifaceAddr); err == nil {
			return
		}
	}
	return "", fmt.Errorf("failed resolve iface address: %s, %v resolving has ended, last err: %s", ifaceAddr, r.Name(), err)
}

func (r FallbackIPResolver) Name() string {
	return fmt.Sprintf("Fallback%v", r.resolvers)
}

// RetryableIPResolver try resolve iface address multiple times until it succeeds
type RetryableIPResolver struct {
	times int
	IPResolver
}

func (r RetryableIPResolver) Get(ifaceAddr string) (ret string, err error) {
	for i := 0; i < r.times; i++ {
		if ret, err = r.IPResolver.Get(ifaceAddr); err == nil {
			return
		}
	}
	return "", fmt.Errorf("failed resolve iface address: %s, try %d times, last err: %s", ifaceAddr, r.times, err)
}

// RoundRobinIPResolver do resolve in round-robin
type RoundRobinIPResolver struct {
	idx       int32 // atomic
	resolvers []IPResolver
}

func (r *RoundRobinIPResolver) Get(ifaceAddr string) (ret string, err error) {
	current := atomic.LoadInt32(&r.idx)
	for !atomic.CompareAndSwapInt32(&r.idx, current, (current+1)%int32(len(resolvers))) {
		current = atomic.LoadInt32(&r.idx)
	}
	pick := resolvers[current]
	return pick.Get(ifaceAddr)
}

func (r *RoundRobinIPResolver) Name() string {
	return fmt.Sprintf("RoundRobin%v", r.resolvers)
}

func NewFallBackIPResolver(names ...string) (IPResolver, error) {
	list := PickIPResolvers(names...)
	if len(list) == 0 {
		return nil, fmt.Errorf("fallback names cannot be empty")
	}
	return &FallbackIPResolver{list}, nil
}

func NewRetryableIPResolver(name string, times int) (IPResolver, error) {
	resolver := PickIPResolver(name)
	if resolver == nil {
		return nil, fmt.Errorf("cannot find resolver that named: %s", name)
	}
	return &RetryableIPResolver{IPResolver: resolver, times: times}, nil
}

func NewRoundRobinIPResolver(names ...string) (IPResolver, error) {
	list := PickIPResolvers(names...)
	if len(list) == 0 {
		return nil, fmt.Errorf("round-robin names cannot be empty")
	}
	return &RoundRobinIPResolver{resolvers: list}, nil
}
