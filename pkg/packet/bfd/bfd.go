package bfd

import (
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	AdminDown SessionState = 0
	Down      SessionState = 1
	Init      SessionState = 2
	Up        SessionState = 3
)

type SessionState int8

func (s SessionState) String() string {
	switch s {
	case Down:
		return "Down"
	case Init:
		return "Init"
	case Up:
		return "Up"
	case AdminDown:
		return "Admin Down"
	default:
		return fmt.Sprintf("SessionState(%d)", s)
	}
}

const (
	NoDiagnostic                DiagnosticCode = 0
	ControlDetectionTimeExpired DiagnosticCode = 1
	EchoFunctionFailed          DiagnosticCode = 2
	NeighborSignaledSessionDown DiagnosticCode = 3
	ForwardingPlaneReset        DiagnosticCode = 4
	PathDown                    DiagnosticCode = 5
	ConcatenatedPathDown        DiagnosticCode = 6
	AdministrativelyDown        DiagnosticCode = 7
	ReverseConcatenatedPathDown DiagnosticCode = 8
)

type DiagnosticCode int8

func (d DiagnosticCode) String() string {
	switch d {
	case NoDiagnostic:
		return "No Diagnostic"
	case ControlDetectionTimeExpired:
		return "Control Detection Time Expired"
	case EchoFunctionFailed:
		return "Echo Function Failed"
	case NeighborSignaledSessionDown:
		return "Neighbor Signaled Session Down"
	case ForwardingPlaneReset:
		return "Forwardling Plane Reset"
	case PathDown:
		return "Path Down"
	case ConcatenatedPathDown:
		return "Concatenated Path Down"
	case AdministrativelyDown:
		return "Administratively Down"
	case ReverseConcatenatedPathDown:
		return "Reverse Concatenated Path Down"
	default:
		return fmt.Sprintf("DiagnosticCode(%d)", d)
	}
}

type AuthenticationType int8

const (
	Reserved            AuthenticationType = 0
	SimplePassword      AuthenticationType = 1
	KeyedMD5            AuthenticationType = 2
	MeticulousKeyedMD5  AuthenticationType = 3
	KeyedSHA1           AuthenticationType = 4
	MeticulousKeyedSHA1 AuthenticationType = 5
)

func (a AuthenticationType) String() string {
	switch a {
	case Reserved:
		return "Reserved"
	case SimplePassword:
		return "Simple Password"
	case KeyedMD5:
		return "Keyed MD5"
	case MeticulousKeyedMD5:
		return "Meticulous Keyed MD5"
	case KeyedSHA1:
		return "Keyed SHA1"
	case MeticulousKeyedSHA1:
		return "Meticulous Keyed SHA1"
	default:
		return fmt.Sprintf("AuthenticationType(%d)", a)
	}
}

type Bool int8

const (
	No  Bool = 0
	Yes Bool = 1
)

const (
	MINIMUM_SIZE = 24
	MAXIMUM_SIZE = MINIMUM_SIZE + 28 // 24 bfd packet min size + 28 auth header max size
)

type ControlPacket struct {
	Version                 int8 // version 1 by default
	DiagnosticCode          DiagnosticCode
	State                   SessionState
	Poll                    Bool // requests a Final bit packet
	Final                   Bool // required in response to poll
	ControlPlaneIndependent Bool //
	Demand                  Bool // stop sending control packets (we know we are up)
	Multipoint              Bool // needs to be zero
	DetectMultiplier        uint8
	MyDiscriminator         uint32
	YourDiscriminator       uint32
	DesiredMinTxInterval    uint32
	RequiredMinRxInterval   uint32
	RequiredMinEchoInterval uint32

	AuthenticationHeader AuthenticationHeader
}

type AuthenticationHeader interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	IsValid(key []byte, packet []byte) bool
	GetAuthenticationType() AuthenticationType
}

type SimplePasswordHeader struct {
	AuthKeyId int8
	Password  string // between 1 and 16 characters
}

var ErrPasswordInvalidLength = errors.New("Password needs to be between 1 and 16")
var ErrInvalidAuthenticationType = errors.New("AuthenticationType is invalid")
var ErrInvalidPacketLength = errors.New("Invalid packet length")

func (s *SimplePasswordHeader) IsValid(key []byte, packet []byte) bool {
	return s.Password == string(key)
}

func (s *SimplePasswordHeader) UnmarshalBinary(buf []byte) error {
	if AuthenticationType(buf[0]) != SimplePassword {
		return ErrInvalidAuthenticationType
	}

	l := int(buf[1])

	if len(buf) != l {
		return ErrInvalidPacketLength
	}

	s.AuthKeyId = int8(buf[2])
	s.Password = string(buf[3:])

	return nil
}

