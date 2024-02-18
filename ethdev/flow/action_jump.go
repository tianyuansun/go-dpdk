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

var _ Action = (*ActionJump)(nil)

type ActionJump struct {
	cPointer

	Group uint32
}

func (action *ActionJump) Reload() {
	cptr := (*C.struct_rte_flow_action_jump)(action.createOrRet(C.sizeof_struct_rte_flow_action_jump))

	cptr.group = C.uint32_t(action.Group)
	runtime.SetFinalizer(action, nil)
	runtime.SetFinalizer(action, (*ActionJump).free)
}

// Type implements Action interface.
func (action *ActionJump) Type() ActionType {
	return ActionTypeJump
}
