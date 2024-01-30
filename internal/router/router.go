package router

import (
	"HorizonBackend/config"
	"HorizonBackend/internal/handler"
	"HorizonBackend/internal/repository/postgres"
	"HorizonBackend/internal/service"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

// CheckResultHandler is a handler that can provide the check result for /check
type CheckResultHandler interface {
	IsCheckSuccessful() bool
	http.Handler
}

// MyHandler is a struct implementing the CheckResultHandler interface
type MyHandler struct {
	checkSuccessful bool
}

// IsCheckSuccessful returns the check result
func (h *MyHandler) IsCheckSuccessful() bool {
	return h.checkSuccessful
}

// SetCheckResult sets the check result
func (h *MyHandler) SetCheckResult(result bool) {
	h.checkSuccessful = result
}

func setCORSHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if the request is from a client
		if origin := r.Header.Get("Origin"); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		}

		if r.Method == "OPTIONS" {
			// Response to OPTIONS request
			w.WriteHeader(http.StatusOK)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}

		// Print request data
		fmt.Printf("Received request: %s %s\nBody: %s\n", r.Method, r.RequestURI, body)

		// Restore the original request body
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// Move to the next handler
		next.ServeHTTP(w, r)
	})
}

func checkMiddleware(next http.Handler, handlerInstance *MyHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log the beginning of the middleware check
		log.Println("checkMiddleware -1: Checking request")

		// Check if the request is OPTIONS
		if r.Method == http.MethodOptions {
			// Response to OPTIONS request
			w.WriteHeader(http.StatusOK)
			return
		}

		// Log request information
		log.Printf("checkMiddleware 0: Request: %+v\n", r.Body)

		// Check if the request is /check
		if r.URL.Path == "/check" && r.Method == http.MethodPost {
			// Log the beginning of handling /check request
			log.Println("checkMiddleware 1: Handling /check request")

			var buf bytes.Buffer
			teeBody := io.TeeReader(r.Body, &buf)
			body, err := io.ReadAll(teeBody)
			if err != nil {
				http.Error(w, "Handler: Error reading request body", http.StatusBadRequest)
				log.Printf("Handler: Error reading request body: %v", err)
				return
			}

			// Restore the original request body
			r.Body = io.NopCloser(&buf)

			// Decode JSON from the request body into CheckRequest structure
			var checkRequest handler.CheckRequest
			if err := json.Unmarshal(body, &checkRequest); err != nil {
				http.Error(w, "Handler: Error decoding JSON", http.StatusBadRequest)
				log.Printf("Handler: Error decoding JSON: %v", err)
				return
			}

			uuid := checkRequest.UUID
			log.Printf("checkMiddleware 2: uuid: %+v\n", uuid)
			if uuid == "" {
				log.Printf("checkMiddleware 2.5: uuid: %+v\n", uuid)

				// If UUID is empty, pass the request to the next middleware
				next.ServeHTTP(w, r)
				return
			}

			// Log checkRequest
			fmt.Printf("checkMiddleware 3: %+v\n", checkRequest)

			// Execute Check request
			checkResponse, err := handler.SendCheckRequest(checkRequest)
			if err != nil {
				// Log the error and return an error in case of an error
				log.Println("checkMiddleware: Error checking request:", err)
				http.Error(w, "checkMiddleware: Error checking request", http.StatusInternalServerError)
				return
			}

			// Add check results to the request context
			ctx := context.WithValue(r.Context(), "checkResult", checkResponse)
			r = r.WithContext(ctx)

			// Handle checkResponse
			// Print the content of the response to the console
			// If there is at least one successful response, allow the request
			if ResponseAllowed(checkResponse) {
				log.Printf("ResponseAllowed: Request allowed. Response: %+v\n", checkResponse)
				handlerInstance.SetCheckResult(true)
				// Execute the code that comes after the ResponseAllowed() function
			} else {
				// If there is no response or other checks did not pass
				log.Println("ResponseAllowed: Request blocked")
				http.Error(w, "ResponseAllowed: Request blocked", http.StatusForbidden)
				log.Printf("ResponseAllowed: Request blocked. Response: %+v\n", checkResponse)
				handlerInstance.SetCheckResult(false)

				return
			}
		}

		// If the request is not /check, pass it to the next middleware
		next.ServeHTTP(w, r)
	})
}

