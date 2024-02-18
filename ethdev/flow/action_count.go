package flow

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_flow.h>

*/
import "C"
import (
	"runtime"
)

var _ Action = (*ActionCount)(nil)

type ActionCount struct {
	cPointer

	ID uint32
}

func (action *ActionCount) Reload() {
	cptr := (*C.struct_rte_flow_action_count)(action.createOrRet(C.sizeof_struct_rte_flow_action_count))

	cptr.id = C.uint32_t(action.ID)
	runtime.SetFinalizer(action, nil)
	runtime.SetFinalizer(action, (*ActionCount).free)
}

// Type implements Action interface.
func (action *ActionCount) Type() ActionType {
	return ActionTypeCount
}
