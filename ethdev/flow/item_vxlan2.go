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

static void encap(uint32_t vni, struct rte_flow_item_vxlan * item) {
    int i = 0;
    for (i = 0; i < 3; i++) {
        item->vni[2-i] = vni >> (i * 8);
    }
}

*/
import "C"
import (
	"unsafe"
)

var _ ItemStruct = (*ItemVXLAN2)(nil)

type ItemVXLAN2 struct {
	cPointer
	VNI uint32
}

func (item *ItemVXLAN2) Reload() {
	cptr := (*C.struct_rte_flow_item_vxlan)(item.createOrRet(C.sizeof_struct_rte_flow_item_vxlan))

	// hdr := (*C.struct_rte_vxlan_hdr)(off(unsafe.Pointer(cptr), C.ITEM_VXLAN_OFF_HDR))

	// if item.VNI != 0 {
	// leU32(8, unsafe.Pointer(&hdr.vx_flags))
	// beU32(item.VNI, unsafe.Pointer(&hdr.vx_vni))
	C.encap(C.uint32_t(item.VNI), (*C.struct_rte_flow_item_vxlan)(cptr))

	// }
	// runtime.SetFinalizer(item, nil)
	// runtime.SetFinalizer(item, (*ItemVXLAN).free)
}

func (item *ItemVXLAN2) Type() ItemType {
	return ItemTypeVxlan
}

func (item *ItemVXLAN2) Mask() unsafe.Pointer {
	return unsafe.Pointer(C.get_item_vxlan_mask())
}
