package dto

// Response 统一响应格式
type Response struct {
	Code    int         `json:"code"`    // 业务状态码，0 表示成功
	Message string      `json:"message"` // 提示信息
	Result  interface{} `json:"result"`  // 数据载体
}

type PageResult struct {
	Items    interface{} `json:"items"` // 数据列表
	Total    int64       `json:"total"` // 总数
	PageSize int         `json:"page_size"`
	PageNum  int         `json:"page_num"`
}

func Success(data interface{}) Response {
	return Response{
		Code:    0,
		Message: "success",
		Result:  data,
	}
}

func Error(code int, message string) Response {
	return Response{
		Code:    code,
		Message: message,
		Result:  nil,
	}
}
