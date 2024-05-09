package flow

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_flow.h>
*/
import "C"
import "unsafe"

var _ Action = (*ActionIPv4Dst)(nil)

// ActionIPv4Dst implements Action which assigns packets to a given
// queue index.
type ActionIPv4Dst struct {
	cPointer
	Addr IPv4
}

// Reload implements Action interface.
func (action *ActionIPv4Dst) Reload() {
	cptr := (*C.struct_rte_flow_action_set_ipv4)(action.createOrRet(C.sizeof_struct_rte_flow_action_set_ipv4))

	cptr.ipv4_addr = *(*C.rte_be32_t)(unsafe.Pointer(&action.Addr[0]))
}

// Type implements Action interface.
func (action *ActionIPv4Dst) Type() ActionType {
	return ActionTypeSetIPv4Dst
}
