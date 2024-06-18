package flow

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_flow.h>
*/
import "C"
import "unsafe"

var _ Action = (*ActionIPv4Src)(nil)

// ActionIPv4Src implements Action which assigns packets to a given
// queue index.
type ActionIPv4Src struct {
	cPointer
	Addr IPv4
}

// Reload implements Action interface.
func (action *ActionIPv4Src) Reload() {
	cptr := (*C.struct_rte_flow_action_set_ipv4)(action.createOrRet(C.sizeof_struct_rte_flow_action_set_ipv4))

	cptr.ipv4_addr = *(*C.rte_be32_t)(unsafe.Pointer(&action.Addr[0]))
}

// Type implements Action interface.
func (action *ActionIPv4Src) Type() ActionType {
	return ActionTypeSetIPv4Src
}
