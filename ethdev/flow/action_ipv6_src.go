package flow

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_flow.h>
*/
import "C"
import "unsafe"

var _ Action = (*ActionIPv6Src)(nil)

// ActionIPv6Src implements Action which assigns packets to a given
// queue index.
type ActionIPv6Src struct {
	cPointer
	Addr IPv6
}

// Reload implements Action interface.
func (action *ActionIPv6Src) Reload() {
	cptr := (*C.struct_rte_flow_action_set_ipv6)(action.createOrRet(C.sizeof_struct_rte_flow_action_set_ipv6))

	cptr.ipv6_addr = *(*C.uint8_t)(unsafe.Pointer(&action.Addr[0]))
}

// Type implements Action interface.
func (action *ActionIPv6Src) Type() ActionType {
	return ActionTypeSetIPv6Src
}
