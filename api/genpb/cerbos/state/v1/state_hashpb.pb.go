// Code generated by protoc-gen-go-hashpb. DO NOT EDIT.
// protoc-gen-go-hashpb v0.4.2
// Source: cerbos/state/v1/state.proto

package statev1

import (
	hash "hash"
)

// HashPB computes a hash of the message using the given hash function
// The ignore set must contain fully-qualified field names (pkg.msg.field) that should be ignored from the hash
func (m *TelemetryState) HashPB(hasher hash.Hash, ignore map[string]struct{}) {
	if m != nil {
		cerbos_state_v1_TelemetryState_hashpb_sum(m, hasher, ignore)
	}
}
