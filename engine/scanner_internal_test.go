package engine

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/jamestjat/sqlce/format"
)

func TestConvertRecordEmptyDataByColumnType(t *testing.T) {
	rec := format.Record{
		Values: [][]byte{
			{},
			{},
		},
	}
	columns := []format.ColumnDef{
		{Name: "ID", TypeID: format.TypeInt, Ordinal: 1},
		{Name: "Name", TypeID: format.TypeNVarchar, Ordinal: 2},
	}

	row, warnings, err := convertRecord(rec, columns, nil, nil)
	if err != nil {
		t.Fatalf("convertRecord: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if row[0] != nil {
		t.Fatalf("empty int = %#v, want nil", row[0])
	}
	if row[1] != "" {
		t.Fatalf("empty nvarchar = %#v, want empty string", row[1])
	}
}

func TestConvertRecordFailedLOBPointerWarnsAndReturnsNull(t *testing.T) {
	ptr := make([]byte, 16)
	binary.LittleEndian.PutUint32(ptr[0:4], 4)
	binary.LittleEndian.PutUint16(ptr[10:12], 99)

	page := make([]byte, format.DefaultPageSize)
	page[6] = byte(format.PageLeaf)
	pr := format.NewPageReader(bytes.NewReader(page), &format.FileHeader{PageSize: format.DefaultPageSize}, 1)
	pm := &format.PageMapping{}

	rec := format.Record{Values: [][]byte{ptr}}
	columns := []format.ColumnDef{
		{Name: "Body", TypeID: format.TypeNText, Ordinal: 1},
	}

	row, warnings, err := convertRecord(rec, columns, pr, pm)
	if err != nil {
		t.Fatalf("convertRecord: %v", err)
	}
	if row[0] != nil {
		t.Fatalf("LOB value = %v, want nil", row[0])
	}
	if len(warnings) == 0 {
		t.Fatal("expected LOB warning")
	}
	if !strings.Contains(warnings[0].Error(), "LOB resolve") {
		t.Fatalf("warning = %q, want LOB resolve", warnings[0])
	}
}
