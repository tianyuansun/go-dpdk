package mbuf

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_version.h>
#include <rte_mbuf.h>

char *reset_and_append(struct rte_mbuf *mbuf, void *ptr, size_t len)
{
	rte_pktmbuf_reset(mbuf);
	char *data = rte_pktmbuf_append(mbuf, len);
	if (data == NULL)
		return NULL;
	rte_memcpy(data, ptr, len);
	return data;
}
struct rte_mbuf *alloc_reset_and_append(struct rte_mempool *mp, void *ptr, size_t len)
{
	struct rte_mbuf *mbuf;
	mbuf = rte_pktmbuf_alloc(mp);
	if (mbuf == NULL)
		return NULL;
	rte_pktmbuf_reset(mbuf);
	char *data = rte_pktmbuf_append(mbuf, len);
	if (data == NULL)
		return NULL;
	rte_memcpy(data, ptr, len);
	return mbuf;
}

static inline void
free_bulk(struct rte_mbuf **pkts, unsigned int count)
{
#if RTE_VERSION < RTE_VERSION_NUM(21, 11, 0, 0)
	unsigned int i;
	for (i = 0; i < count; i++)
		rte_pktmbuf_free(pkts[i]);
#else
	rte_pktmbuf_free_bulk(pkts, count);
#endif
}

enum {
	MBUF_RSS_OFF = offsetof(struct rte_mbuf, hash.rss),
};

*/
import "C"

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/yerden/go-dpdk/common"
	"github.com/yerden/go-dpdk/mempool"
	"github.com/yerden/go-dpdk/types"
)

// ErrNullData is returned if NULL is returned by Cgo call.
var ErrNullData = errors.New("NULL response returned")

// Mbuf contains a packet.
type Mbuf C.struct_rte_mbuf

func ToCMbuf(m *Mbuf) *C.struct_rte_mbuf {
	return (*C.struct_rte_mbuf)(unsafe.Pointer(m))
}

func mbufs(ms []*Mbuf) **C.struct_rte_mbuf {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&ms))
	return (**C.struct_rte_mbuf)(unsafe.Pointer(sh.Data))
}

func mp(m *mempool.Mempool) *C.struct_rte_mempool {
	return (*C.struct_rte_mempool)(unsafe.Pointer(m))
}

// PktMbufFree returns this mbuf into its originating mempool along
// with all its segments.
func (m *Mbuf) PktMbufFree() {
	C.rte_pktmbuf_free(ToCMbuf(m))
}

// RawFree returns this mbuf into its originating mempool.
func (m *Mbuf) RawFree() {
	C.rte_mbuf_raw_free(ToCMbuf(m))
}

// PktMbufClone clones the mbuf using supplied mempool as the buffer
// source. NOTE: NULL may return if allocation fails.
func (m *Mbuf) PktMbufClone(p *mempool.Mempool) *Mbuf {
	return (*Mbuf)(C.rte_pktmbuf_clone(ToCMbuf(m), mp(p)))
}

// PktMbufAlloc allocate an uninitialized mbuf from mempool p.
// Note that NULL may be returned if allocation failed.
func PktMbufAlloc(p *mempool.Mempool) *Mbuf {
	m := C.rte_pktmbuf_alloc(mp(p))
	return (*Mbuf)(m)
}

// PktMbufAllocBulk allocate a bulk of mbufs.
func PktMbufAllocBulk(p *mempool.Mempool, ms []*Mbuf) error {
	e := C.rte_pktmbuf_alloc_bulk(mp(p), mbufs(ms), C.uint(len(ms)))
	return common.IntErr(int64(e))
}

// PktMbufReset frees a bulk of packet mbufs back into their original mempools.
func PktMbufFreeBulk(ms []*Mbuf) {
	C.free_bulk(mbufs(ms), C.uint(len(ms)))
}

// PktMbufPrivSize get the application private size of mbufs
// stored in a pktmbuf_pool. The private size of mbuf is a zone
// located between the rte_mbuf structure and the data buffer
// where an application can store data associated to a packet.
func PktMbufPrivSize(p *mempool.Mempool) int {
	return (int)(C.rte_pktmbuf_priv_size(mp(p)))
}

var packetStructSize int

