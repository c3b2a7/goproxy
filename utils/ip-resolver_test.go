package utils

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func TestCommonIpResolver_Get(t *testing.T) {
	tests := AvailableIPRResolvers()
	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			resolver := PickIPResolver(test)
			addr, _ := resolver.Get("")
			assert.NotEmptyf(t, addr, "test %s failed", test)
		})
	}
}

func TestTransport(t *testing.T) {
	resolver := PickIPResolver("ip.sb")
	var wg sync.WaitGroup

	do := func(ifaceAddr string) {
		go func() {
			resolver.Get(ifaceAddr)
			wg.Done()
		}()
	}

	assert.Exactly(t, 0, len(transportHolder.transportMap))

	wg.Add(1)
	do("")
	wg.Wait()
	// default transport
	assert.Exactly(t, len(transportHolder.transportMap), 1)

	wg.Add(1)
	do("127.0.0.1")
	wg.Wait()
	// default transport + 127.0.0.1
	assert.Exactly(t, 2, len(transportHolder.transportMap))

	wg.Add(1)
	do("192.168.15.141")
	wg.Wait()
	// default transport + 127.0.0.1 + 192.168.15.141
	assert.Exactly(t, 3, len(transportHolder.transportMap))

	wg.Add(1)
	do("")
	wg.Wait()
	// default transport + 127.0.0.1 + 192.168.15.141
	assert.Exactly(t, len(transportHolder.transportMap), 3)
}

func TestNewFallBackIPResolver(t *testing.T) {
	resolver, _ := NewFallBackIPResolver(AvailableIPRResolvers()...)
	name := resolver.Name()
	assert.NotEmpty(t, name)
	addr, err := resolver.Get("")
	assert.NotEmpty(t, addr, err)
}

func TestNewRetryableIPResolver(t *testing.T) {
	resolver, _ := NewRetryableIPResolver("ip-api.com", 3)
	name := resolver.Name()
	assert.NotEmpty(t, name)
	addr, err := resolver.Get("")
	assert.NotEmpty(t, addr, err)
}

func TestNewRoundRobinIPResolver(t *testing.T) {
	resolver, _ := NewRoundRobinIPResolver(AvailableIPRResolvers()...)
	name := resolver.Name()
	assert.NotEmpty(t, name)
	var addr string
	var err error
	addr, err = resolver.Get("")
	addr, err = resolver.Get("")
	addr, err = resolver.Get("")
	addr, err = resolver.Get("")
	addr, err = resolver.Get("")
	assert.NotEmpty(t, addr, err)
}
