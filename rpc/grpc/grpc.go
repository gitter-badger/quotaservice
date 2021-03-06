// Licensed under the Apache License, Version 2.0
// Details: https://raw.githubusercontent.com/maniksurtani/quotaservice/master/LICENSE

package grpc

import (
	"net"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"golang.org/x/net/context"
	"github.com/maniksurtani/quotaservice/logging"
	"github.com/maniksurtani/quotaservice"
	qspb "github.com/maniksurtani/quotaservice/protos"
	"github.com/maniksurtani/quotaservice/lifecycle"
	"github.com/golang/protobuf/proto"
	"strings"
)

type GrpcEndpoint struct {
	hostport      string
	grpcServer    *grpc.Server
	currentStatus lifecycle.Status
	qs            quotaservice.QuotaService
}
// New creates a new GrpcEndpoint, listening on hostport. Hostport is a string in the form
// "host:port"
func New(hostport string) *GrpcEndpoint {
	if !strings.Contains(hostport, ":") {
		panic(fmt.Sprintf("hostport should be in the format 'host:port', but is currently %v",
			hostport))
	}
	return &GrpcEndpoint{hostport: hostport}
}

func (g *GrpcEndpoint) Init(qs quotaservice.QuotaService) {
	g.qs = qs
}

func (g *GrpcEndpoint) Start() {
	lis, err := net.Listen("tcp", g.hostport)
	if err != nil {
		logging.Fatalf("Cannot start server on port %v. Error %v", g.hostport, err)
		panic(fmt.Sprintf("Cannot start server on port %v. Error %v", g.hostport, err))
	}

	grpclog.SetLogger(logging.CurrentLogger())
	g.grpcServer = grpc.NewServer()
	// Each service should be registered
	qspb.RegisterQuotaServiceServer(g.grpcServer, g)
	go g.grpcServer.Serve(lis)
	g.currentStatus = lifecycle.Started
	logging.Printf("Starting server on %v", g.hostport)
	logging.Printf("Server status: %v", g.currentStatus)
}

func (g *GrpcEndpoint) Stop() {
	g.currentStatus = lifecycle.Stopped
}

func (g *GrpcEndpoint) Allow(ctx context.Context, req *qspb.AllowRequest) (*qspb.AllowResponse, error) {
	rsp := new(qspb.AllowResponse)
	if invalid(req) {
		logging.Printf("Invalid request %+v", req)
		s := qspb.AllowResponse_FAILED
		rsp.Status = &s
		return rsp, nil
	}

	var numTokensRequested int64 = 1
	if req.NumTokensRequested != nil {
		numTokensRequested = req.GetNumTokensRequested()
	}

	var maxWaitMillisOverride int64 = -1
	if req.MaxWaitMillisOverride != nil {
		maxWaitMillisOverride = req.GetMaxWaitMillisOverride()
	}

	granted, wait, err := g.qs.Allow(req.GetNamespace(), req.GetName(), numTokensRequested, maxWaitMillisOverride)
	var status qspb.AllowResponse_Status;

	if err != nil {
		if qsErr, ok := err.(quotaservice.QuotaServiceError); ok {
			switch qsErr.Reason {
			case quotaservice.ER_NO_SUCH_BUCKET:
				status = qspb.AllowResponse_REJECTED
			case quotaservice.ER_REJECTED:
				status = qspb.AllowResponse_REJECTED
			case quotaservice.ER_TIMED_OUT_WAITING:
				status = qspb.AllowResponse_REJECTED
			}
		} else {
			logging.Printf("Caught error %v", err)
			status = qspb.AllowResponse_FAILED
		}
	} else {
		if wait > 0 {
			status = qspb.AllowResponse_OK_WAIT
		} else {
			status = qspb.AllowResponse_OK
		}
		rsp.NumTokensGranted = proto.Int64(granted)
		rsp.WaitMillis = proto.Int64(wait.Nanoseconds())
	}
	rsp.Status = &status
	return rsp, nil
}

func invalid(req *qspb.AllowRequest) bool {
	// Negative tokens are allowed!
	return req.GetName() == "" || req.GetNamespace() == "" || (req.NumTokensRequested != nil && req.GetNumTokensRequested() == 0)
}
