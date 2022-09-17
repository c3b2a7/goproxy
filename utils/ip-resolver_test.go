package utils

import (
	"github.com/stretchr/testify/assert"
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
