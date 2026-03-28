package api

import (
	"context"

	"google.golang.org/grpc"
)

const (
	ServiceName               = "bnpl.v1.DecisionService"
	SubmitApplicationFullName = "/" + ServiceName + "/SubmitApplication"
	GetApplicationFullName    = "/" + ServiceName + "/GetApplication"
	GetDecisionTraceFullName  = "/" + ServiceName + "/GetDecisionTrace"
)

type DecisionServer interface {
	SubmitApplication(context.Context, *SubmitApplicationRequest) (*SubmitApplicationResponse, error)
	GetApplication(context.Context, *GetApplicationRequest) (*GetApplicationResponse, error)
	GetDecisionTrace(context.Context, *GetDecisionTraceRequest) (*GetDecisionTraceResponse, error)
}

type DecisionClient struct {
	cc grpc.ClientConnInterface
}

func NewDecisionClient(cc grpc.ClientConnInterface) *DecisionClient {
	return &DecisionClient{cc: cc}
}

func (c *DecisionClient) SubmitApplication(ctx context.Context, in *SubmitApplicationRequest, opts ...grpc.CallOption) (*SubmitApplicationResponse, error) {
	out := new(SubmitApplicationResponse)
	err := c.cc.Invoke(ctx, SubmitApplicationFullName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *DecisionClient) GetApplication(ctx context.Context, in *GetApplicationRequest, opts ...grpc.CallOption) (*GetApplicationResponse, error) {
	out := new(GetApplicationResponse)
	err := c.cc.Invoke(ctx, GetApplicationFullName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *DecisionClient) GetDecisionTrace(ctx context.Context, in *GetDecisionTraceRequest, opts ...grpc.CallOption) (*GetDecisionTraceResponse, error) {
	out := new(GetDecisionTraceResponse)
	err := c.cc.Invoke(ctx, GetDecisionTraceFullName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func RegisterDecisionServiceServer(s *grpc.Server, srv DecisionServer) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: ServiceName,
		HandlerType: (*DecisionServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "SubmitApplication",
				Handler:    submitApplicationHandler,
			},
			{
				MethodName: "GetApplication",
				Handler:    getApplicationHandler,
			},
			{
				MethodName: "GetDecisionTrace",
				Handler:    getDecisionTraceHandler,
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "decision-json",
	}, srv)
}

func submitApplicationHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(SubmitApplicationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DecisionServer).SubmitApplication(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: SubmitApplicationFullName}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(DecisionServer).SubmitApplication(ctx, req.(*SubmitApplicationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func getApplicationHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(GetApplicationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DecisionServer).GetApplication(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: GetApplicationFullName}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(DecisionServer).GetApplication(ctx, req.(*GetApplicationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func getDecisionTraceHandler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(GetDecisionTraceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DecisionServer).GetDecisionTrace(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: GetDecisionTraceFullName}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(DecisionServer).GetDecisionTrace(ctx, req.(*GetDecisionTraceRequest))
	}
	return interceptor(ctx, in, info, handler)
}