// SetPacketStructSize sets the size of the packet.
func SetPacketStructSize(t int) error {
	if t > C.RTE_PKTMBUF_HEADROOM {
		msg := fmt.Sprintf("Packet structure can't be placed inside mbuf.",
			"Increase CONFIG_RTE_PKTMBUF_HEADROOM in dpdk/config/common_base and rebuild dpdk.")
		return fmt.Errorf("%s", msg)
	}
	minPacketHeadroom := 64
	if C.RTE_PKTMBUF_HEADROOM-t < minPacketHeadroom {
		fmt.Printf("Packet will have only %d bytes for prepend something, increase CONFIG_RTE_PKTMBUF_HEADROOM in dpdk/config/common_base and rebuild dpdk.\n", C.RTE_PKTMBUF_HEADROOM-t)
	}
	packetStructSize = t
	return nil
}

// PrependMbuf prepends length bytes to mbuf data area.
// TODO 4 following functions support only not chained mbufs now
// Heavily based on DPDK rte_pktmbuf_prepend
func PrependMbuf(mb *Mbuf, length uint) bool {
	if C.uint16_t(length) > mb.data_off-C.uint16_t(packetStructSize) {
		return false
	}
	mb.data_off -= C.uint16_t(length)
	mb.data_len += C.uint16_t(length)
	mb.pkt_len += C.uint32_t(length)
	return true
}

// AppendMbuf appends length bytes to mbuf.
// Heavily based on DPDK rte_pktmbuf_append
func AppendMbuf(mb *Mbuf, length uint) bool {
	if C.uint16_t(length) > mb.buf_len-mb.data_off-mb.data_len {
		return false
	}
	mb.data_len += C.uint16_t(length)
	mb.pkt_len += C.uint32_t(length)
	return true
}

// GetPacketDataStartPointer returns the pointer to the
// beginning of packet.
func GetPacketDataStartPointer(mb *Mbuf) uintptr {
	return uintptr(mb.buf_addr) + uintptr(mb.data_off)
}

// WriteDataToMbuf copies data to mbuf.
func WriteDataToMbuf(mb *Mbuf, data []byte) {
	d := unsafe.Pointer(GetPacketDataStartPointer(mb))
	slice := (*[types.MaxLength]byte)(d)[:len(data)] // copy requires slice
	//TODO need to investigate maybe we need to use C function C.rte_memcpy here
	copy(slice, data)
}

func SetNextMbuf(next *Mbuf, prev *Mbuf) {
	prev.next = (*C.struct_rte_mbuf)(next)
}

// func setMbufLen(mb *Mbuf, l2len, l3len uint32) {
// 	// Assign l2_len:7 and l3_len:9 fields in rte_mbuf
// 	mb.anon5[0] = uint8((l2len & 0x7f) | ((l3len & 1) << 7))
// 	mb.anon5[1] = uint8(l3len >> 1)
// 	mb.anon5[2] = 0
// 	mb.anon5[3] = 0
// 	mb.anon5[4] = 0
// 	mb.anon5[5] = 0
// 	mb.anon5[6] = 0
// 	mb.anon5[7] = 0
// }

// SetTXIPv4OLFlags sets mbuf flags for IPv4 header
// checksum calculation hardware offloading.
// func SetTXIPv4OLFlags(mb *Mbuf, l2len, l3len uint32) {
// 	// PKT_TX_IP_CKSUM | PKT_TX_IPV4
// 	mb.ol_flags = (1 << 54) | (1 << 55)
// 	setMbufLen(mb, l2len, l3len)
// }

// SetTXIPv4UDPOLFlags sets mbuf flags for IPv4 and UDP
// headers checksum calculation hardware offloading.
// func SetTXIPv4UDPOLFlags(mb *Mbuf, l2len, l3len uint32) {
// 	// PKT_TX_UDP_CKSUM | PKT_TX_IP_CKSUM | PKT_TX_IPV4
// 	mb.ol_flags = (3 << 52) | (1 << 54) | (1 << 55)
// 	setMbufLen(mb, l2len, l3len)
// }

// SetTXIPv4TCPOLFlags sets mbuf flags for IPv4 and TCP
// headers checksum calculation hardware offloading.
// func SetTXIPv4TCPOLFlags(mb *Mbuf, l2len, l3len uint32) {
// 	// PKT_TX_TCP_CKSUM | PKT_TX_IP_CKSUM | PKT_TX_IPV4
// 	mb.ol_flags = (1 << 52) | (1 << 54) | (1 << 55)
// 	setMbufLen(mb, l2len, l3len)
// }

