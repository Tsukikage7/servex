package errors

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// grpcDetail gRPC Status detail 中携带的错误详情.
type grpcDetail struct {
	Code     int               `json:"code"`
	Key      string            `json:"key"`
	Message  string            `json:"message"`
	HTTP     int               `json:"http,omitempty"`
	Metadata map[string]string `json:"metadata,omitzero"`
}

// ToGRPCStatus 将 error 转为 gRPC Status.
// 除了在 message 中嵌入 JSON 详情外，还会附加标准 errdetails:
//   - ErrorInfo: 携带错误域（domain）、reason 和 metadata
//   - BadRequest: 当 metadata 中存在 "field" 和 "field_violation" 时附加字段校验详情
//   - RetryInfo: 当 metadata 中存在 "retry_delay" 时附加重试延迟建议
func ToGRPCStatus(err error) *status.Status {
	if err == nil {
		return status.New(codes.OK, "")
	}

	e, ok := FromError(err)
	if !ok {
		return status.New(codes.Internal, "内部错误")
	}

	code := e.GRPC
	if code == codes.OK {
		code = codes.Internal
	}

	// 内部错误（5xxxx、6xxxx）不透传详细消息，避免暴露敏感信息.
	// 业务码 1xxxxx+ 不受影响.
	message := e.Message
	if e.Code >= 50000 && e.Code < 70000 {
		message = "内部错误"
	}

	detail := &grpcDetail{
		Code:     e.Code,
		Key:      e.Key,
		Message:  message,
		HTTP:     e.HTTP,
		Metadata: e.Metadata,
	}
	detailJSON, jsonErr := json.Marshal(detail)
	if jsonErr != nil {
		// JSON 序列化失败时使用 fallback 纯文本
		return status.New(code, message)
	}

	st := status.New(code, string(detailJSON))

	// 附加标准 gRPC error details
	// 1. ErrorInfo: 始终附加，携带 reason/domain/metadata
	metadata := e.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	errorInfo := &errdetails.ErrorInfo{
		Reason:   e.Key,
		Domain:   "servex",
		Metadata: metadata,
	}

	// 2. BadRequest: 当 metadata 中包含 field 和 field_violation 时附加
	var badRequest *errdetails.BadRequest
	if field := e.Metadata["field"]; field != "" {
		badRequest = &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       field,
					Description: e.Metadata["field_violation"],
				},
			},
		}
	}

	// 3. RetryInfo: 当 metadata 中包含 retry_delay 时附加
	var retryInfo *errdetails.RetryInfo
	if delayStr := e.Metadata["retry_delay"]; delayStr != "" {
		if d, parseErr := time.ParseDuration(delayStr); parseErr == nil {
			retryInfo = &errdetails.RetryInfo{
				RetryDelay: durationpb.New(d),
			}
		}
	}

	// 逐个附加 details（WithDetails 接受 protoadapt.MessageV1 可变参数）
	if stWithDetails, detailErr := st.WithDetails(errorInfo); detailErr == nil {
		st = stWithDetails
	}
	if badRequest != nil {
		if stWithDetails, detailErr := st.WithDetails(badRequest); detailErr == nil {
			st = stWithDetails
		}
	}
	if retryInfo != nil {
		if stWithDetails, detailErr := st.WithDetails(retryInfo); detailErr == nil {
			st = stWithDetails
		}
	}

	return st
}

// FromGRPCStatus 从 gRPC Status 还原 *Error.
// 优先从标准 errdetails（ErrorInfo）中提取结构化信息，
// 再从 message JSON 中补全 Code 等自定义字段.
// 同时解析 BadRequest 和 RetryInfo 并合并到 Metadata.
func FromGRPCStatus(st *status.Status) *Error {
	if st == nil || st.Code() == codes.OK {
		return nil
	}

	result := &Error{
		GRPC: st.Code(),
	}

	// 尝试从 message JSON 中还原基本字段
	var detail grpcDetail
	jsonOK := json.Unmarshal([]byte(st.Message()), &detail) == nil && detail.Code != 0

	if jsonOK {
		result.Code = detail.Code
		result.Key = detail.Key
		result.Message = detail.Message
		result.HTTP = detail.HTTP
		result.Metadata = detail.Metadata
	} else {
		result.Code = int(st.Code())
		result.Key = st.Code().String()
		result.Message = st.Message()
	}

	// 确保 Metadata 已初始化
	if result.Metadata == nil {
		result.Metadata = make(map[string]string)
	}

	// 从标准 errdetails 中提取结构化信息
	for _, d := range st.Details() {
		switch v := d.(type) {
		case *errdetails.ErrorInfo:
			// 如果 JSON 还原失败，则从 ErrorInfo 中补全 Key
			if !jsonOK && v.Reason != "" {
				result.Key = v.Reason
			}
			// 合并 ErrorInfo.Metadata（不覆盖已有的）
			for mk, mv := range v.Metadata {
				if _, exists := result.Metadata[mk]; !exists {
					result.Metadata[mk] = mv
				}
			}
		case *errdetails.BadRequest:
			// 将第一个 FieldViolation 写入 metadata
			if len(v.FieldViolations) > 0 {
				fv := v.FieldViolations[0]
				if _, exists := result.Metadata["field"]; !exists {
					result.Metadata["field"] = fv.Field
				}
				if _, exists := result.Metadata["field_violation"]; !exists {
					result.Metadata["field_violation"] = fv.Description
				}
			}
		case *errdetails.RetryInfo:
			if v.RetryDelay != nil {
				if _, exists := result.Metadata["retry_delay"]; !exists {
					result.Metadata["retry_delay"] = v.RetryDelay.AsDuration().String()
				}
			}
		}
	}

	// 如果 Metadata 为空则置 nil，保持与原始行为一致
	if len(result.Metadata) == 0 {
		result.Metadata = nil
	}

	return result
}

// UnaryServerInterceptor 返回 gRPC 一元拦截器.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		return nil, ToGRPCStatus(err).Err()
	}
}

// StreamServerInterceptor 返回 gRPC 流式拦截器.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		err := handler(srv, ss)
		if err == nil {
			return nil
		}
		return ToGRPCStatus(err).Err()
	}
}
