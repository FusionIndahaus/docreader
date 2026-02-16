package pdfutil

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// CountPagesFromBytes возвращает количество страниц в PDF, представленном в виде []byte.
// Использует внешнюю утилиту pdfinfo (poppler-utils). Если pdfinfo недоступен
// или файл не является корректным PDF, возвращается ошибка.
func CountPagesFromBytes(pdfBytes []byte) (int, error) {
	f, err := os.CreateTemp("", "docai_pages_*.pdf")
	if err != nil {
		return 0, fmt.Errorf("создание временного файла: %w", err)
	}
	path := f.Name()
	if _, err := f.Write(pdfBytes); err != nil {
		f.Close()
		os.Remove(path)
		return 0, fmt.Errorf("запись PDF: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return 0, fmt.Errorf("закрытие файла: %w", err)
	}
	defer os.Remove(path)

	cmd := exec.Command("pdfinfo", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("pdfinfo: %w (output: %s)", err, truncate(string(out), 200))
	}

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Pages:") {
			s := strings.TrimSpace(strings.TrimPrefix(line, "Pages:"))
			n, err := strconv.Atoi(s)
			if err != nil {
				return 0, fmt.Errorf("разбор числа страниц %q: %w", s, err)
			}
			return n, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("чтение вывода pdfinfo: %w", err)
	}
	return 0, fmt.Errorf("в выводе pdfinfo не найдена строка Pages:")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
