package h264reader

import "strconv"

type NalUnitType uint8

const (
	NalUnitTypeUnspecified              NalUnitType = 0  // Unspecified
	NalUnitTypeCodedSliceNonIdr         NalUnitType = 1  // Coded slice of a non-IDR picture
	NalUnitTypeCodedSliceDataPartitionA NalUnitType = 2  // Coded slice data partition A
	NalUnitTypeCodedSliceDataPartitionB NalUnitType = 3  // Coded slice data partition B
	NalUnitTypeCodedSliceDataPartitionC NalUnitType = 4  // Coded slice data partition C
	NalUnitTypeCodedSliceIdr            NalUnitType = 5  // Coded slice of an IDR picture
	NalUnitTypeSEI                      NalUnitType = 6  // Supplemental enhancement information (SEI)
	NalUnitTypeSPS                      NalUnitType = 7  // Sequence parameter set
	NalUnitTypePPS                      NalUnitType = 8  // Picture parameter set
	NalUnitTypeAUD                      NalUnitType = 9  // Access unit delimiter
	NalUnitTypeEndOfSequence            NalUnitType = 10 // End of sequence
	NalUnitTypeEndOfStream              NalUnitType = 11 // End of stream
	NalUnitTypeFiller                   NalUnitType = 12 // Filler data
	NalUnitTypeSpsExt                   NalUnitType = 13 // Sequence parameter set extension
	NalUnitTypeCodedSliceAux            NalUnitType = 19 // Coded slice of an auxiliary coded picture without partitioning
	// 14..18                                            // Reserved
	// 20..23                                            // Reserved
	// 24..31                                            // Unspecified
)

func (n *NalUnitType) String() string {
	var str string
	switch *n {
	case 0:
		str = "Unspecified"
	case 1:
		str = "CodedSliceNonIdr"
	case 2:
		str = "CodedSliceDataPartitionA"
	case 3:
		str = "CodedSliceDataPartitionB"
	case 4:
		str = "CodedSliceDataPartitionC"
	case 5:
		str = "CodedSliceIdr"
	case 6:
		str = "SEI"
	case 7:
		str = "SPS"
	case 8:
		str = "PPS"
	case 9:
		str = "AUD"
	case 10:
		str = "EndOfSequence"
	case 11:
		str = "EndOfStream"
	case 12:
		str = "Filler"
	case 13:
		str = "SpsExt"
	case 19:
		str = "NalUnitTypeCodedSliceAux"
	default:
		str = "Unknown"
	}
	str = str + "(" + strconv.FormatInt(int64(*n), 10) + ")"
	return str
}
