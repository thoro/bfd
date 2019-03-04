package bfd

import (
	"reflect"
	"testing"
)

func TestSessionStateString(t *testing.T) {
	if "Admin Down" != AdminDown.String() {
		t.Fail()
	}

	if "Down" != Down.String() {
		t.Fail()
	}

	if "Init" != Init.String() {
		t.Fail()
	}

	if "Up" != Up.String() {
		t.Fail()
	}

	if "SessionState(4)" != SessionState(4).String() {
		t.Fail()
	}
}

func TestDiagnsticCodeString(t *testing.T) {
	if "No Diagnostic" != NoDiagnostic.String() {
		t.Fail()
	}

	if "Control Detection Time Expired" != ControlDetectionTimeExpired.String() {
		t.Fail()
	}

	if "Echo Function Failed" != EchoFunctionFailed.String() {
		t.Fail()
	}

	if "Neighbor Signaled Session Down" != NeighborSignaledSessionDown.String() {
		t.Fail()
	}

	if "Forwardling Plane Reset" != ForwardingPlaneReset.String() {
		t.Fail()
	}

	if "Path Down" != PathDown.String() {
		t.Fail()
	}

	if "Concatenated Path Down" != ConcatenatedPathDown.String() {
		t.Fail()
	}

	if "Administratively Down" != AdministrativelyDown.String() {
		t.Fail()
	}

	if "Reverse Concatenated Path Down" != ReverseConcatenatedPathDown.String() {
		t.Fail()
	}

	if "DiagnosticCode(9)" != DiagnosticCode(9).String() {
		t.Fail()
	}
}

func TestAuthenticationTypeString(t *testing.T) {
	if "Reserved" != Reserved.String() {
		t.Fail()
	}

	if "Simple Password" != SimplePassword.String() {
		t.Fail()
	}

	if "Keyed MD5" != KeyedMD5.String() {
		t.Fail()
	}

	if "Meticulous Keyed MD5" != MeticulousKeyedMD5.String() {
		t.Fail()
	}

	if "Keyed SHA1" != KeyedSHA1.String() {
		t.Fail()
	}

	if "Meticulous Keyed SHA1" != MeticulousKeyedSHA1.String() {
		t.Fail()
	}

	if "AuthenticationType(6)" != AuthenticationType(6).String() {
		t.Fail()
	}
}

func TestControlPacketMarshalBinary(t *testing.T) {
	target := []byte{
		1<<5 | byte(NoDiagnostic),
		byte(Init)<<6 | byte(Yes)<<5 | byte(No)<<4 |
			byte(No)<<3 | byte(No)<<2 |
			byte(Yes)<<1 | byte(No),
		0,
		24,
		0, 79, 211, 106,
		0, 105, 208, 84,
		0, 15, 66, 64,
		0, 30, 132, 128,
		0, 0, 90, 12,
	}

	packet := ControlPacket{
		Version:                 1,
		State:                   Init,
		Poll:                    Yes,
		Demand:                  Yes,
		MyDiscriminator:         5231466,
		YourDiscriminator:       6934612,
		DesiredMinTxInterval:    1000000,
		RequiredMinRxInterval:   2000000,
		RequiredMinEchoInterval: 23052,
	}

	bytes, err := packet.MarshalBinary()

	if err != nil {
		t.Error(err.Error())
		return
	}

	if !reflect.DeepEqual(bytes, target) {
		t.Errorf("%v differs from %v", bytes, target)
	}

	// Validate Unmarshal code path
	var parsed ControlPacket
	err = parsed.UnmarshalBinary(target)

	if err != nil {
		t.Error(err.Error())
		return
	}

	if !reflect.DeepEqual(packet, parsed) {
		t.Errorf("%v differs from %v", parsed, packet)
	}

	if packet.GetAuthenticationType() != Reserved {
		t.Fail()
	}
}

func TestControlPacketWithSimplePassword(t *testing.T) {
	target := []byte{
		1<<5 | byte(NoDiagnostic),
		byte(Init)<<6 | byte(Yes)<<5 | byte(No)<<4 |
			byte(No)<<3 | byte(Yes)<<2 |
			byte(Yes)<<1 | byte(No),
		0,
		37,
		0, 79, 211, 106,
		0, 105, 208, 84,
		0, 15, 66, 64,
		0, 30, 132, 128,
		0, 0, 90, 12,
		1, 13, 5,
	}

	target = append(target, []byte("HelloWorld")...)

	// Validate if > 16 chars errors out
	packet := ControlPacket{
		Version:                 1,
		State:                   Init,
		Poll:                    Yes,
		Demand:                  Yes,
		MyDiscriminator:         5231466,
		YourDiscriminator:       6934612,
		DesiredMinTxInterval:    1000000,
		RequiredMinRxInterval:   2000000,
		RequiredMinEchoInterval: 23052,
		AuthenticationHeader: &SimplePasswordHeader{
			AuthKeyId: 5,
			Password:  "HelloWorldWithALongPassword",
		},
	}

	pw := packet.AuthenticationHeader.(*SimplePasswordHeader)

	bytes, err := packet.MarshalBinary()

	if err != ErrPasswordInvalidLength {
		t.Errorf("Expected %s Error", ErrPasswordInvalidLength)
		return
	}

	// validate if < 1 chars errors out
	pw.Password = ""

	bytes, err = packet.MarshalBinary()

	if err != ErrPasswordInvalidLength {
		t.Errorf("Expected %s Error", ErrPasswordInvalidLength)
		return
	}

	pw.Password = "HelloWorld"

	bytes, err = packet.MarshalBinary()

	if err != nil {
		t.Error(err.Error())
		return
	}

	if !reflect.DeepEqual(bytes, target) {
		t.Errorf("%v differs from %v", bytes, target)
	}

	// Validate Unmarshal code path
	var parsed ControlPacket
	err = parsed.UnmarshalBinary(target)

	if err != nil {
		t.Error(err.Error())
		return
	}

	if !reflect.DeepEqual(packet, parsed) {
		t.Errorf("%v differs from %v", parsed, packet)
	}

	if packet.GetAuthenticationType() != SimplePassword {
		t.Fail()
	}
}

