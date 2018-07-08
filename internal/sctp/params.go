package sctp

import "fmt"

type ParamType uint16

const (
	PARAM_HEARTBEAT_INFO       ParamType = 1     //Heartbeat Info	[RFC4960]
	PARAM_IPV4_ADDR            ParamType = 5     //IPv4 Address	[RFC4960]
	PARAM_IPV6_ADDR            ParamType = 6     //IPv6 Address	[RFC4960]
	PARAM_STATE_COOKIE         ParamType = 7     //State Cookie	[RFC4960]
	PARAM_UNRECOG_PARAMS       ParamType = 8     //Unrecognized Parameters	[RFC4960]
	PARAM_COOKIE_PRESERVATIVE  ParamType = 9     //Cookie Preservative	[RFC4960]
	PARAM_HOST_NAME_ADDR       ParamType = 11    //Host Name Address	[RFC4960]
	PARAM_SUPPORTED_ADDR_TYPES ParamType = 12    //Supported Address Types	[RFC4960]
	PARAM_OUT_SSN_RESET_REQ    ParamType = 13    //Outgoing SSN Reset Request Parameter	[RFC6525]
	PARAM_INC_SSN_RESET_REQ    ParamType = 14    //Incoming SSN Reset Request Parameter	[RFC6525]
	PARAM_SSN_TSN_RESET_REQ    ParamType = 15    //SSN/TSN Reset Request Parameter	[RFC6525]
	PARAM_RECONFIG_RESP        ParamType = 16    //Re-configuration Response Parameter	[RFC6525]
	PARAM_ADD_OUT_STREAM_REQ   ParamType = 17    //Add Outgoing Streams Request Parameter	[RFC6525]
	PARAM_ADD_INC_STREAM_REQ   ParamType = 18    //Add Incoming Streams Request Parameter	[RFC6525]
	PARAM_RANDOM               ParamType = 32770 //Random (0x8002)	[RFC4805]
	PARAM_CHUNK_LIST           ParamType = 32771 //Chunk List (0x8003)	[RFC4895]
	PARAM_REQ_HMAC_ALGO        ParamType = 32772 //Requested HMAC Algorithm Parameter (0x8004)	[RFC4895]
	PARAM_PADDING              ParamType = 32773 //Padding (0x8005)
	PARAM_SUPP_EXT             ParamType = 32776 //Supported Extensions (0x8008)	[RFC5061]
	PARAM_FORWARD_TSN_SUPP     ParamType = 49152 //Forward TSN supported (0xC000)	[RFC3758]
	PARAM_ADD_IP_ADDR          ParamType = 49153 //Add IP Address (0xC001)	[RFC5061]
	PARAM_DEL_IP_ADDR          ParamType = 49154 //Delete IP Address (0xC002)	[RFC5061]
	PARAM_ERR_CLAUSE_IND       ParamType = 49155 //Error Cause Indication (0xC003)	[RFC5061]
	PARAM_SET_PRI_ADDR         ParamType = 49156 //Set Primary Address (0xC004)	[RFC5061]
	PARAM_SUCCESS_IND          ParamType = 49157 //Success Indication (0xC005)	[RFC5061]
	PARAM_ADAPT_LAYER_IND      ParamType = 49158 //Adaptation Layer Indication (0xC006)	[RFC5061]
)

func (p ParamType) String() string {
	switch p {
	case PARAM_HEARTBEAT_INFO:
		return "Heartbeat Info"
	case PARAM_IPV4_ADDR:
		return "IPv4 Address"
	case PARAM_IPV6_ADDR:
		return "IPv6 Address"
	case PARAM_STATE_COOKIE:
		return "State Cookie"
	case PARAM_UNRECOG_PARAMS:
		return "Unrecognized Parameters"
	case PARAM_COOKIE_PRESERVATIVE:
		return "Cookie Preservative"
	case PARAM_HOST_NAME_ADDR:
		return "Host Name Address"
	case PARAM_SUPPORTED_ADDR_TYPES:
		return "Supported Address Types"
	case PARAM_OUT_SSN_RESET_REQ:
		return "Outgoing SSN Reset Request Parameter"
	case PARAM_INC_SSN_RESET_REQ:
		return "Incoming SSN Reset Request Parameter"
	case PARAM_SSN_TSN_RESET_REQ:
		return "SSN/TSN Reset Request Parameter"
	case PARAM_RECONFIG_RESP:
		return "Re-configuration Response Parameter"
	case PARAM_ADD_OUT_STREAM_REQ:
		return "Add Outgoing Streams Request Parameter"
	case PARAM_ADD_INC_STREAM_REQ:
		return "Add Incoming Streams Request Parameter"
	case PARAM_RANDOM:
		return "Random"
	case PARAM_CHUNK_LIST:
		return "Chunk List"
	case PARAM_REQ_HMAC_ALGO:
		return "Requested HMAC Algorithm Parameter"
	case PARAM_PADDING:
		return "Padding"
	case PARAM_SUPP_EXT:
		return "Supported Extensions"
	case PARAM_FORWARD_TSN_SUPP:
		return "Forward TSN supported"
	case PARAM_ADD_IP_ADDR:
		return "Add IP Address"
	case PARAM_DEL_IP_ADDR:
		return "Delete IP Address"
	case PARAM_ERR_CLAUSE_IND:
		return "Error Cause Indication"
	case PARAM_SET_PRI_ADDR:
		return "Set Primary Address"
	case PARAM_SUCCESS_IND:
		return "Success Indication"
	case PARAM_ADAPT_LAYER_IND:
		return "Adaptation Layer Indication"
	default:
		return fmt.Sprintf("Unknown ParamType: %d", p)
	}
}
