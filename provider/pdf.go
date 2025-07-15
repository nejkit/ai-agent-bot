package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

func GeneratePdf(body, registerDate string) ([]byte, error) {
	request := struct {
		Id   string `json:"id"`
		Data struct {
			ReportText   string `json:"reportText"`
			RegisterDate string `json:"registerDate"`
		} `json:"data"`
	}{
		Id: "reportId",
		Data: struct {
			ReportText   string `json:"reportText"`
			RegisterDate string `json:"registerDate"`
		}{ReportText: body, RegisterDate: registerDate},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		panic(fmt.Errorf("ошибка сериализации: %w", err))
	}

	url := os.Getenv("PDF_URL")
	// Отправка POST-запроса
	resp, err := http.Post(url+"/v1/generate", "application/json", bytes.NewReader(jsonData))
	if err != nil {
		panic(fmt.Errorf("ошибка запроса: %w", err))
	}
	defer resp.Body.Close()

	// Проверка HTTP-кода
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("ошибка ответа: %s", resp.Status))
	}

	// Чтение []byte из тела ответа (предположим, что это PDF)
	resultBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(fmt.Errorf("ошибка чтения тела ответа: %w", err))
	}

	return resultBytes, nil
}
