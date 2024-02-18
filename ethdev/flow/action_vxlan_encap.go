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

var _ Action = (*ActionVxlanEncap)(nil)

// ActionVxlanEncap implements Action which assigns packets to a given
// queue index.
type ActionVxlanEncap struct {
	cPointer
	Ether ItemEth
	IPv4  ItemIPv4
	UDP   ItemUDP
	Vxlan ItemVXLAN
}

// Reload implements Action interface.
func (action *ActionVxlanEncap) Reload() {
	cptr := (*C.struct_rte_flow_action_vxlan_encap)(action.createOrRet(C.sizeof_struct_rte_flow_action_vxlan_encap))
	patterns := []Item{
		{Spec: &action.Ether},
		{Spec: &action.IPv4},
		{Spec: &action.UDP},
		{Spec: &action.Vxlan},
		// {Spec: ItemTypeEnd},
	}
	pat := cPattern(patterns)
	cptr.definition = &pat[0]
	runtime.SetFinalizer(action, nil)
	runtime.SetFinalizer(action, (*ActionVxlanEncap).free)
}

// Type implements Action interface.
func (action *ActionVxlanEncap) Type() ActionType {
	return ActionTypeVxlanEncap
}
