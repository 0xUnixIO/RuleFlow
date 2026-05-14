package app

import (
	"net/http"
)

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not found", http.StatusNotFound)
	return
}
