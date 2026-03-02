package docxutil

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
)

const documentPath = "word/document.xml"

func CountPagesFromBytes(docxBytes []byte) (int, error) {
	zr, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
	if err != nil {
		return 0, fmt.Errorf("открытие docx (zip): %w", err)
	}
	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == documentPath {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return 0, fmt.Errorf("в архиве не найден %s", documentPath)
	}
	rc, err := docFile.Open()
	if err != nil {
		return 0, fmt.Errorf("чтение %s: %w", documentPath, err)
	}
	defer rc.Close()
	xmlData, err := io.ReadAll(rc)
	if err != nil {
		return 0, fmt.Errorf("чтение тела %s: %w", documentPath, err)
	}
	xml := string(xmlData)
	nPageBr := strings.Count(xml, `w:type="page"`)
	// Разрывы, сохранённые при последнем рендере в Word
	nLastRendered := strings.Count(xml, "lastRenderedPageBreak")
	breaks := nPageBr + nLastRendered
	if breaks == 0 {
		return 1, nil
	}
	return 1 + breaks, nil
}
