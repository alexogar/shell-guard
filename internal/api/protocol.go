package api

import "shell-guard/internal/types"

const (
	ActionStartSession       = "StartSession"
	ActionSessionStatus      = "GetSessionStatus"
	ActionGetState           = "GetState"
	ActionListRecentCommands = "ListRecentCommands"
)

type Request struct {
	Action string `json:"action"`
	Shell  string `json:"shell,omitempty"`
	CWD    string `json:"cwd,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Rows   int    `json:"rows,omitempty"`
	Cols   int    `json:"cols,omitempty"`
	Attach bool   `json:"attach,omitempty"`
}

type Response struct {
	OK      bool                  `json:"ok"`
	Error   string                `json:"error,omitempty"`
	Message string                `json:"message,omitempty"`
	Session *types.Session        `json:"session,omitempty"`
	State   *types.StateView      `json:"state,omitempty"`
	Recent  []types.RecentCommand `json:"recent,omitempty"`
}
