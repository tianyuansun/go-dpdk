package main

// /*
// #include <rte_ethdev.h>
// */
import "C"

import (
	"flag"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/yerden/go-dpdk/eal"
	"github.com/yerden/go-dpdk/ethdev"
	"github.com/yerden/go-dpdk/ethdev/flow"
	"github.com/yerden/go-dpdk/mempool"
)

type CmdMempool struct{}

var mbufElts = flag.Int("poolMbufs", 100000, "Specify number of mbufs in mempool")
var mbufSize = flag.Int("dataRoomSize", 3000, "Specify mbuf size in mempool")
var poolCache = flag.Int("poolCache", 512, "Specify amount of mbufs in per-lcore cache")
var rxQueues = flag.Int("nrxq", 1, "Specify number of RX queues per port")
var txQueues = flag.Int("ntxq", 1, "Specify number of TX queues per port")
var hairpinQueues = flag.Int("nhpq", 1, "Specify number of Hairpin Queues per port")
var rxDesc = flag.Int("nrxdesc", 256, "Specify number of RX desc per port")
var txDesc = flag.Int("ntxdesc", 256, "Specify number of TX desc per port")
var isolate = flag.Int("isolate", 1, "Whether isolate flow")

func (*CmdMempool) NewMempool(name string, opts []mempool.Option) (*mempool.Mempool, error) {
	return mempool.CreateMbufPool(name, uint32(*mbufElts), uint16(*mbufSize), opts...)
}

type App struct {
	RxqMempooler
	Stats *Stats
	Ports []ethdev.Port
	Work  map[uint]PortQueue
	QCR   *QueueCounterReporter
	Flows []*flow.Flow
}

func NewApp(reg prometheus.Registerer) (*App, error) {
	var app *App
	return app, doOnMain(func() error {
		var err error
		app, err = newApp(reg)
		return err
	})
}

func newApp(reg prometheus.Registerer) (*App, error) {
	rxqPools, err := NewMempoolPerPort("mbuf_pool", &CmdMempool{},
		mempool.OptCacheSize(uint32(*poolCache)),
		mempool.OptOpsName("ring_mp_mc"),
		mempool.OptPrivateDataSize(64), // for each mbuf
	)

	if err != nil {
		return nil, err
	}

	rssConf := &ethdev.RssConf{
		Key: nil,
		Hf:  0xf00000000003afbc,
		// Hf:  ethdev.ETH_RSS_IPV4,
	}

	ethdevCfg := &EthdevConfig{
		Options: []ethdev.Option{
			ethdev.OptRss(*rssConf),
			ethdev.OptRxMode(ethdev.RxMode{
				MqMode: ethdev.ETH_MQ_RX_RSS,
			}),
		},
		RxQueues:      uint16(*rxQueues),
		TxQueues:      uint16(*txQueues),
		HairpinQueues: uint16(*hairpinQueues),
		OnConfig: []EthdevCallback{
			EthdevCallbackFunc((ethdev.Port).Start),
			&RssConfig{rssConf},
		},
		Pooler:        rxqPools,
		RxDescriptors: uint16(*rxDesc),
		TxDescriptors: uint16(*txDesc),
		FcMode:        fcMode.Mode,
	}

	ports := make([]ethdev.Port, 0, ethdev.CountTotal())

	for i := 0; i < cap(ports); i++ {
		if pid := ethdev.Port(i); pid.IsValid() {
			ports = append(ports, pid)
		}
	}

	for i := range ports {
		if *isolate != 0 {
			fmt.Printf("try isolat port %d flow\n", ports[i])
			var ferr flow.Error
			err := flow.Isolate(ports[i], 1, &ferr)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
		}
		fmt.Printf("configuring port %d: %s... ", ports[i], ifaceName(ports[i]))
		if err := ethdevCfg.Configure(ports[i]); err != nil {
			fmt.Println(err)
			return nil, err
		}
		printPortConfig(ports[i])
		fmt.Println("OK")
	}

	metrics, err := NewStats(reg, ports)
	if err != nil {
		return nil, err
	}

	work, err := DistributeQueues(ports, eal.LcoresWorker())
	if err != nil {
		return nil, err
	}

	app := &App{
		RxqMempooler: rxqPools,
		Ports:        ports,
		Stats:        metrics,
		Work:         work,
		QCR:          &QueueCounterReporter{reg: reg},
		Flows:        make([]*flow.Flow, 0),
	}
	for i := range ports {
		if *isolate != 0 {
			fmt.Printf("try create flow to port %d\n", ports[i])
			var ferr flow.Error
			var flowAttr flow.Attr
			flowAttr.Ingress = true
			flowAttr.Group = 0

			// mac, _ := net.ParseMAC("fa:16:3e:ff:ff:fe")

			// vtepIP := net.ParseIP("172.16.8.4").To4()

			flowPatterns := []flow.Item{
				{Spec: flow.ItemTypeEth},
				{Spec: &flow.ItemIPv4{}, Mask: &flow.ItemIPv4{}},
				// {Spec: &flow.ItemUDP{}, Mask: &flow.ItemUDP{}},
				// {Spec: &flow.ItemUDP{Header: flow.UDPHeader{DstPort: 8088}}, Mask: &flow.ItemUDP{Header: flow.UDPHeader{DstPort: 65535}}},
				// {Spec: &flow.ItemVXLAN{VNI: 0}, Mask: &flow.ItemVXLAN{VNI: 0xffffff00}},
				// {Spec: flow.ItemTypeVxlan},
				// {Spec: &flow.ItemEth{}, Mask: &flow.ItemEth{}},
				// {Spec: &flow.ItemIPv4{}, Mask: &flow.ItemIPv4{}},
				// {Spec: &flow.ItemUDP{Header: flow.UDPHeader{DstPort: 8888}}, Mask: &flow.ItemUDP{Header: flow.UDPHeader{DstPort: 65535}}},
			}
			flowActions := []flow.Action{
				&flow.ActionCount{ID: 0},
				// &flow.ActionMacDst{Mac: mac},
				&flow.ActionQueue{Index: 0},
			}
			if err := flow.Validate(ports[i], &flowAttr, flowPatterns, flowActions, &ferr); err != nil {
				fmt.Println(err)
				return nil, err
			}
			f, err := flow.Create(ports[i], &flowAttr, flowPatterns, flowActions, &ferr)
			if err != nil {
				fmt.Println(err)
				return nil, err
			}
			app.Flows = append(app.Flows, f)
		}
	}

	return app, nil
}

func (app *App) ReportFlowStats() {
	var stats flow.RXTXStats
	var flowErr flow.Error
	var err error
	for _, f := range app.Flows {
		err = flow.Query(app.Ports[0], f, &stats, &flowErr)
		if err != nil {
			fmt.Println(err)
			return
		}
		// if stats.Hits != 0 {
		// 	fmt.Printf("packets %d, bytes %d\n", stats.Hits, stats.Bytes)
		// }
	}
}