// SetTXIPv6UDPOLFlags sets mbuf flags for IPv6 UDP header
// checksum calculation hardware offloading.
// func SetTXIPv6UDPOLFlags(mb *Mbuf, l2len, l3len uint32) {
// 	// PKT_TX_UDP_CKSUM | PKT_TX_IPV6
// 	mb.ol_flags = (3 << 52) | (1 << 56)
// 	setMbufLen(mb, l2len, l3len)
// }

// SetTXIPv6TCPOLFlags sets mbuf flags for IPv6 TCP
// header checksum calculation hardware offloading.
// func SetTXIPv6TCPOLFlags(mb *Mbuf, l2len, l3len uint32) {
// 	// PKT_TX_TCP_CKSUM | PKT_TX_IPV4
// 	mb.ol_flags = (1 << 52) | (1 << 56)
// 	setMbufLen(mb, l2len, l3len)
// }

// GetRawPacketBytesMbuf returns raw data from packet.
func GetRawPacketBytesMbuf(mb *Mbuf) []byte {
	dataLen := uintptr(mb.data_len)
	dataPtr := uintptr(mb.buf_addr) + uintptr(mb.data_off)
	return (*[1 << 30]byte)(unsafe.Pointer(dataPtr))[:dataLen]
}

// GetPktLenMbuf returns amount of data in a given chain of Mbufs - whole packet
func GetPktLenMbuf(mb *Mbuf) uint {
	return uint(mb.pkt_len)
}

// GetDataLenMbuf returns amount of data in a given Mbuf - one segment if scattered
func GetDataLenMbuf(mb *Mbuf) uint {
	return uint(mb.data_len)
}

// AdjMbuf removes length bytes at mbuf beginning.
// Heavily based on DPDK rte_pktmbuf_adj
func AdjMbuf(m *Mbuf, length uint) bool {
	if C.uint16_t(length) > m.data_len {
		return false
	}
	m.data_off += C.uint16_t(length)
	m.data_len -= C.uint16_t(length)
	m.pkt_len -= C.uint32_t(length)
	return true
}

// TrimMbuf removes length bytes at the mbuf end.
// Heavily based on DPDK rte_pktmbuf_trim
func TrimMbuf(m *Mbuf, length uint) bool {
	if C.uint16_t(length) > m.data_len {
		return false
	}
	m.data_len -= C.uint16_t(length)
	m.pkt_len -= C.uint32_t(length)
	return true
}

// PktMbufAppend append the given data to an mbuf.
// Error may be returned if there is not enough tailroom
// space in the last segment of mbuf.
func (m *Mbuf) PktMbufAppend(data []byte) error {
	ptr := C.rte_pktmbuf_append(ToCMbuf(m), C.uint16_t(len(data)))
	if ptr == nil {
		return ErrNullData
	}

	copy(unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(data)), data)
	return nil
}

// RefCntUpdate adds given value to an mbuf's refcnt and returns its
// new value.
func (m *Mbuf) RefCntUpdate(v int16) uint16 {
	return uint16(C.rte_mbuf_refcnt_update(ToCMbuf(m), C.int16_t(v)))
}

// RefCntRead reads the value of an mbuf's refcnt.
func (m *Mbuf) RefCntRead() uint16 {
	return uint16(C.rte_mbuf_refcnt_read(ToCMbuf(m)))
}

// RefCntSet sets an mbuf's refcnt to the defined value.
func (m *Mbuf) RefCntSet(v uint16) {
	C.rte_mbuf_refcnt_set(ToCMbuf(m), C.uint16_t(v))
}

// PktMbufReset reset the fields of a packet mbuf to their default values.
func (m *Mbuf) PktMbufReset() {
	C.rte_pktmbuf_reset(ToCMbuf(m))
}

// Mempool return a pool from which mbuf was allocated.
func (m *Mbuf) Mempool() *mempool.Mempool {
	rteMbuf := ToCMbuf(m)
	memp := rteMbuf.pool
	return (*mempool.Mempool)(unsafe.Pointer(memp))
}

// PrivSize return a size of private data area.
func (m *Mbuf) PrivSize() uint16 {
	rteMbuf := ToCMbuf(m)
	s := rteMbuf.priv_size
	return uint16(s)
}

func (m *Mbuf) L2() unsafe.Pointer {
	return unsafe.Pointer(uintptr(m.buf_addr) + uintptr(m.data_off))
}

