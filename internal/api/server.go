// Package api provides HTTP handlers for the MRLI-Agent platform.
package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mrli-agent/internal/agentctl"
	"mrli-agent/internal/db"
	"mrli-agent/internal/dispatch"
	"mrli-agent/internal/events"

	"github.com/gorilla/websocket"
)

// Server holds the HTTP handlers and dependencies.
type Server struct {
	db         *db.DB
	hub        *events.Hub
	checker    *agentctl.Checker
	dispatcher *dispatch.Dispatcher
}

// NewServer creates a new API server.
func NewServer(db *db.DB, hub *events.Hub) *Server {
	return &Server{
		db:         db,
		hub:        hub,
		checker:    agentctl.NewChecker(db, hub),
		dispatcher: dispatch.New(db),
	}
}

// NewServerWithDispatch creates a new API server with dispatch config.
func NewServerWithDispatch(db *db.DB, hub *events.Hub, dispatchCfg dispatch.Config) *Server {
	return &Server{
		db:         db,
		hub:        hub,
		checker:    agentctl.NewChecker(db, hub),
		dispatcher: dispatch.NewWithConfig(db, dispatchCfg),
	}
}

// Handler returns the main HTTP handler with all routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/dashboard", s.dashboard)

	// Agents CRUD
	mux.HandleFunc("GET /api/agents", s.listAgents)
	mux.HandleFunc("POST /api/agents", s.createAgent)
	mux.HandleFunc("GET /api/agents/{id}", s.getAgent)
	mux.HandleFunc("PUT /api/agents/{id}", s.updateAgent)
	mux.HandleFunc("DELETE /api/agents/{id}", s.deleteAgent)

	// API Keys CRUD
	mux.HandleFunc("GET /api/api-keys", s.listAPIKeys)
	mux.HandleFunc("POST /api/api-keys", s.createAPIKey)
	mux.HandleFunc("GET /api/api-keys/{id}", s.getAPIKey)
	mux.HandleFunc("PUT /api/api-keys/{id}", s.updateAPIKey)
	mux.HandleFunc("DELETE /api/api-keys/{id}", s.deleteAPIKey)

	// Projects CRUD
	mux.HandleFunc("GET /api/projects", s.listProjects)
	mux.HandleFunc("POST /api/projects", s.createProject)
	mux.HandleFunc("GET /api/projects/{id}", s.getProject)
	mux.HandleFunc("PUT /api/projects/{id}", s.updateProject)
	mux.HandleFunc("DELETE /api/projects/{id}", s.deleteProject)
	mux.HandleFunc("GET /api/projects/{id}/stats", s.getProjectStats)
	mux.HandleFunc("POST /api/projects/{id}/pause", s.pauseProject)
	mux.HandleFunc("POST /api/projects/{id}/resume", s.resumeProject)
	mux.HandleFunc("POST /api/projects/{id}/favorite", s.toggleProjectFavorite)
	mux.HandleFunc("GET /api/projects/{id}/skills", s.listProjectSkills)
	mux.HandleFunc("POST /api/projects/{id}/skills", s.bindProjectSkill)
	mux.HandleFunc("DELETE /api/projects/{id}/skills/{skill_id}", s.unbindProjectSkill)

	// Tasks CRUD
	mux.HandleFunc("GET /api/tasks", s.listTasks)
	mux.HandleFunc("POST /api/tasks", s.createTask)
	mux.HandleFunc("GET /api/tasks/{id}", s.getTask)
	mux.HandleFunc("PUT /api/tasks/{id}/status", s.updateTaskStatus)
	mux.HandleFunc("PUT /api/tasks/{id}/assign/{agent_id}", s.assignTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", s.deleteTask)

	// Chat
	mux.HandleFunc("GET /api/chat/{agent_id}", s.listChatMessages)
	mux.HandleFunc("POST /api/chat/{agent_id}", s.sendChatMessage)
	mux.HandleFunc("DELETE /api/chat/{agent_id}", s.clearChatHistory)

	// WebSocket
	mux.HandleFunc("GET /ws", s.handleWebSocket)

	// Token Usage
	mux.HandleFunc("GET /api/tokens", s.listTokenUsage)
	mux.HandleFunc("GET /api/tokens/summary", s.getTokenUsageSummary)
	mux.HandleFunc("POST /api/tokens", s.recordTokenUsage)

	// Memory
	mux.HandleFunc("GET /api/memories", s.listMemories)
	mux.HandleFunc("GET /api/memories/search", s.searchMemories)
	mux.HandleFunc("GET /api/memories/stats", s.getMemoryStats)
	mux.HandleFunc("POST /api/memories", s.createMemory)
	mux.HandleFunc("PUT /api/memories/{id}", s.updateMemory)
	mux.HandleFunc("DELETE /api/memories/{id}", s.deleteMemory)

	// Roles
	mux.HandleFunc("GET /api/roles", s.listRoles)
	mux.HandleFunc("POST /api/roles", s.createRole)
	mux.HandleFunc("GET /api/roles/{id}", s.getRole)
	mux.HandleFunc("PUT /api/roles/{id}", s.updateRole)
	mux.HandleFunc("DELETE /api/roles/{id}", s.deleteRole)

	// Agent Lifecycle
	mux.HandleFunc("POST /api/agents/{id}/wake", s.wakeAgent)
	mux.HandleFunc("POST /api/agents/{id}/restart", s.restartAgent)
	mux.HandleFunc("POST /api/agents/{id}/stop", s.stopAgent)
	mux.HandleFunc("POST /api/agents/{id}/ping", s.pingAgent)

	// Agent-Role binding
	mux.HandleFunc("POST /api/agents/{id}/role", s.assignRole)
	mux.HandleFunc("DELETE /api/agents/{id}/role", s.unassignRole)

	// Agent Status
	mux.HandleFunc("PATCH /api/agents/{id}/status", s.updateAgentStatus)

	// Skills CRUD
	mux.HandleFunc("GET /api/skills", s.listSkills)
	mux.HandleFunc("POST /api/skills", s.createSkill)
	mux.HandleFunc("GET /api/skills/stats", s.getSkillsStats)
	mux.HandleFunc("GET /api/skills/agent-map", s.getAgentSkillsMap)
	mux.HandleFunc("GET /api/skills/{id}", s.getSkill)
	mux.HandleFunc("PUT /api/skills/{id}", s.updateSkill)
	mux.HandleFunc("DELETE /api/skills/{id}", s.deleteSkill)
	mux.HandleFunc("POST /api/skills/{id}/favorite", s.toggleSkillFavorite)
	mux.HandleFunc("GET /api/skills/{id}/logs", s.getSkillLogs)

	// Role-Skill binding
	mux.HandleFunc("GET /api/roles/{id}/skills", s.listRoleSkills)
	mux.HandleFunc("POST /api/roles/{id}/skills", s.bindRoleSkill)
	mux.HandleFunc("DELETE /api/roles/{id}/skills/{skill_id}", s.unbindRoleSkill)

	// Agent-Skill binding
	mux.HandleFunc("GET /api/agents/{id}/skills", s.listAgentSkills)
	mux.HandleFunc("POST /api/agents/{id}/skills", s.bindAgentSkill)
	mux.HandleFunc("DELETE /api/agents/{id}/skills/{skill_id}", s.unbindAgentSkill)

	// Serve static files
	mux.Handle("/", http.FileServer(http.Dir("web")))

	return s.corsMiddleware(mux)
}

