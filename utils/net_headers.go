package utils

import "net/http"

func GetIPAddress(r *http.Request) string {
	IPAddress := r.Header.Get("CF-Connecting-IP")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Real-Ip")
	}
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}
