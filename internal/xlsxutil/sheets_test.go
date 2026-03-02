package xlsxutil

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestSheetCountFromBytes_OneSheet(t *testing.T) {
	f := excelize.NewFile()
	defer f.Close()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("запись xlsx в буфер: %v", err)
	}
	got, err := SheetCountFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("SheetCountFromBytes: %v", err)
	}
	if got != 1 {
		t.Errorf("ожидалось 1 лист, получено %d", got)
	}
}

func TestSheetCountFromBytes_ThreeSheets(t *testing.T) {
	f := excelize.NewFile()
	_, _ = f.NewSheet("Sheet2")
	_, _ = f.NewSheet("Sheet3")
	defer f.Close()
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("запись xlsx в буфер: %v", err)
	}
	got, err := SheetCountFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("SheetCountFromBytes: %v", err)
	}
	if got != 3 {
		t.Errorf("ожидалось 3 листа, получено %d", got)
	}
}

func TestSheetCountFromBytes_InvalidBytes(t *testing.T) {
	_, err := SheetCountFromBytes([]byte("not xlsx at all"))
	if err == nil {
		t.Error("ожидалась ошибка для невалидных данных")
	}
}

func TestSheetCountFromBytes_Empty(t *testing.T) {
	_, err := SheetCountFromBytes(nil)
	if err == nil {
		t.Error("ожидалась ошибка для nil")
	}
}
