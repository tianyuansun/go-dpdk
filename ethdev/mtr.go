package ethdev

/*
#include <stdlib.h>
#include <stdint.h>
#include <stdio.h>              // snprintf
#include <net/if.h>

#include <rte_config.h>
#include <rte_version.h>
#include <rte_mtr.h>

static int add_srtcm_mtr_profile(uint16_t port, uint32_t profile_id, uint64_t cir, uint64_t cbs, uint64_t ebs) {
	int ret = 0;
	struct rte_mtr_error error;
	struct rte_mtr_meter_profile profile;
	memset(&error, 0, sizeof(error));
	memset(&profile, 0, sizeof(profile));

	profile.alg = RTE_MTR_SRTCM_RFC2697;
	profile.srtcm_rfc2697.cir = cir;
	profile.srtcm_rfc2697.cbs = cbs;
	profile.srtcm_rfc2697.ebs = ebs;
	profile.packet_mode = 0;

	ret = rte_mtr_meter_profile_add(port, profile_id, &profile, &error);
	if (ret != 0) {
		fprintf(stderr, "failed to add_srtcm_mtr_profile, profile_id is %d, error is %s\n", profile_id, error.message);
	}
	return ret;
}

static int add_mtr_policy(uint16_t port, uint32_t policy_id) {
	int ret = 0;
	struct rte_mtr_error error;
	memset(&error, 0, sizeof(error));
	struct rte_mtr_meter_policy_params policy = \
    { \
	    .actions[RTE_COLOR_GREEN] = NULL, \
	    .actions[RTE_COLOR_YELLOW] = NULL, \
	    .actions[RTE_COLOR_RED] = (struct rte_flow_action[]) { \
	    	{ \
		    	.type = RTE_FLOW_ACTION_TYPE_DROP, \
		    }, \
		    { \
		    	.type = RTE_FLOW_ACTION_TYPE_END, \
		    }, \
	    }, \
    };
	ret = rte_mtr_meter_policy_add(port, policy_id, &policy, &error);
	if (ret != 0) {
		fprintf(stderr, "failed to add meter policy, policy id is %d, error is %s\n", policy_id, error.message);
	}
	return ret;
}

static int add_mtr(uint16_t port, uint32_t mtr_id, uint32_t profile_id, uint32_t policy_id) {
	int ret = 0;
	struct rte_mtr_error error;
	memset(&error, 0, sizeof(error));
	struct rte_mtr_params params;
	memset(&params, 0, sizeof(params));
	params.meter_profile_id = profile_id;
	params.meter_policy_id = policy_id;
	params.use_prev_mtr_color = 0;
	params.meter_enable = 1;
	ret = rte_mtr_create(port, mtr_id, &params, 1, &error);
	if (ret != 0) {
		fprintf(stderr, "failed to add mtr, mtr id is %d, error is %s\n", mtr_id, error.message);
	}
	return ret;
}

*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/tianyuansun/go-dpdk/common"
)

type MtrError C.struct_rte_mtr_error

func (e *MtrError) Error() string {
	return fmt.Sprintf("%v: %s", e.Unwrap(), C.GoString(e.message))
}

func (e *MtrError) Unwrap() error {
	return ErrorType(e._type)
}

// ErrorType is a type of an error.
type ErrorType uint

func (e ErrorType) Error() string {
	if s, ok := errStr[e]; ok {
		return s
	}
	return ""
}

var (
	errStr = make(map[ErrorType]string)
)

func (e *MtrError) Cause() unsafe.Pointer {
	return e.cause
}

func AddMeterProfile(port Port, profileID uint32, cir, cbs, ebs uint64) error {
	return common.IntToErr(C.add_srtcm_mtr_profile(C.ushort(port), C.uint32_t(profileID), C.uint64_t(cir), C.uint64_t(cbs), C.uint64_t(ebs)))
}

func DeleteMeterProfile(port Port, profileID uint32, mtrError *MtrError) error {
	return common.IntToErr(C.rte_mtr_meter_profile_delete(C.ushort(port), C.uint32_t(profileID), (*C.struct_rte_mtr_error)(mtrError)))
}

func AddMeterPolicy(port Port, policyID uint32) error {
	return common.IntToErr(C.add_mtr_policy(C.ushort(port), C.uint32_t(policyID)))
}

func DeleteMeterPolicy(port Port, policyID uint32, mtrError *MtrError) error {
	return common.IntToErr(C.rte_mtr_meter_policy_delete(C.ushort(port), C.uint32_t(policyID), (*C.struct_rte_mtr_error)(mtrError)))
}

func AddMtr(port Port, mtrID uint32, profileID, policyID uint32) error {
	return common.IntToErr(C.add_mtr(C.ushort(port), C.uint32_t(mtrID), C.uint32_t(profileID), C.uint32_t(policyID)))
}

func DeleteMtr(port Port, mtrID uint32, mtrError *MtrError) error {
	return common.IntToErr(C.rte_mtr_destroy(C.ushort(port), C.uint32_t(mtrID), (*C.struct_rte_mtr_error)(mtrError)))
}
