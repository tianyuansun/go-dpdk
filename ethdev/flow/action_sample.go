package flow

/*
#include <stdint.h>
#include <rte_config.h>
#include <rte_flow.h>
*/
import "C"

var _ Action = (*ActionSample)(nil)

// ActionSample implements Action which assigns packets to a given
// queue index.
type ActionSample struct {
	cPointer

	Ratio  uint32
	PortID ActionPortID
}

// Reload implements Action interface.
func (action *ActionSample) Reload() {
	cptr := (*C.struct_rte_flow_action_sample)(action.createOrRet(C.sizeof_struct_rte_flow_action_sample))

	cptr.ratio = C.uint32_t(action.Ratio)

	actions := []Action{
		&action.PortID,
	}
	act := cActions(actions)
	cptr.actions = &act[0]

	// patterns := []Item{
	// 	{Spec: &action.Ether},
	// 	{Spec: &action.IPv4},
	// 	{Spec: &action.UDP},
	// 	{Spec: &action.Vxlan},
	// 	// {Spec: ItemTypeEnd},
	// }
	// pat := cPattern(patterns)
	// cptr.definition = &pat[0]
	// runtime.SetFinalizer(action, nil)
	// runtime.SetFinalizer(action, (*ActionSample).free)
}

// Type implements Action interface.
func (action *ActionSample) Type() ActionType {
	return ActionTypeSample
}
