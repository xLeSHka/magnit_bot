package checklist

import (
	"fmt"
	"strconv"

	"github.com/xuri/excelize/v2"
)

const reportSheetName = "Чеклист"

func (r Result) SaveSpreadsheet(path string) error {
	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()

	defaultSheet := file.GetSheetName(0)
	if err := file.SetSheetName(defaultSheet, reportSheetName); err != nil {
		return err
	}

	if err := setupReportLayout(file); err != nil {
		return err
	}

	sectionStyle, err := file.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"D9D9D9"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "bottom"},
	})
	if err != nil {
		return err
	}

	headerStyle, err := file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "bottom"},
	})
	if err != nil {
		return err
	}

	wrapStyle, err := file.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{WrapText: true, Vertical: "bottom"},
	})
	if err != nil {
		return err
	}

	if err = file.SetCellValue(reportSheetName, "A1", "Примечание: каждый параметр было предложено оценить по шкале 0/10"); err != nil {
		return err
	}
	if err = file.SetCellValue(reportSheetName, "A2", "Магазин(магнит или пятерочка)"); err != nil {
		return err
	}
	if err = file.SetCellValue(reportSheetName, "B2", r.Session.Shop); err != nil {
		return err
	}
	if err = file.SetCellValue(reportSheetName, "A3", "Адрес"); err != nil {
		return err
	}
	if err = file.SetCellValue(reportSheetName, "B3", r.Session.Address); err != nil {
		return err
	}
	if err = file.SetCellValue(reportSheetName, "A4", "Время"); err != nil {
		return err
	}
	if err = file.SetCellValue(reportSheetName, "B4", r.FinishedAt.Format("02.01.2006 15:04")); err != nil {
		return err
	}
	if err = file.SetCellValue(reportSheetName, "B5", r.Session.FullName); err != nil {
		return err
	}
	if err = file.SetCellStyle(reportSheetName, "A1", "C5", headerStyle); err != nil {
		return err
	}

	row := 6
	section := ""
	for _, answer := range r.Session.Answers {
		if answer.Question.Section != section {
			if section != "" {
				row++
			}
			section = answer.Question.Section
			if section != "" {
				sectionCell := fmt.Sprintf("A%d", row)
				if err = file.SetCellValue(reportSheetName, sectionCell, section); err != nil {
					return err
				}
				if err = file.SetCellStyle(reportSheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row), sectionStyle); err != nil {
					return err
				}
				row++
			}
		}

		if err = file.SetCellValue(reportSheetName, fmt.Sprintf("A%d", row), answer.Question.Text); err != nil {
			return err
		}

		scoreStr := "➖"
		if answer.HasScore {
			scoreStr = strconv.Itoa(answer.Score)
		}
		if err = file.SetCellValue(reportSheetName, fmt.Sprintf("B%d", row), scoreStr); err != nil {
			return err
		}

		if err = file.SetCellValue(reportSheetName, fmt.Sprintf("C%d", row), answer.Comment); err != nil {
			return err
		}

		if err = file.SetCellStyle(reportSheetName, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row), wrapStyle); err != nil {
			return err
		}
		row++
	}

	return file.SaveAs(path)
}

func setupReportLayout(file *excelize.File) error {
	if err := file.SetColWidth(reportSheetName, "A", "A", 76.78); err != nil {
		return err
	}
	if err := file.SetColWidth(reportSheetName, "B", "B", 20); err != nil {
		return err
	}
	if err := file.SetColWidth(reportSheetName, "C", "C", 40); err != nil {
		return err
	}
	if err := file.SetPanes(reportSheetName, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      1,
		YSplit:      5,
		TopLeftCell: "B6",
		ActivePane:  "bottomRight",
	}); err != nil {
		return err
	}

	return nil
}
