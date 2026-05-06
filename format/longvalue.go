package format

import (
	"encoding/binary"
	"fmt"
)

const lvPageDataOffset = 16
const lvPageDataSize = 4096 - lvPageDataOffset // 4080 bytes per LV page
const maxLOBSize = 64 * 1024 * 1024            // 64MB safety cap

// ResolveLOB reads overflow data from LongValue pages referenced by a 16-byte
// inline pointer stored in ntext/image columns.
//
// Pointer layout (16 bytes):
//
//	bytes 0-3:   total data length (uint32 LE) — high 16 bits may be zero for small LOBs
//	bytes 8-11:  logical page ID of first LV page (uint32 LE)
//
// For backward compatibility with databases where only bytes 2-3 held the
// length as uint16: if the uint32 read yields a value where the low 16 bits
// match bytes[2:4] and high bits are zero, the result is the same.
//
// Data spans consecutive logical page IDs, 4080 bytes per page.
func ResolveLOB(pr *PageReader, pm *PageMapping, ptr []byte) ([]byte, error) {
	if len(ptr) < 12 {
		return ptr, nil
	}

	le := binary.LittleEndian

	// Read full uint32 for both length and page ID
	totalLen := int(le.Uint32(ptr[0:4]))
	firstLogID := int(le.Uint32(ptr[8:12]))

	// Fallback: if uint32 length looks wrong (zero or huge) but uint16 at [2:4] is valid,
	// use the uint16 value. This handles the original pointer layout where bytes [0:2]
	// were unused (should be zero).
	if (totalLen == 0 || totalLen > maxLOBSize) && len(ptr) >= 4 {
		// Only fallback if this looks like old uint16 format (bytes [0:2] should be zero)
		if le.Uint16(ptr[0:2]) == 0 {
			totalLen16 := int(le.Uint16(ptr[2:4]))
			if totalLen16 > 0 {
				totalLen = totalLen16
			}
		}
	}

	if totalLen == 0 {
		return ptr, nil
	}

	if totalLen > maxLOBSize {
		return nil, fmt.Errorf("LOB too large: %d bytes (max %d)", totalLen, maxLOBSize)
	}

	start, ok := resolveLOBStart(pr, pm, ptr)
	if !ok {
		if firstLogID == 0 {
			return ptr, nil
		}
		return nil, fmt.Errorf("LOB page mapping missing for logical page %d", firstLogID)
	}

	buf := make([]byte, 0, totalLen)
	remaining := totalLen
	logID := start.logicalID
	filePage := start.filePage

	for remaining > 0 {
		page, err := pr.ReadPage(filePage)
		if err != nil {
			return nil, fmt.Errorf("reading LOB page %d (file page %d): %w", logID, filePage, err)
		}

		if ClassifyPage(page) != PageLongValue {
			return nil, fmt.Errorf("expected LongValue page at %d, got %s", filePage, ClassifyPage(page))
		}

		chunk := lvPageDataSize
		if chunk > remaining {
			chunk = remaining
		}
		if lvPageDataOffset+chunk > len(page) {
			chunk = len(page) - lvPageDataOffset
		}
		if chunk <= 0 {
			break
		}

		buf = append(buf, page[lvPageDataOffset:lvPageDataOffset+chunk]...)
		remaining -= chunk

		if remaining <= 0 {
			break
		}
		if start.physicalContinuation {
			filePage++
			continue
		}
		logID++
		fp, ok := pm.FilePageNum(logID)
		if !ok {
			return nil, fmt.Errorf("LOB page mapping missing for logical page %d", logID)
		}
		filePage = fp
	}

	if len(buf) == 0 {
		return ptr, nil
	}
	if remaining > 0 {
		return nil, fmt.Errorf("LOB incomplete: expected %d bytes total, got %d", totalLen, len(buf))
	}
	return buf, nil
}

type lobStart struct {
	logicalID            int
	filePage             int
	physicalContinuation bool
}

func resolveLOBStart(pr *PageReader, pm *PageMapping, ptr []byte) (lobStart, bool) {
	le := binary.LittleEndian

	fullID := int(le.Uint32(ptr[8:12]))
	if fp, ok := longValueFilePage(pr, pm, fullID); ok {
		return lobStart{logicalID: fullID, filePage: fp}, true
	}

	if le.Uint16(ptr[8:10]) == 0 {
		highID := int(le.Uint16(ptr[10:12]))
		if fp, ok := longValueFilePage(pr, pm, highID); ok {
			return lobStart{logicalID: highID, filePage: fp}, true
		}
	}

	lowID := int(le.Uint16(ptr[8:10]))
	if fp, ok := longValueFilePage(pr, pm, lowID); ok {
		return lobStart{logicalID: lowID, filePage: fp, physicalContinuation: true}, true
	}

	return lobStart{}, false
}

func longValueFilePage(pr *PageReader, pm *PageMapping, logicalID int) (int, bool) {
	if logicalID == 0 {
		return 0, false
	}
	fp, ok := pm.FilePageNum(logicalID)
	if !ok {
		return 0, false
	}
	page, err := pr.ReadPage(fp)
	if err != nil || ClassifyPage(page) != PageLongValue {
		return 0, false
	}
	return fp, true
}
