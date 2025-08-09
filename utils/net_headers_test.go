package utils

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetIPAddressFromHeader(t *testing.T) {
	ip := "123.45.67.89"

	// 4 methods of getting IP Address, CF-Connecting-IP preferred, X-Real-Ip, then X-Forwarded-For, then RemoteAddr

	request, _ := http.NewRequest(http.MethodPost, "appditto.com", bytes.NewReader([]byte("")))
	request.Header.Set("CF-Connecting-IP", ip)
	request.Header.Set("X-Real-Ip", "not-the-ip")
	request.Header.Set("X-Forwarded-For", "not-the-ip")
	assert.Equal(t, ip, GetIPAddress(request))

	request, _ = http.NewRequest(http.MethodPost, "appditto.com", bytes.NewReader([]byte("")))
	request.Header.Set("X-Real-Ip", ip)
	request.Header.Set("X-Forwarded-For", "not-the-ip")

	assert.Equal(t, ip, GetIPAddress(request))

	request, _ = http.NewRequest(http.MethodPost, "appditto.com", bytes.NewReader([]byte("")))
	request.Header.Set("X-Forwarded-For", ip)
	assert.Equal(t, ip, GetIPAddress(request))

	request, _ = http.NewRequest(http.MethodPost, "appditto.com", bytes.NewReader([]byte("")))
	request.RemoteAddr = ip
	assert.Equal(t, ip, GetIPAddress(request))
}
