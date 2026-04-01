package response

// APIResponse 모든 HTTP API가 공통으로 사용하는 응답 본문입니다.
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}
