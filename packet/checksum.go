package packet

import (
	"unsafe"

	"github.com/tianyuansun/go-dpdk/types"
)

// Software calculation of protocol headers. It is required for hardware checksum calculation offload

// CalculatePseudoHdrIPv4TCPCksum implements one step of TCP checksum calculation. Separately computes checksum
// for TCP pseudo-header for case if L3 protocol is IPv4.
// This precalculation is required for checksum compute by hardware offload.
// Result should be put into TCP.Cksum field. See testCksum as an example.
func CalculatePseudoHdrIPv4TCPCksum(hdr *IPv4Hdr) uint16 {
	dataLength := SwapBytesUint16(hdr.TotalLength) - types.IPv4MinLen
	pHdrCksum := calculateIPv4AddrChecksum(hdr) +
		uint32(hdr.NextProtoID) +
		uint32(dataLength)
	return reduceChecksum(pHdrCksum)
}

// CalculatePseudoHdrIPv4UDPCksum implements one step of UDP checksum calculation. Separately computes checksum
// for UDP pseudo-header for case if L3 protocol is IPv4.
// This precalculation is required for checksum compute by hardware offload.
// Result should be put into UDP.DgramCksum field. See testCksum as an example.
func CalculatePseudoHdrIPv4UDPCksum(hdr *IPv4Hdr, udp *UDPHdr) uint16 {
	pHdrCksum := calculateIPv4AddrChecksum(hdr) +
		uint32(hdr.NextProtoID) +
		uint32(SwapBytesUint16(udp.DgramLen))
	return reduceChecksum(pHdrCksum)
}

// CalculatePseudoHdrIPv6TCPCksum implements one step of TCP checksum calculation. Separately computes checksum
// for TCP pseudo-header for case if L3 protocol is IPv6.
// This precalculation is required for checksum compute by hardware offload.
// Result should be put into TCP.Cksum field. See testCksum as an example.
func CalculatePseudoHdrIPv6TCPCksum(hdr *IPv6Hdr) uint16 {
	dataLength := SwapBytesUint16(hdr.PayloadLen)
	pHdrCksum := calculateIPv6AddrChecksum(hdr) +
		uint32(dataLength) +
		uint32(hdr.Proto)
	return reduceChecksum(pHdrCksum)
}

// CalculatePseudoHdrIPv6UDPCksum implements one step of UDP checksum calculation. Separately computes checksum
// for UDP pseudo-header for case if L3 protocol is IPv6.
// This precalculation is required for checksum compute by hardware offload.
// Result should be put into UDP.DgramCksum field. See testCksum as an example.
func CalculatePseudoHdrIPv6UDPCksum(hdr *IPv6Hdr, udp *UDPHdr) uint16 {
	pHdrCksum := calculateIPv6AddrChecksum(hdr) +
		uint32(hdr.Proto) +
		uint32(SwapBytesUint16(udp.DgramLen))
	return reduceChecksum(pHdrCksum)
}

// Software calculation of checksums

// Calculates checksum of memory for a given pointer. Length and
// offset are in bytes. Offset is signed, so negative offset is
// possible. Checksum is calculated in uint16 words. Returned is
// checksum with carry, so carry should be added and value negated for
// use as network checksum.
func calculateDataChecksum(ptr unsafe.Pointer, length, offset int) uint32 {
	var sum uint32
	uptr := uintptr(ptr) + uintptr(offset)

	slice := (*[1 << 30]uint16)(unsafe.Pointer(uptr))[0 : length/2]
	for i := range slice {
		sum += uint32(SwapBytesUint16(slice[i]))
	}

	if length&1 != 0 {
		sum += uint32(*(*byte)(unsafe.Pointer(uptr + uintptr(length-1)))) << 8
	}

	return sum
}

func reduceChecksum(sum uint32) uint16 {
	for sum > 0xffff {
		sum = (sum >> 16) + (sum & 0xffff)
	}
	return uint16(sum)
}

