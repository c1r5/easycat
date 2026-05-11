package mcp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/c1r5/easycat/internal/domain"
	"github.com/c1r5/easycat/internal/incidents"
	"github.com/c1r5/easycat/internal/observer"
)

const DefaultAddr = "127.0.0.1:8765"

type Source interface {
	Snapshot() observer.Snapshot
	GetIncident(id string) (incidents.Incident, bool)
}

type Server struct {
	addr   string
	source Source

	mu     sync.Mutex
	server *http.Server
}

func NewServer(addr string, source Source) *Server {
	if addr == "" {
		addr = DefaultAddr
	}
	return &Server{addr: addr, source: source}
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server != nil {
		return nil
	}
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	api := s.buildMCPServer()
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return api
	}, nil))
	httpServer := &http.Server{Handler: mux}
	s.server = httpServer
	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			_ = httpServer.Close()
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	httpServer := s.server
	s.server = nil
	s.mu.Unlock()
	if httpServer == nil {
		return nil
	}
	return httpServer.Shutdown(ctx)
}

func (s *Server) buildMCPServer() *mcpsdk.Server {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "easycat", Version: "v0.1.0"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get_active_context",
		Description: "Return current easycat device, package, PID, recent log count, and incident count.",
	}, s.getActiveContext)
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get_recent_logs",
		Description: "Return recent in-memory logcat lines.",
	}, s.getRecentLogs)
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list_incidents",
		Description: "List incidents detected during this easycat session.",
	}, s.listIncidents)
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get_incident",
		Description: "Return one incident by ID.",
	}, s.getIncident)
	return server
}

type noInput struct{}

type activeContextOutput struct {
	Context       observer.Context `json:"context"`
	RecentLogs    int              `json:"recent_logs"`
	IncidentCount int              `json:"incident_count"`
}

func (s *Server) getActiveContext(ctx context.Context, req *mcpsdk.CallToolRequest, input noInput) (*mcpsdk.CallToolResult, activeContextOutput, error) {
	snapshot := s.source.Snapshot()
	return nil, activeContextOutput{
		Context:       snapshot.Context,
		RecentLogs:    len(snapshot.Logs),
		IncidentCount: len(snapshot.Incidents),
	}, nil
}

type recentLogsInput struct {
	Package string `json:"package,omitempty" jsonschema:"optional package name filter"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum number of logs to return"`
}

type recentLogsOutput struct {
	Logs []domain.LogLine `json:"logs"`
}

func (s *Server) getRecentLogs(ctx context.Context, req *mcpsdk.CallToolRequest, input recentLogsInput) (*mcpsdk.CallToolResult, recentLogsOutput, error) {
	snapshot := s.source.Snapshot()
	logs := snapshot.Logs
	if input.Package != "" && input.Package != snapshot.Context.Package {
		logs = nil
	}
	if input.Limit > 0 && len(logs) > input.Limit {
		logs = logs[len(logs)-input.Limit:]
	}
	return nil, recentLogsOutput{Logs: logs}, nil
}

type listIncidentsInput struct {
	ActiveOnly bool `json:"active_only,omitempty" jsonschema:"reserved for future active incident filtering"`
}

type listIncidentsOutput struct {
	Incidents []incidents.Incident `json:"incidents"`
}

func (s *Server) listIncidents(ctx context.Context, req *mcpsdk.CallToolRequest, input listIncidentsInput) (*mcpsdk.CallToolResult, listIncidentsOutput, error) {
	return nil, listIncidentsOutput{Incidents: s.source.Snapshot().Incidents}, nil
}

type getIncidentInput struct {
	ID string `json:"id" jsonschema:"incident id"`
}

type getIncidentOutput struct {
	Incident incidents.Incident `json:"incident"`
	Found    bool               `json:"found"`
}

func (s *Server) getIncident(ctx context.Context, req *mcpsdk.CallToolRequest, input getIncidentInput) (*mcpsdk.CallToolResult, getIncidentOutput, error) {
	incident, ok := s.source.GetIncident(input.ID)
	return nil, getIncidentOutput{Incident: incident, Found: ok}, nil
}
