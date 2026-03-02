package xlsxutil

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// SheetCountFromBytes - Нам прилетает эксель в байтах (типа с загрузки),
// мы его не на диск пишем, а сразу в память пихаем через bytes.NewReader. Открываем через
// excelize.OpenReader, берём SheetCount — это и есть количество листов (вкладок внизу).
// Если файл битый или не xlsx — вернётся ошибка, по-другому никак.
func SheetCountFromBytes(xlsxBytes []byte) (int, error) {
	f, err := excelize.OpenReader(bytes.NewReader(xlsxBytes))
	if err != nil {
		return 0, fmt.Errorf("открытие xlsx: %w", err)
	}
	defer f.Close() // не забудь закрыть
	return f.SheetCount, nil
}
