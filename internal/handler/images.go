package handler

import (
	"HorizonBackend/config"
	"HorizonBackend/internal/service"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

type CheckRequest struct {
	IPAddress string `json:"ipAddress"`
	UUID      string `json:"uuid"`
}

type CheckResponse struct {
	Message string `json:"message"`
}

// CheckHandler обрабатывает запросы для проверки доступа
func CheckHandler(w http.ResponseWriter, r *http.Request) {
	// Ответ на OPTIONS-запрос
	// if r.Method == http.MethodOptions {
	// 	w.WriteHeader(http.StatusOK)
	// 	return
	// }

	// Чтение и копирование тела запроса
	var buf bytes.Buffer
	teeBody := io.TeeReader(r.Body, &buf)
	body, err := io.ReadAll(teeBody)
	if err != nil {
		http.Error(w, "Handler: Error reading request body", http.StatusBadRequest)
		log.Printf("Handler: Error reading request body: %v", err)
		return
	}

	// Восстановление оригинального тела запроса
	r.Body = io.NopCloser(&buf)

	// Декодирование JSON из тела запроса в структуру CheckRequest
	var checkRequest CheckRequest
	if err := json.Unmarshal(body, &checkRequest); err != nil {
		http.Error(w, "Handler: Error decoding JSON", http.StatusBadRequest)
		log.Printf("Handler: Error decoding JSON: %v", err)
		return
	}

	// Предположим, что результаты проверки уже находятся в контексте запроса
	checkResult, ok := r.Context().Value("checkResult").(CheckResponse)
	if !ok {
		// Если результаты отсутствуют или имеют неверный формат, возвращаем ошибку
		http.Error(w, "Handler: Missing or invalid check result", http.StatusInternalServerError)
		log.Printf("Handler: Missing or invalid check result in the request context")
		return
	}

	// Логирование содержимого ответа
	log.Printf("Handler: Check response: %+v\n", checkResult)

	// Запись в ResponseWriter
	w.Header().Set("Content-Type", "application/json")

	// преобразование в байты и отправка
	jsonBytes, err := json.Marshal(checkResult)
	if err != nil {
		http.Error(w, "Handler: Error encoding JSON", http.StatusInternalServerError)
		log.Printf("Handler: Error encoding JSON: %v", err)
		return
	}

	if _, err := w.Write(jsonBytes); err != nil {
		http.Error(w, "Handler: Error writing JSON", http.StatusInternalServerError)
		log.Printf("Handler: Error writing JSON: %v", err)
		return
	}

}

func SendCheckRequest(request CheckRequest) (CheckResponse, error) {
	cfg, err := config.Load()
	if err != nil {
		return CheckResponse{}, fmt.Errorf("SendCheckRequest: error loading config: %v", err)
	}
	checkURL := cfg.CheckURL

	// Кодирование JSON для тела запроса
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return CheckResponse{}, fmt.Errorf("SendCheckRequest: error encoding JSON: %v", err)
	}

	// Отправка POST-запроса на указанный URL
	response, err := http.Post(checkURL, "application/json", bytes.NewBuffer(requestJSON))
	if err != nil {
		return CheckResponse{}, fmt.Errorf("SendCheckRequest: error sending POST request: %v", err)
	}

	// Отложенное закрытие тела ответа
	defer func() {
		if response != nil && response.Body != nil {
			log.Println("SendCheckRequest: defer func: Closing response.Body")
			response.Body.Close()
		}
	}()

	// Чтение тела ответа в буфер
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return CheckResponse{}, fmt.Errorf("SendCheckRequest: error reading response body: %v", err)
	}

	// Вывод декодированного тела ответа в консоль
	log.Printf("SendCheckRequest: Decoded response body 1: %s\n", responseBody)

	var checkResponse CheckResponse

	// Попробуем сначала декодировать ответ как строку
	if err := json.Unmarshal(responseBody, &checkResponse.Message); err == nil {
		// Извлекаем строку из json.RawMessage
		checkResponseStr := string(checkResponse.Message)
		log.Printf("SendCheckRequest: Decoded response body 2: %s\n", checkResponseStr)
		return checkResponse, nil
	}

	// Если не удалось декодировать как строку, вернем ошибку
	log.Printf("SendCheckRequest: error decoding JSON response: %v\n", err)
	return CheckResponse{}, fmt.Errorf("SendCheckRequest: error decoding JSON response: %v", err)
}