func TestControlPacketInvalidPacketLength(t *testing.T) {
	target := []byte{
		1<<5 | byte(NoDiagnostic),
		byte(Init)<<6 | byte(Yes)<<5 | byte(No)<<4 |
			byte(No)<<3 | byte(Yes)<<2 |
			byte(Yes)<<1 | byte(No),
		0,
		37,
		0, 79, 211, 106,
		0, 105, 208, 84,
	}

	var parsed ControlPacket
	err := parsed.UnmarshalBinary(target)

	if err != ErrInvalidPacketLength {
		t.Error(err.Error())
		return
	}
}

func TestControlPacketInvalidPacketLength2(t *testing.T) {
	target := []byte{
		1<<5 | byte(NoDiagnostic),
		byte(Init)<<6 | byte(Yes)<<5 | byte(No)<<4 |
			byte(No)<<3 | byte(Yes)<<2 |
			byte(Yes)<<1 | byte(No),
		0,
		37,
		0, 79, 211, 106,
		0, 105, 208, 84,
		0, 105, 208, 84,
		0, 105, 208, 84,
		0, 105, 208, 84,
		0, 105, 208, 84,
		0, 105, 208, 84,
	}

	var parsed ControlPacket
	err := parsed.UnmarshalBinary(target)

	if err != ErrInvalidPacketLength {
		t.Error(err.Error())
		return
	}
}

func TestControlPacketInvalidAuthType(t *testing.T) {
	target := []byte{
		1<<5 | byte(NoDiagnostic),
		byte(Init)<<6 | byte(Yes)<<5 | byte(No)<<4 |
			byte(No)<<3 | byte(Yes)<<2 |
			byte(Yes)<<1 | byte(No),
		0,
		37,
		0, 79, 211, 106,
		0, 105, 208, 84,
		0, 15, 66, 64,
		0, 30, 132, 128,
		0, 0, 90, 12,
		9, 13, 5,
	}
	target = append(target, []byte("HelloWorld")...)

	var parsed ControlPacket
	err := parsed.UnmarshalBinary(target)

	if err != ErrInvalidAuthenticationType {
		t.Error(err.Error())
		return
	}

}

func TestSimplePasswordInvalidPacketLength(t *testing.T) {
	target := []byte{
		1, 11, 5,
	}

	target = append(target, []byte("HelloWorld")...)

	pw := SimplePasswordHeader{}

	err := pw.UnmarshalBinary(target)

	if err != ErrInvalidPacketLength {
		t.Fail()
	}
}

func TestSimplePasswordInvalidAuthenticationTypeLength(t *testing.T) {
	target := []byte{
		4, 13, 5,
	}

	target = append(target, []byte("HelloWorld")...)

	pw := SimplePasswordHeader{}

	err := pw.UnmarshalBinary(target)

	if err != ErrInvalidAuthenticationType {
		t.Fail()
	}
}

func TestCascadeSimplePasswordError(t *testing.T) {
	target := []byte{
		1<<5 | byte(NoDiagnostic),
		byte(Init)<<6 | byte(Yes)<<5 | byte(No)<<4 |
			byte(No)<<3 | byte(Yes)<<2 |
			byte(Yes)<<1 | byte(No),
		0,
		37,
		0, 79, 211, 106,
		0, 105, 208, 84,
		0, 15, 66, 64,
		0, 30, 132, 128,
		0, 0, 90, 12,
		1, 12, 5,
	}

	target = append(target, []byte("HelloWorld")...)

	var parsed ControlPacket
	err := parsed.UnmarshalBinary(target)

	if err != ErrInvalidPacketLength {
		t.Fail()
	}
}

func TestSimplePasswordValidation(t *testing.T) {
	password := "Hello World"
	pwd := &SimplePasswordHeader{
		AuthKeyId: 1,
		Password:  password,
	}

	if !pwd.IsValid([]byte(password), []byte{}) {
		t.Fail()
	}

	if pwd.IsValid([]byte("Another password"), []byte{}) {
		t.Fail()
	}
}
