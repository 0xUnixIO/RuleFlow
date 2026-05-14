package api

import (
	"encoding/json"
	"net/http"
)

// Response 统一响应格式
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error string `json:"error"`
}

// SendJSON 发送 JSON 响应
func SendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// SendSuccess 发送成功响应
func SendSuccess(w http.ResponseWriter, data interface{}) {
	SendJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// SendError 发送错误响应
func SendError(w http.ResponseWriter, statusCode int, err string) {
	SendJSON(w, statusCode, Response{
		Success: false,
		Error:   err,
	})
}

// SendText 发送文本响应
func SendText(w http.ResponseWriter, statusCode int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)
	w.Write([]byte(text))
}
