package tproxy

import (
	"bufio"
)

func sniffSNI(br *bufio.Reader) string {
	peek, err := br.Peek(5)
	if err != nil || len(peek) < 5 || peek[0] != 0x16 {
		return ""
	}
	recordLen := int(peek[3])<<8 | int(peek[4])
	if recordLen <= 0 || recordLen > 8192 {
		return ""
	}
	buf, err := br.Peek(5 + recordLen)
	if err != nil {
		return ""
	}
	return parseTLSClientHelloSNI(buf[5:])
}

func parseTLSClientHelloSNI(data []byte) string {
	if len(data) < 42 {
		return ""
	}
	pos := 38
	if pos >= len(data) {
		return ""
	}
	sidLen := int(data[pos])
	pos += 1 + sidLen
	if pos+2 > len(data) {
		return ""
	}
	cipherLen := int(data[pos])<<8 | int(data[pos+1])
	pos += 2 + cipherLen
	if pos >= len(data) {
		return ""
	}
	compLen := int(data[pos])
	pos += 1 + compLen
	if pos+2 > len(data) {
		return ""
	}
	extLen := int(data[pos])<<8 | int(data[pos+1])
	pos += 2
	end := pos + extLen
	for pos+4 <= end && pos+4 <= len(data) {
		extType := int(data[pos])<<8 | int(data[pos+1])
		extSize := int(data[pos+2])<<8 | int(data[pos+3])
		pos += 4
		if pos+extSize > len(data) {
			break
		}
		if extType == 0 {
			return parseSNIExtension(data[pos : pos+extSize])
		}
		pos += extSize
	}
	return ""
}

func parseSNIExtension(data []byte) string {
	if len(data) < 5 {
		return ""
	}
	listLen := int(data[0])<<8 | int(data[1])
	pos := 2
	end := 2 + listLen
	for pos+3 <= end && pos+3 <= len(data) {
		nameType := data[pos]
		nameLen := int(data[pos+1])<<8 | int(data[pos+2])
		pos += 3
		if pos+nameLen > len(data) {
			break
		}
		if nameType == 0 {
			return string(data[pos : pos+nameLen])
		}
		pos += nameLen
	}
	return ""
}
