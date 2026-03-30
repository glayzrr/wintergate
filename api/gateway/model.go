package gatewayapi

// ReceiveResponse 게이트웨이가 트래픽을 수신했음을 반환하는 응답 본문입니다.
type ReceiveResponse struct {
	Received bool   `json:"received"`
	Method   string `json:"method"`
	Path     string `json:"path"`
}
