package server

import (
	"github.com/Thoro/bfd/pkg/api"
	"github.com/Thoro/bfd/pkg/packet/bfd"
)

type PeerState struct {
	discriminator         uint32
	sessionState          bfd.SessionState
	diagnosticCode        bfd.DiagnosticCode
	desiredMinTxInterval  uint32
	requiredMinRxInterval uint32
	detectMultiplier      uint8
	demandMode            bool
}

func (state *PeerState) GetDiscriminator() uint32 {
	return state.discriminator
}

func (state *PeerState) GetDesiredMinTxInterval() uint32 {
	return state.desiredMinTxInterval
}

func (state *PeerState) GetRequiredMinRxInterval() uint32 {
	return state.requiredMinRxInterval
}

func (state *PeerState) GetDetectMultiplier() uint8 {
	return state.detectMultiplier
}

// Clone creates a copy and applies the passed state updates
func (state *PeerState) Clone(updates []PeerStateUpdate) *PeerState {
	newstate := &PeerState{
		discriminator:         state.discriminator,
		sessionState:          state.sessionState,
		diagnosticCode:        state.diagnosticCode,
		desiredMinTxInterval:  state.desiredMinTxInterval,
		requiredMinRxInterval: state.requiredMinRxInterval,
		detectMultiplier:      state.detectMultiplier,
		demandMode:            state.demandMode,
	}

	for _, update := range updates {
		update(newstate)
	}

	return newstate
}

func (state *PeerState) ToApi() *api.PeerState {
	return &api.PeerState{
		State:      api.SessionState(state.sessionState),
		Diagnostic: api.DiagnosticCode(state.diagnosticCode),
	}
}

type PeerStateUpdate func(*PeerState)

func setDetectMultiplier(multiplier uint8) PeerStateUpdate {
	return func(state *PeerState) {
		state.detectMultiplier = multiplier
	}
}

func setRequiredMinRxInterval(required uint32) PeerStateUpdate {
	return func(state *PeerState) {
		state.requiredMinRxInterval = required
	}
}

func setDesiredMinTxInterval(desired uint32) PeerStateUpdate {
	return func(state *PeerState) {
		state.desiredMinTxInterval = desired
	}
}

func setSessionState(sessionState bfd.SessionState) PeerStateUpdate {
	return func(state *PeerState) {
		state.sessionState = sessionState
	}
}

func setDiagnosticCode(diagnostic bfd.DiagnosticCode) PeerStateUpdate {
	return func(state *PeerState) {
		state.diagnosticCode = diagnostic
	}
}

func setDemandMode(demandMode bool) PeerStateUpdate {
	return func(state *PeerState) {
		state.demandMode = demandMode
	}
}

func setDiscriminator(discriminator uint32) PeerStateUpdate {
	return func(state *PeerState) {
		state.discriminator = discriminator
	}
}
