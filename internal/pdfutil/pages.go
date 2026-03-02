package pdfutil

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// CountPagesFromBytes возвращает количество страниц в PDF.
// Сначала пробует pdfinfo (poppler-utils), при недоступности — чистый Go-парсер.
func CountPagesFromBytes(pdfBytes []byte) (int, error) {
	if n, err := countViaPdfinfo(pdfBytes); err == nil {
		return n, nil
	}
	return countViaScan(pdfBytes)
}

// countViaPdfinfo использует внешнюю утилиту pdfinfo.
func countViaPdfinfo(pdfBytes []byte) (int, error) {
	f, err := os.CreateTemp("", "docai_pages_*.pdf")
	if err != nil {
		return 0, err
	}
	path := f.Name()
	if _, err := f.Write(pdfBytes); err != nil {
		f.Close()
		os.Remove(path)
		return 0, err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return 0, err
	}
	defer os.Remove(path)

	cmd := exec.Command("pdfinfo", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("pdfinfo unavailable: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Pages:") {
			s := strings.TrimSpace(strings.TrimPrefix(line, "Pages:"))
			return strconv.Atoi(s)
		}
	}
	return 0, fmt.Errorf("Pages: not found in pdfinfo output")
}

// countViaScan — чистый Go fallback: ищет /Count в словаре Pages PDF.
// Работает без внешних утилит на любой ОС.
func countViaScan(pdfBytes []byte) (int, error) {
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		return 0, fmt.Errorf("not a PDF file")
	}

	// /Count N встречается в объекте страничного дерева (/Type /Pages)
	re := regexp.MustCompile(`/Count\s+(\d+)`)
	matches := re.FindAllSubmatch(pdfBytes, -1)
	if len(matches) == 0 {
		// Запасной вариант: считаем объекты страниц
		return countPageObjects(pdfBytes), nil
	}

	// Берём максимальное значение /Count — это корневой узел дерева страниц
	max := 0
	for _, m := range matches {
		if n, err := strconv.Atoi(string(m[1])); err == nil && n > max {
			max = n
		}
	}
	if max > 0 {
		return max, nil
	}
	return countPageObjects(pdfBytes), nil
}

// countPageObjects считает объекты /Type /Page как последний запасной вариант.
func countPageObjects(pdfBytes []byte) int {
	re := regexp.MustCompile(`/Type\s*/Page[^s]`)
	return len(re.FindAllIndex(pdfBytes, -1))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
