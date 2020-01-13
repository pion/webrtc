package main

import (
    "io"
    "log"
    "os"
    "strconv"
    "time"
)

type FindNalState struct {
    PrefixCount int
    LastNullCount int
    buf []byte
}
func NewFindNalState() FindNalState {
    return FindNalState{PrefixCount: 0, LastNullCount: 0, buf: make([]byte, 0)}
}
func (h *FindNalState) NalScan(data []byte) [][]byte {
    if len(h.buf) > 1024 * 1024 { panic("FindNalState buf len panic") }
    nals := make([][]byte, 0)

    // offset after a NAL prefix (0x00_00_01 or 0x00_00_00_01) in the data buffer
    var lastPrefixOffset *int = nil
    i := 0
    for {
        if i >= len(data) {
            if lastPrefixOffset != nil {
                // prefix was founded
                // copy a part of data buffer from the end of the last prefix into the temporary buffer
                h.buf = make([]byte, 0)
                h.buf = append(h.buf, data[*lastPrefixOffset:]...)
            } else {
                // a prefix was not found, save all data into the temporary buffer
                h.buf = append(h.buf, data...)
            }
            break
        }
        b := data[i]; i += 1
        switch b {
        case 0x00: { if h.LastNullCount < 3 { h.LastNullCount += 1 }; continue }
        case 0x01: {
            if h.LastNullCount >= 2 { // found a NAL prefix 0x00_00_01 or 0x00_00_00_01

                prefixOffset := i
                if lastPrefixOffset != nil {
                    // NAL is a part of data from the end of the last prefix to the beginning of the current prefix. Save it
                    size := (i - h.LastNullCount) - *lastPrefixOffset - 1
                    if size > 0 && h.PrefixCount > 0 {
                        nal := data[*lastPrefixOffset : *lastPrefixOffset + size]
                        // save nal
                        nals = append(nals, nal)
                    }
                } else {
                    // a previous (last) prefix isn't exist
                    // NAL is the temporary buffer and a part of data from the beginning to the current prefix
                    size := i - h.LastNullCount - 1
                    nal := make([]byte, 0)
                    if size < 0 {
                        if len(h.buf) > 0 {
                            nal = append(nal, h.buf[0 : len(h.buf) + size]...)
                        }
                    } else {
                        nal = append(nal, h.buf...)
                        nal = append(nal, data[0 : size]...)
                    }

                    // save non-empty NAL only after at least one prefix was detected
                    if len(nal) > 0 && h.PrefixCount > 0 {
                        nals = append(nals, nal)
                    }
                    h.buf = make([]byte, 0)
                }
                p := prefixOffset
                lastPrefixOffset = &p
                h.PrefixCount += 1
            }
        }
        default:
        }
        h.LastNullCount = 0
    }
    return nals
}

type NalUnitType uint8
const( //   Table 7-1 NAL unit type codes
    Unspecified NalUnitType = 0                // Unspecified
    CodedSliceNonIdr NalUnitType = 1           // Coded slice of a non-IDR picture
    CodedSliceDataPartitionA NalUnitType = 2   // Coded slice data partition A
    CodedSliceDataPartitionB NalUnitType = 3   // Coded slice data partition B
    CodedSliceDataPartitionC NalUnitType = 4   // Coded slice data partition C
    CodedSliceIdr NalUnitType = 5              // Coded slice of an IDR picture
    SEI NalUnitType = 6                        // Supplemental enhancement information (SEI)
    SPS NalUnitType = 7                        // Sequence parameter set
    PPS NalUnitType = 8                        // Picture parameter set
    AUD NalUnitType = 9                        // Access unit delimiter
    EndOfSequence NalUnitType = 10             // End of sequence
    EndOfStream NalUnitType = 11               // End of stream
    Filler NalUnitType = 12                    // Filler data
    SpsExt NalUnitType = 13                    // Sequence parameter set extension
    // 14..18           // Reserved
    NalUnitTypeCodedSliceAux NalUnitType = 19  // Coded slice of an auxiliary coded picture without partitioning
    // 20..23           // Reserved
    // 24..31           // Unspecified
)
func NalUnitTypeStr(v NalUnitType) string {
    str := "Unknown"
    switch v {
    case 0: { str = "Unspecified" }
    case 1: { str = "CodedSliceNonIdr" }
    case 2: { str = "CodedSliceDataPartitionA" }
    case 3: { str = "CodedSliceDataPartitionB" }
    case 4: { str = "CodedSliceDataPartitionC" }
    case 5: { str = "CodedSliceIdr" }
    case 6: { str = "SEI" }
    case 7: { str = "SPS" }
    case 8: { str = "PPS" }
    case 9: { str = "AUD" }
    case 10: { str = "EndOfSequence" }
    case 11: { str = "EndOfStream" }
    case 12: { str = "Filler" }
    case 13: { str = "SpsExt" }
    case 19: { str = "NalUnitTypeCodedSliceAux" }
    default: { str = "Unknown" }
    }
    str = str + "(" + strconv.FormatInt(int64(v), 10) + ")"
    return str
}

type Nal struct {
    PictureOrderCount uint32

    // NAL header
    ForbiddenZeroBit bool
    RefIdc uint8
    UnitType NalUnitType

    Data []byte // header byte + rbsp
}
func NewNal() Nal {
    return Nal {PictureOrderCount: 0, ForbiddenZeroBit: false, RefIdc: 0, UnitType: Unspecified, Data: make([]byte, 0)}
}
func (h *Nal) ParseHeader(firstByte byte) {
    h.ForbiddenZeroBit = (((firstByte & 0x80) >> 7) == 1)  // 0x80 = 0b10000000
    h.RefIdc =             (firstByte & 0x60) >> 5    // 0x60 = 0b01100000
    h.UnitType = NalUnitType((firstByte & 0x1F) >> 0) // 0x1F = 0b00011111
}

func loadFile(path string, nalChn chan Nal, fps uint32) error {
    file, err := os.Open(path)
    if err != nil {
        log.Println("Can't open file '" + path + "':\n", err)
        return err
    }

    nals := make([][]byte, 0)
    nalStream := NewFindNalState()
    for {
        buf := make([]byte, 1024)
        n, err := file.Read(buf)
        if err != nil && err != io.EOF { log.Fatal("Error Reading: ", err); break }
        if n == 0 { break }
        nal := nalStream.NalScan(buf[0:n])
        nals = append(nals, nal...)
    }

    i := 0
    pic := int64(0)
    start := time.Now()
    for {
        if i >= len(nals) { i = 0 }
        nalData := nals[i]; i+=1
        nal := NewNal()
        nal.ParseHeader(nalData[0])
        if nal.UnitType == SEI { continue }
        nal.Data = nalData
        // log.Println(i, "nal: ", NalUnitTypeStr(nal.UnitType))
        if nal.UnitType == CodedSliceNonIdr || nal.UnitType == CodedSliceIdr {
            pic+=1
            currentMustBeUs := int64(float64(pic) * 1000000.0 / float64(fps))
            now := time.Now()
            fromStart := now.Sub(start).Microseconds()
            delta := currentMustBeUs - fromStart
            if delta > 0 {
                time.Sleep(time.Duration(delta) * time.Microsecond)
            }
            //if fromStart > 0 {
            //    log.Println("fps: ", float64(pic) / (float64(fromStart) / 1000000.0))
            //}
        }
        nalChn <- nal
    }
    return nil
}
