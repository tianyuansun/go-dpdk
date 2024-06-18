package flow

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_flow.h>

*/
import "C"

var _ Action = (*ActionPortID)(nil)

type ActionPortID struct {
	cPointer

	ID uint32
}

func (action *ActionPortID) Reload() {
	cptr := (*C.struct_rte_flow_action_port_id)(action.createOrRet(C.sizeof_struct_rte_flow_action_port_id))

	cptr.id = C.uint32_t(action.ID)
	// runtime.SetFinalizer(action, nil)
	// runtime.SetFinalizer(action, (*ActionPortID).free)
}

// Type implements Action interface.
func (action *ActionPortID) Type() ActionType {
	return ActionTypePortID
}
