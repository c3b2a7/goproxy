package services

import . "github.com/c3b2a7/goproxy/utils"

const (
	TYPE_TCP     = "tcp"
	TYPE_UDP     = "udp"
	TYPE_HTTP    = "http"
	TYPE_TLS     = "tls"
	CONN_CONTROL = uint8(1)
	CONN_SERVER  = uint8(2)
	CONN_CLIENT  = uint8(3)
)

type Args struct {
	Local     *string
	Parent    *string
	Mapping   Mapping
	CertBytes []byte
	KeyBytes  []byte
}
type TunnelServerArgs struct {
	Args
	IsUDP   *bool
	Key     *string
	Timeout *int
}
type TunnelClientArgs struct {
	Args
	IsUDP   *bool
	Key     *string
	Timeout *int
}
type TunnelBridgeArgs struct {
	Args
	Timeout *int
}
type TCPArgs struct {
	Args
	ParentType          *string
	IsTLS               *bool
	Timeout             *int
	PoolSize            *int
	CheckParentInterval *int
}

type HTTPArgs struct {
	Args
	Always               *bool
	HTTPTimeout          *int
	Interval             *int
	Blocked              *string
	Direct               *string
	AuthFile             *string
	Auth                 *[]string
	ParentType           *string
	LocalType            *string
	Timeout              *int
	PoolSize             *int
	CheckParentInterval  *int
	MagicHeader          *string
	MappingFile          *string
	AutoMapping          *bool
	CheckMappingInterval *int
}
type UDPArgs struct {
	Args
	ParentType          *string
	Timeout             *int
	PoolSize            *int
	CheckParentInterval *int
}

func (a *TCPArgs) Protocol() string {
	if *a.IsTLS {
		return "tls"
	}
	return "tcp"
}