func ResponseAllowed(checkResponse handler.CheckResponse) bool {
	log.Printf("ResponseAllowed: Received a response from the server: %+v\n", checkResponse)

	message := checkResponse.Message

	if message == "Доступ открыт!" {
		log.Println("ResponseAllowed: Access granted!", message)
		return true
	} else if message == "Доступ закрыт!" {
		log.Println("ResponseAllowed: Access denied!", message)
		return false
	} else {
		log.Printf("ResponseAllowed: Unexpected response from the server: %s\n", message)
		return false
	}
}

func NewRouter(db *sql.DB, cfg *config.Config) *mux.Router {
	r := mux.NewRouter()

	// Initialize the repository and service
	imageRepo := postgres.NewImageRepository(db)
	imageService := service.NewImageService(imageRepo)

	// Create an instance of MyHandler
	myHandler := &MyHandler{}

	r.Use(loggingMiddleware)

	r.Use(setCORSHeaders)
	// Use checkMiddleware before all other handlers
	r.Use(func(next http.Handler) http.Handler {
		return checkMiddleware(next, myHandler)
	})

	r.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		// Handle /check request
		handler.CheckHandler(w, r)
		log.Println("/CHECK!", myHandler.IsCheckSuccessful())
		// Set the flag to true upon successful completion of /check
		myHandler.SetCheckResult(myHandler.IsCheckSuccessful())
	}).Methods("POST", "OPTIONS")

	r.HandleFunc("/increase-usage/{thumbPath:.*}", func(w http.ResponseWriter, r *http.Request) {
		// Check the flag before executing other routes
		log.Println("/IncreaseImageUsage! 1", myHandler.IsCheckSuccessful())

		if !myHandler.IsCheckSuccessful() {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Println("/IncreaseImageUsage! 2", myHandler.IsCheckSuccessful())

		handler.IncreaseImageUsage(imageService)(w, r)
	}).Methods("POST", "OPTIONS")

	r.PathPrefix("/static/images/").Handler(http.StripPrefix("/static/images/", http.FileServer(http.Dir("./static/images/"))))

	r.HandleFunc("/{family}/{group}/{subgroup}/{number:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
		log.Println("/GetImageByNumber! 1", myHandler.IsCheckSuccessful())

		if !myHandler.IsCheckSuccessful() {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Println("/GetImageByNumber! 2", myHandler.IsCheckSuccessful())

		handler.GetImageByNumber(imageService, cfg)(w, r)
	}).Methods("GET")

	r.HandleFunc("/{family}/{group}/{subgroup}/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("/GetImagesByFamilyGroupSubgroup! 1", myHandler.IsCheckSuccessful())

		if !myHandler.IsCheckSuccessful() {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Println("/GetImagesByFamilyGroupSubgroup! 2", myHandler.IsCheckSuccessful())

		handler.GetImagesByFamilyGroupSubgroup(imageService, cfg)(w, r)
	}).Methods("GET")

	r.HandleFunc("/least-used", func(w http.ResponseWriter, r *http.Request) {

		log.Println("/GetLeastUsedImages! 1", myHandler.IsCheckSuccessful())
		if !myHandler.IsCheckSuccessful() {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Println("/GetLeastUsedImages! 2", myHandler.IsCheckSuccessful())

		handler.GetLeastUsedImages(imageService, cfg)(w, r)
	}).Methods("GET")

	r.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		log.Println("/SearchImages! 1", myHandler.IsCheckSuccessful())

		if !myHandler.IsCheckSuccessful() {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Println("/SearchImages! 2", myHandler.IsCheckSuccessful())

		handler.SearchImages(imageService, cfg)(w, r)
	}).Methods("GET")

	return r
}
