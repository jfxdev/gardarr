package schemas

// TransferReportSettingsRequest updates the singleton transfer ranking scheduler.
type TransferReportSettingsRequest struct {
	Enabled          bool   `json:"enabled"`
	SnapshotsPerDay  int    `json:"snapshots_per_day" binding:"required"`
	DailyReportTime  string `json:"daily_report_time" binding:"required"`
	WeeklyReportDay  int    `json:"weekly_report_day" binding:"min=0,max=6"`
	WeeklyReportTime string `json:"weekly_report_time" binding:"required"`
	TopN             int    `json:"top_n" binding:"required"`
}

// TransferReportDiscordSendRequest selects the report rendered in a manual Discord notification.
type TransferReportDiscordSendRequest struct {
	Source     string `json:"source" binding:"required,oneof=current completed"`
	PeriodType string `json:"period_type" binding:"required,oneof=daily weekly"`
}
