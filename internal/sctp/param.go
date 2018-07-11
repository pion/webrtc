package sctp

import (
	"fmt"
	"github.com/pkg/errors"
)

// ParamType represents a SCTP INIT/INITACK parameter
type ParamType uint16

// Param interface
type Param interface {
	Length() int
}

// BuildParam delegates the building of a parameter from raw bytes to the correct structure
func BuildParam(t ParamType, rawParam []byte) (Param, error) {
	switch t {
	case ForwardTSNSupp:
		return (&ParamForwardTSNSupported{}).Unmarshal(rawParam)
	case SupportedExt:
		return (&ParamSupportedExtensions{}).Unmarshal(rawParam)
	case Random:
		return (&ParamRandom{}).Unmarshal(rawParam)
	case ReqHMACAlgo:
		return (&ParamRequestedHMACAlgorithm{}).Unmarshal(rawParam)
	case ChunkList:
		return (&ParamChunkList{}).Unmarshal(rawParam)
	}

	return nil, errors.Errorf("Unhandled ParamType %v", t)
}

// Parameter Types
const (
	HeartbeanInfo      ParamType = 1     //Heartbeat Info	[RFC4960]
	IPV4Addr           ParamType = 5     //IPv4 Address	[RFC4960]
	IPV6Addr           ParamType = 6     //IPv6 Address	[RFC4960]
	StateCookie        ParamType = 7     //State Cookie	[RFC4960]
	UnrecognizedParam  ParamType = 8     //Unrecognized Parameters	[RFC4960]
	CookiePreservative ParamType = 9     //Cookie Preservative	[RFC4960]
	HostNameAddr       ParamType = 11    //Host Name Address	[RFC4960]
	SupportedAddrTypes ParamType = 12    //Supported Address Types	[RFC4960]
	OutSSNResetReq     ParamType = 13    //Outgoing SSN Reset Request Parameter	[RFC6525]
	IncSSNResetReq     ParamType = 14    //Incoming SSN Reset Request Parameter	[RFC6525]
	SSNTSNResetReq     ParamType = 15    //SSN/TSN Reset Request Parameter	[RFC6525]
	ReconfigResp       ParamType = 16    //Re-configuration Response Parameter	[RFC6525]
	AddOutStreamsReq   ParamType = 17    //Add Outgoing Streams Request Parameter	[RFC6525]
	AddIncStreamsReq   ParamType = 18    //Add Incoming Streams Request Parameter	[RFC6525]
	Random             ParamType = 32770 //Random (0x8002)	[RFC4805]
	ChunkList          ParamType = 32771 //Chunk List (0x8003)	[RFC4895]
	ReqHMACAlgo        ParamType = 32772 //Requested HMAC Algorithm Parameter (0x8004)	[RFC4895]
	Padding            ParamType = 32773 //Padding (0x8005)
	SupportedExt       ParamType = 32776 //Supported Extensions (0x8008)	[RFC5061]
	ForwardTSNSupp     ParamType = 49152 //Forward TSN supported (0xC000)	[RFC3758]
	AddIPAddr          ParamType = 49153 //Add IP Address (0xC001)	[RFC5061]
	DelIPAddr          ParamType = 49154 //Delete IP Address (0xC002)	[RFC5061]
	ErrClauseInd       ParamType = 49155 //Error Cause Indication (0xC003)	[RFC5061]
	SetPriAddr         ParamType = 49156 //Set Primary Address (0xC004)	[RFC5061]
	SuccessInd         ParamType = 49157 //Success Indication (0xC005)	[RFC5061]
	AdaptLayerInd      ParamType = 49158 //Adaptation Layer Indication (0xC006)	[RFC5061]
)

func (p ParamType) String() string {
	switch p {
	case HeartbeanInfo:
		return "Heartbeat Info"
	case IPV4Addr:
		return "IPv4 Address"
	case IPV6Addr:
		return "IPv6 Address"
	case StateCookie:
		return "State Cookie"
	case UnrecognizedParam:
		return "Unrecognized Parameters"
	case CookiePreservative:
		return "Cookie Preservative"
	case HostNameAddr:
		return "Host Name Address"
	case SupportedAddrTypes:
		return "Supported Address Types"
	case OutSSNResetReq:
		return "Outgoing SSN Reset Request Parameter"
	case IncSSNResetReq:
		return "Incoming SSN Reset Request Parameter"
	case SSNTSNResetReq:
		return "SSN/TSN Reset Request Parameter"
	case ReconfigResp:
		return "Re-configuration Response Parameter"
	case AddOutStreamsReq:
		return "Add Outgoing Streams Request Parameter"
	case AddIncStreamsReq:
		return "Add Incoming Streams Request Parameter"
	case Random:
		return "Random"
	case ChunkList:
		return "Chunk List"
	case ReqHMACAlgo:
		return "Requested HMAC Algorithm Parameter"
	case Padding:
		return "Padding"
	case SupportedExt:
		return "Supported Extensions"
	case ForwardTSNSupp:
		return "Forward TSN supported"
	case AddIPAddr:
		return "Add IP Address"
	case DelIPAddr:
		return "Delete IP Address"
	case ErrClauseInd:
		return "Error Cause Indication"
	case SetPriAddr:
		return "Set Primary Address"
	case SuccessInd:
		return "Success Indication"
	case AdaptLayerInd:
		return "Adaptation Layer Indication"
	default:
		return fmt.Sprintf("Unknown ParamType: %d", p)
	}
}
