package engine

import (
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
