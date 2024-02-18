package flow

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_flow.h>

static const struct rte_flow_item_tcp *get_item_tcp_mask() {
	return &rte_flow_item_tcp_mask;
}

*/
import "C"
import (
	"runtime"
	"unsafe"

	"github.com/yerden/go-dpdk/types"
)

// TCPHdr L4 header from DPDK: lib/librte_net/rte_tcp.h
type TCPHdr struct {
	SrcPort  uint16         // TCP source port
	DstPort  uint16         // TCP destination port
	SentSeq  uint32         // TX data sequence number
	RecvAck  uint32         // RX data acknowledgement sequence number
	DataOff  uint8          // Data offset
	TCPFlags types.TCPFlags // TCP flags
	RxWin    uint16         // RX flow control window
	Cksum    uint16         // TCP checksum
	TCPUrp   uint16         // TCP urgent pointer, if any
}

// ItemTCP matches an UDP header.
type ItemTCP struct {
	cPointer

	Header TCPHdr
}

var _ ItemStruct = (*ItemTCP)(nil)

// Reload implements ItemStruct interface.
func (item *ItemTCP) Reload() {
	cptr := (*C.struct_rte_flow_item_tcp)(item.createOrRet(C.sizeof_struct_rte_flow_item_tcp))
	cvtTCPHeader(&cptr.hdr, &item.Header)
	runtime.SetFinalizer(item, nil)
	runtime.SetFinalizer(item, (*ItemTCP).free)
}

func cvtTCPHeader(dst *C.struct_rte_tcp_hdr, src *TCPHdr) {
	beU16(uint16(src.SrcPort), unsafe.Pointer(&dst.src_port))
	beU16(uint16(src.DstPort), unsafe.Pointer(&dst.dst_port))
	beU16(src.Cksum, unsafe.Pointer(&dst.cksum))
}

// Type implements ItemStruct interface.
func (item *ItemTCP) Type() ItemType {
	return ItemTypeTCP
}

// Mask implements ItemStruct interface.
func (item *ItemTCP) Mask() unsafe.Pointer {
	return unsafe.Pointer(C.get_item_tcp_mask())
}