func GetImagesByFamilyGroupSubgroup(s service.ImageService, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baseURL := cfg.BaseURL

		vars := mux.Vars(r)
		family := vars["family"]
		group := vars["group"]
		subgroup := vars["subgroup"]

		images, err := s.GetImagesByFamilyGroupSubgroup(family, group, subgroup)
		if err != nil {
			log.Printf("Error fetching images by family, group and subgroup: %v", err)
			http.Error(w, "Failed to fetch images", http.StatusInternalServerError)
			return
		}

		for i := range images {
			images[i].FilePath = baseURL + images[i].FilePath
			images[i].ThumbPath = baseURL + images[i].ThumbPath
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(images)
		if err != nil {
			log.Printf("Failed to encode images to JSON: %v", err)
			http.Error(w, "Failed to encode images to JSON", http.StatusInternalServerError)
		}
	}
}

func SearchImages(s service.ImageService, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baseURL := cfg.BaseURL

		keyword := r.URL.Query().Get("keyword")
		family := r.URL.Query().Get("family")

		images, err := s.SearchImages(keyword, family)
		if err != nil {
			http.Error(w, "Failed to fetch images", http.StatusInternalServerError)
			return
		}

		for i := range images {
			images[i].FilePath = baseURL + images[i].FilePath
			images[i].ThumbPath = baseURL + images[i].ThumbPath
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(images)
		if err != nil {
			log.Printf("Failed to encode images to JSON: %v", err)
			http.Error(w, "Failed to encode images to JSON", http.StatusInternalServerError)
		}
	}
}

type ImageResponse struct {
	FilePath string `json:"file_path"`
}

func IncreaseImageUsage(service service.ImageService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("IncreaseImageUsage handler called")
		vars := mux.Vars(r)
		thumbPath, ok := vars["thumbPath"]
		fmt.Printf("Extracted thumbPath: %s, success: %v\n", thumbPath, ok)

		if !ok {
			http.Error(w, "Thumb path is required", http.StatusBadRequest)
			return
		}

		err := service.IncreaseUsageCount(thumbPath)
		if err != nil {
			fmt.Printf("Error increasing usage count: %v\n", err)
			http.Error(w, fmt.Sprintf("Error increasing usage count: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("Usage count increased"))
	}
}

func GetImageByNumber(service service.ImageService, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baseURL := cfg.BaseURL

		vars := mux.Vars(r)
		family := vars["family"]
		group := vars["group"]
		subgroup := vars["subgroup"]
		number := vars["number"]

		image, err := service.GetImageByNumber(family, group, subgroup, number)
		if err != nil {
			log.Printf("Error fetching image by number: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := ImageResponse{
			FilePath: baseURL + image.FilePath,
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			log.Printf("Failed to encode image to JSON: %v", err)
			http.Error(w, "Failed to encode image to JSON", http.StatusInternalServerError)
			return
		}
	}
}

func GetLeastUsedImages(s service.ImageService, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// параметры family и count из строки запроса
		family := r.URL.Query().Get("family")
		if family == "" {
			http.Error(w, "Family parameter is missing", http.StatusBadRequest)
			return
		}

		// count из запроса и установка значения по умолчанию на 6
		count := 6
		countStr := r.URL.Query().Get("count")
		if countStr != "" {
			var err error
			count, err = strconv.Atoi(countStr)
			if err != nil {
				http.Error(w, "Invalid count parameter", http.StatusBadRequest)
				return
			}
		} else {
			count = 6
		}

		// Логирование входящих параметров
		log.Printf("Fetching least used images for family: %s and count: %d", family, count)

		images, err := s.GetLeastUsedImages(family, count)
		if err != nil {
			log.Printf("Error fetching least used images: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Логирование количества извлеченных изображений
		log.Printf("Fetched %d images", len(images))

		for i := range images {
			images[i].FilePath = cfg.BaseURL + images[i].FilePath
			images[i].ThumbPath = cfg.BaseURL + images[i].ThumbPath
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(images)
	}
}
