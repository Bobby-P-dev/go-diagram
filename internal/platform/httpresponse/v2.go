package httpresponse

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func RespondV2Error(w http.ResponseWriter, status int, code, message string) {
	var entropy [16]byte
	requestID := ""
	if _, err := rand.Read(entropy[:]); err == nil {
		requestID = hex.EncodeToString(entropy[:])
	} else {
		requestID = fmt.Sprintf("%x", time.Now().UnixNano())
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", requestID)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Status    string `json:"status"`
		Message   string `json:"message"`
		Code      string `json:"code"`
		RequestID string `json:"request_id"`
	}{Status: "error", Message: message, Code: code, RequestID: requestID})
}
