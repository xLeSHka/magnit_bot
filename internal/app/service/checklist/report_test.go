package checklist

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestResultSaveSpreadsheet(t *testing.T) {
	result := Result{
		Session: Session{
			Username: "xleshka",
			FullName: "Никита Башлыков",
			Shop:     "Магнит",
			Address:  "Тестовый адрес",
			Answers: []Answer{
				{
					Question: Question{
						Number:  1,
						Text:    "Заметность входа снаружи/из ТЦ",
						Section: "🚪 ВХОД И ИНВЕНТАРЬ",
					},
					Score:    8,
					HasScore: true,
					Comment:  "Отличный вход",
				},
			},
		},
		FinishedAt: time.Date(2026, 9, 19, 12, 30, 0, 0, time.UTC),
	}

	path := filepath.Join(t.TempDir(), "report.xlsx")
	if err := result.SaveSpreadsheet(path); err != nil {
		t.Fatalf("SaveSpreadsheet() error = %v", err)
	}

	file, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	assertCell(t, file, "A1", "Примечание: каждый параметр было предложено оценить по шкале 0/10")
	assertCell(t, file, "B2", "Магнит")
	assertCell(t, file, "B3", "Тестовый адрес")
	assertCell(t, file, "A4", "Время")
	assertCell(t, file, "B4", "19.09.2026 12:30")
	assertCell(t, file, "A5", "Автор") // Добавлено
	assertCell(t, file, "B5", "Никита Башлыков")
	assertCell(t, file, "A6", "🚪 ВХОД И ИНВЕНТАРЬ")
	assertCell(t, file, "A7", "Заметность входа снаружи/из ТЦ")
	assertCell(t, file, "B7", "8")
	assertCell(t, file, "C7", "Отличный вход")
}

func assertCell(t *testing.T, file *excelize.File, cell string, want string) {
	t.Helper()

	got, err := file.GetCellValue(reportSheetName, cell)
	if err != nil {
		t.Fatalf("GetCellValue(%s) error = %v", cell, err)
	}
	if got != want {
		t.Fatalf("GetCellValue(%s) = %q, want %q", cell, got, want)
	}
}
