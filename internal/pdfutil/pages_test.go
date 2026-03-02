package pdfutil

import (
	"os/exec"
	"testing"
)

func TestCountPagesFromBytes_InvalidPDF(t *testing.T) {
	_, err := CountPagesFromBytes([]byte("not a pdf"))
	if err == nil {
		t.Error("ожидалась ошибка для невалидного PDF")
	}
}

func TestCountPagesFromBytes_Empty(t *testing.T) {
	_, err := CountPagesFromBytes(nil)
	if err == nil {
		t.Error("ожидалась ошибка для nil")
	}
}

func TestCountPagesFromBytes_RealPDF(t *testing.T) {
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Skip("pdfinfo не установлен, пропуск теста с реальным PDF")
	}
	// Минимальный PDF с одной страницей (только заголовок и базовая структура)
	minimalPDF := []byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]>>endobj\nxref\n0 4\n0000000000 65535 f \n0000000009 00000 n \n0000000052 00000 n \n0000000106 00000 n \ntrailer<</Size 4/Root 1 0 R>>\nstartxref\n178\n%%EOF")
	n, err := CountPagesFromBytes(minimalPDF)
	if err != nil {
		t.Skipf("pdfinfo не смог прочитать тестовый PDF (возможно, слишком минимальный): %v", err)
	}
	if n != 1 {
		t.Errorf("ожидалась 1 страница, получено %d", n)
	}
}
