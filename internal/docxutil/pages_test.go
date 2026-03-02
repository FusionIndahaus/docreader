package docxutil

import (
	"archive/zip"
	"bytes"
	"testing"
)

func makeDocx(pageBreakCount int) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	body := `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Test</w:t>`
	for i := 0; i < pageBreakCount; i++ {
		body += `<w:lastRenderedPageBreak/>`
	}
	body += `</w:r></w:p>
  </w:body>
</w:document>`
	fw, _ := zw.Create(documentPath)
	_, _ = fw.Write([]byte(body))
	_ = zw.Close()
	return buf.Bytes()
}

func TestCountPagesFromBytes_NoBreaks(t *testing.T) {
	docx := makeDocx(0)
	got, err := CountPagesFromBytes(docx)
	if err != nil {
		t.Fatalf("CountPagesFromBytes: %v", err)
	}
	if got != 1 {
		t.Errorf("ожидалось 1 страница (без разрывов), получено %d", got)
	}
}

func TestCountPagesFromBytes_WithBreaks(t *testing.T) {
	docx := makeDocx(2)
	got, err := CountPagesFromBytes(docx)
	if err != nil {
		t.Fatalf("CountPagesFromBytes: %v", err)
	}
	// 1 + 2 разрыва = 3 страницы
	if got != 3 {
		t.Errorf("ожидалось 3 страницы, получено %d", got)
	}
}

func TestCountPagesFromBytes_NotZip(t *testing.T) {
	_, err := CountPagesFromBytes([]byte("not a zip"))
	if err == nil {
		t.Error("ожидалась ошибка для не-zip данных")
	}
}

func TestCountPagesFromBytes_NoDocumentXml(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, _ := zw.Create("other.txt")
	_, _ = fw.Write([]byte("hello"))
	_ = zw.Close()
	_, err := CountPagesFromBytes(buf.Bytes())
	if err == nil {
		t.Error("ожидалась ошибка при отсутствии word/document.xml")
	}
}
