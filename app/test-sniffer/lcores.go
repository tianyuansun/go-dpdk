package main

import (
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
				pkt.VxlanDecap()
				pkt.ParseData()

				if *printMetadata {
					msg := "rx packet:\n"
					if pkt.Overlay {
						msg += "ether %s\nipv4 %s\nudp %s\nvxlan %s\n"
						msg += "inner ether %s\narp %s\nipv4 %s\nicmp %s\n"
						msg += "ipv6 %s\n"
						fmt.Printf(
							msg,
							pkt.OuterEther,
							pkt.OuterIPv4Hdr,
							pkt.OuterUDPHdr,
							pkt.VxlanHeader,
							pkt.GetEther(),
							pkt.GetARP(),
							pkt.GetIPv4(),
							pkt.GetICMPForIPv4(),
							pkt.GetIPv6(),
						)
					} else {
						msg += "ether %s\nipv4 %s\nicmp %s\n"
						fmt.Printf(
							msg,
							pkt.GetEther(),
							pkt.GetIPv4(),
							pkt.GetICMPForIPv4(),
						)
					}
				}
				ether := pkt.GetEther()
				tmpMac := ether.SAddr
				ether.SAddr = ether.DAddr
				ether.DAddr = tmpMac
				if ether.EtherType == packet.SwapBytesUint16(types.ARPNumber) {
					arp := pkt.GetARP()
					if arp.Operation != packet.SwapBytesUint16(packet.ARPRequest) {
						pkt.CMbuf.PktMbufFree()
						continue
					}
					arp.Operation = packet.SwapBytesUint16(packet.ARPReply)
					tmp := arp.SHA
					// 0a:58:da:97:5a:6f
					arp.SHA = types.MACAddress{0x0a, 0x58, 0xda, 0x97, 0x5a, 0x6f}
					ether.SAddr = arp.SHA
					arp.THA = tmp
					tmpIP := arp.SPA
					arp.SPA = arp.TPA
					arp.TPA = tmpIP
				} else if ether.EtherType == packet.SwapBytesUint16(types.IPV4Number) {
					ipv4 := pkt.GetIPv4()
					tmpIP := pkt.GetIPv4().SrcAddr
					pkt.GetIPv4().SrcAddr = pkt.GetIPv4().DstAddr
					pkt.GetIPv4().DstAddr = tmpIP
					if ipv4.NextProtoID == types.ICMPNumber {
						icmp := pkt.GetICMPForIPv4()
						if icmp.Type != types.ICMPTypeEchoRequest {
							pkt.CMbuf.PktMbufFree()
							continue
						}
						icmp.Type = types.ICMPTypeEchoResponse
						icmp.Cksum = packet.SwapBytesUint16(packet.CalculateIPv4ICMPChecksum(ipv4, icmp, pkt.Data))
					} else if ipv4.NextProtoID == types.UDPNumber {
						// TODO: not support udp, drop
						tmpPort := pkt.GetUDPForIPv4().SrcPort
						pkt.GetUDPForIPv4().SrcPort = pkt.GetUDPForIPv4().DstPort
						pkt.GetUDPForIPv4().DstPort = tmpPort
						// pkt.GetUDPForIPv4().DgramCksum = packet.SwapBytesUint16(packet.CalculateIPv4UDPChecksum(pkt.GetIPv4(), pkt.GetUDPForIPv4(), pkt.Data))
					} else {
						pkt.CMbuf.PktMbufFree()
						continue
					}
					ipCsum := packet.CalculateIPv4Checksum(ipv4)
					pkt.GetIPv4().HdrChecksum = packet.SwapBytesUint16(ipCsum)
				} else if ether.EtherType == packet.SwapBytesUint16(types.IPV6Number) {
					ipv6 := pkt.GetIPv6()
					tmpIP := pkt.GetIPv6NoCheck().SrcAddr
					pkt.GetIPv6NoCheck().SrcAddr = pkt.GetIPv6NoCheck().DstAddr
					pkt.GetIPv6NoCheck().DstAddr = tmpIP
					if ipv6.Proto == types.ICMPv6Number {
						icmp := pkt.GetICMPForIPv6()
						icmp6Type := icmp.Type
						if icmp6Type == types.ICMPv6TypeEchoRequest {
							icmp.Type = types.ICMPv6TypeEchoResponse
							icmp.Cksum = packet.SwapBytesUint16(packet.CalculateIPv6ICMPChecksum(ipv6, icmp, pkt.Data))
						} else if icmp6Type == types.ICMPv6NeighborSolicitation {
							pkt.ParseL7(types.ICMPv6Number)
							rawTargetIP := pkt.GetICMPv6NeighborSolicitationMessage().TargetAddr

							icmp.Type = types.ICMPv6NeighborAdvertisement
							icmp.Identifier = packet.SwapBytesUint16(packet.ICMPv6NDSolicitedFlag | packet.ICMPv6NDOverrideFlag)
							// pkt.GetIPv6NoCheck().DstAddr = types.IPv6Address{0xff, 0x02, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}
							pkt.GetIPv6NoCheck().SrcAddr = rawTargetIP
							srcMAC := types.MACAddress{0x0a, 0x58, 0xda, 0x97, 0x5a, 0x6f}
							ether.SAddr = srcMAC

							// if mbuf.AppendMbuf(pkt.CMbuf, packet.ICMPv6NDTargetLinkLayerAddressOptionSize) == false {
							// 	pkt.CMbuf.PktMbufFree()
							// 	continue
							// }

							msg := pkt.GetICMPv6NeighborAdvertisementMessage()
							msg.TargetAddr = rawTargetIP
							option := pkt.GetICMPv6NDTargetLinkLayerAddressOption(packet.ICMPv6NeighborAdvertisementMessageSize)
							option.Type = packet.ICMPv6NDTargetLinkLayerAddress
							option.Length = uint8(packet.ICMPv6NDTargetLinkLayerAddressOptionSize / packet.ICMPv6NDMessageOptionUnitSize)
							option.LinkLayerAddress = srcMAC
							icmp.Cksum = packet.SwapBytesUint16(packet.CalculateIPv6ICMPChecksum(ipv6, icmp, pkt.Data))
						} else if icmp6Type == types.ICMPv6TypeRouterSolicitation {
							pkt.CMbuf.PktMbufFree()
							continue
						}
					} else if ipv6.Proto == types.UDPNumber {
						// TODO: not support udp, drop
						tmpPort := pkt.GetUDPForIPv6().SrcPort
						pkt.GetUDPForIPv6().SrcPort = pkt.GetUDPForIPv6().DstPort
						pkt.GetUDPForIPv6().DstPort = tmpPort
					} else {
						pkt.CMbuf.PktMbufFree()
						continue
					}
				}
				if pkt.Overlay {
					tmpMac = pkt.OuterEther.SAddr
					pkt.OuterEther.SAddr = pkt.OuterEther.DAddr
					pkt.OuterEther.DAddr = tmpMac
					outIPv4 := pkt.OuterIPv4Hdr
					tmpIP := outIPv4.SrcAddr
					outIPv4.SrcAddr = outIPv4.DstAddr
					outIPv4.DstAddr = tmpIP
					pkt.OuterUDPHdr.DgramCksum = 0
					// pkt.VxlanHeader.VNI = packet.SwapBytesUint32(42 << 8)
					// tmpPort := pkt.OuterUDPHdr.SrcPort
					// pkt.OuterUDPHdr.SrcPort = pkt.OuterUDPHdr.DstPort
					// pkt.OuterUDPHdr.DstPort = tmpPort
				}

				if *printMetadata {
					msg := "tx packet:\n"
					if pkt.Overlay {
						msg += "ether %s\nipv4 %s\nudp %s\nvxlan %s\n"
						msg += "inner ether %s\narp %s\nipv4 %s\nicmp %s\n"
						msg += "ipv6 %s\n"
						fmt.Printf(
							msg,
							pkt.OuterEther,
							pkt.OuterIPv4Hdr,
							pkt.OuterUDPHdr,
							pkt.VxlanHeader,
							pkt.GetEther(),
							pkt.GetARP(),
							pkt.GetIPv4(),
							pkt.GetICMPForIPv4(),
							pkt.GetIPv6(),
						)
					} else {
						msg += "ether %s\nipv4 %s\nicmp %s\n"
						fmt.Printf(
							msg,
							pkt.GetEther(),
							pkt.GetIPv4(),
							pkt.GetICMPForIPv4(),
						)
					}
				}
				pid.TxBuffer(rxQid, txBuf, pkt.CMbuf)
			}
			qc.Incr(buf[:n])
		}

	}
}