// corsMiddleware adds CORS headers.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// health returns a simple health check.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// dashboard returns aggregated statistics.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetDashboardStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// === Agents Handlers ===

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.db.ListAgents()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request) {
	var a db.Agent
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Validate required fields
	if a.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if a.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required")
		return
	}

	// Set defaults
	if a.Args == "" {
		a.Args = "{}"
	}
	a.Enabled = true

	if err := s.db.CreateAgent(&a); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, a)
}

func (s *Server) getAgent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	a, err := s.db.GetAgent(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	writeJSON(w, http.StatusOK, a)
}

func (s *Server) updateAgent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var a db.Agent
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	a.ID = id
	if a.Args == "" {
		a.Args = "{}"
	}

	if err := s.db.UpdateAgent(&a); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, a)
}

func (s *Server) deleteAgent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := s.db.DeleteAgent(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// === API Keys Handlers ===

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.db.ListAPIKeys()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var k db.APIKey
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if k.Provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}
	if k.APIKey == "" {
		writeError(w, http.StatusBadRequest, "api_key is required")
		return
	}
	k.Enabled = true

	if err := s.db.CreateAPIKey(&k); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	k.MaskKey()
	writeJSON(w, http.StatusCreated, k)
}

func (s *Server) getAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	k, err := s.db.GetAPIKey(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "api key not found")
		return
	}

	writeJSON(w, http.StatusOK, k)
}

