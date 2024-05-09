package flow

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_vxlan.h>
#include <rte_flow.h>

enum {
	ITEM_VXLAN_OFF_HDR = offsetof(struct rte_flow_item_vxlan, hdr),
};

enum {
	VXLAN_HDR_OFF_FLAGS = offsetof(struct rte_vxlan_hdr, vx_flags),
	VXLAN_HDR_OFF_VNI = offsetof(struct rte_vxlan_hdr, vx_vni),
};

static const struct rte_flow_item_vxlan *get_item_vxlan_mask() {
	return &rte_flow_item_vxlan_mask;
}

*/
import "C"
import (
	"unsafe"

	"icode.baidu.com/baidu/edge-os/xvr/mem"
)

var _ ItemStruct = (*ItemVXLAN)(nil)

type ItemVXLAN struct {
	cPointer
	VNI uint32
}

func (item *ItemVXLAN) Reload() {
	cptr := (*C.struct_rte_flow_item_vxlan)(item.createOrRet(C.sizeof_struct_rte_flow_item_vxlan))

	hdr := (*C.struct_rte_vxlan_hdr)(off(unsafe.Pointer(cptr), C.ITEM_VXLAN_OFF_HDR))

	// if item.VNI != 0 {
	// leU32(8, unsafe.Pointer(&hdr.vx_flags))
	// beU32(item.VNI, unsafe.Pointer(&hdr.vx_vni))
	mem.Memcpy(unsafe.Pointer(&hdr.vx_vni), unsafe.Pointer(&item.VNI), 3)

	// }
	// runtime.SetFinalizer(item, nil)
	// runtime.SetFinalizer(item, (*ItemVXLAN).free)
}

func (item *ItemVXLAN) Type() ItemType {
	return ItemTypeVxlan
}

func (item *ItemVXLAN) Mask() unsafe.Pointer {
	return unsafe.Pointer(C.get_item_vxlan_mask())
}