func (s *SimplePasswordHeader) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 3)

	if len(s.Password) < 1 || len(s.Password) > 16 {
		return nil, ErrPasswordInvalidLength
	}

	buf[0] = byte(SimplePassword)
	buf[2] = byte(s.AuthKeyId)

	buf = append(buf, []byte(s.Password)...)

	buf[1] = byte(len(buf))

	return buf, nil
}

func (s *SimplePasswordHeader) GetAuthenticationType() AuthenticationType {
	return SimplePassword
}

type KeyedMD5Header struct {
	AuthType AuthenticationType
	// Length int8 = inferred
	AuthKeyId      int8
	SequenceNumber int32
	AuthKey        []byte // 16 bytes long
}

type KeyedSHA1Header struct {
	AuthType AuthenticationType
	// Length int8 = inferred
	AuthKeyId      int8
	SequenceNumber int32
	AuthKey        []byte // 20 bytes long
}

/*
    0                   1                   2                   3
    0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |Vers |  Diag   |Sta|P|F|C|A|D|M|  Detect Mult  |    Length     |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                       My Discriminator                        |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                      Your Discriminator                       |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                    Desired Min TX Interval                    |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                   Required Min RX Interval                    |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |                 Required Min Echo RX Interval                 |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

   An optional Authentication Section MAY be present:

    0                   1                   2                   3
    0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
   |   Auth Type   |   Auth Len    |    Authentication Data...     |
   +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
*/

func (c *ControlPacket) UnmarshalBinary(buf []byte) error {
	if len(buf) < MINIMUM_SIZE {
		return ErrInvalidPacketLength
	}

	l := int(buf[3])

	if len(buf) != l {
		return ErrInvalidPacketLength
	}

	c.Version = int8(buf[0] & 0xE0 >> 5)
	c.DiagnosticCode = DiagnosticCode(buf[0] & 0x1F)
	c.State = SessionState((buf[1] & 0xC0) >> 6)
	c.Poll = Bool((buf[1] >> 5) & 1)
	c.Final = Bool((buf[1] >> 4) & 1)
	c.ControlPlaneIndependent = Bool((buf[1] >> 3) & 1)
	c.Demand = Bool((buf[1] >> 1) & 1)
	c.Multipoint = Bool((buf[1] >> 0) & 1)
	c.DetectMultiplier = uint8(buf[2])

	c.MyDiscriminator = binary.BigEndian.Uint32(buf[4:])
	c.YourDiscriminator = binary.BigEndian.Uint32(buf[8:])
	c.DesiredMinTxInterval = binary.BigEndian.Uint32(buf[12:])
	c.RequiredMinRxInterval = binary.BigEndian.Uint32(buf[16:])
	c.RequiredMinEchoInterval = binary.BigEndian.Uint32(buf[20:])

	if ((buf[1] >> 2) & 1) == 1 {
		// Parse authentication header
		switch AuthenticationType(buf[24]) {
		case SimplePassword:
			var pw SimplePasswordHeader

			if err := pw.UnmarshalBinary(buf[24:]); err != nil {
				return err
			}

			c.AuthenticationHeader = &pw
		default:
			return ErrInvalidAuthenticationType
		}
	}

	return nil
}

func (c *ControlPacket) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 24)

	buf[0] = byte(c.Version)<<5 | byte(c.DiagnosticCode)
	buf[1] = byte(c.State)<<6 | byte(c.Poll)<<5 | byte(c.Final)<<4 |
		byte(c.ControlPlaneIndependent)<<3 |
		byte(c.Demand)<<1 | byte(c.Multipoint)
	buf[2] = byte(c.DetectMultiplier)

	binary.BigEndian.PutUint32(buf[4:], uint32(c.MyDiscriminator))
	binary.BigEndian.PutUint32(buf[8:], uint32(c.YourDiscriminator))
	binary.BigEndian.PutUint32(buf[12:], uint32(c.DesiredMinTxInterval))
	binary.BigEndian.PutUint32(buf[16:], uint32(c.RequiredMinRxInterval))
	binary.BigEndian.PutUint32(buf[20:], uint32(c.RequiredMinEchoInterval))

	if c.AuthenticationHeader != nil {
		// Set AuthenticationHeader present flag
		buf[1] = buf[1] | byte(Yes)<<2
		bytes, err := c.AuthenticationHeader.MarshalBinary()

		if err != nil {
			return nil, err
		}

		buf = append(buf, bytes...)
	}

	// Set the length in the end
	buf[3] = byte(len(buf))

	return buf, nil
}

func (c *ControlPacket) GetAuthenticationType() AuthenticationType {
	if c.AuthenticationHeader == nil {
		return Reserved
	}

	return c.AuthenticationHeader.GetAuthenticationType()
}
