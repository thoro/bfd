package server

import (
	"reflect"
	"testing"

	"github.com/Thoro/bfd/pkg/packet/bfd"
)

func NewState() *PeerState {
	return &PeerState{
		discriminator:         45230,
		sessionState:          bfd.Up,
		diagnosticCode:        bfd.ControlDetectionTimeExpired,
		desiredMinTxInterval:  4293,
		requiredMinRxInterval: 502891,
		detectMultiplier:      20,
		demandMode:            true,
	}
}

func TestPeerStateGetters(t *testing.T) {
	peerState := NewState()

	if peerState.GetDiscriminator() != peerState.discriminator {
		t.Fail()
	}

	if peerState.GetDesiredMinTxInterval() != peerState.desiredMinTxInterval {
		t.Fail()
	}

	if peerState.GetRequiredMinRxInterval() != peerState.requiredMinRxInterval {
		t.Fail()
	}

	if peerState.GetDetectMultiplier() != peerState.detectMultiplier {
		t.Fail()
	}

}

func TestPeerStateClone(t *testing.T) {
	peerState := NewState()

	clone := peerState.Clone([]PeerStateUpdate{})

	if !reflect.DeepEqual(peerState, clone) {
		t.Fail()
	}
}

func TestPeerStateSetDetectMultiplier(t *testing.T) {
	peerState := NewState()

	clone := peerState.Clone([]PeerStateUpdate{setDetectMultiplier(50)})

	if clone.detectMultiplier != 50 {
		t.Fail()
	}
}

func TestPeerStateSetRequiredMinRxInterval(t *testing.T) {
	peerState := NewState()

	clone := peerState.Clone([]PeerStateUpdate{setRequiredMinRxInterval(500)})

	if clone.requiredMinRxInterval != 500 {
		t.Fail()
	}
}

func TestPeerStateSetDesiredMinTxInterval(t *testing.T) {
	peerState := NewState()

	clone := peerState.Clone([]PeerStateUpdate{setDesiredMinTxInterval(500)})

	if clone.desiredMinTxInterval != 500 {
		t.Fail()
	}
}

func TestPeerStateSetSessionState(t *testing.T) {
	peerState := NewState()

	clone := peerState.Clone([]PeerStateUpdate{setSessionState(bfd.Down)})

	if clone.sessionState != bfd.Down {
		t.Fail()
	}
}

func TestPeerStateSetDiagnosticCode(t *testing.T) {
	peerState := NewState()

	clone := peerState.Clone([]PeerStateUpdate{setDiagnosticCode(bfd.NoDiagnostic)})

	if clone.diagnosticCode != bfd.NoDiagnostic {
		t.Fail()
	}
}

func TestPeerStateSetDemandMode(t *testing.T) {
	peerState := NewState()

	clone := peerState.Clone([]PeerStateUpdate{setDemandMode(false)})

	if clone.demandMode != false {
		t.Fail()
	}

	clone = peerState.Clone([]PeerStateUpdate{setDemandMode(true)})

	if clone.demandMode != true {
		t.Fail()
	}
}