func (s *Server) updateAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var k db.APIKey
	if err := json.NewDecoder(r.Body).Decode(&k); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	k.ID = id
	if err := s.db.UpdateAPIKey(&k); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	k.MaskKey()
	writeJSON(w, http.StatusOK, k)
}

func (s *Server) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := s.db.DeleteAPIKey(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// === Projects Handlers ===

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.db.ListProjects()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var p db.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if p.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if p.Branch == "" {
		p.Branch = "main"
	}
	if p.Tags == "" {
		p.Tags = "[]"
	}
	if p.Status == "" {
		p.Status = "Draft"
	}

	if err := s.db.CreateProject(&p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	p, err := s.db.GetProject(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	writeJSON(w, http.StatusOK, p)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var p db.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	p.ID = id
	if p.Branch == "" {
		p.Branch = "main"
	}

	if err := s.db.UpdateProject(&p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := s.db.DeleteProject(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) getProjectStats(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	stats, err := s.db.GetProjectStats(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) pauseProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.UpdateProjectStatus(id, "Blocked"); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (s *Server) resumeProject(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.UpdateProjectStatus(id, "Active"); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

func (s *Server) toggleProjectFavorite(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.ToggleProjectFavorite(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

func (s *Server) listProjectSkills(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	skills, err := s.db.ListProjectSkills(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func (s *Server) bindProjectSkill(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	var body struct {
		SkillID int64 `json:"skill_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.db.BindProjectSkill(projectID, body.SkillID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "bound"})
}

func (s *Server) unbindProjectSkill(w http.ResponseWriter, r *http.Request) {
	projectID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}
	skillID, err := parseID(r.PathValue("skill_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid skill id")
		return
	}
	if err := s.db.UnbindProjectSkill(projectID, skillID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unbound"})
}

// === Tasks Handlers ===

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	// Optional project_id filter
	projectIDStr := r.URL.Query().Get("project_id")
	var tasks []db.Task
	var err error

	if projectIDStr != "" {
		projectID, e := parseID(projectIDStr)
		if e != nil {
			writeError(w, http.StatusBadRequest, "invalid project_id")
			return
		}
		tasks, err = s.db.ListTasks(projectID)
	} else {
		tasks, err = s.db.ListAllTasks()
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var t db.Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if t.ProjectID == 0 {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	if t.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if t.Status == "" {
		t.Status = "todo"
	}

	if err := s.db.CreateTask(&t); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	t, err := s.db.GetTask(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	writeJSON(w, http.StatusOK, t)
}

func (s *Server) updateTaskStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body struct {
		Status   string `json:"status"`
		Progress *int   `json:"progress"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	progress := 0
	if body.Progress != nil {
		progress = *body.Progress
	}
	if err := s.db.UpdateTaskStatus(id, body.Status, progress); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) assignTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	agentID, err := parseID(r.PathValue("agent_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	if err := s.db.AssignTask(taskID, agentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := s.db.DeleteTask(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// === Chat Handlers ===

func (s *Server) listChatMessages(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseID(r.PathValue("agent_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent_id")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, e := parseID(l); e == nil && parsed > 0 {
			limit = int(parsed)
		}
	}

	messages, err := s.db.ListChatMessages(agentID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (s *Server) sendChatMessage(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseID(r.PathValue("agent_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent_id")
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if body.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	// Verify agent exists
	agent, err := s.db.GetAgent(agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}

	// Save user message
	userMsg := &db.ChatMessage{
		AgentID: agentID,
		Role:    "user",
		Content: body.Content,
	}
	if err := s.db.CreateChatMessage(userMsg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Get recent chat history for context
	history, _ := s.db.ListChatMessages(agentID, 20)

	// Call agent via dispatcher
	reply, err := s.dispatcher.Send(agent, body.Content, history)
	if err != nil {
		log.Printf("[chat] Agent dispatch error: %v", err)
		reply = fmt.Sprintf("[%s] Error: %s", agent.Name, err.Error())
	}

	// Save assistant response
	assistantMsg := &db.ChatMessage{
		AgentID: agentID,
		Role:    "assistant",
		Content: reply,
	}
	if err := s.db.CreateChatMessage(assistantMsg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Broadcast via WebSocket
	s.hub.Broadcast("chat.message", assistantMsg)

	writeJSON(w, http.StatusCreated, assistantMsg)
}

func (s *Server) clearChatHistory(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseID(r.PathValue("agent_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent_id")
		return
	}

	if err := s.db.DeleteChatMessages(agentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

// === WebSocket Handler ===

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] Upgrade error: %v", err)
		return
	}

	s.hub.Register(conn)

	// Read loop (just to detect disconnect)
	go func() {
		defer s.hub.Unregister(conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

// === Token Usage Handlers ===

func (s *Server) listTokenUsage(w http.ResponseWriter, r *http.Request) {
	records, err := s.db.ListTokenUsage(100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *Server) getTokenUsageSummary(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.db.GetTokenUsageSummary()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	totalCost, totalTokens, err := s.db.GetTokenUsageTotal()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"by_model":     summaries,
		"total_cost":   totalCost,
		"total_tokens": totalTokens,
	})
}

func (s *Server) recordTokenUsage(w http.ResponseWriter, r *http.Request) {
	var u db.TokenUsage
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if u.Model == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}
	u.TotalTokens = u.PromptTokens + u.CompletionTokens

	if err := s.db.RecordTokenUsage(&u); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, u)
}

// === Memory Handlers ===

func (s *Server) listMemories(w http.ResponseWriter, r *http.Request) {
	level := r.URL.Query().Get("level")
	var agentID, projectID int64
	if v := r.URL.Query().Get("agent_id"); v != "" {
		agentID, _ = parseID(v)
	}
	if v := r.URL.Query().Get("project_id"); v != "" {
		projectID, _ = parseID(v)
	}

	memories, err := s.db.ListMemories(level, agentID, projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func (s *Server) searchMemories(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "q parameter is required")
		return
	}

	memories, err := s.db.SearchMemories(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func (s *Server) getMemoryStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetMemoryStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) createMemory(w http.ResponseWriter, r *http.Request) {
	var m db.Memory
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if m.Key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}
	if m.Value == "" {
		writeError(w, http.StatusBadRequest, "value is required")
		return
	}
	if m.Level == "" {
		m.Level = "working"
	}
	if m.Tags == "" {
		m.Tags = "[]"
	}

	if err := s.db.CreateMemory(&m); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateMemory(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var m db.Memory
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	m.ID = id
	if err := s.db.UpdateMemory(&m); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, m)
}

func (s *Server) deleteMemory(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := s.db.DeleteMemory(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// === Roles Handlers ===

func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.db.ListRoles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

func (s *Server) createRole(w http.ResponseWriter, r *http.Request) {
	var role db.Role
	if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if role.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	role.Enabled = true

	if err := s.db.CreateRole(&role); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, role)
}

func (s *Server) getRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	role, err := s.db.GetRole(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "role not found")
		return
	}

	writeJSON(w, http.StatusOK, role)
}

func (s *Server) updateRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var role db.Role
	if err := json.NewDecoder(r.Body).Decode(&role); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	role.ID = id
	if err := s.db.UpdateRole(&role); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, role)
}

func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := s.db.DeleteRole(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) assignRole(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	var body struct {
		RoleID int64 `json:"role_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := s.db.AssignRole(agentID, body.RoleID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

func (s *Server) unassignRole(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	if err := s.db.UnassignRole(agentID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unassigned"})
}

func (s *Server) updateAgentStatus(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := s.db.UpdateAgentStatus(agentID, body.Status); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// === Agent Lifecycle Handlers ===

func (s *Server) wakeAgent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	agent, err := s.db.GetAgent(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err := s.checker.Wake(agent); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a, _ := s.db.GetAgent(id)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "status": a.Status})
}

func (s *Server) restartAgent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	agent, err := s.db.GetAgent(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err := s.checker.Restart(agent); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a, _ := s.db.GetAgent(id)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "status": a.Status})
}

func (s *Server) stopAgent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	agent, err := s.db.GetAgent(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err := s.checker.Stop(agent); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "status": "offline"})
}

func (s *Server) pingAgent(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	agent, err := s.db.GetAgent(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	result, err := s.checker.Ping(agent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	a, _ := s.db.GetAgent(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":    result.Healthy,
		"status":     a.Status,
		"message":    result.Message,
		"latency_ms": result.Latency,
	})
}

// === Skills Handlers ===

func (s *Server) listSkills(w http.ResponseWriter, r *http.Request) {
	skills, err := s.db.ListSkills()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func (s *Server) createSkill(w http.ResponseWriter, r *http.Request) {
	var sk db.Skill
	if err := json.NewDecoder(r.Body).Decode(&sk); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if sk.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if sk.UUID == "" {
		sk.UUID = fmt.Sprintf("skill-%d", time.Now().UnixNano())
	}
	sk.Enabled = true

	if err := s.db.CreateSkill(&sk); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sk)
}

func (s *Server) getSkill(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	sk, err := s.db.GetSkill(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "skill not found")
		return
	}
	writeJSON(w, http.StatusOK, sk)
}

func (s *Server) updateSkill(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var sk db.Skill
	if err := json.NewDecoder(r.Body).Decode(&sk); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	sk.ID = id
	if err := s.db.UpdateSkill(&sk); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sk)
}

func (s *Server) deleteSkill(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.DeleteSkill(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) getSkillsStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetSkillsStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) toggleSkillFavorite(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.db.ToggleSkillFavorite(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

func (s *Server) getSkillLogs(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	logs, err := s.db.ListSkillLogs(id, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

func (s *Server) getAgentSkillsMap(w http.ResponseWriter, r *http.Request) {
	m, err := s.db.ListAgentSkillsMap()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// === Role-Skill Handlers ===

func (s *Server) listRoleSkills(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	skills, err := s.db.ListRoleSkills(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func (s *Server) bindRoleSkill(w http.ResponseWriter, r *http.Request) {
	roleID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid role id")
		return
	}
	var body struct {
		SkillID int64 `json:"skill_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.db.BindRoleSkill(roleID, body.SkillID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "bound"})
}

func (s *Server) unbindRoleSkill(w http.ResponseWriter, r *http.Request) {
	roleID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid role id")
		return
	}
	skillID, err := parseID(r.PathValue("skill_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid skill id")
		return
	}
	if err := s.db.UnbindRoleSkill(roleID, skillID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unbound"})
}

// === Agent-Skill Handlers ===

func (s *Server) listAgentSkills(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	skills, err := s.db.ListAgentSkills(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, skills)
}

func (s *Server) bindAgentSkill(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	var body struct {
		SkillID int64 `json:"skill_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.db.BindAgentSkill(agentID, body.SkillID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "bound"})
}

func (s *Server) unbindAgentSkill(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseID(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	skillID, err := parseID(r.PathValue("skill_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid skill id")
		return
	}
	if err := s.db.UnbindAgentSkill(agentID, skillID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unbound"})
}

// === Helpers ===

func parseID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	return strconv.ParseInt(s, 10, 64)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[api] write json error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Start starts the HTTP server.
func Start(addr string, db *db.DB, hub *events.Hub) error {
	srv := NewServer(db, hub)
	log.Printf("[api] Server listening on %s", addr)
	return fmt.Errorf("server stopped: %w", http.ListenAndServe(addr, srv.Handler()))
}

// StartWithDispatch starts the HTTP server with dispatch config.
func StartWithDispatch(addr string, db *db.DB, hub *events.Hub, dispatchCfg dispatch.Config) error {
	srv := NewServerWithDispatch(db, hub, dispatchCfg)
	log.Printf("[api] Server listening on %s", addr)
	return fmt.Errorf("server stopped: %w", http.ListenAndServe(addr, srv.Handler()))
}
