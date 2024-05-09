package main

import (
	"fmt"
	"log"

	"github.com/tianyuansun/go-dpdk/ethdev"
	"github.com/tianyuansun/go-dpdk/util"
)

// EthdevCallback specifies callback to call on ethdev.Port.
type EthdevCallback interface {
	EthdevCall(ethdev.Port) error
}

type EthdevCallbackFunc func(ethdev.Port) error

func (f EthdevCallbackFunc) EthdevCall(pid ethdev.Port) error {
	return f(pid)
}

// EthdevConfig specifies information on how to configure ethdev.Port.
type EthdevConfig struct {
	Options       []ethdev.Option
	RxQueues      uint16
	TxQueues      uint16
	HairpinQueues uint16

	// Hooks to call after configuration
	OnConfig []EthdevCallback

	// RX queue config
	Pooler        RxqMempooler
	RxDescriptors uint16
	TxDescriptors uint16
	RxOptions     []ethdev.QueueOption
	TxOptions     []ethdev.QueueOption

	// Flow Control Mode
	FcMode uint32
}

func ifaceName(pid ethdev.Port) string {
	name, _ := pid.Name()
	return name
}

// Configure must be called on main lcore to configure ethdev.Port.
func (conf *EthdevConfig) Configure(pid ethdev.Port) error {
	var info ethdev.DevInfo
	if err := pid.InfoGet(&info); err != nil {
		return err
	}

	opts := conf.Options
	lscOpt := ethdev.OptIntrConf(ethdev.IntrConf{LSC: true})
	if info.DevFlags().IsIntrLSC() {
		opts = append(conf.Options, lscOpt)
		pid.RegisterCallbackLSC()
	} else {
		fmt.Printf("port %d doesn't support LSC interrupt\n", pid)
	}

	if err := pid.DevConfigure(conf.RxQueues+conf.HairpinQueues, conf.TxQueues+conf.HairpinQueues, opts...); err != nil {
		return err
	}

	if err := pid.PromiscEnable(); err != nil {
		return err
	}

	for qid := uint16(0); qid < conf.RxQueues; qid++ {
		fmt.Printf("configuring rxq: %d@%d\n", pid, qid)
		mp, err := conf.Pooler.GetRxMempool(pid, qid)
		if err != nil {
			return err
		}
		if err := pid.RxqSetup(qid, conf.RxDescriptors, mp, conf.RxOptions...); err != nil {
			return err
		}
	}

	for qid := uint16(0); qid < conf.TxQueues; qid++ {
		fmt.Printf("configuring txq: %d@%d\n", pid, qid)
		if err := pid.TxqSetup(qid, conf.TxDescriptors, conf.TxOptions...); err != nil {
			return err
		}
	}

	if conf.HairpinQueues > 0 {
		pid.HairpinQueueSetup(conf.RxQueues, conf.TxQueues, conf.HairpinQueues, conf.RxDescriptors, conf.TxDescriptors)
	}

	// var fc ethdev.FcConf

	// if err := pid.FlowCtrlGet(&fc); err == nil {
	// 	fc.SetMode(conf.FcMode)
	// 	if err := pid.FlowCtrlSet(&fc); err != nil {
	// 		return util.ErrWrapf(err, "FlowCtrlSet")
	// 	}

	// 	log.Printf("pid=%d: Flow Control set to %d", pid, conf.FcMode)
	// } else if !errors.Is(err, syscall.ENOTSUP) {
	// 	return util.ErrWrapf(err, "FlowCtrlGet")
	// }

	for i := range conf.OnConfig {
		if err := conf.OnConfig[i].EthdevCall(pid); err != nil {
			return util.ErrWrapf(err, "OnConfig %d: %v", i, conf.OnConfig[i])
		}
	}

	return nil
}

func printPortConfig(pid ethdev.Port) error {
	var info ethdev.DevInfo
	var conf ethdev.DevConf
	if err := pid.InfoGet(&info); err != nil {
		return err
	}
	if err := pid.DevConfGet(&conf); err != nil {
		return err
	}

	log.Printf("port %d:\nnrxq=%d, ntxq=%d\ndriver=%s, flags=%d, tx offloads= %s\n", pid,
		info.NbRxQueues(),
		info.NbTxQueues(),
		info.DriverName(),
		info.DevFlags(),
		printTXOffloads(conf.TxOffloadCapa()),
	)
	return nil
}

func printTXOffloads(offloads uint64) string {
	ret := ""
	var single_offload uint64
	var begin, end, bit int
	if offloads == 0 {
		return ret
	}
	begin = int(ctzll(offloads))
	end = 64 - int(clzll(offloads))
	single_offload = 1 << begin
	for bit = begin; bit < end; bit++ {
		log.Printf("bit %d begin %d end %d single_offload %d\n", bit, begin, end, single_offload)
		if offloads&single_offload != 0 {
			name := single_offload
			ret += fmt.Sprintf(" %d", name)
		}
		single_offload <<= 1
	}
	return ret
}

func ctzll(x uint64) uint64 {
	if x == 0 {
		return 64
	}

	var i uint64
	for i = 0; x&1 == 0; i++ {
		x >>= 1
	}
	return i
}

func clzll(x uint64) uint64 {
	if x == 0 {
		return 64
	}

	var i uint64
	for x&0x8000000000000000 == 0 {
		x <<= 1
		i++
	}
	return i
}
