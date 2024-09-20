package main

import (
	"archive/zip"
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
    "time"
	"github.com/schollz/progressbar/v3"
)

var bitCountTable [256]int

func init() {
	for i := 0; i < 256; i++ {
		b := byte(i)
		count := 0
		for b != 0 {
			count++
			b &= b - 1
		}
		bitCountTable[i] = count
	}
}

// bitsInByte возвращает количество установленных битов в байте
func bitsInByte(b byte) int {
	return bitCountTable[b]
}

// ipToUint32 преобразует строковое представление IP в 32-битное целое число
func ipToUint32(ipStr string) (uint32, error) {
	ip := net.ParseIP(strings.TrimSpace(ipStr)).To4()
	if ip == nil {
		return 0, fmt.Errorf("недопустимый IP адрес: %s", ipStr)
	}
	return binary.BigEndian.Uint32(ip), nil
}

func main() {
	// Инициализируем битовый массив размером 512 МБ (2^32 бит)
	bitSetSize := uint64(1) << 32
	bitSet := make([]byte, bitSetSize/8)

	// Открываем ZIP-архив
	zipReader, err := zip.OpenReader("ip_addresses.zip")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка при открытии ZIP-файла: %v\n", err)
		os.Exit(1)
	}
	defer zipReader.Close()

	// Проверяем наличие файлов в архиве
	if len(zipReader.File) == 0 {
		fmt.Fprintf(os.Stderr, "ZIP-архив пустой\n")
		os.Exit(1)
	}

	// Выбираем первый файл в архиве
	zipFile := zipReader.File[0]

	// Получаем не сжатый размер файла из заголовка ZIP
	uncompressedSize := zipFile.UncompressedSize64
	if uncompressedSize == 0 {
		fmt.Fprintf(os.Stderr, "Не удалось определить размер не сжатого файла\n")
		os.Exit(1)
	}

	// Открываем файл внутри архива
	fileInZip, err := zipFile.Open()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка при открытии файла в ZIP: %v\n", err)
		os.Exit(1)
	}
	defer fileInZip.Close()

	// Создаём прогресс-бар с оптимизированными параметрами
	bar := progressbar.NewOptions64(
		int64(uncompressedSize),
		progressbar.OptionSetDescription("Обработка файла"),
		progressbar.OptionShowBytes(true), // Показываем байты
		progressbar.OptionSetWidth(30),    // Уменьшаем ширину прогресс-бара
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionThrottle(65*time.Millisecond), // Ограничение скорости обновления
		progressbar.OptionSetWriter(os.Stdout), // Перенаправление прогресс-бара в stdout
		progressbar.OptionClearOnFinish(),
	)

	// Создаём Reader, который будет обновлять прогресс-бар при чтении
	reader := io.TeeReader(fileInZip, bar)

	// Используем bufio.Scanner для построчного чтения
	scanner := bufio.NewScanner(reader)
	// Увеличиваем буфер для обработки длинных строк
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	lineCount := 0
	errorCount := 0
	for scanner.Scan() {
		ipStr := scanner.Text()
		ipInt, err := ipToUint32(ipStr)
		if err != nil {
			errorCount++
			continue
		}
		index := ipInt / 8
		offset := ipInt % 8
		bitSet[index] |= 1 << offset
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка при чтении файла: %v\n", err)
		os.Exit(1)
	}

	// Подсчёт уникальных IP-адресов
	uniqueCount := 0
	for _, b := range bitSet {
		uniqueCount += bitsInByte(b)
	}

	fmt.Printf("\nОбщее количество уникальных IP-адресов: %d\n", uniqueCount)
	if errorCount > 0 {
		fmt.Fprintf(os.Stderr, "Пропущено %d строк из-за ошибок.\n", errorCount)
	}
}
