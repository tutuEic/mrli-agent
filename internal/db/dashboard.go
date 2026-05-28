package db

// DashboardStats holds aggregated statistics for the dashboard.
type DashboardStats struct {
	AgentCount    int    `json:"agent_count"`
	OnlineCount   int    `json:"online_count"`
	ProjectCount  int    `json:"project_count"`
	TaskCount     int    `json:"task_count"`
	DoneCount     int    `json:"done_count"`
	FailedCount   int    `json:"failed_count"`
	APIKeyCount   int    `json:"api_key_count"`
	SkillCount    int    `json:"skill_count"`
	SkillUses     int    `json:"skill_uses"`
	MostUsedSkill string `json:"most_used_skill"`
	RecoverCount  int    `json:"recover_count"`
}

// GetDashboardStats returns aggregated counts for the dashboard.
func (db *DB) GetDashboardStats() (*DashboardStats, error) {
	s := &DashboardStats{}

	db.Conn.QueryRow("SELECT COUNT(*) FROM agents").Scan(&s.AgentCount)
	db.Conn.QueryRow("SELECT COUNT(*) FROM agents WHERE status IN ('online','busy')").Scan(&s.OnlineCount)
	db.Conn.QueryRow("SELECT COUNT(*) FROM projects").Scan(&s.ProjectCount)
	db.Conn.QueryRow("SELECT COUNT(*) FROM api_keys").Scan(&s.APIKeyCount)

	// Tasks table may not exist yet, so we ignore errors
	db.Conn.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&s.TaskCount)
	db.Conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE status='done'").Scan(&s.DoneCount)
	db.Conn.QueryRow("SELECT COUNT(*) FROM tasks WHERE status='failed'").Scan(&s.FailedCount)

	// Skills stats
	db.Conn.QueryRow("SELECT COUNT(*) FROM skills").Scan(&s.SkillCount)
	db.Conn.QueryRow("SELECT COALESCE(SUM(use_count), 0) FROM skills").Scan(&s.SkillUses)
	db.Conn.QueryRow("SELECT COALESCE(SUM(recover_count), 0) FROM agents").Scan(&s.RecoverCount)
	var mostUsed string
	if err := db.Conn.QueryRow("SELECT name FROM skills ORDER BY use_count DESC LIMIT 1").Scan(&mostUsed); err == nil {
		s.MostUsedSkill = mostUsed
	}

	return s, nil
}