// CalculateIPv4Checksum calculates checksum of IP header
func CalculateIPv4Checksum(hdr *IPv4Hdr) uint16 {
	var sum uint32
	sum = uint32(hdr.VersionIhl)<<8 + uint32(hdr.TypeOfService) +
		uint32(SwapBytesUint16(hdr.TotalLength)) +
		uint32(SwapBytesUint16(hdr.PacketID)) +
		uint32(SwapBytesUint16(hdr.FragmentOffset)) +
		uint32(hdr.TimeToLive)<<8 + uint32(hdr.NextProtoID) +
		uint32(SwapBytesUint16(uint16(hdr.SrcAddr>>16))) +
		uint32(SwapBytesUint16(uint16(hdr.SrcAddr))) +
		uint32(SwapBytesUint16(uint16(hdr.DstAddr>>16))) +
		uint32(SwapBytesUint16(uint16(hdr.DstAddr)))

	return ^reduceChecksum(sum)
}

func calculateIPv4AddrChecksum(hdr *IPv4Hdr) uint32 {
	return uint32(SwapBytesUint16(uint16(hdr.SrcAddr>>16))) +
		uint32(SwapBytesUint16(uint16(hdr.SrcAddr))) +
		uint32(SwapBytesUint16(uint16(hdr.DstAddr>>16))) +
		uint32(SwapBytesUint16(uint16(hdr.DstAddr)))
}

// CalculateIPv4UDPChecksum calculates UDP checksum for case if L3 protocol is IPv4.
func CalculateIPv4UDPChecksum(hdr *IPv4Hdr, udp *UDPHdr, data unsafe.Pointer) uint16 {
	dataLength := SwapBytesUint16(hdr.TotalLength) - types.IPv4MinLen

	sum := calculateDataChecksum(data, int(dataLength-types.UDPLen), 0)

	sum += calculateIPv4AddrChecksum(hdr) +
		uint32(hdr.NextProtoID) +
		uint32(SwapBytesUint16(udp.DgramLen)) +
		uint32(SwapBytesUint16(udp.SrcPort)) +
		uint32(SwapBytesUint16(udp.DstPort)) +
		uint32(SwapBytesUint16(udp.DgramLen))

	retSum := ^reduceChecksum(sum)
	// If the checksum calculation results in the value zero (all 16 bits 0) it
	// should be sent as the one's complement (all 1s).
	if retSum == 0 {
		retSum = ^retSum
	}
	return retSum
}

func calculateTCPChecksum(tcp *TCPHdr) uint32 {
	return uint32(SwapBytesUint16(tcp.SrcPort)) +
		uint32(SwapBytesUint16(tcp.DstPort)) +
		uint32(SwapBytesUint16(uint16(tcp.SentSeq>>16))) +
		uint32(SwapBytesUint16(uint16(tcp.SentSeq))) +
		uint32(SwapBytesUint16(uint16(tcp.RecvAck>>16))) +
		uint32(SwapBytesUint16(uint16(tcp.RecvAck))) +
		uint32(tcp.DataOff)<<8 +
		uint32(tcp.TCPFlags) +
		uint32(SwapBytesUint16(tcp.RxWin)) +
		uint32(SwapBytesUint16(tcp.TCPUrp))
}

// CalculateIPv4TCPChecksum calculates TCP checksum for case if L3
// protocol is IPv4. Here data pointer should point to end of minimal
// TCP header because we consider TCP options as part of data.
func CalculateIPv4TCPChecksum(hdr *IPv4Hdr, tcp *TCPHdr, data unsafe.Pointer) uint16 {
	dataLength := SwapBytesUint16(hdr.TotalLength) - types.IPv4MinLen

	sum := calculateDataChecksum(data, int(dataLength-types.TCPMinLen), 0)

	sum += calculateIPv4AddrChecksum(hdr) +
		uint32(hdr.NextProtoID) +
		uint32(dataLength) +
		calculateTCPChecksum(tcp)

	return ^reduceChecksum(sum)
}

