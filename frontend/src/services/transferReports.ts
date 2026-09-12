import { api, type ApiResponse } from '@/lib/api';
import type { LatestTransferReports, TransferReportSettings } from '@/types/transferReports';

class TransferReportsService {
  getSettings(): Promise<ApiResponse<TransferReportSettings>> { return api.get('/reports/transfer/settings'); }
  updateSettings(settings: TransferReportSettings): Promise<ApiResponse<TransferReportSettings>> { return api.put('/reports/transfer/settings', settings); }
  getLatest(): Promise<ApiResponse<LatestTransferReports>> { return api.get('/reports/transfer/latest'); }
  getCurrent(): Promise<ApiResponse<LatestTransferReports>> { return api.get('/reports/transfer/current'); }
  captureSnapshot(): Promise<ApiResponse<null>> { return api.post('/reports/transfer/snapshot'); }
  sendDiscord(source: 'current' | 'completed', periodType: 'daily' | 'weekly'): Promise<ApiResponse<{ delivered: number; failed: number; first_error?: string; outcomes: { destination: string; delivered: boolean; error?: string }[] }>> { return api.post('/reports/transfer/send-discord', { source, period_type: periodType }); }
}
export const transferReportsService = new TransferReportsService();
