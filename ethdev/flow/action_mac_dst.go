package flow

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_flow.h>
*/
import "C"
import (
	"net"
	"runtime"
)

var _ Action = (*ActionMacDst)(nil)

// ActionMacDst implements Action which assigns packets to a given
// queue index.
type ActionMacDst struct {
	cPointer
	Mac net.HardwareAddr
}

// Reload implements Action interface.
func (action *ActionMacDst) Reload() {
	cptr := (*C.struct_rte_flow_action_set_mac)(action.createOrRet(C.sizeof_struct_rte_flow_action_set_mac))

	for index := 0; index < 6; index++ {
		cptr.mac_addr[index] = C.uchar(action.Mac[index])
	}
	runtime.SetFinalizer(action, nil)
	runtime.SetFinalizer(action, (*ActionMacDst).free)
}

// Type implements Action interface.
func (action *ActionMacDst) Type() ActionType {
	return ActionTypeSetMacDst
}