// Data returns contained packet.
func (m *Mbuf) Data() []byte {
	var d []byte
	buf := ToCMbuf(m)
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&d))
	sh.Data = uintptr(buf.buf_addr) + uintptr(buf.data_off)
	sh.Len = int(buf.data_len)
	sh.Cap = int(buf.data_len)
	return d
}

// PktLen returns total packet length: sum of all segments.
func (m *Mbuf) PktLen() uint32 {
	rteMbuf := ToCMbuf(m)
	return uint32(rteMbuf.pkt_len)
}

// Next returns next segment of scattered packet.
func (m *Mbuf) Next() *Mbuf {
	rteMbuf := ToCMbuf(m)
	return (*Mbuf)(rteMbuf.next)
}

// PrivData sets ptr to point to mbuf's private area. Private data
// length is set to the priv_size field of an mbuf itself. Although
// this length may be 0 the private area may still be usable as
// HeadRoomSize is not 0.
//
// Feel free to edit the contents. A pointer to the headroom
// will be returned if the length of the private zone is 0.
func (m *Mbuf) PrivData(ptr *common.CStruct) {
	rteMbuf := ToCMbuf(m)
	ptr.Init(unsafe.Add(unsafe.Pointer(m), unsafe.Sizeof(*m)), int(rteMbuf.priv_size))
}

// ResetAndAppend reset the fields of a mbuf to their default values
// and append the given data to an mbuf. Error may be returned
// if there is not enough tailroom space in the last segment of mbuf.
// Len is the amount of data to append (in bytes).
func (m *Mbuf) ResetAndAppend(data *common.CStruct) error {
	ptr := C.reset_and_append(ToCMbuf(m), data.Ptr, C.size_t(data.Len))
	if ptr == nil {
		return ErrNullData
	}
	return nil
}

// AllocResetAndAppend allocates an uninitialized mbuf from mempool p,
// resets the fields of the mbuf to default values and appends the
// given data to an mbuf.
//
// Note that NULL may be returned if allocation failed or if there is
// not enough tailroom space in the last segment of mbuf.  p is the
// mempool from which the mbuf is allocated. Data is C array
// representation of data to add.
func AllocResetAndAppend(p *mempool.Mempool, data *common.CStruct) *Mbuf {
	m := C.alloc_reset_and_append(mp(p), data.Ptr, C.size_t(data.Len))
	return (*Mbuf)(unsafe.Pointer(m))
}

// HeadRoomSize returns the value of the data_off field,
// which must be equal to the size of the headroom in concrete mbuf.
func (m *Mbuf) HeadRoomSize() uint16 {
	rteMbuf := ToCMbuf(m)
	return uint16(rteMbuf.data_off)
}

// TailRoomSize returns available length that can be appended to mbuf.
func (m *Mbuf) TailRoomSize() uint16 {
	rteMbuf := ToCMbuf(m)
	return uint16(rteMbuf.buf_len - rteMbuf.data_off - rteMbuf.data_len)
}

// BufLen represents DataRoomSize that was initialized in
// mempool.CreateMbufPool.
//
// NOTE: Max available data length that mbuf can hold is BufLen -
// HeadRoomSize.
func (m *Mbuf) BufLen() uint16 {
	rteMbuf := ToCMbuf(m)
	return uint16(rteMbuf.buf_len)
}

// PktMbufHeadRoomSize represents RTE_PKTMBUF_HEADROOM size in
// concrete mbuf.
//
// NOTE: This implies Cgo call and is used for testing purposes only.
// Use HeadRoomSize instead.
func (m *Mbuf) PktMbufHeadRoomSize() uint16 {
	return uint16(C.rte_pktmbuf_headroom(ToCMbuf(m)))
}

// PktMbufTailRoomSize represents RTE_PKTMBUF_TAILROOM which is
// available length that can be appended to mbuf.
//
// NOTE: This implies Cgo call and is used for testing purposes only.
// Use TailRoomSize instead.
func (m *Mbuf) PktMbufTailRoomSize() uint16 {
	return uint16(C.rte_pktmbuf_tailroom(ToCMbuf(m)))
}

// HashRss returns hash.rss field of an mbuf.
func (m *Mbuf) HashRss() uint32 {
	p := unsafe.Pointer(m)
	p = unsafe.Pointer(uintptr(p) + C.MBUF_RSS_OFF)
	return *(*uint32)(p)
}
