package main

import (
	"./logger"
	util "./utils"
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"math/big"
	// "net"
	"sync"
	"time"
)

var cfg *util.Config

func log_init() {
	logger.SetConsole(cfg.Log.Console)
	logger.SetRollingFile(cfg.Log.Dir, cfg.Log.Name, cfg.Log.Num, cfg.Log.Size, logger.KB)
	//ALL，DEBUG，INFO，WARN，ERROR，FATAL，OFF
	logger.SetLevel(logger.ERROR)
	if cfg.Log.Level == "info" {
		logger.SetLevel(logger.INFO)
	} else if cfg.Log.Level == "error" {
		logger.SetLevel(logger.ERROR)
	}
}
func init() {
	cfg = &util.Config{}

	if !util.LoadConfig("seeker.toml", cfg) {
		return
	}
	log_init()
	// initialize()
}

const (
	ua          = "manspreading"
	ver         = "1.0.0"
	upstreamUrl = "enode://344d2d76587b931a8dccb61f5f3280c9486068ef2758252cf5c6ebc29d4385581137c45e2c218e4ee23a0b14d23ecb6ec12521362e9919380c3b00ff5401bea2@10.81.64.116:30304" //geth2
	// upstreamUrl = "enode://2998c333662a61620126e8a5a44545b8c0b362ec8a89b246a3e2e15a076983525e148ef113152d2836b976fb8de860b03f997012793870d78ae0a56e565d8398@118.31.112.214:30304" //getf1

	listenAddr = "0.0.0.0:36666"
	privkey    = ""
	//设置初值
	// 5294375 2881436154511909728
)

var (
	// startBlock = common.StringToHash("0x58f3ea40c3d1ffdea3c88b8d77ede6bdc2ecd6dc88b24aa2479304c359a043e5")
	// startTD    = big.NewInt(2881436154511909728)
	// 换个低一些的高度10000
	startBlock = common.StringToHash("0xdc2d938e4cd0a149681e9e04352953ef5ab399d59bcd5b0357f6c0797470a524")
	startTD    = big.NewInt(2303762395359969)
	genesis    = common.StringToHash("0xd4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa3")
	gversion   = uint32(63)
	gnetworkid = uint64(888888)
)

// statusData is the network packet for the status message.
type statusData struct {
	ProtocolVersion uint32
	NetworkId       uint64
	TD              *big.Int
	CurrentBlock    common.Hash
	GenesisBlock    common.Hash
}
type newBlockHashesData []struct {
	Hash   common.Hash   // Hash of one particular block being announced
	Number uint64        // Number of one particular block being announced
	header *types.Header // Header of the block partially reassembled (new protocol)	重新组装的区块头
	time   time.Time     // Timestamp of the announcement

	origin string
}

// newBlockData is the network packet for the block propagation message.
type newBlockData struct {
	Block *types.Block
	TD    *big.Int
}

type conn struct {
	p  *p2p.Peer
	rw p2p.MsgReadWriter
}

type proxy struct {
	lock         sync.RWMutex
	upstreamNode *discover.Node //第一个连接的node
	// upstreamConn *conn
	upstreamConn map[discover.NodeID]*conn //后面自动连接的peer
	// downstreamConn *conn
	// upstreamState map[discover.NodeID]statusData
	bestState     statusData
	bestStateChan chan statusData
	srv           *p2p.Server
	// maxtd         *big.Int
	// bestHash      common.Hash
	bestHeiChan    chan bestHeiPeer
	bestHeiAndPeer bestHeiPeer
}
type bestHeiPeer struct {
	bestHei uint64
	p       *p2p.Peer
}

func (pxy *proxy) Start() {
	tick := time.Tick(5000 * time.Millisecond)
	go func() {
		for {
			select {
			case hei, ok := <-pxy.bestHeiChan:
				if !ok {
					break
				}
				if hei.bestHei > pxy.bestHeiAndPeer.bestHei {
					pxy.bestHeiAndPeer = hei
				}
			case beststate, ok := <-pxy.bestStateChan:
				if !ok {
					break
				}
				if beststate.TD.Cmp(pxy.bestState.TD) > 0 {
					pxy.bestState = beststate
				}
			case <-tick:
				fmt.Println("besthei:", pxy.bestHeiAndPeer.bestHei, " from:", pxy.bestHeiAndPeer.p)
				fmt.Println("beststate:", pxy.bestState.TD)
			}
		}
	}()
}

var pxy *proxy

func test2() {
	var nodekey *ecdsa.PrivateKey
	if privkey != "" {
		nodekey, _ = crypto.LoadECDSA(privkey)
		fmt.Println("Node Key loaded from ", privkey)
	} else {
		nodekey, _ = crypto.GenerateKey()
		crypto.SaveECDSA("./nodekey", nodekey)
		fmt.Println("Node Key generated and saved to ./nodekey")
	}

	node, err := discover.ParseNode(MainnetBootnodes[0])
	if err != nil {
		fmt.Println("discover.ParseNode:", err)
		return
	}

	pxy = &proxy{
		upstreamNode: node,
		upstreamConn: make(map[discover.NodeID]*conn, 0),
		// upstreamState: make(map[discover.NodeID]statusData, 0),
		bestState: statusData{
			ProtocolVersion: gversion,
			NetworkId:       gnetworkid,
			TD:              startTD,
			CurrentBlock:    startBlock,
			GenesisBlock:    genesis,
		},
		bestStateChan: make(chan statusData),
		bestHeiChan:   make(chan bestHeiPeer),
	}
	bootstrapNodes := make([]*discover.Node, 0)
	for _, boot := range MainnetBootnodes {
		old, err := discover.ParseNode(boot)
		if err != nil {
			fmt.Println("discover.ParseNode2:", err)
			continue
		}
		// pxy.srv.AddPeer(old)
		bootstrapNodes=append(bootstrapNodes,old)
	}
	config := p2p.Config{
		PrivateKey:     nodekey,
		MaxPeers:       200,
		NoDiscovery:    false,
		DiscoveryV5:    false,
		Name:           common.MakeName(fmt.Sprintf("%s/%s", ua, node.ID.String()), ver),
		// BootstrapNodes: []*discover.Node{node},
		BootstrapNodes:bootstrapNodes,
		StaticNodes:    []*discover.Node{node},
		TrustedNodes:   []*discover.Node{node},

		Protocols: []p2p.Protocol{newManspreadingProtocol()},

		ListenAddr: listenAddr,
		Logger:     log.New(),
	}
	// config.Logger.SetHandler(log.StdoutHandler)

	pxy.srv = &p2p.Server{Config: config}

	// Wait forever
	var wg sync.WaitGroup
	wg.Add(2)
	err = pxy.srv.Start()
	pxy.Start()
	wg.Done()
	if err != nil {
		fmt.Println(err)
	}
	wg.Wait()
}
func test() {
	i := new(big.Int)
	i.SetString("0e", 16) // octal
	fmt.Println(i.Uint64())
}
func main() {
	test()
	test2()
	c := make(chan int, 1)

	<-c
}
