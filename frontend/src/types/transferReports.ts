export interface TransferReportSettings {
  enabled: boolean;
  snapshots_per_day: number;
  daily_report_time: string;
  weekly_report_day: number;
  weekly_report_time: string;
  top_n: number;
  updated_at?: string;
}

export interface TransferRankItem { rank: number; name: string; hash: string; bytes: number }
export interface TransferReport {
  uuid: string;
  period_type: 'daily' | 'weekly';
  period_start: string;
  period_end: string;
  timezone: string;
  generated_at: string;
  coverage: 'complete' | 'partial' | 'unavailable';
  unavailable_workers: string[];
  upload: TransferRankItem[];
  download: TransferRankItem[];
}
export interface LatestTransferReports { daily: TransferReport | null; weekly: TransferReport | null }
