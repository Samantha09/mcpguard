// JSON-RPC 2.0 消息类型 — 可导出的公共库
package jsonrpc

// Version JSON-RPC 协议版本
const Version = "2.0"

// Request JSON-RPC 请求
type Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// Response JSON-RPC 响应
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error JSON-RPC 错误
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// 标准错误码
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Notification JSON-RPC 通知（无 ID 的请求）
type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// NewRequest 创建 JSON-RPC 请求
func NewRequest(id any, method string, params any) *Request {
	return &Request{
		JSONRPC: Version,
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// NewResponse 创建 JSON-RPC 成功响应
func NewResponse(id any, result any) *Response {
	return &Response{
		JSONRPC: Version,
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse 创建 JSON-RPC 错误响应
func NewErrorResponse(id any, code int, message string) *Response {
	return &Response{
		JSONRPC: Version,
		ID:      id,
		Error:   &Error{Code: code, Message: message},
	}
}