func calculateIPv6AddrChecksum(hdr *IPv6Hdr) uint32 {
	return uint32(uint16(hdr.SrcAddr[0])<<8|uint16(hdr.SrcAddr[1])) +
		uint32(uint16(hdr.SrcAddr[2])<<8|uint16(hdr.SrcAddr[3])) +
		uint32(uint16(hdr.SrcAddr[4])<<8|uint16(hdr.SrcAddr[5])) +
		uint32(uint16(hdr.SrcAddr[6])<<8|uint16(hdr.SrcAddr[7])) +
		uint32(uint16(hdr.SrcAddr[8])<<8|uint16(hdr.SrcAddr[9])) +
		uint32(uint16(hdr.SrcAddr[10])<<8|uint16(hdr.SrcAddr[11])) +
		uint32(uint16(hdr.SrcAddr[12])<<8|uint16(hdr.SrcAddr[13])) +
		uint32(uint16(hdr.SrcAddr[14])<<8|uint16(hdr.SrcAddr[15])) +
		uint32(uint16(hdr.DstAddr[0])<<8|uint16(hdr.DstAddr[1])) +
		uint32(uint16(hdr.DstAddr[2])<<8|uint16(hdr.DstAddr[3])) +
		uint32(uint16(hdr.DstAddr[4])<<8|uint16(hdr.DstAddr[5])) +
		uint32(uint16(hdr.DstAddr[6])<<8|uint16(hdr.DstAddr[7])) +
		uint32(uint16(hdr.DstAddr[8])<<8|uint16(hdr.DstAddr[9])) +
		uint32(uint16(hdr.DstAddr[10])<<8|uint16(hdr.DstAddr[11])) +
		uint32(uint16(hdr.DstAddr[12])<<8|uint16(hdr.DstAddr[13])) +
		uint32(uint16(hdr.DstAddr[14])<<8|uint16(hdr.DstAddr[15]))
}

// CalculateIPv6UDPChecksum calculates UDP checksum for case if L3 protocol is IPv6.
func CalculateIPv6UDPChecksum(hdr *IPv6Hdr, udp *UDPHdr, data unsafe.Pointer) uint16 {
	dataLength := SwapBytesUint16(hdr.PayloadLen)

	sum := calculateDataChecksum(data, int(dataLength-types.UDPLen), 0)

	sum += calculateIPv6AddrChecksum(hdr) +
		uint32(SwapBytesUint16(udp.DgramLen)) +
		uint32(hdr.Proto) +
		uint32(SwapBytesUint16(udp.SrcPort)) +
		uint32(SwapBytesUint16(udp.DstPort)) +
		uint32(SwapBytesUint16(udp.DgramLen))

	retSum := ^reduceChecksum(sum)
	// If the checksum calculation results in the value zero (all 16 bits 0) it
	// should be sent as the one's complement (all 1s).
	if retSum == 0 {
		retSum = ^retSum
	}
	return retSum
}

// CalculateIPv6TCPChecksum calculates TCP checksum for case if L3 protocol is IPv6.
func CalculateIPv6TCPChecksum(hdr *IPv6Hdr, tcp *TCPHdr, data unsafe.Pointer) uint16 {
	dataLength := SwapBytesUint16(hdr.PayloadLen)

	sum := calculateDataChecksum(data, int(dataLength-types.TCPMinLen), 0)

	sum += calculateIPv6AddrChecksum(hdr) +
		uint32(dataLength) +
		uint32(hdr.Proto) +
		calculateTCPChecksum(tcp)

	return ^reduceChecksum(sum)
}

// CalculateIPv4ICMPChecksum calculates ICMP checksum in case if L3
// protocol is IPv4.
func CalculateIPv4ICMPChecksum(hdr *IPv4Hdr, icmp *ICMPHdr, data unsafe.Pointer) uint16 {
	dataLength := SwapBytesUint16(hdr.TotalLength) - types.IPv4MinLen - types.ICMPLen

	sum := uint32(uint16(icmp.Type)<<8|uint16(icmp.Code)) +
		uint32(SwapBytesUint16(icmp.Identifier)) +
		uint32(SwapBytesUint16(icmp.SeqNum)) +
		calculateDataChecksum(unsafe.Pointer(data), int(dataLength), 0)

	return ^reduceChecksum(sum)
}

// CalculateIPv6ICMPChecksum calculates ICMP checksum in case if L3
// protocol is IPv6.
func CalculateIPv6ICMPChecksum(hdr *IPv6Hdr, icmp *ICMPHdr, data unsafe.Pointer) uint16 {
	dataLength := SwapBytesUint16(hdr.PayloadLen)

	// ICMP payload
	sum := calculateDataChecksum(data, int(dataLength-types.ICMPLen), 0)

	sum += calculateIPv6AddrChecksum(hdr) + // IPv6 Header
		uint32(dataLength) +
		uint32(hdr.Proto) +
		// ICMP header excluding checksum
		uint32(uint16(icmp.Type)<<8|uint16(icmp.Code)) +
		uint32(SwapBytesUint16(icmp.Identifier)) +
		uint32(SwapBytesUint16(icmp.SeqNum))

	return ^reduceChecksum(sum)
}
