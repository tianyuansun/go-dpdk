package flow

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_flow.h>

*/
import "C"

var _ Action = (*ActionMeter)(nil)

type ActionMeter struct {
	cPointer

	MtrID uint32
}

func (action *ActionMeter) Reload() {
	cptr := (*C.struct_rte_flow_action_meter)(action.createOrRet(C.sizeof_struct_rte_flow_action_meter))

	cptr.mtr_id = C.uint32_t(action.MtrID)
	// runtime.SetFinalizer(action, nil)
	// runtime.SetFinalizer(action, (*ActionMeter).free)
}

// Type implements Action interface.
func (action *ActionMeter) Type() ActionType {
	return ActionTypeMeter
}
