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

var _ Action = (*ActionMacSrc)(nil)

// ActionMacSrc implements Action which assigns packets to a given
// queue index.
type ActionMacSrc struct {
	cPointer
	Mac net.HardwareAddr
}

// Reload implements Action interface.
func (action *ActionMacSrc) Reload() {
	cptr := (*C.struct_rte_flow_action_set_mac)(action.createOrRet(C.sizeof_struct_rte_flow_action_set_mac))

	for index := 0; index < 6; index++ {
		cptr.mac_addr[index] = C.uchar(action.Mac[index])
	}
	runtime.SetFinalizer(action, nil)
	runtime.SetFinalizer(action, (*ActionMacSrc).free)
}

// Type implements Action interface.
func (action *ActionMacSrc) Type() ActionType {
	return ActionTypeSetMacSrc
}
