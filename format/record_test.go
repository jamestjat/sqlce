package format

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"testing"
)

func TestParsePageRecords_DataArrayTypes(t *testing.T) {
	// Page 393 (obj 1321) contains DataArrayTypes: 1 row
	// Columns: Identifier (GUID), ArrayType (nvarchar(50)), Interval (INT), Unit (nvarchar(50))
	// Expected: fd73ae57-f7e8-46b8-bc67-c027d4004f33, 'ContinuousData', 60, 'Second'
	f, err := os.Open("../data/Depropanizer.sdf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	h, _ := ReadHeader(f)
	pr := NewPageReader(f, h, 64)

	page, err := pr.ReadPage(393)
	if err != nil {
		t.Fatal(err)
	}

	columns := []ColumnDef{
		{Name: "Identifier", TypeID: TypeUniqueIdentifier, Ordinal: 1},
		{Name: "ArrayType", TypeID: TypeNVarchar, Ordinal: 2, MaxLength: 100},
		{Name: "Interval", TypeID: TypeInt, Ordinal: 3},
		{Name: "Unit", TypeID: TypeNVarchar, Ordinal: 4, MaxLength: 100},
	}

	bmpExtra := computeNullBmpExtra(columns)
	parsed, err := ParsePageRecords(page, columns, bmpExtra)
	if err != nil {
		t.Fatalf("ParsePageRecords: %v", err)
	}
	if parsed == nil {
		t.Fatal("no records parsed")
	}

	t.Logf("ObjectID: %d, Records: %d", parsed.ObjectID, len(parsed.Records))

	if len(parsed.Records) == 0 {
		t.Fatal("expected at least 1 record")
	}

	rec := parsed.Records[0]
	if len(rec.Values) != 4 {
		t.Fatalf("expected 4 values, got %d", len(rec.Values))
	}

	// Check GUID (column 0, fixed)
	if len(rec.Values[0]) != 16 {
		t.Errorf("GUID: expected 16 bytes, got %d", len(rec.Values[0]))
	} else {
		// fd73ae57 in mixed-endian: 57 ae 73 fd
		if rec.Values[0][0] != 0x57 || rec.Values[0][1] != 0xae {
			t.Errorf("GUID mismatch: first 2 bytes = %x %x", rec.Values[0][0], rec.Values[0][1])
		}
	}

	// Check INT (column 2, fixed) = 60
	if len(rec.Values[2]) != 4 {
		t.Errorf("INT: expected 4 bytes, got %d", len(rec.Values[2]))
	} else {
		val := binary.LittleEndian.Uint32(rec.Values[2])
		if val != 60 {
			t.Errorf("INT: expected 60, got %d", val)
		}
	}

	// Check ArrayType (column 1, variable) = "ContinuousData"
	if rec.Values[1] == nil {
		t.Error("ArrayType: nil")
	} else {
		s := string(rec.Values[1])
		if s != "ContinuousData" {
			t.Errorf("ArrayType: expected 'ContinuousData', got %q", s)
		}
	}

	// Check Unit (column 3, variable) = "Second"
	if rec.Values[3] == nil {
		t.Error("Unit: nil")
	} else {
		s := string(rec.Values[3])
		if s != "Second" {
			t.Errorf("Unit: expected 'Second', got %q", s)
		}
	}
}

func TestParsePageRecords_ExternalRuntimeDataSource(t *testing.T) {
	// Page 872 (obj 1697): 1 row, 5 cols
	// GUID, GUID, nvarchar(32), INT, bit
	f, err := os.Open("../data/Depropanizer.sdf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	h, _ := ReadHeader(f)
	pr := NewPageReader(f, h, 64)

	page, err := pr.ReadPage(872)
	if err != nil {
		t.Fatal(err)
	}

	columns := []ColumnDef{
		{Name: "ExternalRuntimeDataSourceIdentifier", TypeID: TypeUniqueIdentifier, Ordinal: 1},
		{Name: "ItemSequenceIdentifier", TypeID: TypeUniqueIdentifier, Ordinal: 2},
		{Name: "FailureBehavior", TypeID: TypeNVarchar, Ordinal: 3, MaxLength: 64},
		{Name: "ReadWriteFailureLimit", TypeID: TypeInt, Ordinal: 4},
		{Name: "IsDefault", TypeID: TypeBit, Ordinal: 5},
	}

	parsed, err := ParsePageRecords(page, columns, 1)
	if err != nil {
		t.Fatalf("ParsePageRecords: %v", err)
	}
	if parsed == nil || len(parsed.Records) == 0 {
		t.Fatal("no records parsed")
	}

	rec := parsed.Records[0]
	t.Logf("Record has %d values", len(rec.Values))

	// Check GUID 1: 1e5f3cc4-50e5-463b-a3e9-86e842a4ea64
	// Mixed-endian: c4 3c 5f 1e e5 50 3b 46
	if len(rec.Values[0]) == 16 {
		if rec.Values[0][0] != 0xc4 || rec.Values[0][1] != 0x3c {
			t.Errorf("GUID1 mismatch: %x", rec.Values[0][:4])
		}
	}

	// Check INT (ReadWriteFailureLimit)
	if len(rec.Values[3]) == 4 {
		val := binary.LittleEndian.Uint32(rec.Values[3])
		t.Logf("ReadWriteFailureLimit = %d", val)
	}

	// Check FailureBehavior = "UseServerSettings"
	if rec.Values[2] != nil {
		s := string(rec.Values[2])
		t.Logf("FailureBehavior = %q", s)
		if s != "UseServerSettings" {
			t.Errorf("FailureBehavior: expected 'UseServerSettings', got %q", s)
		}
	}
}

func TestParsePageRecords_Properties(t *testing.T) {
	// Page 240 (obj 1305): 6 rows, 2 nvarchar cols (Name, Value)
	f, err := os.Open("../data/Depropanizer.sdf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	h, _ := ReadHeader(f)
	pr := NewPageReader(f, h, 64)

	page, err := pr.ReadPage(240)
	if err != nil {
		t.Fatal(err)
	}

	columns := []ColumnDef{
		{Name: "Name", TypeID: TypeNVarchar, Ordinal: 1, MaxLength: 64},
		{Name: "Value", TypeID: TypeNVarchar, Ordinal: 2, MaxLength: 512},
	}

	bmpExtra := computeNullBmpExtra(columns)
	parsed, err := ParsePageRecords(page, columns, bmpExtra)
	if err != nil {
		t.Fatalf("ParsePageRecords: %v", err)
	}
	if parsed == nil {
		t.Fatal("no records parsed")
	}

	t.Logf("Parsed %d records (expected 6)", len(parsed.Records))

	// We should get exactly 6 records
	if len(parsed.Records) != 6 {
		t.Errorf("expected 6 records, got %d", len(parsed.Records))
	}

	// Check first few records have Name values
	for i, rec := range parsed.Records {
		if i >= 6 {
			break
		}
		name := ""
		if rec.Values[0] != nil {
			name = string(rec.Values[0])
		}
		value := ""
		if rec.Values[1] != nil {
			value = string(rec.Values[1])
		}
		t.Logf("  Row %d: Name=%q, Value=%q", i, name, value)
	}
}

func TestParsePageRecords_NTextUsesVariableSection(t *testing.T) {
	page := make([]byte, DefaultPageSize)
	page[pageTypeOffset] = byte(PageLeaf)
	binary.LittleEndian.PutUint32(page[20:24], 1)

	text := []byte("ApplicationVersion")
	entry := make([]byte, 0, 4+4+1+4+1+1+len(text)+1)
	entry = append(entry,
		0, 0, 0, 0, // nextChunk
		2, 0, 0, 0, // colCount
		0, // null bitmap
	)
	entry = append(entry, 42, 0, 0, 0) // fixed int column
	entry = append(entry,
		0x00, // fixed/variable separator
		0x80, // variable-column header: present
	)
	entry = append(entry, text...)
	entry = append(entry, 0x00) // terminator for final variable column

	copy(page[24:], entry)
	slot := uint32(0) | uint32(len(entry))<<12 | uint32(2)<<24
	binary.LittleEndian.PutUint32(page[len(page)-4:], slot)

	columns := []ColumnDef{
		{Name: "ID", TypeID: TypeInt, Ordinal: 1, Position: 0},
		{Name: "Logic", TypeID: TypeNText, Ordinal: 2, Position: 0},
	}

	parsed, err := ParsePageRecords(page, columns, computeNullBmpExtra(columns))
	if err != nil {
		t.Fatalf("ParsePageRecords: %v", err)
	}
	if parsed == nil || len(parsed.Records) != 1 {
		t.Fatalf("expected 1 parsed record, got %#v", parsed)
	}

	rec := parsed.Records[0]
	if got := binary.LittleEndian.Uint32(rec.Values[0]); got != 42 {
		t.Fatalf("fixed int = %d, want 42", got)
	}
	if got := string(rec.Values[1]); got != string(text) {
		t.Fatalf("ntext variable payload = %q, want %q", got, string(text))
	}
}

func TestParsePageRecords_NTextLOBPointersWithZeroFlags(t *testing.T) {
	page := make([]byte, DefaultPageSize)
	page[pageTypeOffset] = byte(PageLeaf)
	binary.LittleEndian.PutUint32(page[20:24], 1)

	modifiedType := []byte("ApplicationVersion")
	undoPtr := make([]byte, 16)
	redoPtr := make([]byte, 16)
	for i := range undoPtr {
		undoPtr[i] = byte(i + 1)
		redoPtr[i] = byte(i + 0x21)
	}

	entry := make([]byte, 0, 4+4+1+8+1+5+len(modifiedType)+len(undoPtr)+len(redoPtr))
	entry = append(entry,
		0, 0, 0, 0, // nextChunk
		5, 0, 0, 0, // colCount
		0, // null bitmap
	)
	entry = append(entry,
		1, 0, 0, 0, // TraceLogIdentity
		8, 0, 0, 0, // GroupIndex
	)
	entry = append(entry,
		0x00,                          // fixed/variable separator
		0x80, byte(len(modifiedType)), // ModifiedType inline string
		0x00, byte(len(modifiedType)+len(undoPtr)), // UndoLogic pointer
		0x00, // RedoLogic pointer (final column; remaining bytes belong to it)
	)
	entry = append(entry, modifiedType...)
	entry = append(entry, undoPtr...)
	entry = append(entry, redoPtr...)

	copy(page[24:], entry)
	slot := uint32(0) | uint32(len(entry))<<12 | uint32(2)<<24
	binary.LittleEndian.PutUint32(page[len(page)-4:], slot)

	columns := []ColumnDef{
		{Name: "TraceLogIdentity", TypeID: TypeInt, Ordinal: 1, Position: 0},
		{Name: "GroupIndex", TypeID: TypeInt, Ordinal: 2, Position: 4},
		{Name: "ModifiedType", TypeID: TypeNVarchar, Ordinal: 3, Position: 0},
		{Name: "UndoLogic", TypeID: TypeNText, Ordinal: 4, Position: 1},
		{Name: "RedoLogic", TypeID: TypeNText, Ordinal: 5, Position: 2},
	}

	parsed, err := ParsePageRecords(page, columns, computeNullBmpExtra(columns))
	if err != nil {
		t.Fatalf("ParsePageRecords: %v", err)
	}
	if parsed == nil || len(parsed.Records) != 1 {
		t.Fatalf("expected 1 parsed record, got %#v", parsed)
	}

	rec := parsed.Records[0]
	if got := string(rec.Values[2]); got != string(modifiedType) {
		t.Fatalf("ModifiedType = %q, want %q", got, string(modifiedType))
	}
	if got := hex.EncodeToString(rec.Values[3]); got != hex.EncodeToString(undoPtr) {
		t.Fatalf("UndoLogic pointer = %s, want %s", got, hex.EncodeToString(undoPtr))
	}
	if got := hex.EncodeToString(rec.Values[4]); got != hex.EncodeToString(redoPtr) {
		t.Fatalf("RedoLogic pointer = %s, want %s", got, hex.EncodeToString(redoPtr))
	}
}

func TestIOItemsRowCount(t *testing.T) {
	f, err := os.Open("../data/Depropanizer.sdf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	h, _ := ReadHeader(f)
	fi, _ := f.Stat()
	totalPages := int(fi.Size()) / h.PageSize
	pr := NewPageReader(f, h, 256)

	catalog, err := ReadCatalog(pr, totalPages)
	if err != nil {
		t.Fatal(err)
	}

	td := catalog.TableByName("IOItems")
	if td == nil {
		t.Fatal("IOItems not found")
	}

	objIDs := catalog.ObjectMap["IOItems"]
	if len(objIDs) == 0 {
		t.Fatal("no objectIDs for IOItems")
	}

	// IOItems is the largest table (2869 rows across 130 pages).
	// 5 records span multiple slots via nextChunk pointers; chunk following is required.
	records, err := ScanTableRecordsMulti(pr, totalPages, objIDs, td.Columns, td.NullBmpExtra)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2869 {
		t.Errorf("IOItems: got %d rows, want 2869", len(records))
	}
}

func TestItemInformationNoGhostRecords(t *testing.T) {
	f, err := os.Open("../data/Depropanizer.sdf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	h, _ := ReadHeader(f)
	fi, _ := f.Stat()
	totalPages := int(fi.Size()) / h.PageSize
	pr := NewPageReader(f, h, 256)

	catalog, err := ReadCatalog(pr, totalPages)
	if err != nil {
		t.Fatal(err)
	}

	td := catalog.TableByName("ItemInformation")
	if td == nil {
		t.Fatal("ItemInformation not found in catalog")
	}

	objIDs := catalog.ObjectMap["ItemInformation"]
	if len(objIDs) == 0 {
		t.Fatal("no objectIDs for ItemInformation")
	}

	records, err := ScanTableRecordsMulti(pr, totalPages, objIDs, td.Columns, td.NullBmpExtra)
	if err != nil {
		t.Fatal(err)
	}

	// Reference SQLite has 203 rows. Old pattern-matching parser returned 204
	// (one ghost/deleted record). Slotted page parsing should return exactly 203.
	if len(records) != 203 {
		t.Errorf("ItemInformation: got %d rows, want 203", len(records))
	}
}

func TestParsePageRecords_BlcModel(t *testing.T) {
	// Page 467 (obj 1395): 3 rows, 9 cols (5 GUIDs + 4 nvarchars)
	f, err := os.Open("../data/Depropanizer.sdf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	h, _ := ReadHeader(f)
	pr := NewPageReader(f, h, 64)

	page, err := pr.ReadPage(467)
	if err != nil {
		t.Fatal(err)
	}

	columns := []ColumnDef{
		{Name: "BlcModelIdentifier", TypeID: TypeUniqueIdentifier, Ordinal: 1, Position: 0},
		{Name: "RelationIdentifier", TypeID: TypeUniqueIdentifier, Ordinal: 2, Position: 16},
		{Name: "LoopIdentifier", TypeID: TypeUniqueIdentifier, Ordinal: 3, Position: 32},
		{Name: "ItemSequenceIdentifier", TypeID: TypeUniqueIdentifier, Ordinal: 4, Position: 48},
		{Name: "Representation", TypeID: TypeNVarchar, Ordinal: 5, MaxLength: 100, Position: 0},
		{Name: "IntendedMV", TypeID: TypeNVarchar, Ordinal: 6, MaxLength: 100, Position: 1},
		{Name: "IntendedModelLoopType", TypeID: TypeNVarchar, Ordinal: 7, MaxLength: 100, Position: 2},
		{Name: "Status", TypeID: TypeNVarchar, Ordinal: 8, MaxLength: 100, Position: 3},
		{Name: "BlcModelBlockId", TypeID: TypeUniqueIdentifier, Ordinal: 9, Position: 64},
	}

	bmpExtra := computeNullBmpExtra(columns)
	parsed, err := ParsePageRecords(page, columns, bmpExtra)
	if err != nil {
		t.Fatalf("ParsePageRecords: %v", err)
	}
	if parsed == nil {
		t.Fatal("no records parsed")
	}

	t.Logf("Parsed %d records (expected 3)", len(parsed.Records))

	for i, rec := range parsed.Records {
		repr := ""
		if rec.Values[4] != nil {
			repr = string(rec.Values[4])
		}
		mv := ""
		if rec.Values[5] != nil {
			mv = string(rec.Values[5])
		}
		t.Logf("  Row %d: Representation=%q, IntendedMV=%q", i, repr, mv)
	}

	// Row 1 should have Representation="Simple_SP", IntendedMV="SP"
	if len(parsed.Records) > 0 {
		rec := parsed.Records[0]
		if string(rec.Values[4]) != "Simple_SP" {
			t.Errorf("Row 0 Representation: expected 'Simple_SP', got %q", string(rec.Values[4]))
		}
		if string(rec.Values[5]) != "SP" {
			t.Errorf("Row 0 IntendedMV: expected 'SP', got %q", string(rec.Values[5]))
		}
	}
}
