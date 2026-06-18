package format

import (
	"encoding/json"

	"exam/internal/model"
)

func JSON(report model.Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
