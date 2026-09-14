package utils

import (
	"encoding/json"
	"net/http"
)

type SuccessResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

func RespondJSON(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := SuccessResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	}

	json.NewEncoder(w).Encode(response)
}

func RespondError(w http.ResponseWriter, statusCode int, message string, errDetail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Status:  "error",
		Message: message,
		Error:   errDetail,
	}

	json.NewEncoder(w).Encode(response)
}
