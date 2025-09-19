package models

// APIResponse 统一API响应格式
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// 常用响应构造函数
func Success(data interface{}) *APIResponse {
	return &APIResponse{
		Code:    200,
		Message: "success",
		Data:    data,
	}
}

func SuccessMessage(message string) *APIResponse {
	return &APIResponse{
		Code:    200,
		Message: message,
	}
}

func Error(code int, message string, err error) *ErrorResponse {
	resp := &ErrorResponse{
		Code:    code,
		Message: message,
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}

// 错误类型常量
const (
	ErrInvalidCookie      = "INVALID_COOKIE"
	ErrAPIRequest         = "API_REQUEST_ERROR"
	ErrDataParsing        = "DATA_PARSING_ERROR"
	ErrDatabaseOperation  = "DATABASE_ERROR"
	ErrTaskNotRunning     = "TASK_NOT_RUNNING"
	ErrTaskAlreadyRunning = "TASK_ALREADY_RUNNING"
)
