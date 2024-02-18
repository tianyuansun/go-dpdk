package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/yerden/go-dpdk/eal"
	"github.com/yerden/go-dpdk/ethdev"
	"github.com/yerden/go-dpdk/packet"
	"github.com/yerden/go-dpdk/types"
	"github.com/yerden/go-dpdk/util"
)

var burstSize = flag.Int("burst", 256, "Specify RX burst size")
var printMetadata = flag.Bool("print", false, "Specify to print each packet's metadata")
var dryRun = flag.Bool("dryRun", false, "If true traffic will not be processed")

// PortQueue describes port and rx queue id.
type PortQueue struct {
	Pid   ethdev.Port
	RxQid uint16
	TxQid uint16
}

// dissect all given lcores and store them into map hashed by affine
// socket id.
func dissectLcores(lcores []uint) map[uint][]uint {
	table := map[uint][]uint{}

	for _, lcore := range lcores {
		socket := eal.LcoreToSocket(lcore)

		if affine, ok := table[socket]; !ok {
			table[socket] = []uint{lcore}
		} else {
			table[socket] = append(affine, lcore)
		}
	}

	return table
}

// DistributeQueues assigns all RX queues for each port in ports to
// lcores. Assignment is NUMA-aware.
//
// Returns os.ErrInvalid if port id is invalid.
// Returns os.ErrNotExist if no lcores are available by NUMA
// constraints.
func DistributeQueues(ports []ethdev.Port, lcores []uint) (map[uint]PortQueue, error) {
	table := map[uint]PortQueue{}
	lcoreMap := dissectLcores(lcores)

	for _, pid := range ports {
		if err := distributeQueuesPort(pid, lcoreMap, table); err != nil {
			return nil, err
		}
	}

	return table, nil
}

func distributeQueuesPort(pid ethdev.Port, lcoreMap map[uint][]uint, table map[uint]PortQueue) error {
	var info ethdev.DevInfo

	if err := pid.InfoGet(&info); err != nil {
		return err
	}

	socket := pid.SocketID()
	if socket < 0 {
		return os.ErrInvalid
	}

	lcores, ok := lcoreMap[uint(socket)]
	if !ok {
		fmt.Println("no lcores for socket:", socket)
		return os.ErrNotExist
	}

	nrx := info.NbRxQueues()
	if nrx == 0 {
		return os.ErrClosed
	}

	if int(nrx) > len(lcores) {
		return fmt.Errorf("pid=%d nrx=%d cannot run on %d lcores", pid, nrx, len(lcores))
	}

	var lcore uint
	var acquired util.LcoresList
	for i := uint16(0); i < nrx; i++ {
		lcore, lcores = lcores[0], lcores[1:]
		acquired = append(acquired, lcore)
		lcoreMap[uint(socket)] = lcores
		table[lcore] = PortQueue{Pid: pid, RxQid: i}
	}

	fmt.Printf("pid=%d runs on socket=%d, lcores=%v\n", pid, socket, util.LcoresList(acquired))

	return nil
}

func LcoreFunc(pq PortQueue, qcr *QueueCounterReporter) func(*eal.LcoreCtx) {
	return func(ctx *eal.LcoreCtx) {
		defer log.Println("lcore", eal.LcoreID(), "exited")

		if *dryRun {
			return
		}
		// eal
		pid := pq.Pid
		rxQid := pq.RxQid
		qc := qcr.Register(pid, rxQid)

		src := util.NewEthdevMbufArray(pid, rxQid, int(eal.SocketID()), uint16(*burstSize))
		defer src.Free()

		buf := src.Buffer()

		txBuf := ethdev.NewTxBuffer(128)

		log.Printf("processing pid=%d, qid=%d, lcore=%d\n", pid, rxQid, eal.LcoreID())
		for {
			n := pid.TxBufferFlush(rxQid, txBuf)
			if n > 0 && *printMetadata {
				log.Printf("Sent %d packets\n", n)
			}
			n = pid.RxBurst(rxQid, buf, uint16(*burstSize))
			if n > 0 && *printMetadata {
				log.Printf("Recv %d packets\n", n)
			}

			for i := uint16(0); i < n; i++ {
				pkt := packet.Packet{
					CMbuf: buf[i],
				}
				pkt.ParseL2()
				pkt.ParseL3()
				pkt.ParseL4ForIPv4()
				pkt.ParseData()

				if *printMetadata {
					pkt.ParseL2()
					pkt.ParseL3()
					pkt.ParseL4ForIPv4()
					pkt.ParseData()
					fmt.Printf("rx packet:\nether %s\nipv4 %s\nicmp: %s\n",
						pkt.GetEther().String(),
						pkt.GetIPv4().String(),
						pkt.GetICMPForIPv4().String(),
					)
				}
				tmpMac := pkt.GetEther().SAddr
				pkt.GetEther().SAddr = pkt.GetEther().DAddr
				pkt.GetEther().DAddr = tmpMac
				ipv4 := pkt.GetIPv4()
				tmpIP := pkt.GetIPv4().SrcAddr
				pkt.GetIPv4().SrcAddr = pkt.GetIPv4().DstAddr
				pkt.GetIPv4().DstAddr = tmpIP
				if ipv4.NextProtoID == types.ICMPNumber {
					icmp := pkt.GetICMPForIPv4()
					icmp.Type = types.ICMPTypeEchoResponse
					icmp.Cksum = packet.SwapBytesUint16(packet.CalculateIPv4ICMPChecksum(ipv4, icmp, pkt.Data))
				} else if ipv4.NextProtoID == types.UDPNumber {
					tmpPort := pkt.GetUDPForIPv4().SrcPort
					pkt.GetUDPForIPv4().SrcPort = pkt.GetUDPForIPv4().DstPort
					pkt.GetUDPForIPv4().DstPort = tmpPort
				}
				ipCsum := packet.CalculateIPv4Checksum(ipv4)
				pkt.GetIPv4().HdrChecksum = packet.SwapBytesUint16(ipCsum)

				if *printMetadata {
					pkt.ParseL2()
					pkt.ParseL3()
					pkt.ParseL4ForIPv4()
					pkt.ParseData()
					fmt.Printf("tx packet:\nether %s\nipv4 %s\nudp: %s\n",
						pkt.GetEther().String(),
						pkt.GetIPv4().String(),
						pkt.GetICMPForIPv4().String(),
					)
					fmt.Printf("raw packet is %s\n", hex.EncodeToString(pkt.CMbuf.Data()))
				}
				pid.TxBuffer(rxQid, txBuf, pkt.CMbuf)
			}

			qc.Incr(buf[:n])
		}

	}
}
