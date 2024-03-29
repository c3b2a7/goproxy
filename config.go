package main

import (
	"fmt"
	"github.com/c3b2a7/goproxy/common/hotupdate"
	"github.com/c3b2a7/goproxy/common/process"
	. "github.com/c3b2a7/goproxy/constant"
	"github.com/c3b2a7/goproxy/services"
	"github.com/c3b2a7/goproxy/utils"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
	"os"
	"strings"
)

func initConfig() (err error) {
	//keygen
	if len(os.Args) > 1 {
		if os.Args[1] == "keygen" {
			utils.Keygen()
			os.Exit(0)
		}
	}
	args := services.Args{}
	//define  args
	tcpArgs := services.TCPArgs{}
	httpArgs := services.HTTPArgs{}
	tunnelServerArgs := services.TunnelServerArgs{}
	tunnelClientArgs := services.TunnelClientArgs{}
	tunnelBridgeArgs := services.TunnelBridgeArgs{}
	udpArgs := services.UDPArgs{}

	//build srvice args
	app = kingpin.New("proxy", "happy with proxy")
	app.Author("snail").Version(Version)

	daemon := app.Flag("daemon", "run in daemon").Default("false").Bool()
	forever := app.Flag("forever", "run in forever").Default("false").Bool()
	doNotUpdate := app.Flag("do-not-update", "do not automatic update").Bool()

	args.Parent = app.Flag("parent", "parent address, such as: \"23.32.32.19:28008\"").Default("").Short('P').String()
	args.Local = app.Flag("local", "local ip:port to listen").Short('p').Default(":33080").String()
	certTLS := app.Flag("cert", "cert file for tls").Short('C').Default("").String()
	keyTLS := app.Flag("key", "key file for tls").Short('K').Default("").String()

	//########http#########
	http := app.Command("http", "proxy on http mode")
	httpArgs.LocalType = http.Flag("local-type", "local protocol type <tls|tcp>").Default("tcp").Short('t').Enum("tls", "tcp")
	httpArgs.ParentType = http.Flag("parent-type", "parent protocol type <tls|tcp>").Short('T').Enum("tls", "tcp")
	httpArgs.Always = http.Flag("always", "always use parent proxy").Default("false").Bool()
	httpArgs.Timeout = http.Flag("timeout", "tcp timeout milliseconds when connect to real server or parent proxy").Default("2000").Int()
	httpArgs.HTTPTimeout = http.Flag("http-timeout", "check domain if blocked , http request timeout milliseconds when connect to host").Default("3000").Int()
	httpArgs.Interval = http.Flag("interval", "check domain if blocked every interval seconds").Default("10").Int()
	httpArgs.Blocked = http.Flag("blocked", "blocked domain file , one domain each line").Default("blocked").Short('b').String()
	httpArgs.Direct = http.Flag("direct", "direct domain file , one domain each line").Default("direct").Short('d').String()
	httpArgs.AuthFile = http.Flag("auth-file", "http basic auth file,\"username:password\" each line in file").Short('F').String()
	httpArgs.Auth = http.Flag("auth", "http basic auth username and password, multiple users repeat with -a, such as: -a user1:pass1 -a user2:pass2").Short('a').Strings()
	httpArgs.PoolSize = http.Flag("pool-size", "conn pool size , which connect to parent proxy, zero means turn off pool").Short('L').Default("20").Int()
	httpArgs.CheckParentInterval = http.Flag("check-parent-interval", "check if proxy is okay every interval seconds, zero means no check").Short('I').Default("3").Int()
	httpArgs.MagicHeader = http.Flag("magic-header", "used to determine which iface to use to connect to target").Short('h').Default("").String()
	httpArgs.MappingFile = http.Flag("mapping-file", "used to mapping external IP to internal IP in nat environment").Short('m').Default("").String()
	httpArgs.AutoMapping = http.Flag("auto-mapping", "mapping external IP to internal IP automatically").Short('M').Default("false").Bool()
	httpArgs.CheckMappingInterval = http.Flag("check-mapping-interval", "monitor internal IP and update mapping every interval seconds, zero means no check").Short('c').Default("30").Int()
	httpArgs.IPResolver = http.Flag("ip-resolver", "ip resolver api, multiple apis repeat with -r, such as: -r ip.sb -r ipinfo.io, available: <"+strings.Join(utils.AvailableIPRResolvers(), "|")+">").Default(utils.AvailableIPRResolvers()...).PlaceHolder("ALL").Short('r').Enums(utils.AvailableIPRResolvers()...)

	//########tcp#########
	tcp := app.Command("tcp", "proxy on tcp mode")
	tcpArgs.Timeout = tcp.Flag("timeout", "tcp timeout milliseconds when connect to real server or parent proxy").Short('t').Default("2000").Int()
	tcpArgs.ParentType = tcp.Flag("parent-type", "parent protocol type <tls|tcp|udp>").Short('T').Enum("tls", "tcp", "udp")
	tcpArgs.IsTLS = tcp.Flag("tls", "proxy on tls mode").Default("false").Bool()
	tcpArgs.PoolSize = tcp.Flag("pool-size", "conn pool size , which connect to parent proxy, zero means turn off pool").Short('L').Default("20").Int()
	tcpArgs.CheckParentInterval = tcp.Flag("check-parent-interval", "check if proxy is okay every interval seconds, zero means no check").Short('I').Default("3").Int()

	//########udp#########
	udp := app.Command("udp", "proxy on udp mode")
	udpArgs.Timeout = udp.Flag("timeout", "tcp timeout milliseconds when connect to parent proxy").Short('t').Default("2000").Int()
	udpArgs.ParentType = udp.Flag("parent-type", "parent protocol type <tls|tcp|udp>").Short('T').Enum("tls", "tcp", "udp")
	udpArgs.PoolSize = udp.Flag("pool-size", "conn pool size , which connect to parent proxy, zero means turn off pool").Short('L').Default("20").Int()
	udpArgs.CheckParentInterval = udp.Flag("check-parent-interval", "check if proxy is okay every interval seconds, zero means no check").Short('I').Default("3").Int()

	//########tunnel-server#########
	tunnelServer := app.Command("tserver", "proxy on tunnel server mode")
	tunnelServerArgs.Timeout = tunnelServer.Flag("timeout", "tcp timeout with milliseconds").Short('t').Default("2000").Int()
	tunnelServerArgs.IsUDP = tunnelServer.Flag("udp", "proxy on udp tunnel server mode").Default("false").Bool()
	tunnelServerArgs.Key = tunnelServer.Flag("k", "key same with client").Default("default").String()

	//########tunnel-client#########
	tunnelClient := app.Command("tclient", "proxy on tunnel client mode")
	tunnelClientArgs.Timeout = tunnelClient.Flag("timeout", "tcp timeout with milliseconds").Short('t').Default("2000").Int()
	tunnelClientArgs.IsUDP = tunnelClient.Flag("udp", "proxy on udp tunnel client mode").Default("false").Bool()
	tunnelClientArgs.Key = tunnelClient.Flag("k", "key same with server").Default("default").String()

	//########tunnel-bridge#########
	tunnelBridge := app.Command("tbridge", "proxy on tunnel bridge mode")
	tunnelBridgeArgs.Timeout = tunnelBridge.Flag("timeout", "tcp timeout with milliseconds").Short('t').Default("2000").Int()

	serviceName := kingpin.MustParse(app.Parse(os.Args[1:]))

	return process.Start(*daemon, *forever, func() (err error) {
		if *certTLS != "" && *keyTLS != "" {
			args.CertBytes, args.KeyBytes, err = tlsBytes(*certTLS, *keyTLS)
			if err != nil {
				return
			}
		}

		//common args
		httpArgs.Args = args
		tcpArgs.Args = args
		udpArgs.Args = args
		tunnelBridgeArgs.Args = args
		tunnelClientArgs.Args = args
		tunnelServerArgs.Args = args

		poster()
		//register services and run service
		services.Register("http", services.NewHTTP(), httpArgs)
		services.Register("tcp", services.NewTCP(), tcpArgs)
		services.Register("udp", services.NewUDP(), udpArgs)
		services.Register("tserver", services.NewTunnelServer(), tunnelServerArgs)
		services.Register("tclient", services.NewTunnelClient(), tunnelClientArgs)
		services.Register("tbridge", services.NewTunnelBridge(), tunnelBridgeArgs)
		service, err = services.Run(serviceName)
		if err != nil {
			return fmt.Errorf("run service [%s] failed, cause: %s", service, err)
		}

		// start automatic update service
		if !*doNotUpdate {
			hotupdate.StartService(func(newVersion string) {
				fmt.Printf("\n[*] New version(%s) avaliable, restart services for update...\n", newVersion)
				service.S.Clean()
				os.Exit(0)
			})
		}
		return
	})
}

func poster() {
	fmt.Printf(`
	########  ########   #######  ##     ## ##    ## 
	##     ## ##     ## ##     ##  ##   ##   ##  ##  
	##     ## ##     ## ##     ##   ## ##     ####   
	########  ########  ##     ##    ###       ##    
	##        ##   ##   ##     ##   ## ##      ##    
	##        ##    ##  ##     ##  ##   ##     ##    
	##        ##     ##  #######  ##     ##    ##    

	Version: %s
	Build on: %s`+"\n\n", Version, BuildTime)
}
func tlsBytes(cert, key string) (certBytes, keyBytes []byte, err error) {
	certBytes, err = os.ReadFile(cert)
	if err != nil {
		return
	}
	keyBytes, err = os.ReadFile(key)
	if err != nil {
		return
	}
	return
}
